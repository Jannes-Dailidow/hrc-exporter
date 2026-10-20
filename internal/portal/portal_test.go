package portal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
)

// fakePortal emulates the observed HRCommunicator flow.
type fakePortal struct {
	t        *testing.T
	code     string
	resent   int
	selected string // year shown on overview
}

const sessionCookie = "ASP.NET_SessionId"

var fakeDocs = map[string][][3]string{ // year -> id, date, type
	"2026": {{"aaaaaaaa-0000-0000-0000-000000000001", "01.09.2026", "Entgeltabrechnung"}},
	"2025": {
		{"bbbbbbbb-0000-0000-0000-000000000001", "01.12.2025", "Entgeltabrechnung"},
		{"bbbbbbbb-0000-0000-0000-000000000002", "01.01.2025", "DEÜV-Meldungen"},
	},
}

func (f *fakePortal) form(action, viewstate, inner string) string {
	return fmt.Sprintf(`<html><body><form method="post" action="./%s" id="form1">
<input type="hidden" name="__VIEWSTATE" value="%s" />
<input type="hidden" name="__EVENTVALIDATION" value="ev-%s" />%s</form></body></html>`, action, viewstate, viewstate, inner)
}

func (f *fakePortal) overview(extra string) string {
	var b strings.Builder
	b.WriteString(`<select name="ctl00$MainContent$ddJahr">`)
	for _, y := range []string{"2026", "2025"} {
		sel := ""
		if y == f.selected {
			sel = ` selected="selected"`
		}
		fmt.Fprintf(&b, `<option%s value="%s">%s</option>`, sel, y, y)
	}
	b.WriteString(`</select><table id="MainContent_Table1"><tr><th>Dokument</th><th>Datum</th><th>Dokumentenart</th></tr>`)
	for _, d := range fakeDocs[f.selected] {
		fmt.Fprintf(&b, `<tr><td><input type="image" name="ctl00$MainContent$%s" src="Images/print_preview.png" /></td><td>%s</td><td>%s</td></tr>`, d[0], d[1], d[2])
	}
	b.WriteString(`</table>` + extra)
	return f.form("Uebersicht.aspx", "vs-overview-"+f.selected, b.String())
}

func (f *fakePortal) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	authed := func() bool {
		c, err := r.Cookie(sessionCookie)
		return err == nil && c.Value == "ok"
	}
	switch r.Method + " " + r.URL.Path {
	case "GET /Default.aspx":
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "anon", Path: "/"})
		fmt.Fprint(w, f.form("Default.aspx", "vs-login", ""))
	case "POST /Default.aspx":
		if r.PostForm.Get("__VIEWSTATE") != "vs-login" || r.PostForm.Get(fieldSubmit+".x") == "" {
			http.Error(w, "bad postback", http.StatusBadRequest)
			return
		}
		if r.PostForm.Get(fieldUser) != "123456" || r.PostForm.Get(fieldPassword) != "secret" {
			fmt.Fprint(w, f.form("Default.aspx", "vs-login", `<script>alert('Anmeldung fehlgeschlagen');</script>`))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "2fa", Path: "/"})
		http.Redirect(w, r, "/Login2Factor.aspx", http.StatusFound)
	case "GET /Login2Factor.aspx":
		fmt.Fprint(w, f.form("Login2Factor.aspx", "vs-2fa", ""))
	case "POST /Login2Factor.aspx":
		if r.PostForm.Get(fieldResend) != "" {
			f.resent++
			f.code = "NEW42"
			fmt.Fprint(w, f.form("Login2Factor.aspx", "vs-2fa", ""))
			return
		}
		if r.PostForm.Get(fieldPassword) != f.code {
			fmt.Fprint(w, f.form("Login2Factor.aspx", "vs-2fa", ""))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "ok", Path: "/"})
		http.Redirect(w, r, "/Uebersicht.aspx", http.StatusFound)
	case "GET /Uebersicht.aspx":
		if !authed() {
			http.Redirect(w, r, "/Default.aspx", http.StatusFound)
			return
		}
		fmt.Fprint(w, f.overview(""))
	case "POST /Uebersicht.aspx":
		if !authed() {
			http.Redirect(w, r, "/Default.aspx", http.StatusFound)
			return
		}
		if r.PostForm.Get("__VIEWSTATE") != "vs-overview-"+f.selected {
			http.Error(w, "viewstate mismatch", http.StatusInternalServerError)
			return
		}
		if r.PostForm.Get(fieldSubmit+".x") != "" { // logout
			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
			http.Redirect(w, r, "/Default.aspx", http.StatusFound)
			return
		}
		if r.PostForm.Get(fieldSearch) != "" {
			f.selected = r.PostForm.Get(fieldYear)
			fmt.Fprint(w, f.overview(""))
			return
		}
		for _, d := range fakeDocs[f.selected] {
			if r.PostForm.Get(docButtonPrefix+d[0]+".x") != "" {
				http.SetCookie(w, &http.Cookie{Name: "doc", Value: d[0], Path: "/"})
				fmt.Fprint(w, f.overview(`<script language='JavaScript'>window.open('PreviewDok.aspx');</script>`))
				return
			}
		}
		http.Error(w, "invalid postback or callback argument", http.StatusInternalServerError)
	case "GET /PreviewDok.aspx":
		c, _ := r.Cookie("doc")
		fmt.Fprintf(w, `<html><body><iframe src="app_datax/%s.pdf"></iframe></body></html>`, c.Value)
	default:
		if strings.HasPrefix(r.URL.Path, "/app_datax/") {
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprintf(w, "%%PDF-1.4 %s", strings.TrimPrefix(r.URL.Path, "/app_datax/"))
			return
		}
		http.NotFound(w, r)
	}
}

func newTestClient(t *testing.T) (*Client, *fakePortal) {
	t.Helper()
	fp := &fakePortal{t: t, code: "CODE1", selected: "2026"}
	srv := httptest.NewServer(fp)
	t.Cleanup(srv.Close)
	c, err := New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return c, fp
}

func TestFullFlow(t *testing.T) {
	ctx := context.Background()
	c, fp := newTestClient(t)

	needsCode, err := c.Login(ctx, domain.Credentials{User: "123456", Password: "secret"})
	if err != nil || !needsCode {
		t.Fatalf("Login = %v, %v", needsCode, err)
	}
	if err := c.Verify2FA(ctx, "wrong"); !errors.Is(err, domain.ErrInvalidCode) {
		t.Fatalf("Verify2FA(wrong) = %v", err)
	}
	if err := c.ResendCode(ctx); err != nil || fp.resent != 1 {
		t.Fatalf("ResendCode = %v (resent %d)", err, fp.resent)
	}
	if err := c.Verify2FA(ctx, "NEW42"); err != nil {
		t.Fatalf("Verify2FA = %v", err)
	}

	years, err := c.Years(ctx)
	if err != nil || len(years) != 2 || years[0] != 2026 {
		t.Fatalf("Years = %v, %v", years, err)
	}
	docs, err := c.List(ctx, 2025)
	if err != nil || len(docs) != 2 || docs[1].Type != "DEÜV-Meldungen" || docs[1].Date.Month() != 1 {
		t.Fatalf("List = %+v, %v", docs, err)
	}

	// Switching year implicitly re-lists before clicking.
	other, _ := c.List(ctx, 2026)
	for _, d := range append(docs, other...) {
		var buf bytes.Buffer
		n, err := c.Download(ctx, d, &buf)
		if err != nil {
			t.Fatalf("Download(%s) = %v", d.ID, err)
		}
		if want := "%PDF-1.4 " + d.ID + ".pdf"; buf.String() != want || n != int64(len(want)) {
			t.Errorf("Download(%s) = %q", d.ID, buf.String())
		}
	}
	if err := c.Logout(ctx); err != nil {
		t.Fatalf("Logout = %v", err)
	}
	if _, err := c.Years(ctx); !errors.Is(err, domain.ErrNotLoggedIn) {
		t.Errorf("Years after logout = %v", err)
	}
}

func TestLoginRejected(t *testing.T) {
	c, _ := newTestClient(t)
	_, err := c.Login(context.Background(), domain.Credentials{User: "123456", Password: "nope"})
	if !errors.Is(err, domain.ErrInvalidCredentials) || !strings.Contains(err.Error(), "Anmeldung fehlgeschlagen") {
		t.Fatalf("Login = %v", err)
	}
}

func TestFindPDFLink(t *testing.T) {
	for in, want := range map[string]string{
		`<embed src="app_datax/tmp-0001.pdf" type="application/pdf">`: "app_datax/tmp-0001.pdf",
		`<script>location.href='/app_datax/x.pdf';</script>`:          "/app_datax/x.pdf",
		`<a href="Files/doc.pdf?x=1&amp;y=2">`:                        "Files/doc.pdf?x=1&y=2",
		`<html>nothing here</html>`:                                   "",
	} {
		if got := findPDFLink([]byte(in)); got != want {
			t.Errorf("findPDFLink(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBaseURL(t *testing.T) {
	if _, err := New(); !errors.Is(err, ErrNoBaseURL) {
		t.Errorf("New() = %v, want ErrNoBaseURL", err)
	}
	for _, bad := range []string{"", "portal.example.com", "ftp://portal.example.com/", "https://"} {
		if _, err := New(WithBaseURL(bad)); err == nil {
			t.Errorf("WithBaseURL(%q) accepted", bad)
		}
	}
	u, err := ParseBaseURL(" https://portal.example.com/hr ")
	if err != nil || u.String() != "https://portal.example.com/hr/" {
		t.Errorf("ParseBaseURL = %v, %v", u, err)
	}
}

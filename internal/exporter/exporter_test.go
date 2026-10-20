package exporter

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
)

type fakePortal struct {
	docs       map[int][]domain.Document
	code       string
	resent     int
	downloads  []string
	failID     string
	loggedOut  bool
	needsCode  bool
	verifyErrs int
}

func (p *fakePortal) Login(context.Context, domain.Credentials) (bool, error) {
	return p.needsCode, nil
}
func (p *fakePortal) ResendCode(context.Context) error { p.resent++; return nil }
func (p *fakePortal) Verify2FA(_ context.Context, code string) error {
	if code != p.code {
		p.verifyErrs++
		return domain.ErrInvalidCode
	}
	return nil
}
func (p *fakePortal) Years(context.Context) ([]int, error) { return []int{2026, 2025}, nil }
func (p *fakePortal) List(_ context.Context, y int) ([]domain.Document, error) {
	return p.docs[y], nil
}
func (p *fakePortal) Download(_ context.Context, d domain.Document, w io.Writer) (int64, error) {
	if d.ID == p.failID {
		return 0, errors.New("boom")
	}
	p.downloads = append(p.downloads, d.ID)
	n, err := io.WriteString(w, "%PDF-"+d.ID)
	return int64(n), err
}
func (p *fakePortal) Logout(context.Context) error { p.loggedOut = true; return nil }

type memStore struct {
	recs  map[string]domain.Record
	saves int
}

func (s *memStore) Get(id string) (domain.Record, bool) { r, ok := s.recs[id]; return r, ok }
func (s *memStore) Put(r domain.Record)                 { s.recs[r.ID] = r }
func (s *memStore) Save() error                         { s.saves++; return nil }

type scriptedAuth struct{ codes []string } // "" means resend

func (a *scriptedAuth) Credentials(context.Context) (domain.Credentials, error) {
	return domain.Credentials{User: "1", Password: "p"}, nil
}
func (a *scriptedAuth) TwoFactorCode(context.Context) (string, bool, error) {
	if len(a.codes) == 0 {
		return "", false, errors.New("no more codes")
	}
	c := a.codes[0]
	a.codes = a.codes[1:]
	return c, c == "", nil
}

type recorder struct{ events []Event }

func (r *recorder) Report(e Event) { r.events = append(r.events, e) }

func doc(id string, year int, month time.Month, typ string) domain.Document {
	return domain.Document{ID: id, Year: year, Date: time.Date(year, month, 1, 0, 0, 0, 0, time.UTC), Type: typ}
}

func setup(t *testing.T) (*Exporter, *fakePortal, *memStore, string) {
	t.Helper()
	out := t.TempDir()
	p := &fakePortal{code: "OK", needsCode: true, docs: map[int][]domain.Document{
		2026: {doc("aaaaaaaa-1", 2026, 9, "Entgeltabrechnung")},
		2025: {doc("bbbbbbbb-1", 2025, 12, "Entgeltabrechnung"), doc("bbbbbbbb-2", 2025, 1, "DEÜV-Meldungen")},
	}}
	s := &memStore{recs: map[string]domain.Record{}}
	e := New(p, s, &scriptedAuth{}, WithOutDir(out), WithClock(func() time.Time { return time.Unix(0, 0) }))
	return e, p, s, out
}

func TestAuthenticateRetriesAndResend(t *testing.T) {
	e, p, _, _ := setup(t)
	e.auth = &scriptedAuth{codes: []string{"bad", "", "OK"}}
	rec := &recorder{}
	user, err := e.Authenticate(context.Background(), rec)
	if err != nil || user != "1" {
		t.Fatalf("Authenticate = %q, %v", user, err)
	}
	if p.resent != 1 || p.verifyErrs != 1 {
		t.Errorf("resent=%d verifyErrs=%d", p.resent, p.verifyErrs)
	}
	if _, ok := rec.events[len(rec.events)-1].(LoginSucceeded); !ok {
		t.Errorf("last event = %T", rec.events[len(rec.events)-1])
	}
}

func TestAuthenticateGivesUp(t *testing.T) {
	e, _, _, _ := setup(t)
	e.auth = &scriptedAuth{codes: []string{"a", "b", "c", "OK"}}
	if _, err := e.Authenticate(context.Background(), Discard); !errors.Is(err, domain.ErrInvalidCode) {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncSkipsKnownAndRecordsNew(t *testing.T) {
	e, p, s, out := setup(t)
	ctx := context.Background()

	// Pretend one document is already downloaded.
	known := p.docs[2025][0]
	rel := RelativePath(known)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(out, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, rel), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Put(domain.Record{ID: known.ID, File: rel})
	p.failID = "bbbbbbbb-2"

	sum, err := e.Sync(ctx, Selection{}, SyncOptions{}, Discard)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Total != 3 || sum.Skipped != 1 || sum.Downloaded != 1 || len(sum.Failures) != 1 {
		t.Fatalf("summary = %+v", sum)
	}
	got, ok := s.Get("aaaaaaaa-1")
	if !ok || got.File != "2026/2026-09-01_entgeltabrechnung_aaaaaaaa.pdf" || got.Size == 0 || got.SHA256 == "" {
		t.Fatalf("record = %+v", got)
	}
	if data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(got.File))); err != nil || string(data) != "%PDF-aaaaaaaa-1" {
		t.Errorf("file = %q, %v", data, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(out, "2025")); len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}

	// Second run: nothing new except the previously failed one.
	p.failID, p.downloads = "", nil
	sum, err = e.Sync(ctx, Selection{Years: []int{2025}, Type: "deüv"}, SyncOptions{}, Discard)
	if err != nil || sum.Downloaded != 1 || len(p.downloads) != 1 || p.downloads[0] != "bbbbbbbb-2" {
		t.Fatalf("second sync = %+v, %v, %v", sum, err, p.downloads)
	}
}

func TestSyncDryRunAndMissingFile(t *testing.T) {
	e, p, s, _ := setup(t)
	s.Put(domain.Record{ID: "aaaaaaaa-1", File: "2026/gone.pdf"}) // recorded but deleted
	sum, err := e.Sync(context.Background(), Selection{Years: []int{2026}}, SyncOptions{DryRun: true}, Discard)
	if err != nil || len(sum.Pending) != 1 || len(p.downloads) != 0 {
		t.Fatalf("dry run = %+v, %v", sum, err)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"DEÜV-Meldungen":            "deuev-meldungen",
		"Einmalige Sondervergütung": "einmalige-sonderverguetung",
		"Upload Pers.Abt.":          "upload-pers-abt",
		"***":                       "dokument",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

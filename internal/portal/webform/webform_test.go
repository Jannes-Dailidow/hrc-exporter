package webform

import (
	"strings"
	"testing"
)

const page = `<!DOCTYPE html><html><head><title>
	HRCommunicator
</title></head><body>
<form method="post" action="./Uebersicht.aspx" id="form1">
<input type="hidden" name="__VIEWSTATE" id="__VIEWSTATE" value="vs+/=" />
<input type="hidden" name="__EVENTVALIDATION" id="__EVENTVALIDATION" value="ev" />
<span id="MainContent_lPersonalnummer">123456</span>
<select name="ctl00$MainContent$ddJahr" id="MainContent_ddJahr">
	<option selected="selected" value="2026">2026</option>
	<option value="2025">2025</option>
</select>
<table id="MainContent_Table1">
	<tr><th>Dokument</th><th>Datum</th><th>Dokumentenart</th></tr>
	<tr><td><input type="image" name="ctl00$MainContent$11111111-0000-0000-0000-000000000001" src="x.png" /></td><td>01.09.2026</td><td>Entgeltabrechnung</td></tr>
	<tr><td><input type="image" name="ctl00$MainContent$11111111-0000-0000-0000-000000000002" /></td><td>01.01.2026</td><td>DE&#220;V-Meldungen</td></tr>
</table>
<script language='JavaScript'>window.open('PreviewDok.aspx');</script>
<script>alert('Kennwort falsch');</script>
</form></body></html>`

func mustParse(t *testing.T) *Page {
	t.Helper()
	p, err := Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHiddenAndPostBack(t *testing.T) {
	p := mustParse(t)
	form := p.PostBack(ImageClick("btn"))
	if got := form.Get("__VIEWSTATE"); got != "vs+/=" {
		t.Errorf("__VIEWSTATE = %q", got)
	}
	if form.Get("__EVENTVALIDATION") != "ev" || form.Get("btn.x") == "" || form.Get("btn.y") == "" {
		t.Errorf("unexpected form %v", form)
	}
}

func TestOptions(t *testing.T) {
	opts := mustParse(t).Options("ctl00$MainContent$ddJahr")
	if len(opts) != 2 || opts[0].Value != "2026" || !opts[0].Selected || opts[1].Selected {
		t.Errorf("options = %+v", opts)
	}
}

func TestTable(t *testing.T) {
	rows := mustParse(t).Table("MainContent_Table1")
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	if !rows[0][0].Header {
		t.Error("first row should be header")
	}
	r := rows[2]
	if r[1].Text != "01.01.2026" || r[2].Text != "DEÜV-Meldungen" || r[0].Inputs[0].Type != "image" {
		t.Errorf("row = %+v", r)
	}
}

func TestScriptsAndText(t *testing.T) {
	p := mustParse(t)
	if target, ok := p.WindowOpen(); !ok || target != "PreviewDok.aspx" {
		t.Errorf("WindowOpen = %q, %v", target, ok)
	}
	if a := p.Alerts(); len(a) != 1 || a[0] != "Kennwort falsch" {
		t.Errorf("Alerts = %v", a)
	}
	if p.Title() != "HRCommunicator" || p.Text("MainContent_lPersonalnummer") != "123456" {
		t.Errorf("Title/Text = %q / %q", p.Title(), p.Text("MainContent_lPersonalnummer"))
	}
}

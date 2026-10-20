package report

import (
	"strings"
	"testing"
	"time"
)

// page builds a minimal `pdftotext -raw` page containing only the lines the
// parser reads.
func page(month, period, group string, body ...string) string {
	return strings.Join(append([]string{
		"Entgeltabrechnung", "Im " + month, "Datum: 26.08.2024",
		"Tarifgr./−stufe: " + group, "für den Zeitraum " + period + ":",
	}, body...), "\n")
}

func TestParsePayslipWithRetro(t *testing.T) {
	text := page("August 2024", "01.08. bis 31.08.2024", "5/1",
		"Basisbezüge",
		"Tarifentgelt LSG 3.446,00",
		"Übertarifliche Zulage LSG 254,00",
		"AVmG ZL PSt /Stf 13,34",
		"Bruttoentgelt",
		"Gesamtbrutto (EBeschV) 3.700,00 27.784,69",
		"Netto",
		"Gesetzl. Netto (EBeschV) 2.417,87",
		"Be− und Abzüge",
		"Nachverrechnung aus Vorm. 53,45",
		"Zahlungen",
		"Überweisung () 2.471,32 EUR",
		"Mitteilung",
		"Bitte beachten.",
		"Bitte beachten.",
	) + "\f" + page("August 2024", "01.07. bis 31.07.2024", "5/1",
		"Basisbezüge",
		"Tarifentgelt LSG 07 323,00",
		"Übertarifliche Zulage LSG 07 314,99 −",
		"Bruttoentgelt",
		"Gesamtbrutto (EBeschV) 07 8,01",
	)
	p, err := ParsePayslip("x.pdf", text)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) > 0 {
		t.Errorf("warnings: %v", p.Warnings)
	}
	want := Payslip{
		Period: Period{2024, time.August}, Printed: time.Date(2024, 8, 26, 0, 0, 0, 0, time.UTC),
		Group: "5/1", Gross: 370000, Net: 241787, Payout: 247132, Carryover: 5345,
	}
	if p.Period != want.Period || !p.Printed.Equal(want.Printed) || p.Group != want.Group || p.Gross != want.Gross ||
		p.Net != want.Net || p.Payout != want.Payout || p.Carryover != want.Carryover {
		t.Errorf("header: got %+v", p)
	}
	if len(p.Messages) != 1 || p.Messages[0] != "Bitte beachten." {
		t.Errorf("messages: %q", p.Messages)
	}
	if len(p.Pages) != 2 || p.Pages[1].Period != (Period{2024, time.July}) || p.Pages[1].Gross != 801 {
		t.Fatalf("pages: %+v", p.Pages)
	}
	if got := p.Pages[1].Lines[1]; got.Name != "Übertarifliche Zulage" || got.Flags != "LSG" || got.Amount != -31499 {
		t.Errorf("retro line: %+v", got)
	}
}

func TestParsePayslipWarnsOnGrossMismatch(t *testing.T) {
	p, err := ParsePayslip("x.pdf", page("August 2024", "01.08. bis 31.08.2024", "5/1",
		"Basisbezüge",
		"Tarifentgelt LSG 3.446,00",
		"Unbekannt ohne Betrag",
		"Bruttoentgelt",
		"Gesamtbrutto (EBeschV) 3.500,00",
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Warnings) != 2 {
		t.Errorf("want unrecognised-row and gross warnings, got %q", p.Warnings)
	}
}

func TestParsePayslipRejectsOtherDocuments(t *testing.T) {
	if _, err := ParsePayslip("x.pdf", "Lohnsteuerbescheinigung\n2024"); err == nil {
		t.Error("want error")
	}
}

func TestParseLine(t *testing.T) {
	for in, want := range map[string]Line{
		"Mehrarbeit 125% LSG 03 0,82 32,25 26,45":   {Name: "Mehrarbeit 125%", Flags: "LSG", Amount: 2645},
		"Urlaubsgeld ELSG 1 536,86":                 {Name: "Urlaubsgeld", Flags: "ELSG", Amount: 53686},
		"Inflationsausgleichsprämi G 2.500,00":      {Name: "Inflationsausgleichsprämi", Flags: "G", Amount: 250000},
		"AVmG ZL PSt /Stf 13,34":                    {Name: "AVmG ZL PSt /Stf", Amount: 1334},
		"Ausgl. Zuschläge Urlaub LSG 1,5 0,20 0,30": {Name: "Ausgl. Zuschläge Urlaub", Flags: "LSG", Amount: 30},
	} {
		got, ok := parseLine("Basisbezüge", in)
		want.Section = "Basisbezüge"
		if !ok || got != want {
			t.Errorf("%q: got %+v, want %+v", in, got, want)
		}
	}
	for _, in := range []string{"Nur Text", "12,00", ""} {
		if _, ok := parseLine("Basisbezüge", in); ok {
			t.Errorf("%q: want rejection", in)
		}
	}
}

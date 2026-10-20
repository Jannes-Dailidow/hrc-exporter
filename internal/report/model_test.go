package report

import (
	"errors"
	"testing"
	"time"
)

func slip(period Period, group string, tariff, aboveTariff Cents, extra ...Line) *Payslip {
	lines := append([]Line{
		{Name: "Tarifentgelt", Flags: "LSG", Amount: tariff},
		{Name: "Übertarifliche Zulage", Flags: "LSG", Amount: aboveTariff},
	}, extra...)
	return &Payslip{
		File: period.String() + ".pdf", Period: period, Group: group, Gross: tariff + aboveTariff,
		Pages: []*Page{{Period: period, Group: group, Lines: lines}},
	}
}

func TestBuildAppliesRetroAndDetectsOffset(t *testing.T) {
	mar, apr, may := Period{2025, time.March}, Period{2025, time.April}, Period{2025, time.May}
	a := slip(mar, "5/1", 344600, 55400)
	b := slip(apr, "5/1", 344600, 55400)
	// paid in May: step-up for May plus a retro tariff rise for April offset against the ÜT
	c := slip(may, "5/2", 361200, 38800)
	c.Pages = append(c.Pages, &Page{Period: apr, Lines: []Line{
		{Name: "Tarifentgelt", Flags: "LSG", Amount: 10000},
		{Name: "Übertarifliche Zulage", Flags: "LSG", Amount: -10000},
	}})

	r, err := Build([]*Payslip{c, a, b})
	if err != nil {
		t.Fatal(err)
	}
	if april := r.Months[1]; april.Tariff != 354600 || april.AboveTariff != 45400 || len(april.Corrections) != 1 {
		t.Errorf("April not corrected: %+v", april)
	}
	if got := r.Months[2].RetroPeriods; len(got) != 1 || got[0] != apr {
		t.Errorf("May retro periods: %v", got)
	}
	if len(r.Events) != 2 || r.Events[0].Offset != 10000 || r.Events[0].Kind != EventTariff ||
		r.Events[1].Kind != EventStep || r.Events[1].Offset != 6600 {
		t.Errorf("events: %+v", r.Events)
	}
	s := r.Summary
	if s == nil || s.Offset != 16600 || s.Gain != 0 || s.Start != r.Months[0] || s.Current != r.Months[2] {
		t.Fatalf("summary: %+v", s)
	}
	if s.AboveTariffPeak != 55400 || s.PeakFrom != mar || s.PeakTo != mar {
		t.Errorf("peak: %d %v–%v", s.AboveTariffPeak, s.PeakFrom, s.PeakTo)
	}
	if len(r.Tariff.Columns) != 2 || r.Tariff.Columns[1].Values["5/2"] != 361200 {
		t.Errorf("tariff table: %+v", r.Tariff)
	}
}

func TestBuildExtrasAndYears(t *testing.T) {
	vl := Line{Name: "VL-Anteil AG", Flags: "LSG", Amount: 2659}
	bonus := Line{Name: "Weihnachtsgeld", Flags: "ELSG", Amount: 100000}
	nov, dec, jan := Period{2024, time.November}, Period{2024, time.December}, Period{2025, time.January}
	slips := []*Payslip{
		slip(nov, "5/1", 300000, 0, vl),
		slip(dec, "5/1", 300000, 0, vl, bonus),
		slip(jan, "5/1", 300000, 0, vl),
	}
	slips[1].Pages[0].Lines = append(slips[1].Pages[0].Lines, Line{Name: "AVmG ZL", Amount: 1334})
	for _, s := range slips {
		s.Gross = 0
		for _, l := range s.Pages[0].Lines {
			if l.InGross() {
				s.Gross += l.Amount
			}
		}
	}

	r, err := Build(slips)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.OneOffs; len(got) != 2 || got[0].Name != vl.Name || got[1].Name != bonus.Name || got[1].Period != dec {
		t.Errorf("one-offs: %+v", got)
	}
	dm := r.Months[1]
	if dm.HolidayPay != 100000 || len(dm.Extras) != 2 {
		t.Errorf("December: %+v", dm)
	}
	if !r.Months[0].NewYear || r.Months[1].NewYear || !r.Months[2].NewYear {
		t.Error("NewYear flags")
	}
	if len(r.Years) != 2 || r.Years[0].Months != 2 || r.Years[0].HolidayPay != 100000 || r.Years[0].Other != 2*2659 {
		t.Errorf("years: %+v", r.Years)
	}
	if r.Summary.Gain != 0 || len(r.Events) != 0 {
		t.Errorf("unexpected change: %+v", r.Events)
	}
}

func TestBuildWarnings(t *testing.T) {
	jun := Period{2025, time.June}
	first, second := slip(jun, "5/1", 300000, 0), slip(jun, "5/1", 300000, 0)
	first.Printed, second.Printed = time.Date(2025, 6, 25, 0, 0, 0, 0, time.UTC), time.Date(2025, 7, 2, 0, 0, 0, 0, time.UTC)
	second.File = "corrected.pdf"
	second.Pages = append(second.Pages, &Page{Period: Period{2025, time.January}, Gross: 500})

	r, err := Build([]*Payslip{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Months) != 1 || r.Months[0].File != "corrected.pdf" {
		t.Errorf("later payslip must win: %+v", r.Months)
	}
	if len(r.Warnings) != 2 {
		t.Errorf("want duplicate and missing-month warnings, got %q", r.Warnings)
	}
}

func TestBuildWithoutSalariedMonths(t *testing.T) {
	p := &Payslip{Period: Period{2022, time.January}, Group: "AZ/2", Pages: []*Page{{
		Lines: []Line{{Name: "Ausbildungsvergütung", Flags: "LSG", Amount: 108000}},
	}}}
	r, err := Build([]*Payslip{p})
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary != nil || r.Months[0].Trainee != 108000 {
		t.Errorf("got %+v", r)
	}
	if _, err := Build(nil); !errors.Is(err, ErrNoPayslips) {
		t.Errorf("Build(nil): %v", err)
	}
}

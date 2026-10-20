package report

import "testing"

func TestFormatEuro(t *testing.T) {
	for c, want := range map[Cents]string{442500: "4.425,00", 5: "0,05", -31499: "−314,99", 123456789: "1.234.567,89"} {
		if got := formatEuro(c, 2); got != want {
			t.Errorf("%d: %s, want %s", c, got, want)
		}
	}
	for c, want := range map[Cents]string{442550: "4.426", 442549: "4.425", -6650: "−67"} {
		if got := formatEuro(c, 0); got != want {
			t.Errorf("%d: %s, want %s", c, got, want)
		}
	}
}

func TestFormatSignedAndPercent(t *testing.T) {
	for c, want := range map[Cents]string{1200: "+12", -350: "−3,50", 0: "±0"} {
		if got := formatSigned(c); got != want {
			t.Errorf("%d: %s, want %s", c, got, want)
		}
	}
	for p, want := range map[float64]string{3.14: "+3,1 %", -0.26: "−0,3 %"} {
		if got := formatPercent(p); got != want {
			t.Errorf("%v: %s, want %s", p, got, want)
		}
	}
}

func TestParseAmount(t *testing.T) {
	for s, want := range map[string]Cents{"1.234,56": 123456, "0,05": 5, "314,9": 31490} {
		if got := parseAmount(s); got != want {
			t.Errorf("%q: %d, want %d", s, got, want)
		}
	}
}

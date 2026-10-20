package i18n

import "testing"

func TestParse(t *testing.T) {
	for in, want := range map[string]Lang{"de_DE.UTF-8": DE, "de-AT": DE, "EN": EN, "en_US@euro": EN} {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %q, %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "fr", "C", "POSIX"} {
		if _, ok := Parse(in); ok {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestDetectFrom(t *testing.T) {
	if got := detectFrom([]string{"", "fr_FR", "de_DE.UTF-8", "en"}); got != DE {
		t.Errorf("detectFrom = %q", got)
	}
	if got := detectFrom([]string{"C"}); got != Default {
		t.Errorf("detectFrom(C) = %q", got)
	}
}

func TestPrinter(t *testing.T) {
	de := New(DE)
	if got := de.N(CountDocuments, 1, 1); got != "1 Dokument" {
		t.Errorf("N(1) = %q", got)
	}
	if got := de.N(CountDocuments, 3, 3); got != "3 Dokumente" {
		t.Errorf("N(3) = %q", got)
	}
	if got := New("fr").Lang(); got != Default {
		t.Errorf("New(fr).Lang() = %q", got)
	}
	if got := de.T("no.such.key"); got != "no.such.key" {
		t.Errorf("unknown key = %q", got)
	}
}

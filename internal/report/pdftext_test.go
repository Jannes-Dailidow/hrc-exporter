package report

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindPdftotext(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	t.Setenv("ProgramFiles", empty)
	t.Setenv("LOCALAPPDATA", empty)
	if _, err := FindPdftotext(""); !errors.Is(err, ErrPdftotextNotFound) {
		t.Errorf("empty system: %v", err)
	}
	if p, err := FindPdftotext("/opt/pdftotext"); err != nil || p.Path != "/opt/pdftotext" {
		t.Errorf("explicit: %v %v", p, err)
	}

	bundled := filepath.Join(empty, "Programs", "Git", "mingw64", "bin", "pdftotext.exe")
	if err := os.MkdirAll(filepath.Dir(bundled), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundled, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if p, err := FindPdftotext(""); err != nil || p.Path != bundled {
		t.Errorf("Git for Windows: %v %v", p, err)
	}
}

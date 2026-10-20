package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrPdftotextNotFound is returned by FindPdftotext when no binary is available.
var ErrPdftotextNotFound = errors.New("pdftotext not found")

// Pdftotext extracts text with poppler's or xpdf's pdftotext in raw mode,
// which keeps the payslip rows on one line each.
type Pdftotext struct {
	Path string
}

// FindPdftotext returns explicit if set, otherwise pdftotext from PATH or
// from a Git for Windows installation, which ships it.
func FindPdftotext(explicit string) (Pdftotext, error) {
	if explicit != "" {
		return Pdftotext{Path: explicit}, nil
	}
	if p, err := exec.LookPath("pdftotext"); err == nil {
		return Pdftotext{Path: p}, nil
	}
	for _, p := range gitForWindowsCandidates() {
		if _, err := os.Stat(p); err == nil {
			return Pdftotext{Path: p}, nil
		}
	}
	return Pdftotext{}, ErrPdftotextNotFound
}

// gitForWindowsCandidates lists pdftotext in the system-wide and per-user
// Git for Windows installation directories.
func gitForWindowsCandidates() []string {
	var paths []string
	for _, c := range []struct{ env, dir string }{
		{"ProgramFiles", "Git"},
		{"LOCALAPPDATA", filepath.Join("Programs", "Git")},
	} {
		if base := os.Getenv(c.env); base != "" {
			paths = append(paths, filepath.Join(base, c.dir, "mingw64", "bin", "pdftotext.exe"))
		}
	}
	return paths
}

// Text returns the text of the PDF at path, pages separated by form feeds.
func (p Pdftotext) Text(ctx context.Context, path string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, p.Path, "-raw", "-enc", "UTF-8", "-q", path, "-")
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("pdftotext %s: %w: %s", path, err, msg)
		}
		return "", fmt.Errorf("pdftotext %s: %w", path, err)
	}
	return string(out), nil
}

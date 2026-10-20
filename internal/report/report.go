// Package report analyses downloaded Entgeltabrechnungen: it extracts their
// text, parses the wage rows, applies retroactive corrections to the months
// they belong to and derives salary changes, yearly totals and key figures.
//
// Like the rest of the core it never touches the terminal. Diagnostics are
// English; the rendered HTML is German, matching the payslips it analyses.
package report

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
)

// DocumentType is the portal document type of a monthly payslip.
const DocumentType = "Entgeltabrechnung"

// Records lists downloaded documents; *store.JSONStore satisfies it.
type Records interface {
	Records() []domain.Record
}

// TextExtractor returns the text of a PDF, pages separated by form feeds.
type TextExtractor interface {
	Text(ctx context.Context, path string) (string, error)
}

// Option configures Generate.
type Option func(*generator)

// WithLogger sets the logger for debug output.
func WithLogger(l *slog.Logger) Option { return func(g *generator) { g.log = l } }

type generator struct {
	log *slog.Logger
}

// Generate parses every downloaded payslip under dir and builds the report.
// Payslips listed in records whose file is missing are skipped with a
// warning; extraction and parse errors abort.
func Generate(ctx context.Context, dir string, records Records, text TextExtractor, opts ...Option) (*Report, error) {
	g := generator{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, o := range opts {
		o(&g)
	}

	var slips []*Payslip
	var skipped []string
	for _, rec := range records.Records() {
		if rec.Type != DocumentType {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, filepath.FromSlash(rec.File))
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			skipped = append(skipped, rec.File+": file missing, skipped")
			continue
		}
		g.log.Debug("parsing payslip", "file", rec.File)
		s, err := text.Text(ctx, path)
		if err != nil {
			return nil, err
		}
		p, err := ParsePayslip(rec.File, s)
		if err != nil {
			return nil, err
		}
		slips = append(slips, p)
	}
	if len(slips) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNoPayslips, dir)
	}

	r, err := Build(slips)
	if err != nil {
		return nil, err
	}
	r.Warnings = append(skipped, r.Warnings...)
	return r, nil
}

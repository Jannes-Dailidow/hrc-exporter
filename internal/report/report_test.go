package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
)

type records []domain.Record

func (r records) Records() []domain.Record { return r }

// fakeText returns canned text per file name.
type fakeText map[string]string

func (f fakeText) Text(_ context.Context, path string) (string, error) {
	text, ok := f[filepath.Base(path)]
	if !ok {
		return "", errors.New("unexpected file " + path)
	}
	return text, nil
}

func monthPage(month, period, tariff, aboveTariff, gross string) string {
	return page(month, period, "5/1",
		"Basisbezüge",
		"Tarifentgelt LSG "+tariff,
		"Übertarifliche Zulage LSG "+aboveTariff,
		"Bruttoentgelt",
		"Gesamtbrutto (EBeschV) "+gross,
	)
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	recs := records{
		{Type: DocumentType, File: "2024/2024-07-01_entgeltabrechnung_a.pdf"},
		{Type: DocumentType, File: "2024/2024-08-01_entgeltabrechnung_b.pdf"},
		{Type: DocumentType, File: "2024/2024-09-01_entgeltabrechnung_gone.pdf"},
		{Type: "Lohnsteuerbescheinigung", File: "2024/2024-12-01_lohnsteuerbescheinigung_c.pdf"},
	}
	for _, r := range recs[:2] {
		path := filepath.Join(dir, filepath.FromSlash(r.File))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	text := fakeText{
		"2024-07-01_entgeltabrechnung_a.pdf": monthPage("Juli 2024", "01.07. bis 31.07.2024", "3.446,00", "254,00", "3.700,00"),
		"2024-08-01_entgeltabrechnung_b.pdf": monthPage("August 2024", "01.08. bis 31.08.2024", "3.546,00", "154,00", "3.700,00"),
	}

	r, err := Generate(context.Background(), dir, recs, text)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Months) != 2 || len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "gone") {
		t.Fatalf("months %d, warnings %q", len(r.Months), r.Warnings)
	}
	if len(r.Events) != 1 || r.Events[0].Offset != 10000 {
		t.Errorf("events: %+v", r.Events)
	}

	var html bytes.Buffer
	if err := r.WriteHTML(&html, time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Gehaltsanalyse 2024–2024", "erzeugt am 20.10.2026 12:00", "07/2024 bis 08/2024", "3.700 €", `"m":"2024-08"`} {
		if !strings.Contains(html.String(), want) {
			t.Errorf("HTML lacks %q", want)
		}
	}

	var buf bytes.Buffer
	if err := r.WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var data chartData
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Months) != 2 || data.Start != "2024-07" || data.Events[0].DeltaTariff != 100 {
		t.Errorf("json: %+v", data)
	}
}

func TestGenerateWithoutPayslips(t *testing.T) {
	_, err := Generate(context.Background(), t.TempDir(), records{}, fakeText{})
	if !errors.Is(err, ErrNoPayslips) {
		t.Errorf("got %v", err)
	}
}

func TestGenerateFailsOnExtractionError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.pdf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Generate(context.Background(), dir, records{{Type: DocumentType, File: "x.pdf"}}, fakeText{})
	if err == nil || errors.Is(err, ErrNoPayslips) {
		t.Errorf("want extraction error, got %v", err)
	}
}

func TestRenderWithoutSalariedMonths(t *testing.T) {
	r, err := Build([]*Payslip{{Period: Period{2022, time.January}, Group: "AZ/2", Pages: []*Page{{
		Lines: []Line{{Name: "Ausbildungsvergütung", Flags: "LSG", Amount: 108000}},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.WriteHTML(&bytes.Buffer{}, time.Now()); err != nil {
		t.Error(err)
	}
}

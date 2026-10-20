package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out", FileName)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rec := domain.Record{
		ID: "abc", Year: 2025, Date: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC), Type: "Entgeltabrechnung",
		File: "2025/x.pdf", SHA256: "ff", Size: 42, DownloadedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	s.Put(rec)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	for _, want := range []string{`"version": 1`, `"date": "2025-12-01"`, `"downloaded_at": "2026-01-01T00:00:00Z"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("meta.json lacks %s:\n%s", want, data)
		}
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Get("abc")
	if !ok || !got.Date.Equal(rec.Date) || got.File != rec.File || got.Size != 42 {
		t.Errorf("Get = %+v, %v", got, ok)
	}
	if n := len(s2.Records()); n != 1 {
		t.Errorf("Records = %d", n)
	}
}

func TestVersionCheck(t *testing.T) {
	for _, content := range []string{`{"documents":{}}`, `{"version":99,"documents":{}}`} {
		path := filepath.Join(t.TempDir(), FileName)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(path); !errors.Is(err, ErrUnsupportedVersion) {
			t.Errorf("Open(%s) = %v", content, err)
		}
	}
}

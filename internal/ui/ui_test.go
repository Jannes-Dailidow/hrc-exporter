package ui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
	"github.com/jannes-dailidow/hrc-exporter/internal/exporter"
	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
	"github.com/jannes-dailidow/hrc-exporter/internal/report"
)

func TestSummaryGerman(t *testing.T) {
	doc := domain.Document{ID: "x", Year: 2025, Date: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC), Type: "Entgeltabrechnung"}
	var out bytes.Buffer
	Renderer{P: i18n.New(i18n.DE)}.Summary(&out, exporter.Summary{
		Total: 3, Skipped: 1, Downloaded: 1, Bytes: 2048,
		Failures: []exporter.Failure{{Doc: doc, Err: domain.ErrSessionExpired}},
	})
	for _, want := range []string{"fertig:", "heruntergeladen", "1 bereits vorhanden", "1 fehlgeschlagen", "01.12.2025", "Portal-Sitzung abgelaufen"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary lacks %q:\n%s", want, out.String())
		}
	}
}

func TestErrorText(t *testing.T) {
	de := i18n.New(i18n.DE)
	err := fmt.Errorf("%w: %s", domain.ErrInvalidCredentials, "Anmeldung fehlgeschlagen")
	if got, want := ErrorText(de, err), "Personalnummer oder Kennwort falsch: Anmeldung fehlgeschlagen"; got != want {
		t.Errorf("ErrorText = %q, want %q", got, want)
	}
	err = fmt.Errorf("%w in %s", report.ErrNoPayslips, "abrechnungen")
	if got, want := ErrorText(de, err), "keine Entgeltabrechnungen gefunden in abrechnungen"; got != want {
		t.Errorf("ErrorText = %q, want %q", got, want)
	}
	if got := ErrorText(de, errors.New("boom")); got != "boom" {
		t.Errorf("unknown error = %q", got)
	}
}

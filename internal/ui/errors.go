package ui

import (
	"errors"
	"strings"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
	"github.com/jannes-dailidow/hrc-exporter/internal/report"
	"github.com/jannes-dailidow/hrc-exporter/internal/store"
)

// knownErrors maps core sentinel errors to their translation.
var knownErrors = []struct {
	err error
	key i18n.Key
}{
	{domain.ErrInvalidCredentials, i18n.ErrInvalidCredentials},
	{domain.ErrInvalidCode, i18n.ErrInvalidCode},
	{domain.ErrSessionExpired, i18n.ErrSessionExpired},
	{domain.ErrNotLoggedIn, i18n.ErrNotLoggedIn},
	{store.ErrUnsupportedVersion, i18n.ErrUnsupportedMeta},
	{report.ErrNoPayslips, i18n.ErrNoPayslips},
	{report.ErrPdftotextNotFound, i18n.ErrPdftotextNotFound},
}

// ErrorText returns a localized message for err. The sentinel part of a
// known error is translated; surrounding context (e.g. portal alert text)
// is kept. Unknown errors are returned unchanged.
func ErrorText(p *i18n.Printer, err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	for _, k := range knownErrors {
		if errors.Is(err, k.err) {
			return strings.Replace(text, k.err.Error(), p.T(k.key), 1)
		}
	}
	return text
}

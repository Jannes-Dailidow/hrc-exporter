package ui

import (
	"github.com/charmbracelet/log"
	"github.com/dustin/go-humanize"

	"github.com/jannes-dailidow/hrc-exporter/internal/exporter"
	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
)

// PlainReporter renders exporter events as log lines. Messages are
// translated; structured keys stay English.
type PlainReporter struct {
	Log *log.Logger
	P   *i18n.Printer
}

func (r PlainReporter) Report(ev exporter.Event) {
	t := r.P.T
	switch e := ev.(type) {
	case exporter.LoginStarted:
		r.Log.Info(t(i18n.LogLoginStarted), "user", e.User)
	case exporter.CodeRequired:
		r.Log.Info(t(i18n.LogCodeRequired))
	case exporter.CodeResent:
		r.Log.Info(t(i18n.LogCodeResent))
	case exporter.CodeRejected:
		r.Log.Warn(t(i18n.LogCodeRejected, e.Attempt, e.Max))
	case exporter.LoginSucceeded:
		r.Log.Info(t(i18n.LogLoggedIn))
	case exporter.YearScanned:
		r.Log.Info(t(i18n.LogYearScanned), "year", e.Year, "documents", e.Total, "new", e.Pending)
	case exporter.ScanFinished:
		r.Log.Info(t(i18n.LogScanFinished), "documents", e.Total, "new", e.Pending)
	case exporter.DownloadStarted:
		r.Log.Debug(t(i18n.LogDownloading), "n", e.Index, "of", e.Total, "doc", docLabel(r.P, e.Doc))
	case exporter.DownloadFinished:
		r.Log.Info(t(i18n.LogDownloaded), "n", e.Index, "of", e.Total, "file", e.Record.File, "size", humanize.Bytes(uint64(e.Record.Size)))
	case exporter.DownloadFailed:
		r.Log.Error(t(i18n.LogDownloadFailed), "n", e.Index, "of", e.Total, "doc", docLabel(r.P, e.Doc), "err", ErrorText(r.P, e.Err))
	case exporter.SyncFinished:
	}
}

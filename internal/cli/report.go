package cli

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
	"github.com/jannes-dailidow/hrc-exporter/internal/report"
	"github.com/jannes-dailidow/hrc-exporter/internal/store"
)

const (
	keyHTML      = "html"
	keyJSON      = "json"
	keyPdftotext = "pdftotext"
	keyOpen      = "open"

	defaultReportFile = "report.html"
)

func (a *app) reportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: a.p.T(i18n.CmdReportShort),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := a.v.GetString(keyOut)
			text, err := report.FindPdftotext(a.v.GetString(keyPdftotext))
			if err != nil {
				return err
			}
			st, err := store.Open(filepath.Join(dir, store.FileName))
			if err != nil {
				return err
			}
			r, err := report.Generate(cmd.Context(), dir, st, text,
				report.WithLogger(slog.New(a.log).With("component", "report")))
			if err != nil {
				return err
			}
			for _, w := range r.Warnings {
				a.log.Warn(a.p.T(i18n.LogReportWarning), "detail", w)
			}

			htmlPath := a.v.GetString(keyHTML)
			if htmlPath == "" {
				htmlPath = filepath.Join(dir, defaultReportFile)
			}
			now := time.Now()
			if err := writeFileAtomic(htmlPath, func(w io.Writer) error { return r.WriteHTML(w, now) }); err != nil {
				return err
			}
			if jsonPath := a.v.GetString(keyJSON); jsonPath != "" {
				if err := writeFileAtomic(jsonPath, r.WriteJSON); err != nil {
					return err
				}
			}
			a.log.Info(a.p.T(i18n.LogReportWritten, len(r.Months), r.First().Period.Label(), r.Last().Period.Label()), "file", htmlPath)

			if a.v.GetBool(keyOpen) {
				return openInBrowser(htmlPath)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.String(keyHTML, "", a.p.T(i18n.FlagHTML, defaultReportFile))
	f.String(keyJSON, "", a.p.T(i18n.FlagJSON))
	f.String(keyPdftotext, "", a.p.T(i18n.FlagPdftotext))
	f.Bool(keyOpen, false, a.p.T(i18n.FlagOpen))
	_ = a.v.BindPFlags(f)
	return cmd
}

// writeFileAtomic renders into memory and replaces path only on success, so
// a failed run never leaves a truncated report behind.
func writeFileAtomic(path string, render func(io.Writer) error) error {
	var buf bytes.Buffer
	if err := render(&buf); err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// openInBrowser opens path with the platform's default handler.
func openInBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

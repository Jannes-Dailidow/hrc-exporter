package ui

import (
	"fmt"
	"io"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/dustin/go-humanize"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
	"github.com/jannes-dailidow/hrc-exporter/internal/exporter"
	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
)

var (
	styleHeader = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	styleCell   = lipgloss.NewStyle().Padding(0, 1)
	styleOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleNew    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleBold   = lipgloss.NewStyle().Bold(true)
)

func docLabel(p *i18n.Printer, d domain.Document) string {
	return p.Date(d.Date) + " " + d.Type
}

// Renderer prints command results.
type Renderer struct{ P *i18n.Printer }

// Status prints a document table.
func (r Renderer) Status(w io.Writer, docs []exporter.DocumentStatus) {
	t := r.P.T
	if len(docs) == 0 {
		fmt.Fprintln(w, styleDim.Render(t(i18n.TableNoDocuments)))
		return
	}
	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(styleDim).
		Headers(t(i18n.TableYear), t(i18n.TableDate), t(i18n.TableType), t(i18n.TableStatus), t(i18n.TableFile)).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return styleHeader
			}
			return styleCell
		})
	for _, st := range docs {
		status, file := styleNew.Render(t(i18n.StatusNew)), ""
		if st.Downloaded {
			status, file = styleOK.Render(t(i18n.StatusDownloaded)), st.File
		}
		tbl.Row(strconv.Itoa(st.Doc.Year), r.P.Date(st.Doc.Date), st.Doc.Type, status, file)
	}
	fmt.Fprintln(w, tbl)
}

// Summary prints the outcome of a sync.
func (r Renderer) Summary(w io.Writer, s exporter.Summary) {
	t := r.P.T
	if s.DryRun {
		fmt.Fprintln(w, styleBold.Render(t(i18n.SummaryDryRun)), t(i18n.SummaryWouldLoad, len(s.Pending), s.Total))
		for _, d := range s.Pending {
			fmt.Fprintf(w, "  %s %s\n", styleNew.Render("+"), exporter.RelativePath(d))
		}
		return
	}
	fmt.Fprint(w, styleBold.Render(t(i18n.SummaryDone)), " ",
		t(i18n.SummaryResult, styleOK.Render(strconv.Itoa(s.Downloaded)), humanize.Bytes(uint64(s.Bytes)), s.Skipped))
	if n := len(s.Failures); n > 0 {
		fmt.Fprint(w, ", ", styleErr.Render(t(i18n.SummaryFailed, n)))
	}
	fmt.Fprintln(w)
	for _, f := range s.Failures {
		fmt.Fprintf(w, "  %s %s: %s\n", styleErr.Render("✗"), docLabel(r.P, f.Doc), ErrorText(r.P, f.Err))
	}
}

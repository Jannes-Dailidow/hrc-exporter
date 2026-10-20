package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"

	"github.com/jannes-dailidow/hrc-exporter/internal/exporter"
	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
)

// RunProgress runs work in the background and renders its events as an
// interactive progress view. Ctrl+C calls cancel; a second Ctrl+C quits
// the view immediately.
func RunProgress(p *i18n.Printer, cancel context.CancelFunc, work func(exporter.Reporter) error) error {
	prog := tea.NewProgram(newProgressModel(p, cancel), tea.WithOutput(os.Stderr))
	done := make(chan error, 1)
	go func() {
		err := work(exporter.ReporterFunc(func(ev exporter.Event) { prog.Send(eventMsg{ev}) }))
		prog.Send(doneMsg{})
		done <- err
	}()
	if _, err := prog.Run(); err != nil {
		cancel()
		return errors.Join(err, <-done)
	}
	// After an early quit (second Ctrl+C) work is already cancelled.
	return <-done
}

type (
	eventMsg struct{ exporter.Event }
	doneMsg  struct{}
)

type progressModel struct {
	p          *i18n.Printer
	cancel     context.CancelFunc
	spinner    spinner.Model
	bar        progress.Model
	status     string
	total      int
	done       int
	cancelling bool
	finished   bool
}

func newProgressModel(p *i18n.Printer, cancel context.CancelFunc) progressModel {
	return progressModel{
		p:       p,
		cancel:  cancel,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styleNew)),
		bar:     progress.New(progress.WithDefaultGradient(), progress.WithWidth(40)),
		status:  p.T(i18n.ProgressScanning),
	}
}

func (m progressModel) Init() tea.Cmd { return m.spinner.Tick }

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			if m.cancelling {
				return m, tea.Quit
			}
			m.cancelling, m.status = true, m.p.T(i18n.ProgressCancelling)
			m.cancel()
		}
	case tea.WindowSizeMsg:
		m.bar.Width = max(10, min(60, msg.Width-20))
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		bar, cmd := m.bar.Update(msg)
		m.bar = bar.(progress.Model)
		return m, cmd
	case eventMsg:
		return m.handle(msg.Event)
	case doneMsg:
		m.finished = true
		return m, tea.Quit
	}
	return m, nil
}

func (m progressModel) handle(ev exporter.Event) (tea.Model, tea.Cmd) {
	switch e := ev.(type) {
	case exporter.YearScanned:
		line := fmt.Sprintf("%s %d  %s", styleOK.Render("✓"), e.Year, m.p.N(i18n.CountDocuments, e.Total, e.Total))
		if e.Pending > 0 {
			line += styleNew.Render(", " + m.p.N(i18n.CountNew, e.Pending, e.Pending))
		}
		return m, tea.Println(line)
	case exporter.ScanFinished:
		m.total = e.Pending
		if m.total == 0 {
			m.status = m.p.T(i18n.ProgressUpToDate)
		}
	case exporter.DownloadStarted:
		if !m.cancelling {
			m.status = docLabel(m.p, e.Doc)
		}
	case exporter.DownloadFinished:
		m.done++
		line := fmt.Sprintf("%s %s %s", styleOK.Render("↓"), e.Record.File, styleDim.Render(humanize.Bytes(uint64(e.Record.Size))))
		return m, tea.Batch(tea.Println(line), m.bar.SetPercent(m.percent()))
	case exporter.DownloadFailed:
		m.done++
		line := fmt.Sprintf("%s %s: %s", styleErr.Render("✗"), docLabel(m.p, e.Doc), ErrorText(m.p, e.Err))
		return m, tea.Batch(tea.Println(line), m.bar.SetPercent(m.percent()))
	}
	return m, nil
}

func (m progressModel) percent() float64 {
	if m.total == 0 {
		return 1
	}
	return float64(m.done) / float64(m.total)
}

func (m progressModel) View() string {
	if m.finished {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.spinner.View() + " " + m.status + "\n")
	if m.total > 0 {
		fmt.Fprintf(&b, "  %s %d/%d\n", m.bar.View(), m.done, m.total)
	}
	return b.String()
}

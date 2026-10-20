package exporter

import "github.com/jannes-dailidow/hrc-exporter/internal/domain"

// Event is a progress notification emitted by the Exporter.
type Event interface{ event() }

type (
	LoginStarted    struct{ User string }
	CodeRequired    struct{}
	CodeResent      struct{}
	CodeRejected    struct{ Attempt, Max int }
	LoginSucceeded  struct{}
	YearScanned     struct{ Year, Total, Pending int }
	ScanFinished    struct{ Total, Pending int }
	DownloadStarted struct {
		Doc          domain.Document
		Index, Total int
	}
	DownloadFinished struct {
		Record       domain.Record
		Index, Total int
	}
	DownloadFailed struct {
		Doc          domain.Document
		Err          error
		Index, Total int
	}
	SyncFinished struct{ Summary Summary }
)

func (LoginStarted) event()     {}
func (CodeRequired) event()     {}
func (CodeResent) event()       {}
func (CodeRejected) event()     {}
func (LoginSucceeded) event()   {}
func (YearScanned) event()      {}
func (ScanFinished) event()     {}
func (DownloadStarted) event()  {}
func (DownloadFinished) event() {}
func (DownloadFailed) event()   {}
func (SyncFinished) event()     {}

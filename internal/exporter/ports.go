package exporter

import (
	"context"
	"io"
	"io/fs"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
)

// Portal is the remote document source.
type Portal interface {
	Login(ctx context.Context, creds domain.Credentials) (needsCode bool, err error)
	ResendCode(ctx context.Context) error
	Verify2FA(ctx context.Context, code string) error
	Years(ctx context.Context) ([]int, error)
	List(ctx context.Context, year int) ([]domain.Document, error)
	Download(ctx context.Context, doc domain.Document, w io.Writer) (int64, error)
	Logout(ctx context.Context) error
}

// Store remembers downloaded documents.
type Store interface {
	Get(id string) (domain.Record, bool)
	Put(r domain.Record)
	Save() error
}

// Authenticator supplies login data, typically by asking the user.
type Authenticator interface {
	Credentials(ctx context.Context) (domain.Credentials, error)
	// TwoFactorCode returns the e-mailed login code, or resend=true if a
	// new code should be requested.
	TwoFactorCode(ctx context.Context) (code string, resend bool, err error)
}

// Reporter receives progress events. Implementations must be safe to call
// from the goroutine running the exporter.
type Reporter interface {
	Report(Event)
}

// ReporterFunc adapts a function to Reporter.
type ReporterFunc func(Event)

func (f ReporterFunc) Report(e Event) { f(e) }

// Discard is a Reporter that drops all events.
var Discard Reporter = ReporterFunc(func(Event) {})

// TempFile is a file being written.
type TempFile interface {
	io.WriteCloser
	Name() string
}

// FileSystem is the target for downloaded files.
type FileSystem interface {
	MkdirAll(path string) error
	CreateTemp(dir, pattern string) (TempFile, error)
	Rename(oldPath, newPath string) error
	Remove(path string) error
	Stat(path string) (fs.FileInfo, error)
}

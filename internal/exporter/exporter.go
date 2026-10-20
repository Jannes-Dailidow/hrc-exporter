// Package exporter orchestrates login, listing and downloading of portal
// documents. It never prints or prompts: input comes from an Authenticator,
// progress goes to a Reporter, results are returned as values.
package exporter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
)

const defaultCodeAttempts = 3

// Exporter syncs portal documents into a local directory.
type Exporter struct {
	portal Portal
	store  Store
	auth   Authenticator
	fs     FileSystem
	log    *slog.Logger
	now    func() time.Time
	outDir string
	tries  int
}

// Option configures an Exporter.
type Option func(*Exporter)

func WithOutDir(dir string) Option          { return func(e *Exporter) { e.outDir = dir } }
func WithFileSystem(fs FileSystem) Option   { return func(e *Exporter) { e.fs = fs } }
func WithLogger(l *slog.Logger) Option      { return func(e *Exporter) { e.log = l } }
func WithClock(now func() time.Time) Option { return func(e *Exporter) { e.now = now } }
func WithCodeAttempts(n int) Option         { return func(e *Exporter) { e.tries = n } }

// New creates an Exporter.
func New(p Portal, s Store, a Authenticator, opts ...Option) *Exporter {
	e := &Exporter{
		portal: p, store: s, auth: a,
		fs:     OSFileSystem{},
		log:    slog.New(slog.DiscardHandler),
		now:    time.Now,
		outDir: ".",
		tries:  defaultCodeAttempts,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Selection restricts which documents are considered.
type Selection struct {
	Years []int  // empty: all years offered by the portal
	Type  string // case-insensitive substring of the document type; empty: all
}

func (s Selection) matches(d domain.Document) bool {
	return s.Type == "" || strings.Contains(strings.ToLower(d.Type), strings.ToLower(s.Type))
}

// SyncOptions tune Sync.
type SyncOptions struct {
	Force  bool // download even if already recorded
	DryRun bool // only determine what would be downloaded
}

// DocumentStatus is a document together with its local state.
type DocumentStatus struct {
	Doc        domain.Document
	Downloaded bool
	File       string // relative path, if downloaded
}

// Failure is a document that could not be downloaded.
type Failure struct {
	Doc domain.Document
	Err error
}

// Summary is the result of a Sync.
type Summary struct {
	Total, Skipped, Downloaded int
	Bytes                      int64
	DryRun                     bool
	Pending                    []domain.Document // dry run only
	Failures                   []Failure
}

// Authenticate logs in, asking for the second factor as needed. It returns
// the user that logged in.
func (e *Exporter) Authenticate(ctx context.Context, rep Reporter) (user string, err error) {
	creds, err := e.auth.Credentials(ctx)
	if err != nil {
		return "", err
	}
	rep.Report(LoginStarted{User: creds.User})
	needsCode, err := e.portal.Login(ctx, creds)
	if err != nil {
		return "", err
	}
	if needsCode {
		rep.Report(CodeRequired{})
		if err := e.verifyCode(ctx, rep); err != nil {
			return "", err
		}
	}
	rep.Report(LoginSucceeded{})
	return creds.User, nil
}

func (e *Exporter) verifyCode(ctx context.Context, rep Reporter) error {
	for attempt := 1; ; {
		code, resend, err := e.auth.TwoFactorCode(ctx)
		if err != nil {
			return err
		}
		if resend {
			if err := e.portal.ResendCode(ctx); err != nil {
				return fmt.Errorf("resend code: %w", err)
			}
			rep.Report(CodeResent{})
			continue
		}
		err = e.portal.Verify2FA(ctx, code)
		if !errors.Is(err, domain.ErrInvalidCode) || attempt >= e.tries {
			return err
		}
		rep.Report(CodeRejected{Attempt: attempt, Max: e.tries})
		attempt++
	}
}

// Logout ends the portal session.
func (e *Exporter) Logout(ctx context.Context) error { return e.portal.Logout(ctx) }

// Status lists the selected documents with their local state.
func (e *Exporter) Status(ctx context.Context, sel Selection, rep Reporter) ([]DocumentStatus, error) {
	years := sel.Years
	if len(years) == 0 {
		var err error
		if years, err = e.portal.Years(ctx); err != nil {
			return nil, err
		}
	}
	var out []DocumentStatus
	for _, year := range years {
		docs, err := e.portal.List(ctx, year)
		if err != nil {
			return nil, fmt.Errorf("list %d: %w", year, err)
		}
		total, pending := 0, 0
		for _, d := range docs {
			if !sel.matches(d) {
				continue
			}
			st := e.status(d)
			total++
			if !st.Downloaded {
				pending++
			}
			out = append(out, st)
		}
		rep.Report(YearScanned{Year: year, Total: total, Pending: pending})
	}
	return out, nil
}

// Sync downloads all selected documents that are not stored locally yet.
// Individual download failures are collected in the Summary; the returned
// error is reserved for problems that abort the whole run.
func (e *Exporter) Sync(ctx context.Context, sel Selection, opts SyncOptions, rep Reporter) (Summary, error) {
	statuses, err := e.Status(ctx, sel, rep)
	if err != nil {
		return Summary{}, err
	}
	sum := Summary{Total: len(statuses), DryRun: opts.DryRun}
	var pending []domain.Document
	for _, st := range statuses {
		if st.Downloaded && !opts.Force {
			sum.Skipped++
			continue
		}
		pending = append(pending, st.Doc)
	}
	rep.Report(ScanFinished{Total: sum.Total, Pending: len(pending)})

	if opts.DryRun {
		sum.Pending = pending
		rep.Report(SyncFinished{Summary: sum})
		return sum, nil
	}
	for i, doc := range pending {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		idx := i + 1
		rep.Report(DownloadStarted{Doc: doc, Index: idx, Total: len(pending)})
		rec, err := e.download(ctx, doc)
		if err != nil {
			if ctx.Err() != nil {
				return sum, ctx.Err()
			}
			e.log.Debug("download failed", "id", doc.ID, "err", err)
			sum.Failures = append(sum.Failures, Failure{Doc: doc, Err: err})
			rep.Report(DownloadFailed{Doc: doc, Err: err, Index: idx, Total: len(pending)})
			continue
		}
		e.store.Put(rec)
		if err := e.store.Save(); err != nil {
			return sum, fmt.Errorf("save metadata: %w", err)
		}
		sum.Downloaded++
		sum.Bytes += rec.Size
		rep.Report(DownloadFinished{Record: rec, Index: idx, Total: len(pending)})
	}
	rep.Report(SyncFinished{Summary: sum})
	return sum, nil
}

func (e *Exporter) status(d domain.Document) DocumentStatus {
	st := DocumentStatus{Doc: d}
	rec, ok := e.store.Get(d.ID)
	if !ok {
		return st
	}
	if _, err := e.fs.Stat(e.abs(rec.File)); err != nil {
		e.log.Debug("recorded file missing, will re-download", "id", d.ID, "file", rec.File)
		return st
	}
	st.Downloaded, st.File = true, rec.File
	return st
}

func (e *Exporter) download(ctx context.Context, doc domain.Document) (domain.Record, error) {
	rel := RelativePath(doc)
	target := e.abs(rel)
	dir := filepath.Dir(target)
	if err := e.fs.MkdirAll(dir); err != nil {
		return domain.Record{}, err
	}
	tmp, err := e.fs.CreateTemp(dir, ".download-*.part")
	if err != nil {
		return domain.Record{}, err
	}
	h := sha256.New()
	n, err := e.portal.Download(ctx, doc, io.MultiWriter(tmp, h))
	if err = errors.Join(err, tmp.Close()); err == nil && n == 0 {
		err = errors.New("empty document")
	}
	if err == nil {
		err = e.fs.Rename(tmp.Name(), target)
	}
	if err != nil {
		_ = e.fs.Remove(tmp.Name())
		return domain.Record{}, err
	}
	return domain.Record{
		ID: doc.ID, Year: doc.Year, Date: doc.Date, Type: doc.Type,
		File: rel, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n,
		DownloadedAt: e.now().UTC(),
	}, nil
}

func (e *Exporter) abs(rel string) string { return filepath.Join(e.outDir, filepath.FromSlash(rel)) }

// RelativePath is the slash-separated location of a document below the
// output directory: <year>/<date>_<type>_<id8>.pdf
func RelativePath(d domain.Document) string {
	id := d.ID
	if len(id) > 8 {
		id = id[:8]
	}
	name := fmt.Sprintf("%s_%s_%s.pdf", d.Date.Format(time.DateOnly), slug(d.Type), slug(id))
	return path.Join(strconv.Itoa(d.Year), name)
}

var umlauts = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "Ä", "ae", "Ö", "oe", "Ü", "ue", "ß", "ss")

func slug(s string) string {
	s = strings.ToLower(umlauts.Replace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimSuffix(b.String(), "-")
	if out == "" {
		return "dokument"
	}
	return out
}

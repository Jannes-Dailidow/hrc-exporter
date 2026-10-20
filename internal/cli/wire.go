package cli

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jannes-dailidow/hrc-exporter/internal/exporter"
	"github.com/jannes-dailidow/hrc-exporter/internal/portal"
	"github.com/jannes-dailidow/hrc-exporter/internal/store"
	"github.com/jannes-dailidow/hrc-exporter/internal/ui"
)

// session is a logged-in exporter plus the ui bits needed to drive it.
type session struct {
	exp   *exporter.Exporter
	plain exporter.Reporter
	a     *app
}

// login wires all dependencies and authenticates against the portal.
func (a *app) login(ctx context.Context) (*session, error) {
	logger := slog.New(a.log)
	out := a.v.GetString(keyOut)

	userCache, urlCache := newLastUsed(lastUserFile), newLastUsed(lastBaseURLFile)
	prompter := &ui.Prompter{
		P:                a.p,
		BaseURL:          a.v.GetString(keyBaseURL),
		User:             a.v.GetString(keyUser),
		Password:         a.v.GetString(keyPassword),
		SuggestedBaseURL: urlCache.load(),
		SuggestedUser:    userCache.load(),
		Accessible:       !ui.IsTerminal(os.Stdin),
	}
	baseURL, err := prompter.PortalURL(ctx)
	if err != nil {
		return nil, err
	}
	client, err := portal.New(
		portal.WithBaseURL(baseURL),
		portal.WithLogger(logger.With("component", "portal")),
	)
	if err != nil {
		return nil, err
	}
	st, err := store.Open(filepath.Join(out, store.FileName))
	if err != nil {
		return nil, err
	}
	exp := exporter.New(client, st, prompter,
		exporter.WithOutDir(out),
		exporter.WithLogger(logger.With("component", "exporter")),
	)

	s := &session{exp: exp, plain: ui.PlainReporter{Log: a.log, P: a.p}, a: a}
	user, err := exp.Authenticate(ctx, s.plain)
	if err != nil {
		return nil, err
	}
	if err := userCache.save(user); err != nil {
		a.log.Debug("could not cache personnel number", "err", err)
	}
	if err := urlCache.save(baseURL); err != nil {
		a.log.Debug("could not cache base url", "err", err)
	}
	return s, nil
}

// close logs out, even if ctx was cancelled.
func (s *session) close(ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.exp.Logout(ctx); err != nil {
		s.a.log.Debug("logout failed", "err", err)
	}
}

// run executes work with interactive progress when attached to a terminal.
func (s *session) run(cancel context.CancelFunc, work func(exporter.Reporter) error) error {
	if s.a.v.GetBool(keyPlain) || !ui.IsTerminal(os.Stderr) {
		return work(s.plain)
	}
	return ui.RunProgress(s.a.p, cancel, work)
}

func (a *app) selection() exporter.Selection {
	return exporter.Selection{Years: a.v.GetIntSlice(keyYear), Type: a.v.GetString(keyType)}
}

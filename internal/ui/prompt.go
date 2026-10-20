// Package ui is the terminal presentation layer: prompts, progress
// rendering and result formatting. It depends on the core packages, never
// the other way round. All text goes through an injected *i18n.Printer.
package ui

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
	"github.com/jannes-dailidow/hrc-exporter/internal/portal"
)

// Prompter asks the user for login data. Preset values skip the prompt.
// It implements exporter.Authenticator.
type Prompter struct {
	P                *i18n.Printer
	BaseURL          string
	User, Password   string
	SuggestedBaseURL string // prefilled when BaseURL has to be prompted
	SuggestedUser    string // prefilled when User has to be prompted
	Accessible       bool   // plain line-based prompts (no TTY / screen readers)
}

// PortalURL returns the preset base URL or prompts for it.
func (p *Prompter) PortalURL(ctx context.Context) (string, error) {
	if p.BaseURL != "" {
		return p.BaseURL, nil
	}
	u := p.SuggestedBaseURL
	msg := p.P.T(i18n.ValidateBaseURL)
	err := p.run(ctx, huh.NewGroup(huh.NewInput().
		Title(p.P.T(i18n.PromptBaseURLTitle)).
		Value(&u).
		Validate(func(s string) error {
			if _, err := portal.ParseBaseURL(s); err != nil {
				return errors.New(msg)
			}
			return nil
		})))
	return strings.TrimSpace(u), err
}

// Credentials returns preset credentials or prompts for missing ones.
func (p *Prompter) Credentials(ctx context.Context) (domain.Credentials, error) {
	c := domain.Credentials{User: p.User, Password: p.Password}
	var fields []huh.Field
	if c.User == "" {
		c.User = p.SuggestedUser
		fields = append(fields, huh.NewInput().
			Title(p.P.T(i18n.PromptUserTitle)).
			Value(&c.User).
			Validate(required(p.P.T(i18n.ValidateUser))))
	}
	if c.Password == "" {
		fields = append(fields, huh.NewInput().
			Title(p.P.T(i18n.PromptPasswordTitle)).
			EchoMode(huh.EchoModePassword).
			Value(&c.Password).
			Validate(required(p.P.T(i18n.ValidatePassword))))
	}
	if len(fields) > 0 {
		if err := p.run(ctx, huh.NewGroup(fields...)); err != nil {
			return domain.Credentials{}, err
		}
	}
	c.User = strings.TrimSpace(c.User)
	return c, nil
}

// TwoFactorCode prompts for the e-mailed login code. An empty answer
// requests a new code.
func (p *Prompter) TwoFactorCode(ctx context.Context) (string, bool, error) {
	var code string
	err := p.run(ctx, huh.NewGroup(huh.NewInput().
		Title(p.P.T(i18n.PromptCodeTitle)).
		Description(p.P.T(i18n.PromptCodeDesc)).
		Value(&code)))
	code = strings.TrimSpace(code)
	return code, code == "", err
}

func (p *Prompter) run(ctx context.Context, g *huh.Group) error {
	return huh.NewForm(g).
		WithTheme(huh.ThemeCharm()).
		WithKeyMap(keyMap(p.P)).
		WithAccessible(p.Accessible).
		WithOutput(os.Stderr).
		RunWithContext(ctx)
}

// keyMap localizes the help labels of the bindings used by input fields.
func keyMap(p *i18n.Printer) *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit.SetHelp("ctrl+c", p.T(i18n.KeyHelpQuit))
	km.Input.Next.SetHelp("enter", p.T(i18n.KeyHelpNext))
	km.Input.Prev.SetHelp("shift+tab", p.T(i18n.KeyHelpBack))
	km.Input.Submit.SetHelp("enter", p.T(i18n.KeyHelpSubmit))
	km.Input.AcceptSuggestion.SetHelp("ctrl+e", p.T(i18n.KeyHelpComplete))
	return km
}

func required(msg string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New(msg)
		}
		return nil
	}
}

// IsAbort reports whether err means the user cancelled.
func IsAbort(err error) bool {
	return errors.Is(err, huh.ErrUserAborted) || errors.Is(err, context.Canceled)
}

// IsTerminal reports whether f is an interactive terminal.
func IsTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

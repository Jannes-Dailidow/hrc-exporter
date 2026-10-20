package portal

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
	"github.com/jannes-dailidow/hrc-exporter/internal/portal/webform"
)

// Pages and form fields, as observed in the portal.
const (
	pathLogin     = "Default.aspx"
	pathTwoFactor = "Login2Factor.aspx"
	pathOverview  = "Uebersicht.aspx"

	fieldUser     = "ctl00$MainContent$ePersonalnummer"
	fieldPassword = "ctl00$MainContent$eKennwort" // password on login, code on 2FA page
	fieldSubmit   = "ctl00$MainContent$ImageButton1"
	fieldResend   = "ctl00$MainContent$Button2"
	resendLabel   = "Email mit Login-Code versenden"
)

// Login submits the first factor. It reports whether a second factor
// (e-mailed login code) is required, which the portal sends automatically.
func (c *Client) Login(ctx context.Context, creds domain.Credentials) (needsCode bool, err error) {
	c.reset()
	if _, err := c.load(ctx, pathLogin); err != nil {
		return false, err
	}
	fields := webform.ImageClick(fieldSubmit)
	fields.Set(fieldUser, creds.User)
	fields.Set(fieldPassword, creds.Password)

	r, err := c.postBack(ctx, fields)
	if err != nil {
		return false, err
	}
	switch {
	case r.redirectsTo(pathTwoFactor):
		_, err := c.load(ctx, pathTwoFactor)
		return true, err
	case r.redirectsTo(pathOverview):
		return false, c.enterOverview(ctx)
	case r.page != nil:
		return false, rejected(domain.ErrInvalidCredentials, r.page)
	default:
		return false, c.unexpected(pathLogin, r)
	}
}

// ResendCode asks the portal to e-mail a new login code.
func (c *Client) ResendCode(ctx context.Context) error {
	if err := c.requirePage(pathTwoFactor); err != nil {
		return err
	}
	r, err := c.postBack(ctx, url.Values{fieldResend: {resendLabel}})
	if err != nil {
		return err
	}
	switch {
	case r.page != nil:
		return nil
	case r.redirectsTo(pathTwoFactor):
		_, err := c.load(ctx, pathTwoFactor)
		return err
	default:
		return c.unexpected(pathTwoFactor, r)
	}
}

// Verify2FA submits the e-mailed login code.
func (c *Client) Verify2FA(ctx context.Context, code string) error {
	if err := c.requirePage(pathTwoFactor); err != nil {
		return err
	}
	fields := webform.ImageClick(fieldSubmit)
	fields.Set(fieldPassword, code)

	r, err := c.postBack(ctx, fields)
	if err != nil {
		return err
	}
	switch {
	case r.redirectsTo(pathOverview):
		return c.enterOverview(ctx)
	case r.page != nil:
		return rejected(domain.ErrInvalidCode, r.page)
	default:
		return c.unexpected(pathTwoFactor, r)
	}
}

// Logout ends the portal session. It is a no-op when not logged in.
func (c *Client) Logout(ctx context.Context) error {
	if !c.loggedIn {
		return nil
	}
	defer c.reset()
	_, err := c.postBack(ctx, webform.ImageClick(fieldSubmit)) // "Abmelden"
	return err
}

func (c *Client) enterOverview(ctx context.Context) error {
	r, err := c.load(ctx, pathOverview)
	if err != nil {
		return err
	}
	c.loggedIn = true
	c.listedYear = selectedYear(r.page)
	return nil
}

func (c *Client) requirePage(page string) error {
	if c.pageURL == nil || !strings.EqualFold(c.pageURL.Path, c.url(page).Path) {
		return fmt.Errorf("%w: not on %s", domain.ErrNotLoggedIn, page)
	}
	return nil
}

// rejected wraps base with the portal's alert messages, if any.
func rejected(base error, p *webform.Page) error {
	if msgs := p.Alerts(); len(msgs) > 0 {
		return fmt.Errorf("%w: %s", base, strings.Join(msgs, "; "))
	}
	return base
}

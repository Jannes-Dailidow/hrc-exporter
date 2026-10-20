// Package portal is an HTTP client for the HRCommunicator payroll portal.
// The portal is an ASP.NET WebForms application:
// every action is a postback of the current page's hidden state, so the
// client is stateful and must be used sequentially.
package portal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
	"github.com/jannes-dailidow/hrc-exporter/internal/portal/webform"
)

const (
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36"
	maxPageSize      = 16 << 20
)

// Client talks to the portal. It is not safe for concurrent use.
type Client struct {
	http      *http.Client
	base      *url.URL
	log       *slog.Logger
	userAgent string

	page       *webform.Page // last page that can be posted back
	pageURL    *url.URL
	loggedIn   bool
	listedYear int // year currently shown on the overview page
}

// Option configures a Client.
type Option func(*Client) error

// ErrNoBaseURL is returned by New when no base URL was configured.
var ErrNoBaseURL = errors.New("portal base url not set")

// ParseBaseURL checks that raw is an absolute http(s) URL and normalizes
// its path to end in a slash.
func ParseBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("base url: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("base url %q: want http(s)://host/", raw)
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return u, nil
}

// WithBaseURL sets the portal base URL (required).
func WithBaseURL(raw string) Option {
	return func(c *Client) error {
		u, err := ParseBaseURL(raw)
		if err != nil {
			return err
		}
		c.base = u
		return nil
	}
}

// WithHTTPClient uses h as transport template. Redirect handling is
// overridden and a cookie jar is added if missing.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) error {
		cp := *h
		c.http = &cp
		return nil
	}
}

// WithLogger sets the debug logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) error {
		c.log = l
		return nil
	}
}

// WithUserAgent overrides the browser user agent sent to the portal.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		c.userAgent = ua
		return nil
	}
}

// New creates a portal client.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		log:       slog.New(slog.DiscardHandler),
		userAgent: defaultUserAgent,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.base == nil {
		return nil, ErrNoBaseURL
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: 2 * time.Minute}
	}
	if c.http.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, err
		}
		c.http.Jar = jar
	}
	// Redirects are part of the protocol (they signal login success), so
	// they are followed manually.
	c.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c, nil
}

// result is the outcome of a page request.
type result struct {
	status   int
	location *url.URL      // set for redirects
	page     *webform.Page // set for 200 responses
}

func (r *result) redirectsTo(page string) bool {
	return r.location != nil && strings.EqualFold(path.Base(r.location.Path), page)
}

func (c *Client) url(page string) *url.URL {
	return c.base.ResolveReference(&url.URL{Path: page})
}

// send performs a request with browser-like headers. form != nil makes it
// a form POST.
func (c *Client) send(ctx context.Context, u *url.URL, form url.Values) (*http.Response, error) {
	method, body := http.MethodGet, io.Reader(nil)
	if form != nil {
		method, body = http.MethodPost, strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,application/pdf,*/*;q=0.8")
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	c.log.DebugContext(ctx, "http", "method", method, "path", u.Path, "status", resp.StatusCode,
		"content_type", resp.Header.Get("Content-Type"), "location", resp.Header.Get("Location"))
	return resp, nil
}

// exchange performs a request and parses the response as page or redirect.
func (c *Client) exchange(ctx context.Context, u *url.URL, form url.Values) (*result, error) {
	resp, err := c.send(ctx, u, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	r := &result{status: resp.StatusCode}
	switch {
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		if r.location, err = resp.Location(); err != nil {
			return nil, fmt.Errorf("%s: redirect without location: %w", u.Path, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
	case resp.StatusCode == http.StatusOK:
		if r.page, err = webform.Parse(io.LimitReader(resp.Body, maxPageSize)); err != nil {
			return nil, fmt.Errorf("%s: %w", u.Path, err)
		}
	default:
		return nil, fmt.Errorf("%s: unexpected status %s", u.Path, resp.Status)
	}
	return r, nil
}

// load GETs a page and makes it the current postback page.
func (c *Client) load(ctx context.Context, page string) (*result, error) {
	u := c.url(page)
	r, err := c.exchange(ctx, u, nil)
	if err != nil {
		return nil, err
	}
	if r.page == nil {
		return nil, c.unexpected(page, r)
	}
	c.page, c.pageURL = r.page, u
	return r, nil
}

// postBack posts fields together with the current page's hidden state.
func (c *Client) postBack(ctx context.Context, fields url.Values) (*result, error) {
	if c.page == nil {
		return nil, domain.ErrNotLoggedIn
	}
	r, err := c.exchange(ctx, c.pageURL, c.page.PostBack(fields))
	if err != nil {
		return nil, err
	}
	if r.page != nil {
		c.page = r.page
	}
	return r, nil
}

func (c *Client) unexpected(page string, r *result) error {
	if r.redirectsTo(pathLogin) {
		c.reset()
		return domain.ErrSessionExpired
	}
	if r.location != nil {
		return fmt.Errorf("%s: unexpected redirect to %s", page, r.location)
	}
	return fmt.Errorf("%s: unexpected response (status %d)", page, r.status)
}

func (c *Client) reset() {
	c.page, c.pageURL, c.loggedIn, c.listedYear = nil, nil, false, 0
}

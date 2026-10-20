package portal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jannes-dailidow/hrc-exporter/internal/domain"
	"github.com/jannes-dailidow/hrc-exporter/internal/portal/webform"
)

const (
	fieldYear       = "ctl00$MainContent$ddJahr"
	fieldDocType    = "ctl00$MainContent$ddDokType"
	fieldSearch     = "ctl00$MainContent$btSuchen"
	searchLabel     = "Suchen"
	docTypeAll      = "alle"
	docButtonPrefix = "ctl00$MainContent$"
	docTableID      = "MainContent_Table1"
	dateLayout      = "02.01.2006"
	maxPreviewHops  = 5
)

var pdfMagic = []byte("%PDF-")

// Years returns the years offered by the overview page, newest first.
func (c *Client) Years(ctx context.Context) ([]int, error) {
	if !c.loggedIn {
		return nil, domain.ErrNotLoggedIn
	}
	var years []int
	for _, o := range c.page.Options(fieldYear) {
		y, err := strconv.Atoi(strings.TrimSpace(o.Value))
		if err != nil {
			return nil, fmt.Errorf("year option %q: %w", o.Value, err)
		}
		years = append(years, y)
	}
	if len(years) == 0 {
		return nil, fmt.Errorf("%s: no years found", pathOverview)
	}
	slices.SortFunc(years, func(a, b int) int { return b - a })
	return years, nil
}

// List returns all documents of a year.
func (c *Client) List(ctx context.Context, year int) ([]domain.Document, error) {
	if !c.loggedIn {
		return nil, domain.ErrNotLoggedIn
	}
	r, err := c.postBack(ctx, url.Values{
		fieldYear:    {strconv.Itoa(year)},
		fieldDocType: {docTypeAll},
		fieldSearch:  {searchLabel},
	})
	if err != nil {
		return nil, err
	}
	if r.page == nil {
		return nil, c.unexpected(pathOverview, r)
	}
	c.listedYear = year
	return parseDocuments(r.page, year)
}

// Download writes the PDF of doc to w and returns the number of bytes written.
func (c *Client) Download(ctx context.Context, doc domain.Document, w io.Writer) (int64, error) {
	if !c.loggedIn {
		return 0, domain.ErrNotLoggedIn
	}
	// Event validation only accepts buttons of the currently listed year.
	if c.listedYear != doc.Year {
		if _, err := c.List(ctx, doc.Year); err != nil {
			return 0, err
		}
	}
	fields := webform.ImageClick(docButtonPrefix + doc.ID)
	fields.Set(fieldYear, strconv.Itoa(doc.Year))
	fields.Set(fieldDocType, docTypeAll)

	r, err := c.postBack(ctx, fields)
	if err != nil {
		return 0, err
	}
	if r.page == nil {
		return 0, c.unexpected(pathOverview, r)
	}
	target, ok := r.page.WindowOpen()
	if !ok {
		return 0, rejected(fmt.Errorf("document %s: portal opened no preview", doc.ID), r.page)
	}
	u, err := c.pageURL.Parse(target)
	if err != nil {
		return 0, fmt.Errorf("preview url %q: %w", target, err)
	}
	return c.fetchPDF(ctx, u, w)
}

// fetchPDF follows redirects and HTML wrapper pages until it reaches a PDF.
func (c *Client) fetchPDF(ctx context.Context, u *url.URL, w io.Writer) (int64, error) {
	for range maxPreviewHops {
		resp, err := c.send(ctx, u, nil)
		if err != nil {
			return 0, err
		}
		next, n, err := c.readPDF(resp, w)
		if err != nil || next == nil {
			return n, err
		}
		u = next
	}
	return 0, fmt.Errorf("%s: too many hops to reach the PDF", u.Path)
}

// readPDF copies a PDF response to w, or returns the next URL to follow.
func (c *Client) readPDF(resp *http.Response, w io.Writer) (*url.URL, int64, error) {
	defer resp.Body.Close()
	u := resp.Request.URL
	switch {
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		next, err := resp.Location()
		return next, 0, err
	case resp.StatusCode != http.StatusOK:
		return nil, 0, fmt.Errorf("%s: unexpected status %s", u.Path, resp.Status)
	}

	br := bufio.NewReader(resp.Body)
	if head, _ := br.Peek(len(pdfMagic)); bytes.Equal(head, pdfMagic) {
		n, err := io.Copy(w, br)
		return nil, n, err
	}
	body, err := io.ReadAll(io.LimitReader(br, maxPageSize))
	if err != nil {
		return nil, 0, err
	}
	link := findPDFLink(body)
	if link == "" {
		return nil, 0, fmt.Errorf("%s: no PDF found (content type %q)", u.Path, resp.Header.Get("Content-Type"))
	}
	c.log.Debug("preview links to pdf", "page", u.Path, "link", link)
	next, err := u.Parse(link)
	return next, 0, err
}

var pdfLinkRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)["']([^"'\s<>]*app_datax/[^"'\s<>]+)["']`),
	regexp.MustCompile(`(?i)["']([^"'\s<>]+\.pdf(?:\?[^"'\s<>]*)?)["']`),
}

func findPDFLink(body []byte) string {
	for _, re := range pdfLinkRes {
		if m := re.FindSubmatch(body); m != nil {
			return strings.ReplaceAll(string(m[1]), "&amp;", "&")
		}
	}
	return ""
}

func parseDocuments(p *webform.Page, year int) ([]domain.Document, error) {
	var docs []domain.Document
	for _, row := range p.Table(docTableID) {
		if len(row) < 3 || row[0].Header {
			continue
		}
		id := documentID(row[0])
		if id == "" {
			continue
		}
		date, err := time.Parse(dateLayout, row[1].Text)
		if err != nil {
			return nil, fmt.Errorf("document %s: date %q: %w", id, row[1].Text, err)
		}
		docs = append(docs, domain.Document{ID: id, Year: year, Date: date, Type: row[2].Text})
	}
	return docs, nil
}

func documentID(cell webform.Cell) string {
	for _, in := range cell.Inputs {
		if in.Type == "image" && strings.HasPrefix(in.Name, docButtonPrefix) {
			return strings.TrimPrefix(in.Name, docButtonPrefix)
		}
	}
	return ""
}

func selectedYear(p *webform.Page) int {
	for _, o := range p.Options(fieldYear) {
		if o.Selected {
			y, _ := strconv.Atoi(o.Value)
			return y
		}
	}
	return 0
}

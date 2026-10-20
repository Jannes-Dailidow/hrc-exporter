// Package webform parses ASP.NET WebForms pages: hidden postback state,
// select options, tables and the small inline scripts these pages emit.
package webform

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Input is an <input> element.
type Input struct {
	Type, Name, ID, Value, Src string
}

// Option is an <option> of a <select>.
type Option struct {
	Value, Label string
	Selected     bool
}

// Cell is a table cell (<td> or <th>).
type Cell struct {
	Header bool
	Text   string
	Inputs []Input
}

// Page is a parsed HTML page.
type Page struct {
	root *html.Node
	raw  []byte
}

// Parse reads and parses an HTML page.
func Parse(r io.Reader) (*Page, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read page: %w", err)
	}
	root, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse page: %w", err)
	}
	return &Page{root: root, raw: raw}, nil
}

// Raw returns the unparsed page source.
func (p *Page) Raw() []byte { return p.raw }

// Title returns the trimmed <title> text.
func (p *Page) Title() string {
	if n := find(p.root, func(n *html.Node) bool { return n.DataAtom == atom.Title }); n != nil {
		return text(n)
	}
	return ""
}

// Hidden returns all named hidden inputs (__VIEWSTATE, __EVENTVALIDATION, …).
func (p *Page) Hidden() url.Values {
	v := url.Values{}
	for _, in := range p.Inputs() {
		if strings.EqualFold(in.Type, "hidden") && in.Name != "" {
			v.Set(in.Name, in.Value)
		}
	}
	return v
}

// PostBack returns the hidden state of the page merged with fields,
// ready to be posted back to the page.
func (p *Page) PostBack(fields url.Values) url.Values {
	v := p.Hidden()
	for k, vs := range fields {
		v[k] = vs
	}
	return v
}

// Inputs returns all <input> elements in document order.
func (p *Page) Inputs() []Input {
	var out []Input
	for _, n := range findAll(p.root, isAtom(atom.Input)) {
		out = append(out, input(n))
	}
	return out
}

// Options returns the options of the <select> with the given name.
func (p *Page) Options(selectName string) []Option {
	sel := find(p.root, func(n *html.Node) bool {
		return n.DataAtom == atom.Select && attr(n, "name") == selectName
	})
	if sel == nil {
		return nil
	}
	var out []Option
	for _, n := range findAll(sel, isAtom(atom.Option)) {
		value, ok := attrOK(n, "value")
		if !ok {
			value = text(n)
		}
		_, selected := attrOK(n, "selected")
		out = append(out, Option{Value: value, Label: text(n), Selected: selected})
	}
	return out
}

// Text returns the whitespace-normalised text of the element with the given id.
func (p *Page) Text(id string) string {
	if n := find(p.root, func(n *html.Node) bool { return attr(n, "id") == id }); n != nil {
		return text(n)
	}
	return ""
}

// Table returns the rows of the table with the given id. Rows of nested
// tables are not included.
func (p *Page) Table(id string) [][]Cell {
	tbl := find(p.root, func(n *html.Node) bool {
		return n.DataAtom == atom.Table && attr(n, "id") == id
	})
	if tbl == nil {
		return nil
	}
	var rows [][]Cell
	var walkRows func(*html.Node)
	walkRows = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch c.DataAtom {
			case atom.Table:
				continue
			case atom.Tr:
				rows = append(rows, cells(c))
			default:
				walkRows(c)
			}
		}
	}
	walkRows(tbl)
	return rows
}

var (
	windowOpenRe = regexp.MustCompile(`window\.open\(\s*['"]([^'"]+)['"]`)
	alertRe      = regexp.MustCompile(`alert\(\s*['"]((?:[^'"\\]|\\.)*)['"]`)
)

// WindowOpen returns the target of the first window.open(...) script call.
func (p *Page) WindowOpen() (string, bool) {
	m := windowOpenRe.FindSubmatch(p.raw)
	if m == nil {
		return "", false
	}
	return html.UnescapeString(string(m[1])), true
}

// Alerts returns the messages of all alert(...) script calls.
func (p *Page) Alerts() []string {
	var out []string
	for _, m := range alertRe.FindAllSubmatch(p.raw, -1) {
		out = append(out, html.UnescapeString(string(m[1])))
	}
	return out
}

// ImageClick returns the form fields a browser sends when an
// <input type="image"> named name is clicked.
func ImageClick(name string) url.Values {
	return url.Values{name + ".x": {"12"}, name + ".y": {"12"}}
}

func cells(tr *html.Node) []Cell {
	var out []Cell
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.DataAtom != atom.Td && c.DataAtom != atom.Th {
			continue
		}
		cell := Cell{Header: c.DataAtom == atom.Th, Text: text(c)}
		for _, n := range findAll(c, isAtom(atom.Input)) {
			cell.Inputs = append(cell.Inputs, input(n))
		}
		out = append(out, cell)
	}
	return out
}

func input(n *html.Node) Input {
	return Input{
		Type:  strings.ToLower(attr(n, "type")),
		Name:  attr(n, "name"),
		ID:    attr(n, "id"),
		Value: attr(n, "value"),
		Src:   attr(n, "src"),
	}
}

func isAtom(a atom.Atom) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Type == html.ElementNode && n.DataAtom == a }
}

func find(n *html.Node, pred func(*html.Node) bool) *html.Node {
	if n.Type == html.ElementNode && pred(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if m := find(c, pred); m != nil {
			return m
		}
	}
	return nil
}

func findAll(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && pred(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func attr(n *html.Node, key string) string {
	v, _ := attrOK(n, key)
	return v
}

func attrOK(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && strings.EqualFold(a.Key, key) {
			return a.Val, true
		}
	}
	return "", false
}

func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

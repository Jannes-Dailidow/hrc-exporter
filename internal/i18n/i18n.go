// Package i18n translates user-facing text. It is a leaf package: catalogs
// are plain Go maps keyed by typed constants (keys.go), one file per language.
//
// Adding a language: add a Lang constant, a catalog_xx.go file and an entry
// in catalogs. The tests enforce that every key is translated.
package i18n

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Lang is a supported language (ISO 639-1 code).
type Lang string

const (
	EN Lang = "en"
	DE Lang = "de"

	Default = EN
)

// Supported returns all supported languages, sorted.
func Supported() []Lang {
	out := make([]Lang, 0, len(catalogs))
	for l := range catalogs {
		out = append(out, l)
	}
	slices.Sort(out)
	return out
}

// Parse accepts language codes and locale names such as "de", "de-AT",
// "de_DE.UTF-8" or "EN" and reports whether the language is supported.
func Parse(s string) (Lang, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexAny(s, "-_"); i >= 0 {
		s = s[:i]
	}
	l := Lang(s)
	_, ok := catalogs[l]
	return l, ok
}

// Printer formats messages in one language, falling back to English.
type Printer struct {
	lang Lang
	cat  Catalog
}

// New returns a Printer for l; unsupported languages yield English.
func New(l Lang) *Printer {
	cat, ok := catalogs[l]
	if !ok {
		l, cat = Default, catalogs[Default]
	}
	return &Printer{lang: l, cat: cat}
}

// Lang returns the printer's language.
func (p *Printer) Lang() Lang { return p.lang }

// T formats the message for k with fmt-style args.
func (p *Printer) T(k Key, args ...any) string {
	return p.format(p.lookup(k).Other, args)
}

// N formats the plural form of k selected by n. n is not passed implicitly;
// include it in args if the message shows it.
func (p *Printer) N(k Key, n int, args ...any) string {
	m := p.lookup(k)
	if n == 1 && m.One != "" {
		return p.format(m.One, args)
	}
	return p.format(m.Other, args)
}

// Date formats t with the language's date layout.
func (p *Printer) Date(t time.Time) string { return t.Format(p.T(FormatDate)) }

func (p *Printer) lookup(k Key) Message {
	if m, ok := p.cat[k]; ok {
		return m
	}
	if m, ok := catalogs[Default][k]; ok {
		return m
	}
	return msg(string(k))
}

func (p *Printer) format(s string, args []any) string {
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// declaredKeys parses keys.go and returns every Key constant's value.
func declaredKeys(t *testing.T) []Key {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "keys.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var keys []Key
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, v := range vs.Values {
			if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				s, _ := strconv.Unquote(lit.Value)
				keys = append(keys, Key(s))
			}
		}
		return true
	})
	if len(keys) < 10 {
		t.Fatalf("found only %d keys in keys.go", len(keys))
	}
	return keys
}

// placeholderRe matches fmt verbs incl. explicit index, flags, width and precision.
var placeholderRe = regexp.MustCompile(`%(\[\d+\])?[-+# 0]*(\*|\d+)?(\.(\*|\d+))?([a-zA-Z%])`)

// placeholders returns the fmt verbs of s in order, normalised to
// "[index]verb" (flags/width/precision may differ between languages, the
// argument index and type may not). Literal %% is ignored.
func placeholders(s string) []string {
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(s, -1) {
		if m[5] == "%" {
			continue
		}
		out = append(out, m[1]+m[5])
	}
	return out
}

func TestPlaceholders(t *testing.T) {
	got := placeholders(`%d of %5.2f, 100%% %[2]q %-8s`)
	if want := []string{"d", "f", "[2]q", "s"}; !slices.Equal(got, want) {
		t.Errorf("placeholders = %v, want %v", got, want)
	}
}

func TestCatalogsDeclareEveryKey(t *testing.T) {
	keys := declaredKeys(t)
	for lang, cat := range catalogs {
		for _, k := range keys {
			if _, ok := cat[k]; !ok {
				t.Errorf("%s: missing key %s", lang, k)
			}
		}
		for k := range cat {
			if !slices.Contains(keys, k) {
				t.Errorf("%s: translation for undeclared key %s", lang, k)
			}
		}
	}
}

func TestCatalogFieldsPopulated(t *testing.T) {
	ref := catalogs[Default]
	for lang, cat := range catalogs {
		for k, m := range cat {
			if strings.TrimSpace(m.Other) == "" {
				t.Errorf("%s: %s has empty Other", lang, k)
			}
			if hasOne := m.One != ""; hasOne != (ref[k].One != "") {
				t.Errorf("%s: %s plural form One present=%v, but %s present=%v", lang, k, hasOne, Default, !hasOne)
			}
			if m.One != "" && strings.TrimSpace(m.One) == "" {
				t.Errorf("%s: %s has blank One", lang, k)
			}
		}
	}
}

func TestCatalogPlaceholdersMatch(t *testing.T) {
	ref := catalogs[Default]
	for lang, cat := range catalogs {
		for k, m := range cat {
			want := placeholders(ref[k].Other)
			forms := map[string]string{"Other": m.Other}
			if m.One != "" {
				forms["One"] = m.One
			}
			for form, s := range forms {
				if got := placeholders(s); !slices.Equal(got, want) {
					t.Errorf("%s: %s.%s placeholders %v, want %v (as in %s.Other)", lang, k, form, got, want, Default)
				}
			}
		}
	}
}

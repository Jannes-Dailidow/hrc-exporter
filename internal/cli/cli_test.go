package cli

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"

	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
)

func newTestApp(l i18n.Lang) *app {
	return &app{v: viper.New(), log: log.New(nil), p: i18n.New(l)}
}

func TestSelectionFromFlagsAndEnv(t *testing.T) {
	t.Setenv("HRCE_TYPE", "entgelt")
	a := newTestApp(i18n.EN)
	root := a.rootCmd()
	if err := root.ParseFlags([]string{"--year", "2025", "-y", "2024"}); err != nil {
		t.Fatal(err)
	}
	if err := a.init(); err != nil {
		t.Fatal(err)
	}
	sel := a.selection()
	if !slices.Equal(sel.Years, []int{2025, 2024}) || sel.Type != "entgelt" {
		t.Errorf("selection = %+v", sel)
	}
}

func TestResolveLang(t *testing.T) {
	env := map[string]string{}
	cfg := ""
	src := func(args ...string) langSources {
		return langSources{
			args:   args,
			getenv: func(k string) string { return env[k] },
			config: func(string) string { return cfg },
			detect: func() i18n.Lang { return i18n.DE },
		}
	}
	check := func(name string, s langSources, want i18n.Lang, wantInvalid int) {
		t.Helper()
		got, invalid := resolveLang(s)
		if got != want || len(invalid) != wantInvalid {
			t.Errorf("%s: got %q (%d invalid), want %q (%d invalid)", name, got, len(invalid), want, wantInvalid)
		}
	}

	check("system", src(), i18n.DE, 0)
	cfg = "en"
	check("config beats system", src(), i18n.EN, 0)
	env["HRCE_LANG"] = "de_DE.UTF-8"
	check("env beats config", src(), i18n.DE, 0)
	check("flag beats env", src("list", "--lang", "en"), i18n.EN, 0)
	check("flag= form", src("--lang=en"), i18n.EN, 0)
	check("invalid flag skipped", src("--lang=fr"), i18n.DE, 1)
	check("after -- ignored", src("--", "--lang=en"), i18n.DE, 0)
}

func TestHelpIsTranslated(t *testing.T) {
	for lang, want := range map[i18n.Lang][]string{
		i18n.DE: {"Verwendung:", "Alle noch nicht vorhandenen Dokumente herunterladen", "Globale Optionen:", "Hilfe zu download"},
		i18n.EN: {"Usage:", "Download all documents not yet", "Global Flags:", "help for download"},
	} {
		root := newTestApp(lang).rootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"download", "--help"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, w := range want {
			if !strings.Contains(out.String(), w) {
				t.Errorf("%s help lacks %q:\n%s", lang, w, out.String())
			}
		}
	}
}

func TestBaseURLFromEnv(t *testing.T) {
	t.Setenv("HRCE_BASE_URL", "https://portal.example.com/")
	a := newTestApp(i18n.EN)
	_ = a.rootCmd()
	if err := a.init(); err != nil {
		t.Fatal(err)
	}
	if got := a.v.GetString(keyBaseURL); got != "https://portal.example.com/" {
		t.Errorf("base url = %q", got)
	}
}

func TestLastUsed(t *testing.T) {
	dir := t.TempDir()
	user := lastUsed{path: filepath.Join(dir, appName, lastUserFile)}
	url := lastUsed{path: filepath.Join(dir, appName, lastBaseURLFile)}
	if user.load() != "" {
		t.Fatal("expected empty cache")
	}
	if err := user.save("123456"); err != nil {
		t.Fatal(err)
	}
	if err := url.save("https://portal.example.com/"); err != nil {
		t.Fatal(err)
	}
	if user.load() != "123456" || url.load() != "https://portal.example.com/" {
		t.Errorf("load = %q / %q", user.load(), url.load())
	}
	if (lastUsed{}).save("x") != nil || (lastUsed{}).load() != "" {
		t.Error("zero lastUsed should be a no-op")
	}
}

func TestDefaultOutDirIsLocalized(t *testing.T) {
	for lang, want := range map[i18n.Lang]string{i18n.EN: "payslips", i18n.DE: "abrechnungen"} {
		a := newTestApp(lang)
		_ = a.rootCmd()
		if err := a.init(); err != nil {
			t.Fatal(err)
		}
		if got := a.v.GetString(keyOut); got != want {
			t.Errorf("%s: out = %q, want %q", lang, got, want)
		}
	}
}

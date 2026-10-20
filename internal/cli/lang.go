package cli

import (
	"strings"

	"github.com/spf13/viper"

	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
)

// langSources are the inputs for language resolution, injectable for tests.
type langSources struct {
	args   []string
	getenv func(string) string
	config func(explicitPath string) string // lang key of the config file
	detect func() i18n.Lang
}

type invalidLang struct{ value, source string }

// resolveLang picks the UI language before any command is built (help
// texts are translated, too): --lang flag > env > config file > system.
// Unsupported values are reported and skipped.
func resolveLang(s langSources) (i18n.Lang, []invalidLang) {
	envKey := envPrefix + "_" + strings.ToUpper(keyLang)
	cfgPath := scanFlag(s.args, keyConfig)
	if cfgPath == "" {
		cfgPath = s.getenv(envPrefix + "_" + strings.ToUpper(keyConfig))
	}
	candidates := []struct{ source, value string }{
		{"--" + keyLang, scanFlag(s.args, keyLang)},
		{envKey, s.getenv(envKey)},
		{"config", s.config(cfgPath)},
	}
	var invalid []invalidLang
	for _, c := range candidates {
		if c.value == "" {
			continue
		}
		if l, ok := i18n.Parse(c.value); ok {
			return l, invalid
		}
		invalid = append(invalid, invalidLang{c.value, c.source})
	}
	return s.detect(), invalid
}

// scanFlag finds --name=value or --name value in args without a full parse.
func scanFlag(args []string, name string) string {
	flag := "--" + name
	for i, a := range args {
		switch {
		case a == "--":
			return ""
		case strings.HasPrefix(a, flag+"="):
			return strings.TrimPrefix(a, flag+"=")
		case a == flag && i+1 < len(args):
			return args[i+1]
		}
	}
	return ""
}

// configLang reads only the lang key from the config file.
func configLang(explicitPath string) string {
	v := viper.New()
	configureConfigLookup(v, explicitPath)
	if err := v.ReadInConfig(); err != nil {
		return ""
	}
	return v.GetString(keyLang)
}

func supportedLangs() string {
	var out []string
	for _, l := range i18n.Supported() {
		out = append(out, string(l))
	}
	return strings.Join(out, ", ")
}

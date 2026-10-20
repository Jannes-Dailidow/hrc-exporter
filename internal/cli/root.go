// Package cli defines the commands and is the composition root: it reads
// configuration and wires the core packages to the ui implementations.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jannes-dailidow/hrc-exporter/internal/i18n"
	"github.com/jannes-dailidow/hrc-exporter/internal/ui"
)

const (
	appName   = "hrc-exporter"
	envPrefix = "HRCE"
)

// Config keys (flag names, config file keys and, upper-cased with prefix, env vars).
const (
	keyConfig   = "config"
	keyUser     = "user"
	keyPassword = "password" // env/config only, no flag
	keyBaseURL  = "base-url"
	keyOut      = "out"
	keyYear     = "year"
	keyType     = "type"
	keyLogLevel = "log-level"
	keyPlain    = "plain"
	keyLang     = "lang"
	keyForce    = "force"
	keyDryRun   = "dry-run"
)

type app struct {
	v       *viper.Viper
	log     *log.Logger
	p       *i18n.Printer
	version string
}

// Execute runs the CLI and returns the process exit code.
func Execute(ctx context.Context, version string) int {
	lang, invalid := resolveLang(langSources{
		args:   os.Args[1:],
		getenv: os.Getenv,
		config: configLang,
		detect: i18n.Detect,
	})
	a := &app{
		v:       viper.New(),
		log:     log.NewWithOptions(os.Stderr, log.Options{Prefix: appName}),
		p:       i18n.New(lang),
		version: version,
	}
	for _, l := range invalid {
		a.log.Warn(a.p.T(i18n.LogLangInvalid, l.value, l.source, supportedLangs()))
	}

	err := a.rootCmd().ExecuteContext(ctx)
	switch {
	case err == nil:
		return 0
	case ui.IsAbort(err):
		a.log.Warn(a.p.T(i18n.LogAborted))
		return 130
	default:
		a.log.Error(ui.ErrorText(a.p, err))
		return 1
	}
}

func (a *app) rootCmd() *cobra.Command {
	t := a.p.T
	root := &cobra.Command{
		Use:               appName,
		Short:             t(i18n.CmdRootShort),
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: func(*cobra.Command, []string) error { return a.init() },
	}
	pf := root.PersistentFlags()
	pf.String(keyConfig, "", t(i18n.FlagConfig, "<user config dir>/"+appName+"/config.yaml"))
	pf.StringP(keyUser, "u", "", t(i18n.FlagUser))
	pf.String(keyBaseURL, "", t(i18n.FlagBaseURL))
	pf.StringP(keyOut, "o", t(i18n.FlagOutDefault), t(i18n.FlagOut))
	pf.IntSliceP(keyYear, "y", nil, t(i18n.FlagYear))
	pf.StringP(keyType, "t", "", t(i18n.FlagType))
	pf.String(keyLogLevel, "info", t(i18n.FlagLogLevel))
	pf.Bool(keyPlain, false, t(i18n.FlagPlain))
	pf.String(keyLang, "", t(i18n.FlagLang, supportedLangs()))
	_ = a.v.BindPFlags(pf)

	root.AddCommand(a.downloadCmd(), a.listCmd(), a.reportCmd(), a.versionCmd())
	a.localizeCobra(root)
	return root
}

// localizeCobra translates cobra's built-in texts: usage template, help and
// completion commands and every command's --help flag.
func (a *app) localizeCobra(root *cobra.Command) {
	root.SetUsageTemplate(usageTemplate(a.p))
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	for _, c := range root.Commands() {
		switch c.Name() {
		case "help":
			c.Short, c.Long = a.p.T(i18n.CmdHelpShort), a.p.T(i18n.CmdHelpLong)
		case "completion":
			c.Short = a.p.T(i18n.CmdCompletionShort)
		}
	}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		c.InitDefaultHelpFlag()
		if f := c.Flags().Lookup("help"); f != nil {
			f.Usage = a.p.T(i18n.FlagHelp, c.Name())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}

// usageTemplate is cobra's classic usage template with translated labels.
func usageTemplate(p *i18n.Printer) string {
	return p.T(i18n.TplUsage) + `{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

` + p.T(i18n.TplAliases) + `
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

` + p.T(i18n.TplExamples) + `
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

` + p.T(i18n.TplAvailableCommands) + `{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

` + p.T(i18n.TplFlags) + `
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

` + p.T(i18n.TplGlobalFlags) + `
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

` + p.T(i18n.TplHelpTopics) + `{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

` + p.T(i18n.TplMoreInfo, "{{.CommandPath}}") + `{{end}}
`
}

func (a *app) init() error {
	v := a.v
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	explicit := v.GetString(keyConfig)
	configureConfigLookup(v, explicit)
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if explicit != "" || !errors.As(err, &notFound) {
			return fmt.Errorf("%s: %w", a.p.T(i18n.ErrReadConfig), err)
		}
	}

	lvl, err := log.ParseLevel(v.GetString(keyLogLevel))
	if err != nil {
		return err
	}
	a.log.SetLevel(lvl)
	if f := v.ConfigFileUsed(); f != "" {
		a.log.Debug("config loaded", "file", f)
	}
	return nil
}

// configureConfigLookup points v at the explicit file or the default
// <user config dir>/hrc-exporter/config.yaml.
func configureConfigLookup(v *viper.Viper, explicit string) {
	if explicit != "" {
		v.SetConfigFile(explicit)
		return
	}
	if dir, err := os.UserConfigDir(); err == nil {
		v.AddConfigPath(filepath.Join(dir, appName))
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}
}

func (a *app) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: a.p.T(i18n.CmdVersionShort),
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, _ []string) { fmt.Fprintln(cmd.OutOrStdout(), a.version) },
	}
}

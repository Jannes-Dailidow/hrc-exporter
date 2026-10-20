package i18n

// Key identifies a message. All keys are declared here; the tests parse this
// file to check that every catalog translates every key.
type Key string

// Formats.
const (
	FormatDate Key = "format.date" // Go time layout
)

// Prompts (huh).
const (
	PromptBaseURLTitle  Key = "prompt.base_url.title"
	PromptUserTitle     Key = "prompt.user.title"
	PromptPasswordTitle Key = "prompt.password.title"
	PromptCodeTitle     Key = "prompt.code.title"
	PromptCodeDesc      Key = "prompt.code.desc"
	ValidateBaseURL     Key = "validate.base_url"
	ValidateUser        Key = "validate.user"
	ValidatePassword    Key = "validate.password"
	KeyHelpNext         Key = "keyhelp.next"
	KeyHelpBack         Key = "keyhelp.back"
	KeyHelpSubmit       Key = "keyhelp.submit"
	KeyHelpComplete     Key = "keyhelp.complete"
	KeyHelpQuit         Key = "keyhelp.quit"
)

// Plain log messages.
const (
	LogLoginStarted   Key = "log.login_started"
	LogCodeRequired   Key = "log.code_required"
	LogCodeResent     Key = "log.code_resent"
	LogCodeRejected   Key = "log.code_rejected" // %d attempt, %d max
	LogLoggedIn       Key = "log.logged_in"
	LogYearScanned    Key = "log.year_scanned"
	LogScanFinished   Key = "log.scan_finished"
	LogDownloading    Key = "log.downloading"
	LogDownloaded     Key = "log.downloaded"
	LogDownloadFailed Key = "log.download_failed"
	LogAborted        Key = "log.aborted"
	LogLangInvalid    Key = "log.lang_invalid" // %q value, %s source, %s supported
	LogReportWarning  Key = "log.report_warning"
	LogReportWritten  Key = "log.report_written" // %d payslips, %s first month, %s last month
)

// Counts (plural).
const (
	CountDocuments Key = "count.documents" // %d
	CountNew       Key = "count.new"       // %d
)

// Interactive progress.
const (
	ProgressScanning   Key = "progress.scanning"
	ProgressCancelling Key = "progress.cancelling"
	ProgressUpToDate   Key = "progress.up_to_date"
)

// Tables and summary.
const (
	TableNoDocuments Key = "table.no_documents"
	TableYear        Key = "table.year"
	TableDate        Key = "table.date"
	TableType        Key = "table.type"
	TableStatus      Key = "table.status"
	TableFile        Key = "table.file"
	StatusNew        Key = "status.new"
	StatusDownloaded Key = "status.downloaded"
	SummaryDryRun    Key = "summary.dry_run"    // label
	SummaryWouldLoad Key = "summary.would_load" // %d pending, %d total
	SummaryDone      Key = "summary.done"       // label
	SummaryResult    Key = "summary.result"     // %s downloaded (styled), %s bytes, %d present
	SummaryFailed    Key = "summary.failed"     // %d
)

// Errors.
const (
	ErrInvalidCredentials Key = "err.invalid_credentials"
	ErrInvalidCode        Key = "err.invalid_code"
	ErrSessionExpired     Key = "err.session_expired"
	ErrNotLoggedIn        Key = "err.not_logged_in"
	ErrUnsupportedMeta    Key = "err.unsupported_meta"
	ErrDownloadsFailed    Key = "err.downloads_failed" // %d, plural
	ErrReadConfig         Key = "err.read_config"
	ErrNoPayslips         Key = "err.no_payslips"
	ErrPdftotextNotFound  Key = "err.pdftotext_not_found"
)

// Commands and flags.
const (
	CmdRootShort       Key = "cmd.root.short"
	CmdDownloadShort   Key = "cmd.download.short"
	CmdListShort       Key = "cmd.list.short"
	CmdReportShort     Key = "cmd.report.short"
	CmdVersionShort    Key = "cmd.version.short"
	CmdHelpShort       Key = "cmd.help.short"
	CmdHelpLong        Key = "cmd.help.long"
	CmdCompletionShort Key = "cmd.completion.short"

	FlagConfig     Key = "flag.config" // %s default path
	FlagUser       Key = "flag.user"
	FlagBaseURL    Key = "flag.base_url"
	FlagOut        Key = "flag.out"
	FlagOutDefault Key = "flag.out.default" // default output directory name
	FlagYear       Key = "flag.year"
	FlagType       Key = "flag.type"
	FlagLogLevel   Key = "flag.log_level"
	FlagPlain      Key = "flag.plain"
	FlagLang       Key = "flag.lang" // %s supported languages
	FlagForce      Key = "flag.force"
	FlagDryRun     Key = "flag.dry_run"
	FlagHelp       Key = "flag.help" // %s command name
	FlagHTML       Key = "flag.html" // %s default file name
	FlagJSON       Key = "flag.json"
	FlagPdftotext  Key = "flag.pdftotext"
	FlagOpen       Key = "flag.open"
)

// Usage template labels (cobra).
const (
	TplUsage             Key = "tpl.usage"
	TplAliases           Key = "tpl.aliases"
	TplExamples          Key = "tpl.examples"
	TplAvailableCommands Key = "tpl.available_commands"
	TplFlags             Key = "tpl.flags"
	TplGlobalFlags       Key = "tpl.global_flags"
	TplHelpTopics        Key = "tpl.help_topics"
	TplMoreInfo          Key = "tpl.more_info" // %s command path
)

# hrc-exporter

> This project was mostly developed and generated with [Claude Code](https://claude.com/claude-code).

Downloads payroll documents (Entgeltabrechnung, Lohnsteuerbescheinigung, …) from an HRCommunicator portal
and builds a salary report from them. Downloads are recorded in `<out>/meta.json` and skipped on later runs.

## Install

Requires Go 1.27.

```sh
go install github.com/jannes-dailidow/hrc-exporter/cmd/hrc-exporter@latest
```

From source, with [just](https://github.com/casey/just): `just build` (→ `bin/`), `just install`, `just check` (fmt, vet, test).

## Usage

```sh
hrc-exporter download                     # all years -> <out>/<year>/<date>_<type>_<id>.pdf
hrc-exporter download -y 2025 --dry-run
hrc-exporter download -t lohnsteuer --force
hrc-exporter list -y 2026
hrc-exporter report --open                # <out>/report.html
```

`download` and `list` prompt for the portal URL, personnel number, password and the login code sent by e-mail.
Leave the code empty to get a new one. The portal URL and personnel number of the last successful login are
pre-filled next time.

`report` works offline. It reads the Entgeltabrechnungen listed in `meta.json` and writes one HTML page (in German)
with:

- base salary per month,
- tariff increases and how much of each was offset against the übertarifliche Zulage,
- yearly gross,
- one-off payments,
- payslip notices.

It needs `pdftotext` from poppler or xpdf. Git for Windows ships it, and the tool finds it there.

## Configuration

Precedence: flag > environment variable > `<user config dir>/hrc-exporter/config.yaml` (keys = flag names).

| Flag | Env | Default |
| --- | --- | --- |
| `--config` | `HRCE_CONFIG` | `<user config dir>/hrc-exporter/config.yaml` |
| `--base-url` | `HRCE_BASE_URL` | prompt |
| `-u, --user` | `HRCE_USER` | prompt |
| – | `HRCE_PASSWORD` | prompt |
| `-o, --out` | `HRCE_OUT` | `payslips` (en) / `abrechnungen` (de) |
| `-y, --year` | – | all |
| `-t, --type` | `HRCE_TYPE` | all |
| `--lang` | `HRCE_LANG` | system language (`en`, `de`) |
| `--plain`, `--log-level` | `HRCE_PLAIN`, `HRCE_LOG_LEVEL` | `false`, `info` |
| `--force`, `--dry-run` (download) | – | `false` |
| `--html` (report) | `HRCE_HTML` | `<out>/report.html` |
| `--json` (report) | `HRCE_JSON` | off; writes the chart data |
| `--pdftotext` (report) | `HRCE_PDFTOTEXT` | `PATH`, then Git for Windows |
| `--open` (report) | `HRCE_OPEN` | `false` |

Design and portal protocol: [ARCHITECTURE.md](ARCHITECTURE.md).

## License

[MIT](LICENSE)

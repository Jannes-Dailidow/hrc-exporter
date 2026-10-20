package report

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Line is one wage-type row ("Lohnart") of a payslip page.
type Line struct {
	Section string // Basisbezüge, Zeitbezüge, ...
	Name    string
	Flags   string // "Kennz" column, a combination of E/L/S/G; empty for Zusatz-only rows such as AVmG
	Amount  Cents  // "Betrag" column, the last number on the row
}

// InGross reports whether the row counts towards Gesamtbrutto (flag G).
func (l Line) InGross() bool { return strings.Contains(l.Flags, "G") }

// Page is the block of a payslip that belongs to one period. The first page
// covers the payslip's own month; further pages are retroactive
// recalculations ("Rückrechnung") whose amounts are differences.
type Page struct {
	Period   Period
	Group    string // "Tarifgr./-stufe" as printed on the page, e.g. "5/2"
	Lines    []Line
	Gross    Cents // Gesamtbrutto (EBeschV); a difference on retro pages
	HasGross bool
}

// Payslip is one parsed Entgeltabrechnung.
type Payslip struct {
	File      string
	Period    Period
	Printed   time.Time // "Datum:" header, orders corrected payslips of the same month
	Group     string
	Gross     Cents
	Net       Cents   // Gesetzl. Netto
	Payout    Cents   // Überweisung to the salary account
	Carryover Cents   // Nachverrechnung aus Vormonat
	Pages     []*Page // [0] is the own month, the rest are retro periods (one per period)
	Messages  []string
	Warnings  []string
}

var (
	reAmount  = regexp.MustCompile(`^\d{1,3}(\.\d{3})*,\d{2}$`)
	reNumeric = regexp.MustCompile(`^(\d{1,3}(\.\d{3})*,\d{1,2}|\d+|−|-)$`)
	reFlags   = regexp.MustCompile(`^[ELSG]{1,4}$`)
	rePeriod  = regexp.MustCompile(`für den Zeitraum \d\d\.(\d\d)\. bis \d\d\.\d\d\.(\d{4}):`)
	reMonth   = regexp.MustCompile(`^Im (\S+) (\d{4})$`)
	rePrinted = regexp.MustCompile(`^Datum: (\d\d\.\d\d\.\d{4})$`)
	reGroup   = regexp.MustCompile(`^Tarifgr\./.stufe: (\S+)$`)
)

var monthNames = map[string]time.Month{
	"Januar": time.January, "Februar": time.February, "März": time.March, "April": time.April,
	"Mai": time.May, "Juni": time.June, "Juli": time.July, "August": time.August,
	"September": time.September, "Oktober": time.October, "November": time.November, "Dezember": time.December,
}

// Section headings. Rows under a wage section are wage types; the other
// sections hold derived figures of which only a few are read.
const (
	sectionGross      = "Bruttoentgelt"
	sectionNet        = "Netto"
	sectionDeductions = "Be− und Abzüge"
	sectionPayments   = "Zahlungen"
	sectionMessage    = "Mitteilung"
)

var (
	wageSections  = map[string]bool{"Basisbezüge": true, "Zeitbezüge": true, "Einmalbezüge": true, "Sonstige Bezüge": true}
	otherSections = map[string]bool{sectionGross: true, "Gesetzliche Abzüge": true, sectionNet: true, sectionDeductions: true, sectionPayments: true, sectionMessage: true}
)

// ParsePayslip parses the `pdftotext -raw` output of one Entgeltabrechnung.
// Unrecognised rows and failed plausibility checks end up in Warnings.
func ParsePayslip(file, text string) (*Payslip, error) {
	p := &Payslip{File: file}
	pages := map[Period]*Page{}
	seenMessages := map[string]bool{}

	for raw := range strings.SplitSeq(text, "\f") {
		lines := strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
		}
		period, group := p.parseHeader(lines)
		if period.IsZero() {
			continue
		}
		pg := pages[period]
		if pg == nil {
			pg = &Page{Period: period, Group: group}
			pages[period] = pg
			p.Pages = append(p.Pages, pg)
		}
		p.parseBody(pg, pg == p.Pages[0], lines, seenMessages)
	}

	if p.Period.IsZero() || len(p.Pages) == 0 {
		return nil, fmt.Errorf("%s: not an Entgeltabrechnung", file)
	}
	own := p.Pages[0]
	if own.Period != p.Period {
		p.warnf("first page covers %s, expected %s", own.Period, p.Period)
	}
	p.Group, p.Gross = own.Group, own.Gross
	p.checkGross(own)
	return p, nil
}

// parseHeader reads the period and pay group of a page and, once, the
// payslip's month and print date.
func (p *Payslip) parseHeader(lines []string) (period Period, group string) {
	for _, ln := range lines {
		if m := rePeriod.FindStringSubmatch(ln); m != nil {
			period = newPeriod(m[2], m[1])
		} else if m := reGroup.FindStringSubmatch(ln); m != nil {
			group = m[1]
		} else if m := reMonth.FindStringSubmatch(ln); m != nil && p.Period.IsZero() {
			if month, ok := monthNames[m[1]]; ok {
				year, _ := strconv.Atoi(m[2])
				p.Period = Period{Year: year, Month: month}
			}
		} else if m := rePrinted.FindStringSubmatch(ln); m != nil && p.Printed.IsZero() {
			p.Printed, _ = time.Parse("02.01.2006", m[1])
		}
	}
	return period, group
}

// parseBody collects the wage rows of a page and, on the own page, the
// totals.
func (p *Payslip) parseBody(pg *Page, own bool, lines []string, seenMessages map[string]bool) {
	section := ""
	for _, ln := range lines {
		switch {
		case ln == "":
			continue
		case wageSections[ln] || otherSections[ln]:
			section = ln
			continue
		case strings.HasPrefix(ln, "Kennzeichen:"), strings.HasPrefix(ln, "Bescheinigung nach"):
			section = ""
			continue
		}
		tokens := strings.Fields(ln)
		switch {
		case wageSections[section]:
			if l, ok := parseLine(section, ln); ok {
				pg.Lines = append(pg.Lines, l)
			} else {
				p.warnf("%s: unrecognised row %q", pg.Period, ln)
			}
		case section == sectionGross && strings.HasPrefix(ln, "Gesamtbrutto (EBeschV)"):
			if v, ok := firstAmount(tokens[2:]); ok {
				pg.Gross, pg.HasGross = pg.Gross+v, true
			}
		case section == sectionNet && own && strings.HasPrefix(ln, "Gesetzl. Netto"):
			p.Net, _ = firstAmount(tokens)
		case section == sectionDeductions && own && strings.HasPrefix(ln, "Nachverrechnung aus Vorm."):
			p.Carryover, _ = lastAmount(tokens)
		case section == sectionPayments && strings.HasPrefix(ln, "Überweisung ()"):
			p.Payout, _ = firstAmount(tokens)
		case section == sectionMessage && !seenMessages[ln]:
			seenMessages[ln] = true
			p.Messages = append(p.Messages, ln)
		}
	}
}

// checkGross verifies that the G-flagged rows add up to Gesamtbrutto. A net
// payment (e.g. "Jubiläum LJ netto") sits in the Zusatz column but is grossed
// up into Gesamtbrutto, so a single such row may close the gap.
func (p *Payslip) checkGross(own *Page) {
	var sum Cents
	for _, l := range own.Lines {
		if l.InGross() {
			sum += l.Amount
		}
	}
	if sum == own.Gross {
		return
	}
	for _, l := range own.Lines {
		if !l.InGross() && classify(l.Name) != componentIgnored && sum+l.Amount == own.Gross {
			return
		}
	}
	p.warnf("wage types add up to %s, Gesamtbrutto is %s", formatEuro(sum, 2), formatEuro(own.Gross, 2))
}

func (p *Payslip) warnf(format string, args ...any) {
	p.Warnings = append(p.Warnings, fmt.Sprintf(format, args...))
}

func newPeriod(year, month string) Period {
	y, _ := strconv.Atoi(year)
	m, _ := strconv.Atoi(month)
	return Period{Year: y, Month: time.Month(m)}
}

// parseLine splits a wage row such as "Übertarifliche Zulage LSG 07 314,99 −"
// into name, flags and amount. Rows without a trailing amount are rejected.
func parseLine(section, s string) (Line, bool) {
	tokens := strings.Fields(s)
	k := len(tokens)
	for k > 0 && reNumeric.MatchString(tokens[k-1]) {
		k--
	}
	if k == len(tokens) || k == 0 {
		return Line{}, false
	}
	amount, ok := lastAmount(tokens[k:])
	if !ok {
		return Line{}, false
	}
	l := Line{Section: section, Amount: amount}
	if k > 1 && reFlags.MatchString(tokens[k-1]) {
		k--
		l.Flags = tokens[k]
	}
	l.Name = strings.Join(tokens[:k], " ")
	return l, true
}

// firstAmount returns the first money amount in tokens.
func firstAmount(tokens []string) (Cents, bool) {
	for i := range tokens {
		if reAmount.MatchString(tokens[i]) {
			return signedAmount(tokens, i), true
		}
	}
	return 0, false
}

// lastAmount returns the last money amount in tokens.
func lastAmount(tokens []string) (Cents, bool) {
	for i := len(tokens) - 1; i >= 0; i-- {
		if reAmount.MatchString(tokens[i]) {
			return signedAmount(tokens, i), true
		}
	}
	return 0, false
}

// signedAmount parses tokens[i], negated when the next token is a minus sign
// (payslips print negative amounts as "314,99 −").
func signedAmount(tokens []string, i int) Cents {
	v := parseAmount(tokens[i])
	if i+1 < len(tokens) && (tokens[i+1] == "−" || tokens[i+1] == "-") {
		return -v
	}
	return v
}

package report

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ErrNoPayslips is returned when there is nothing to analyse.
var ErrNoPayslips = errors.New("no payslips found")

// Item is a named amount.
type Item struct {
	Name   string
	Amount Cents
}

// Month is the effective view of one payroll month: its own payslip plus
// every later retroactive correction for it.
type Month struct {
	Pay            // base salary, including later corrections
	Period         Period
	Group          string // Tarifgruppe/-stufe, e.g. "5/2" or "AZ/2"
	HolidayPay     Cents  // Urlaubs- und Weihnachtsgeld
	Gross          Cents  // Gesamtbrutto as printed
	GrossEffective Cents  // Gesamtbrutto including later corrections
	Net            Cents
	Payout         Cents
	Carryover      Cents
	Extras         []Item // wage types other than the base salary on the own page
	File           string
	Messages       []string

	// Derived by Build.
	Delta        Cents    // base salary change against the previous month
	NewYear      bool     // first month of a calendar year in the report
	NewExtras    []Item   // Extras not paid with the same amount in the previous month
	RetroPeriods []Period // periods this month's payslip recalculated, newest first
	Corrections  []string // later payslips that corrected this month
	Notes        []string // human-readable summary of the above
}

// salaried reports whether m is a full month on Tarifentgelt, i.e. neither
// training nor the partial month of the switch.
func (m *Month) salaried() bool { return m.Tariff > 0 && m.Trainee == 0 }

// EventKind classifies a change of the base salary.
type EventKind int

const (
	EventGroup       EventKind = iota + 1 // regrouping (Tarifgruppe)
	EventStep                             // step within the group (Stufe)
	EventTariff                           // general tariff increase
	EventAboveTariff                      // change of the ÜT only
)

// Event is a change of the base salary between two consecutive salaried months.
type Event struct {
	Month            *Month
	Kind             EventKind
	Title            string
	DeltaTariff      Cents
	DeltaAboveTariff Cents
	TariffPercent    float64 // relative tariff change, set for EventTariff
	Offset           Cents   // part of the tariff increase offset against the ÜT
}

// Net is the change that actually reached the employee.
func (e Event) Net() Cents { return e.DeltaTariff + e.DeltaAboveTariff }

// IsRegrouping reports whether the event is a change of group or step.
func (e Event) IsRegrouping() bool { return e.Kind == EventGroup || e.Kind == EventStep }

// IsTariffIncrease reports whether the event is a general tariff increase.
func (e Event) IsTariffIncrease() bool { return e.Kind == EventTariff }

// Year aggregates the effective gross pay of a calendar year.
type Year struct {
	Year       int
	Months     int
	Base       Cents
	HolidayPay Cents
	Other      Cents
	Total      Cents
}

// TariffTable lists the Tarifentgelt seen per group/step, one column per
// tariff period.
type TariffTable struct {
	Columns []TariffColumn
	Groups  []string
}

// TariffColumn is one tariff period of a TariffTable.
type TariffColumn struct {
	Label  string
	Values map[string]Cents // by group
}

// Payment is a one-off or changed wage type.
type Payment struct {
	Period Period
	Item
}

// Notice is the "Mitteilung" text printed on a payslip.
type Notice struct {
	Period Period
	Text   string
}

// Summary holds the key figures of the salaried period.
type Summary struct {
	Start, Current         *Month // first salaried month, last month
	Gain                   Cents  // base salary change from Start to Current
	GainPercent            float64
	TariffRise             Cents // Tarifentgelt change from Start to Current
	Offset                 Cents // tariff increases offset against the ÜT
	AboveTariffRaises      Cents // sum of ÜT increases
	AboveTariffPeak        Cents
	PeakFrom, PeakTo       Period
	OffsetEvents           []Event
	FullEvents             []Event // tariff increases passed on in full
	AboveTariffRaiseEvents []Event
}

// Report is the analysis of all payslips.
type Report struct {
	Months   []*Month // sorted, one per period
	Events   []Event
	Years    []Year
	Tariff   TariffTable
	OneOffs  []Payment
	Notices  []Notice
	Summary  *Summary // nil without salaried months
	Warnings []string
}

// First returns the earliest month.
func (r *Report) First() *Month { return r.Months[0] }

// Last returns the latest month.
func (r *Report) Last() *Month { return r.Months[len(r.Months)-1] }

// Build turns parsed payslips into the report model. If several payslips
// cover the same month, the one printed last wins.
func Build(slips []*Payslip) (*Report, error) {
	if len(slips) == 0 {
		return nil, ErrNoPayslips
	}
	slips = slices.Clone(slips)
	slices.SortStableFunc(slips, func(a, b *Payslip) int {
		if c := a.Period.Compare(b.Period); c != 0 {
			return c
		}
		return a.Printed.Compare(b.Printed)
	})

	r := &Report{}
	byPeriod := map[Period]*Month{}
	for _, p := range slips {
		for _, w := range p.Warnings {
			r.warnf("%s: %s", p.File, w)
		}
		if old, dup := byPeriod[p.Period]; dup {
			r.warnf("%s: second payslip for %s, replaces %s", p.File, p.Period, old.File)
		}
		byPeriod[p.Period] = newMonth(p)
	}
	r.Months = slices.SortedFunc(maps.Values(byPeriod), func(a, b *Month) int { return a.Period.Compare(b.Period) })

	r.applyCorrections(slips, byPeriod)
	r.linkMonths()
	r.buildEvents()
	r.buildNotes()
	r.buildYears()
	r.buildTariffTable()
	r.buildSummary()
	r.collectHighlights()
	return r, nil
}

func newMonth(p *Payslip) *Month {
	m := &Month{
		Period: p.Period, Group: p.Group, Gross: p.Gross, GrossEffective: p.Gross,
		Net: p.Net, Payout: p.Payout, Carryover: p.Carryover, File: p.File, Messages: p.Messages,
	}
	for _, l := range p.Pages[0].Lines {
		c := classify(l.Name)
		if m.add(c, l.Amount) || c == componentIgnored {
			continue
		}
		if c == componentHolidayPay {
			m.HolidayPay += l.Amount
		}
		m.Extras = append(m.Extras, l.item())
	}
	return m
}

func (l Line) item() Item { return Item{Name: l.Name, Amount: l.Amount} }

// applyCorrections books the retro pages of each payslip onto the months they
// recalculate. Amounts on retro pages are differences.
func (r *Report) applyCorrections(slips []*Payslip, byPeriod map[Period]*Month) {
	for _, p := range slips {
		paidIn := byPeriod[p.Period]
		for _, pg := range p.Pages[1:] {
			var diff Pay
			var other []string
			for _, l := range pg.Lines {
				if c := classify(l.Name); !diff.add(c, l.Amount) && c != componentIgnored {
					other = append(other, l.Name)
				}
			}
			if diff.IsZero() && pg.Gross == 0 {
				continue // e.g. a recalculation of travel allowance only
			}
			paidIn.RetroPeriods = append(paidIn.RetroPeriods, pg.Period)
			target := byPeriod[pg.Period]
			if target == nil {
				r.warnf("%s: recalculation for %s, but no payslip for that month", p.File, pg.Period)
				continue
			}
			target.Tariff += diff.Tariff
			target.AboveTariff += diff.AboveTariff
			target.Trainee += diff.Trainee
			target.GrossEffective += pg.Gross
			desc := "korrigiert in " + paidIn.Period.Label()
			if len(other) > 0 {
				desc += " (" + strings.Join(unique(other), ", ") + ")"
			}
			target.Corrections = append(target.Corrections, desc)
		}
	}
}

// linkMonths derives the fields that compare a month with its predecessor.
func (r *Report) linkMonths() {
	for i, m := range r.Months {
		if i == 0 {
			m.NewYear, m.NewExtras = true, m.Extras
			continue
		}
		prev := r.Months[i-1]
		m.NewYear = m.Period.Year != prev.Period.Year
		m.Delta = m.Base() - prev.Base()
		for _, x := range m.Extras {
			if !slices.Contains(prev.Extras, x) {
				m.NewExtras = append(m.NewExtras, x)
			}
		}
	}
}

func (r *Report) buildEvents() {
	for i := 1; i < len(r.Months); i++ {
		a, b := r.Months[i-1], r.Months[i]
		if !a.salaried() || !b.salaried() {
			continue
		}
		dT, dU := b.Tariff-a.Tariff, b.AboveTariff-a.AboveTariff
		if dT == 0 && dU == 0 {
			continue
		}
		e := Event{Month: b, DeltaTariff: dT, DeltaAboveTariff: dU}
		groupA, stepA, _ := strings.Cut(a.Group, "/")
		groupB, stepB, _ := strings.Cut(b.Group, "/")
		switch {
		case groupA != groupB:
			e.Kind, e.Title = EventGroup, fmt.Sprintf("Gruppe %s→%s", groupA, groupB)
		case stepA != stepB:
			e.Kind, e.Title = EventStep, fmt.Sprintf("Stufe %s→%s", stepA, stepB)
		case dT != 0:
			e.Kind = EventTariff
			e.TariffPercent = float64(dT) / float64(a.Tariff) * 100
			e.Title = "Tarif " + formatPercent(e.TariffPercent)
			if dU > 0 {
				e.Title += " + ÜT"
			}
		default:
			e.Kind, e.Title = EventAboveTariff, "ÜT"
		}
		if dT > 0 && dU < 0 {
			e.Offset = min(dT, -dU)
		}
		r.Events = append(r.Events, e)
	}
}

func (r *Report) buildNotes() {
	events := make(map[*Month]Event, len(r.Events))
	for _, e := range r.Events {
		events[e.Month] = e
	}
	for i, m := range r.Months {
		if i > 0 {
			prev := r.Months[i-1]
			if prev.Trainee > 0 && prev.Tariff == 0 && m.Tariff > 0 {
				m.note("Übernahme")
			}
			if m.Tariff == 0 && m.Trainee > 0 && prev.Trainee > 0 && m.Trainee != prev.Trainee {
				m.note("Ausbildungsvergütung " + formatSigned(m.Trainee-prev.Trainee))
			}
		}
		if e, ok := events[m]; ok {
			n := fmt.Sprintf("%s: Tarif %s, ÜT %s → %s", e.Title, formatSigned(e.DeltaTariff), formatSigned(e.DeltaAboveTariff), formatSigned(e.Net()))
			switch {
			case e.Offset > 0 && e.Offset == e.DeltaTariff:
				n += " (ganz verrechnet)"
			case e.Offset > 0:
				n += " (" + formatEuro(e.Offset, 0) + " verrechnet)"
			}
			m.note(n)
		}
		for _, x := range m.NewExtras {
			m.note(x.Name + " " + formatEuro(x.Amount, 2))
		}
		if n := len(m.RetroPeriods); n > 0 {
			periods := make([]string, n)
			for i, p := range m.RetroPeriods {
				periods[i] = p.Short()
			}
			text := strings.Join(periods, ", ")
			if n > 3 {
				text = periods[n-1] + " bis " + periods[0]
			}
			m.note("Rückrechnung für " + text)
		}
		if m.Carryover != 0 {
			m.note("Nachverrechnung " + formatEuro(m.Carryover, 2) + " netto")
		}
		m.Notes = append(m.Notes, m.Corrections...)
		if len(m.Messages) > 0 {
			m.note("Mitteilung auf der Abrechnung")
		}
	}
}

func (m *Month) note(s string) { m.Notes = append(m.Notes, s) }

func (r *Report) buildYears() {
	for _, m := range r.Months {
		if len(r.Years) == 0 || r.Years[len(r.Years)-1].Year != m.Period.Year {
			r.Years = append(r.Years, Year{Year: m.Period.Year})
		}
		y := &r.Years[len(r.Years)-1]
		y.Months++
		y.Base += m.Base()
		y.HolidayPay += m.HolidayPay
		y.Total += m.GrossEffective
	}
	for i := range r.Years {
		y := &r.Years[i]
		y.Other = max(0, y.Total-y.Base-y.HolidayPay)
	}
}

// buildTariffTable records the Tarifentgelt per group/step. A new column
// starts with every general tariff increase.
func (r *Report) buildTariffTable() {
	increases := map[*Month]Event{}
	for _, e := range r.Events {
		if e.IsTariffIncrease() {
			increases[e.Month] = e
		}
	}
	col := TariffColumn{Values: map[string]Cents{}}
	for _, m := range r.Months {
		if !m.salaried() {
			continue
		}
		if e, ok := increases[m]; ok {
			r.Tariff.Columns = append(r.Tariff.Columns, col)
			col = TariffColumn{Label: fmt.Sprintf("ab %s (%s)", m.Period.Label(), formatPercent(e.TariffPercent)), Values: map[string]Cents{}}
		} else if col.Label == "" {
			col.Label = "ab " + m.Period.Label()
		}
		if !slices.Contains(r.Tariff.Groups, m.Group) {
			r.Tariff.Groups = append(r.Tariff.Groups, m.Group)
		}
		col.Values[m.Group] = m.Tariff
	}
	r.Tariff.Columns = append(r.Tariff.Columns, col)
	slices.Sort(r.Tariff.Groups)
}

func (r *Report) buildSummary() {
	i := slices.IndexFunc(r.Months, (*Month).salaried)
	if i < 0 {
		return
	}
	s := &Summary{Start: r.Months[i], Current: r.Last()}
	s.Gain = s.Current.Base() - s.Start.Base()
	s.GainPercent = float64(s.Gain) / float64(s.Start.Base()) * 100
	s.TariffRise = s.Current.Tariff - s.Start.Tariff
	for _, e := range r.Events {
		s.Offset += e.Offset
		if e.DeltaAboveTariff > 0 {
			s.AboveTariffRaises += e.DeltaAboveTariff
			s.AboveTariffRaiseEvents = append(s.AboveTariffRaiseEvents, e)
		}
		if e.Offset > 0 {
			s.OffsetEvents = append(s.OffsetEvents, e)
		}
		if e.DeltaTariff > 0 && e.DeltaAboveTariff >= 0 {
			s.FullEvents = append(s.FullEvents, e)
		}
	}
	for _, m := range r.Months {
		if !m.salaried() {
			continue
		}
		switch {
		case m.AboveTariff > s.AboveTariffPeak:
			s.AboveTariffPeak, s.PeakFrom, s.PeakTo = m.AboveTariff, m.Period, m.Period
		case m.AboveTariff == s.AboveTariffPeak && !s.PeakTo.IsZero():
			s.PeakTo = m.Period
		}
	}
	r.Summary = s
}

// collectHighlights lists one-off payments and payslip notices.
func (r *Report) collectHighlights() {
	for _, m := range r.Months {
		for _, x := range m.NewExtras {
			r.OneOffs = append(r.OneOffs, Payment{Period: m.Period, Item: x})
		}
		if len(m.Messages) > 0 {
			r.Notices = append(r.Notices, Notice{Period: m.Period, Text: strings.Join(m.Messages, " ")})
		}
	}
}

func (r *Report) warnf(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// unique returns s without duplicates, keeping the first occurrence.
func unique(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := s[:0:0]
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

package report

import "strings"

// component is the part of the salary a wage type contributes to.
type component int

const (
	componentOther       component = iota // one-off and irregular payments
	componentTariff                       // Tarifentgelt
	componentAboveTariff                  // übertarifliche Zulage (ÜT)
	componentTrainee                      // Ausbildungsvergütung
	componentHolidayPay                   // Urlaubs- und Weihnachtsgeld
	componentIgnored                      // Zusatz rows paid elsewhere (AVmG)
)

// wageTypes maps wage-type name prefixes to components, checked in order.
// Extend it if the payroll renames a wage type.
var wageTypes = []struct {
	component component
	prefixes  []string
}{
	{componentTariff, []string{"Tarifentgelt", "Tarifgehalt"}},
	{componentAboveTariff, []string{"Übertarifliche Zulage"}},
	{componentTrainee, []string{"Ausbildungsvergütung"}},
	{componentHolidayPay, []string{"Urlaubsgeld", "Weihnachtsgeld"}},
	{componentIgnored, []string{"AVmG"}},
}

func classify(name string) component {
	for _, w := range wageTypes {
		for _, p := range w.prefixes {
			if strings.HasPrefix(name, p) {
				return w.component
			}
		}
	}
	return componentOther
}

// Pay holds the base-salary components of a month.
type Pay struct {
	Tariff, AboveTariff, Trainee Cents
}

// add books a wage row onto p and reports whether it was a base component.
func (p *Pay) add(c component, amount Cents) bool {
	switch c {
	case componentTariff:
		p.Tariff += amount
	case componentAboveTariff:
		p.AboveTariff += amount
	case componentTrainee:
		p.Trainee += amount
	default:
		return false
	}
	return true
}

// Base is the regular monthly salary: Tarifentgelt + ÜT + Ausbildungsvergütung.
func (p Pay) Base() Cents { return p.Tariff + p.AboveTariff + p.Trainee }

// IsZero reports whether all components are zero.
func (p Pay) IsZero() bool { return p == Pay{} }

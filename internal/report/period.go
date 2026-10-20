package report

import (
	"cmp"
	"fmt"
	"time"
)

// Period is a payroll month.
type Period struct {
	Year  int
	Month time.Month
}

// String returns "2006-01".
func (p Period) String() string { return fmt.Sprintf("%04d-%02d", p.Year, int(p.Month)) }

// Label returns "01/2006".
func (p Period) Label() string { return fmt.Sprintf("%02d/%04d", int(p.Month), p.Year) }

// Short returns "01/06".
func (p Period) Short() string { return fmt.Sprintf("%02d/%02d", int(p.Month), p.Year%100) }

// IsZero reports whether p is unset.
func (p Period) IsZero() bool { return p == Period{} }

// Compare returns -1, 0 or +1 depending on whether p is before, equal to or after q.
func (p Period) Compare(q Period) int {
	if c := cmp.Compare(p.Year, q.Year); c != 0 {
		return c
	}
	return cmp.Compare(p.Month, q.Month)
}

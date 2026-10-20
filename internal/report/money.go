package report

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Cents is an amount of money in euro cents. Integer arithmetic keeps sums and
// comparisons exact.
type Cents int64

// Euros returns the amount in euros.
func (c Cents) Euros() float64 { return float64(c) / 100 }

// parseAmount parses a de-DE amount such as "1.234,56". The caller has
// validated the format.
func parseAmount(s string) Cents {
	whole, frac, _ := strings.Cut(strings.ReplaceAll(s, ".", ""), ",")
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt((frac + "00")[:2], 10, 64)
	return Cents(w*100 + f)
}

// formatEuro formats c in de-DE notation: "4.425,00" with two decimals, or
// "4.425" (rounded half up) with zero.
func formatEuro(c Cents, decimals int) string {
	neg := c < 0
	if neg {
		c = -c
	}
	whole, frac := int64(c)/100, int64(c)%100
	if decimals == 0 && frac >= 50 {
		whole++
	}
	digits := strconv.FormatInt(whole, 10)
	var b strings.Builder
	if neg {
		b.WriteString("−")
	}
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	if decimals == 2 {
		fmt.Fprintf(&b, ",%02d", frac)
	}
	return b.String()
}

// formatEuroAuto drops ",00" for whole euros.
func formatEuroAuto(c Cents) string {
	if c%100 == 0 {
		return formatEuro(c, 0)
	}
	return formatEuro(c, 2)
}

// formatSigned is formatEuroAuto with an explicit sign ("+12", "−3,50", "±0").
func formatSigned(c Cents) string {
	switch {
	case c > 0:
		return "+" + formatEuroAuto(c)
	case c < 0:
		return formatEuroAuto(c)
	}
	return "±0"
}

// formatPercent formats p with one decimal and a sign: "+3,1 %".
func formatPercent(p float64) string {
	s := strings.Replace(strconv.FormatFloat(math.Abs(p), 'f', 1, 64), ".", ",", 1)
	if p < 0 {
		return "−" + s + " %"
	}
	return "+" + s + " %"
}

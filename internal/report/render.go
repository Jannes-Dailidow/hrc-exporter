package report

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"io"
	"strconv"
	"strings"
	"time"
)

//go:embed report.html.tmpl
var htmlSource string

var htmlTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"eur":    formatEuroAuto,
	"eur2":   func(c Cents) string { return formatEuro(c, 2) },
	"signed": formatSigned,
	"pct":    formatPercent,
	"join":   strings.Join,
	"date":   func(t time.Time) string { return t.Format("02.01.2006 15:04") },
	"cell": func(col TariffColumn, group string) string {
		if v, ok := col.Values[group]; ok {
			return formatEuroAuto(v)
		}
		return "–"
	},
}).Parse(htmlSource))

// WriteHTML renders r as a self-contained HTML page (German, inline SVG
// charts, light and dark theme). generated is shown as the creation time.
func (r *Report) WriteHTML(w io.Writer, generated time.Time) error {
	return htmlTemplate.Execute(w, struct {
		*Report
		Generated time.Time
		Charts    chartData
	}{r, generated, r.chartData()})
}

// WriteJSON writes the monthly, event and yearly series behind the charts.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r.chartData())
}

type chartMonth struct {
	Month       string  `json:"m"`
	Group       string  `json:"grp"`
	Tariff      float64 `json:"tarif"`
	AboveTariff float64 `json:"ut"`
	Trainee     float64 `json:"azubi"`
	Note        string  `json:"note"`
}

type chartEvent struct {
	Month            string  `json:"m"`
	Title            string  `json:"t"`
	DeltaTariff      float64 `json:"dT"`
	DeltaAboveTariff float64 `json:"dU"`
}

type chartYear struct {
	Year       string  `json:"y"`
	Months     int     `json:"n"`
	Base       float64 `json:"base"`
	HolidayPay float64 `json:"uw"`
	Other      float64 `json:"other"`
	Total      float64 `json:"total"`
}

type chartData struct {
	Months []chartMonth `json:"months"`
	Events []chartEvent `json:"events"`
	Years  []chartYear  `json:"years"`
	Start  string       `json:"start"` // first salaried month
}

func (r *Report) chartData() chartData {
	var c chartData
	for _, m := range r.Months {
		c.Months = append(c.Months, chartMonth{
			m.Period.String(), m.Group, m.Tariff.Euros(), m.AboveTariff.Euros(), m.Trainee.Euros(), strings.Join(m.Notes, "; "),
		})
	}
	for _, e := range r.Events {
		c.Events = append(c.Events, chartEvent{e.Month.Period.Short(), e.Title, e.DeltaTariff.Euros(), e.DeltaAboveTariff.Euros()})
	}
	for _, y := range r.Years {
		c.Years = append(c.Years, chartYear{
			strconv.Itoa(y.Year), y.Months, y.Base.Euros(), y.HolidayPay.Euros(), y.Other.Euros(), y.Total.Euros(),
		})
	}
	if r.Summary != nil {
		c.Start = r.Summary.Start.Period.String()
	}
	return c
}

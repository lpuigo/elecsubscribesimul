package export

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/pricing"
)

// summaryColumn is one column of the daily summary sheet: a metric, its summary values
// (rows 2 to 5) and its value for each day (rows below).
type summaryColumn struct {
	header    string
	format    string
	bold      bool
	total     *float64
	average   *float64
	extrapol  *float64
	gap       string
	days      func(i int) any
	highlight bool
}

func ptr(v float64) *float64 { return &v }

// writeSummary writes the daily summary sheet: one column per metric, summary rows first (REQ-OUT-02),
// then one row per civil day.
func writeSummary(f *excelize.File, st *styleFactory, r pricing.Result, detail bool) error {
	w := &writer{f: f, sheet: summarySheet}
	numDays := float64(len(r.Days))
	bold := st.id(styleSpec{bold: true})
	w.val(1, 1, "", bold)
	for i, h := range []string{"Total période", "Moyenne / jour", "Extrapolation annuelle", "Écart vs moins cher"} {
		w.val(1, 2+i, h, bold)
	}
	for i, d := range r.Days {
		w.val(1, summaryHeaderRows+1+i, d.Date.Format(shortDateFormat), st.id(styleSpec{bold: true, topBorder: i == 0}))
	}

	totalKWh := r.TotalKWh()
	columns := []summaryColumn{{
		header: "Consommation (kWh)", format: kwhFormat, bold: true,
		total: ptr(totalKWh), average: ptr(totalKWh / numDays), extrapol: ptr(totalKWh / numDays * 365),
		days: func(i int) any { return r.Days[i].KWh },
	}}

	cheapest := r.Cheapest()
	for is, s := range r.Subscriptions {
		gap := ""
		if is != cheapest {
			diff := s.Total() - r.Subscriptions[cheapest].Total()
			gap = fmt.Sprintf("+%s € (+%s %%)", frNumber(diff), frNumber(diff/r.Subscriptions[cheapest].Total()*100))
		}
		columns = append(columns, summaryColumn{
			header: s.Subscription.Name + " (€)", format: euroFormat, bold: true,
			total: ptr(s.Total()), average: ptr(r.DailyAverage(is)), extrapol: ptr(r.Extrapolation(is)),
			gap: gap, highlight: is == cheapest,
			days: func(i int) any { return s.Days[i].Total() },
		})
		if detail {
			columns = append(columns, subColumns(r, is, numDays)...)
		}
	}

	if usesColors(r) {
		columns = append(columns, summaryColumn{header: "Couleur Tempo", days: func(i int) any { return r.Days[i].Color }})
	}
	missing := 0
	for _, d := range r.Days {
		missing += d.Missing
	}
	columns = append(columns, summaryColumn{
		header: "Créneaux manquants", total: ptr(float64(missing)),
		days: func(i int) any { return r.Days[i].Missing },
	})

	for k, c := range columns {
		col := 2 + k
		w.val(col, 1, c.header, bold)
		for row, v := range []*float64{c.total, c.average, c.extrapol} {
			if v != nil {
				fill := ""
				if row == 0 && c.highlight {
					fill = lightGreen
				}
				w.val(col, 2+row, *v, st.id(styleSpec{format: c.format, bold: c.bold, fill: fill}))
			}
		}
		if c.gap != "" {
			w.val(col, 5, c.gap, st.id(styleSpec{}))
		}
		for i := range r.Days {
			w.val(col, summaryHeaderRows+1+i, c.days(i), st.id(styleSpec{format: c.format, topBorder: i == 0}))
		}
	}
	w.width(1, 1, 24)
	w.width(2, 1+len(columns), 22)
	w.freeze(1, summaryHeaderRows)
	return w.err
}

// subColumns builds the detail columns of a subscription: fixed fee, variable cost and kWh per label.
func subColumns(r pricing.Result, is int, numDays float64) []summaryColumn {
	s := r.Subscriptions[is]
	name := s.Subscription.Name
	fixed, variable := s.TotalFixed(), s.TotalVariable()
	out := []summaryColumn{
		{header: name + " — part fixe", format: euroFormat, total: ptr(fixed), average: ptr(fixed / numDays), extrapol: ptr(fixed / numDays * 365),
			days: func(i int) any { return s.Days[i].Fixed }},
		{header: name + " — coût variable", format: euroFormat, total: ptr(variable), average: ptr(variable / numDays), extrapol: ptr(variable / numDays * 365),
			days: func(i int) any { return s.Days[i].Variable }},
	}
	for _, label := range s.Subscription.Labels() {
		total := 0.0
		for _, d := range s.Days {
			total += d.KWhByLabel[label]
		}
		out = append(out, summaryColumn{
			header: name + " — kWh " + label, format: kwhFormat, total: ptr(total), average: ptr(total / numDays),
			days: func(i int) any { return s.Days[i].KWhByLabel[label] },
		})
	}
	return out
}

func usesColors(r pricing.Result) bool {
	for _, s := range r.Subscriptions {
		if s.Subscription.UsesColors() {
			return true
		}
	}
	return false
}

// frNumber formats a number the French way (decimal comma).
func frNumber(v float64) string { return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1) }

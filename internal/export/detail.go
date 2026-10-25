package export

import (
	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/loadcurve"
	"elecsubscribesimul/internal/pricing"
)

func writeDetail(f *excelize.File, st *styleFactory, c loadcurve.Curve, r pricing.Result, detail bool) error {
	w := &writer{f: f, sheet: detailSheet}
	bold := st.id(styleSpec{bold: true})
	first := summary30Columns + 1

	w.val(1, 1, "Début", bold)
	for i, h := range []string{"Total période", "Moyenne par créneau", "Maximum"} {
		w.val(2+i, 1, h, bold)
	}

	// Summary statistics over the non-missing slots.
	count, sumKW, sumKWh, maxKW := 0, 0.0, 0.0, 0.0
	for _, m := range c.Measurements {
		if m.Missing {
			continue
		}
		count++
		sumKW += m.KW
		sumKWh += m.KWh()
		if m.KW > maxKW {
			maxKW = m.KW
		}
	}
	avg := func(sum float64) float64 {
		if count == 0 {
			return 0
		}
		return sum / float64(count)
	}

	w.val(1, 2, "Puissance moyenne (kW)", bold)
	w.val(3, 2, avg(sumKW), st.id(styleSpec{format: kwFormat}))
	w.val(4, 2, maxKW, st.id(styleSpec{format: kwFormat}))
	w.val(1, 3, "Énergie (kWh)", bold)
	w.val(2, 3, sumKWh, st.id(styleSpec{format: kwhFormat}))
	w.val(3, 3, avg(sumKWh), st.id(styleSpec{format: kwhFormat}))

	row := 4
	// costRow locates a subscription row; a negative row holds the applied tariff label.
	type costRow struct{ row, sub int }
	var costs []costRow
	for is, s := range r.Subscriptions {
		w.val(1, row, s.Subscription.Name+" (€)", bold)
		total, maxi := 0.0, 0.0
		for i, m := range c.Measurements {
			if !m.Missing {
				total += s.Slots[i].Cost
				if s.Slots[i].Cost > maxi {
					maxi = s.Slots[i].Cost
				}
			}
		}
		w.val(2, row, total, st.id(styleSpec{format: euroFormat}))
		w.val(3, row, avg(total), st.id(styleSpec{format: euroFormat}))
		w.val(4, row, maxi, st.id(styleSpec{format: euroFormat}))
		costs = append(costs, costRow{row, is})
		row++
		if detail {
			w.val(1, row, "   tarif appliqué : "+s.Subscription.Name, st.id(styleSpec{}))
			costs = append(costs, costRow{-row, is})
			row++
		}
	}

	for i, m := range c.Measurements {
		col := first + i
		border := i == 0
		w.val(col, 1, m.Start.Format(dateTimeFormat), st.id(styleSpec{bold: true, leftBorder: border}))
		if m.Missing {
			continue
		}
		w.val(col, 2, m.KW, st.id(styleSpec{format: kwFormat, leftBorder: border}))
		w.val(col, 3, m.KWh(), st.id(styleSpec{format: kwhFormat, leftBorder: border}))
		for _, cr := range costs {
			if cr.row > 0 {
				w.val(col, cr.row, r.Subscriptions[cr.sub].Slots[i].Cost, st.id(styleSpec{format: euroFormat, leftBorder: border}))
			} else {
				w.val(col, -cr.row, r.Subscriptions[cr.sub].Slots[i].Label, st.id(styleSpec{leftBorder: border}))
			}
		}
	}
	w.width(1, 1, 34)
	w.width(2, summary30Columns, 18)
	w.width(first, first+len(c.Measurements)-1, 16)
	w.freeze(summary30Columns)
	return w.err
}

package export

import (
	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/loadcurve"
	"elecsubscribesimul/internal/pricing"
)

// writeDetail writes the 30-minute detail sheet: one column per metric, summary rows first (REQ-OUT-02),
// then one row per slot.
func writeDetail(f *excelize.File, st *styleFactory, c loadcurve.Curve, r pricing.Result, detail bool) error {
	w := &writer{f: f, sheet: detailSheet}
	bold := st.id(styleSpec{bold: true})

	w.val(1, 1, "Début", bold)
	for i, h := range []string{"Total période", "Moyenne par créneau", "Maximum"} {
		w.val(1, 2+i, h, bold)
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

	w.val(2, 1, "Puissance moyenne (kW)", bold)
	w.val(2, 3, avg(sumKW), st.id(styleSpec{format: kwFormat}))
	w.val(2, 4, maxKW, st.id(styleSpec{format: kwFormat}))
	w.val(3, 1, "Énergie (kWh)", bold)
	w.val(3, 2, sumKWh, st.id(styleSpec{format: kwhFormat}))
	w.val(3, 3, avg(sumKWh), st.id(styleSpec{format: kwhFormat}))

	col := 4
	// costCol locates the cost column of a subscription and its optional "applied tariff" column.
	type costCol struct{ col, labelCol, sub int }
	var costs []costCol
	for is, s := range r.Subscriptions {
		w.val(col, 1, s.Subscription.Name+" (€)", bold)
		total, maxi := 0.0, 0.0
		for i, m := range c.Measurements {
			if !m.Missing {
				total += s.Slots[i].Cost
				if s.Slots[i].Cost > maxi {
					maxi = s.Slots[i].Cost
				}
			}
		}
		w.val(col, 2, total, st.id(styleSpec{format: euroFormat}))
		w.val(col, 3, avg(total), st.id(styleSpec{format: euroFormat}))
		w.val(col, 4, maxi, st.id(styleSpec{format: euroFormat}))
		cc := costCol{col: col, sub: is}
		col++
		if detail {
			w.val(col, 1, "tarif appliqué : "+s.Subscription.Name, bold)
			cc.labelCol = col
			col++
		}
		costs = append(costs, cc)
	}

	for i, m := range c.Measurements {
		row := detailHeaderRows + 1 + i
		border := i == 0
		w.val(1, row, m.Start.Format(dateTimeFormat), st.id(styleSpec{bold: true, topBorder: border}))
		if m.Missing {
			continue
		}
		w.val(2, row, m.KW, st.id(styleSpec{format: kwFormat, topBorder: border}))
		w.val(3, row, m.KWh(), st.id(styleSpec{format: kwhFormat, topBorder: border}))
		for _, cc := range costs {
			slot := r.Subscriptions[cc.sub].Slots[i]
			w.val(cc.col, row, slot.Cost, st.id(styleSpec{format: euroFormat, topBorder: border}))
			if cc.labelCol > 0 {
				w.val(cc.labelCol, row, slot.Label, st.id(styleSpec{topBorder: border}))
			}
		}
	}
	w.width(1, 1, 24)
	w.width(2, col-1, 22)
	w.freeze(1, detailHeaderRows)
	return w.err
}

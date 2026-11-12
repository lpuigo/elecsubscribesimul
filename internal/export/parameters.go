package export

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/loadcurve"
	"elecsubscribesimul/internal/pricing"
	"elecsubscribesimul/internal/subscription"
)

func writeParameters(f *excelize.File, st *styleFactory, c loadcurve.Curve, r pricing.Result, opts Options, exportWarnings []string) error {
	w := &writer{f: f, sheet: parametersSheet}
	bold := st.id(styleSpec{bold: true})
	normal := st.id(styleSpec{})
	row := 1
	put := func(key string, value any) {
		w.val(1, row, key, bold)
		if value != nil {
			w.val(2, row, value, normal)
		}
		row++
	}

	put("PRM", c.PRM)
	put("Période", fmt.Sprintf("du %s au %s", c.Start().Format(dateTimeFormat), c.End().Format(dateTimeFormat)))
	put("Jours", len(r.Days))
	put("Généré le", opts.Generated.Format("02/01/2006 15:04:05"))
	for i, s := range c.Sources {
		key := ""
		if i == 0 {
			key = "Fichiers sources"
		}
		w.val(1, row, key, bold)
		w.val(2, row, s, normal)
		row++
	}

	row++
	put("Abonnements", nil)
	for _, s := range r.Subscriptions {
		sub := s.Subscription
		put(sub.Name, fmt.Sprintf("%d kVA, part fixe %s € %s, début de journée tarifaire %s",
			sub.PowerKVA, frNumber(sub.FixedFee.Amount), sub.FixedFee.Periodicity, sub.TariffDayStart))
		for _, t := range sub.Tariffs {
			color := "toutes couleurs"
			if t.Color != "" {
				color = t.Color
			}
			w.val(2, row, fmt.Sprintf("%s — %s : %s à %.4f €/kWh", color, t.Label, describeRanges(t), t.PricePerKWh), normal)
			row++
		}
	}

	row++
	put("Anomalies", nil)
	put("Créneaux manquants", r.Missing())
	if len(r.AssumedColors) > 0 {
		days := make([]string, len(r.AssumedColors))
		for i, d := range r.AssumedColors {
			days[i] = d.Format(shortDateFormat)
		}
		put("Couleurs supposées (par défaut)", strings.Join(days, ", "))
	}
	for _, a := range append(append([]string{}, opts.Warnings...), exportWarnings...) {
		put("Avertissement", a)
	}
	w.width(1, 1, 34)
	w.width(2, 2, 110)
	return w.err
}

// describeRanges lists the time ranges of a tariff, or marks it as the default one.
func describeRanges(t subscription.Tariff) string {
	if t.IsDefault() {
		return "par défaut (hors plages des autres tarifs)"
	}
	parts := make([]string, len(t.Ranges))
	for i, r := range t.Ranges {
		parts[i] = fmt.Sprintf("%s → %s", r.Start, r.End)
	}
	return strings.Join(parts, ", ")
}

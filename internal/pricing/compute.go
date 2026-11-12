// Package pricing is the pure computation engine: measurements + subscriptions + calendar -> costs.
package pricing

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"elecsubscribesimul/internal/calendar"
	"elecsubscribesimul/internal/loadcurve"
	"elecsubscribesimul/internal/subscription"
)

const dayKeyFormat = "2006-01-02"

type dayColor struct {
	color string
	assumed,
	ok bool
}

type computation struct {
	cal     calendar.Calendar
	cache   map[string]dayColor
	missing map[string]bool
	assumed map[string]time.Time
}

// Compute applies each subscription to each slot of the curve.
func Compute(c loadcurve.Curve, subs []subscription.Subscription, cal calendar.Calendar) (Result, error) {
	if len(c.Measurements) == 0 {
		return Result{}, errors.New("courbe de charge vide")
	}
	k := &computation{cal: cal, cache: map[string]dayColor{}, missing: map[string]bool{}, assumed: map[string]time.Time{}}

	days, dayIndex := aggregateDays(c.Measurements)
	res := Result{Days: days}
	for _, s := range subs {
		res.Subscriptions = append(res.Subscriptions, k.subscriptionCost(s, c.Measurements, days, dayIndex))
	}
	if len(k.missing) > 0 {
		return Result{}, fmt.Errorf("couleur de jour manquante pour : %s (fournir --couleurs ou --couleur-par-defaut)", listDays(k.missing))
	}
	k.civilColors(&res)
	for _, t := range k.assumed {
		res.AssumedColors = append(res.AssumedColors, t)
	}
	sort.Slice(res.AssumedColors, func(i, j int) bool { return res.AssumedColors[i].Before(res.AssumedColors[j]) })
	return res, nil
}

func civilDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func aggregateDays(measurements []loadcurve.Measurement) ([]Day, map[string]int) {
	var days []Day
	index := map[string]int{}
	for _, m := range measurements {
		d := civilDay(m.Start)
		key := d.Format(dayKeyFormat)
		i, ok := index[key]
		if !ok {
			i = len(days)
			index[key] = i
			days = append(days, Day{Date: d})
		}
		if m.Missing {
			days[i].Missing++
			continue
		}
		days[i].KWh += m.KWh()
		if m.KW > days[i].MaxKW {
			days[i].MaxKW = m.KW
		}
	}
	return days, index
}

// plan holds what the slot loop needs for one subscription, computed once instead of per minute.
type plan struct {
	sub        subscription.Subscription
	usesColors bool
	labels     []string
	schedules  map[string]*subscription.Schedule // per day color, built on first use
}

func newPlan(s subscription.Subscription) *plan {
	return &plan{sub: s, usesColors: s.UsesColors(), labels: s.Labels(), schedules: map[string]*subscription.Schedule{}}
}

func (p *plan) schedule(color string) *subscription.Schedule {
	sch, ok := p.schedules[color]
	if !ok {
		sch = p.sub.Schedule(color)
		p.schedules[color] = sch
	}
	return sch
}

func (k *computation) subscriptionCost(s subscription.Subscription, measurements []loadcurve.Measurement, days []Day, dayIndex map[string]int) SubscriptionCost {
	out := SubscriptionCost{Subscription: s, Slots: make([]SlotCost, len(measurements)), Days: make([]DayCost, len(days))}
	for i, d := range days {
		out.Days[i] = DayCost{Fixed: s.FixedFee.DailyAmount(d.Date), KWhByLabel: map[string]float64{}}
	}
	p := newPlan(s)
	for i, m := range measurements {
		if m.Missing {
			continue
		}
		cost, byLabel, dominant := k.slotCost(p, m)
		out.Slots[i] = SlotCost{Cost: cost, Label: dominant}
		day := &out.Days[dayIndex[civilDay(m.Start).Format(dayKeyFormat)]]
		day.Variable += cost
		for label, kwh := range byLabel {
			day.KWhByLabel[label] += kwh
		}
	}
	return out
}

// slotCost splits the slot pro rata of the time spent in each tariff (REQ-CALC-01).
func (k *computation) slotCost(p *plan, m loadcurve.Measurement) (float64, map[string]float64, string) {
	duration := m.Duration()
	kwh := m.KWh()
	cost := 0.0
	byLabel := map[string]float64{}
	for t := m.Start; t.Before(m.End); {
		step := time.Minute
		if rest := m.End.Sub(t); rest < step {
			step = rest
		}
		part := kwh * float64(step) / float64(duration)
		if tariff, ok := k.tariffAt(p, t); ok {
			cost += part * tariff.PricePerKWh
			byLabel[tariff.Label] += part
		}
		t = t.Add(step)
	}
	dominant, max := "", -1.0
	for _, label := range p.labels {
		if v, ok := byLabel[label]; ok && v > max {
			dominant, max = label, v
		}
	}
	return cost, byLabel, dominant
}

// tariffAt finds the tariff applying at an instant, through the tariff day (REQ-TIME-03).
func (k *computation) tariffAt(p *plan, t time.Time) (subscription.Tariff, bool) {
	color := ""
	if p.usesColors {
		day := k.resolve(tariffDay(t, p.sub.TariffDayStart))
		color = day.color
	}
	return p.schedule(color).At(subscription.Minutes(t.Hour()*60 + t.Minute()))
}

// tariffDay returns the date of the tariff day, in wall-clock arithmetic
// (independent of time changes).
func tariffDay(t time.Time, start subscription.Minutes) time.Time {
	d := civilDay(t)
	if subscription.Minutes(t.Hour()*60+t.Minute()) < start {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

func (k *computation) resolve(day time.Time) dayColor {
	key := day.Format(dayKeyFormat)
	if v, ok := k.cache[key]; ok {
		return v
	}
	color, assumed, ok := k.cal.Color(day)
	v := dayColor{color, assumed, ok}
	k.cache[key] = v
	if !ok {
		k.missing[key] = true
	}
	if assumed {
		k.assumed[key] = day
	}
	return v
}

// civilColors fills in the color displayed for each civil day.
func (k *computation) civilColors(r *Result) {
	used := false
	for _, s := range r.Subscriptions {
		used = used || s.Subscription.UsesColors()
	}
	if !used {
		return
	}
	for i := range r.Days {
		if c, _, ok := k.cal.Color(r.Days[i].Date); ok {
			r.Days[i].Color = c
		}
	}
}

func listDays(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for c := range m {
		keys = append(keys, c)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

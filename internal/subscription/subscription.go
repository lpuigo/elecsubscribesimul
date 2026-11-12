// Package subscription defines the tariff offers and their validation.
package subscription

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Periodicity is the period over which a fixed fee amount is billed.
type Periodicity string

// Values match the "periodicite" field of the YAML configuration.
const (
	Annual  Periodicity = "annuelle"
	Monthly Periodicity = "mensuelle"
)

// TimeRange is a time-of-day range; Start == End means the whole day, End < Start a range spanning midnight.
type TimeRange struct {
	Start Minutes
	End   Minutes
}

// Contains reports whether minute m belongs to the range.
func (r TimeRange) Contains(m Minutes) bool {
	end := r.End % minutesPerDay
	start := r.Start % minutesPerDay
	switch {
	case start == end:
		return true
	case start < end:
		return m >= start && m < end
	default:
		return m >= start || m < end
	}
}

func (r TimeRange) String() string { return r.Start.String() + "-" + r.End.String() }

// Tariff is a price per kWh over time ranges, optionally limited to a day color.
// An empty Color means every day. A tariff without ranges is the default one: it applies
// whenever no ranged tariff does (REQ-CALC-05).
type Tariff struct {
	Color       string
	Label       string
	Ranges      []TimeRange
	PricePerKWh float64
}

// IsDefault reports whether the tariff applies outside the ranges of the other tariffs.
func (t Tariff) IsDefault() bool { return len(t.Ranges) == 0 }

// Contains reports whether one of the tariff ranges contains minute m; always false for a default tariff.
func (t Tariff) Contains(m Minutes) bool {
	for _, r := range t.Ranges {
		if r.Contains(m) {
			return true
		}
	}
	return false
}

// appliesTo reports whether the tariff is part of the set of a day color (REQ-IN-14).
func (t Tariff) appliesTo(color string) bool { return t.Color == color || t.Color == "" }

// FixedFee is the fixed part of a subscription (standing charge).
type FixedFee struct {
	Amount      float64
	Periodicity Periodicity
}

// DailyAmount is the fixed fee attributed to one civil day (REQ-CALC-03).
func (p FixedFee) DailyAmount(day time.Time) float64 {
	if p.Periodicity == Monthly {
		return p.Amount / float64(daysInMonth(day))
	}
	return p.Amount / float64(daysInYear(day.Year()))
}

func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func daysInYear(year int) int {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	return int(start.AddDate(1, 0, 0).Sub(start).Hours() / 24)
}

// Subscription is a tariff offer: a fixed fee and a set of tariffs.
type Subscription struct {
	Name           string
	PowerKVA       int
	FixedFee       FixedFee
	TariffDayStart Minutes
	Tariffs        []Tariff
}

// Colors returns the explicitly priced colors, sorted.
func (a Subscription) Colors() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range a.Tariffs {
		if t.Color != "" && !seen[t.Color] {
			seen[t.Color] = true
			out = append(out, t.Color)
		}
	}
	sort.Strings(out)
	return out
}

// UsesColors reports whether the price depends on the day color.
func (a Subscription) UsesColors() bool { return len(a.Colors()) > 0 }

// Labels returns the tariff labels in order of appearance.
func (a Subscription) Labels() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range a.Tariffs {
		if !seen[t.Label] {
			seen[t.Label] = true
			out = append(out, t.Label)
		}
	}
	return out
}

// Schedule gives, for each minute of the day, the tariff applying to one day color.
type Schedule struct {
	tariffs []Tariff
	index   [minutesPerDay]int // index in tariffs, -1 if no tariff applies
}

// At returns the tariff applying at minute m.
func (s *Schedule) At(m Minutes) (Tariff, bool) {
	if i := s.index[m%minutesPerDay]; i >= 0 {
		return s.tariffs[i], true
	}
	return Tariff{}, false
}

// Schedule resolves the tariffs of a day color (empty if the subscription has none) minute by minute:
// a ranged tariff takes precedence over the default one (REQ-CALC-05). On a valid subscription,
// every minute has exactly one tariff.
func (a Subscription) Schedule(color string) *Schedule {
	s := &Schedule{tariffs: a.Tariffs}
	def := -1
	for i, t := range a.Tariffs {
		if t.appliesTo(color) && t.IsDefault() && def < 0 {
			def = i
		}
	}
	for m := range s.index {
		s.index[m] = def
	}
	for i, t := range a.Tariffs {
		if !t.appliesTo(color) || t.IsDefault() {
			continue
		}
		for m := Minutes(0); m < minutesPerDay; m++ {
			if t.Contains(m) && (s.index[m] < 0 || s.index[m] == def) {
				s.index[m] = i
			}
		}
	}
	return s
}

// Validate checks the subscription consistency and gathers every anomaly (REQ-CALC-02, REQ-IN-14).
func (a Subscription) Validate() error {
	var errs []error
	if strings.TrimSpace(a.Name) == "" {
		errs = append(errs, errors.New("abonnement sans nom"))
	}
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("abonnement %q : "+format, append([]any{a.Name}, args...)...))
	}
	if a.FixedFee.Amount < 0 {
		add("part fixe négative")
	}
	if a.FixedFee.Periodicity != Annual && a.FixedFee.Periodicity != Monthly {
		add("périodicité %q invalide (annuelle ou mensuelle)", a.FixedFee.Periodicity)
	}
	if a.TariffDayStart < 0 || a.TariffDayStart >= minutesPerDay {
		add("debut_journee_tarifaire hors de la journée")
	}
	if len(a.Tariffs) == 0 {
		add("aucun tarif")
	}
	for _, t := range a.Tariffs {
		if t.PricePerKWh < 0 {
			add("prix négatif pour le tarif %q", t.Label)
		}
		if strings.TrimSpace(t.Label) == "" {
			add("tarif sans libellé")
		}
	}
	if len(a.Tariffs) > 0 {
		colors := a.Colors()
		if len(colors) == 0 {
			colors = []string{""}
		}
		for _, c := range colors {
			errs = append(errs, a.checkCoverage(c)...)
		}
	}
	return errors.Join(errs...)
}

// checkCoverage verifies, for the tariffs applying to a color, that the ranges do not overlap,
// that there is at most one default tariff, and that the 24 hours are covered exactly once,
// by the ranges or by the default tariff (REQ-CALC-02, REQ-CALC-05).
func (a Subscription) checkCoverage(color string) []error {
	var counts [minutesPerDay]int
	var defaults []string
	prices := map[string]float64{}
	context := fmt.Sprintf("abonnement %q", a.Name)
	if color != "" {
		context += fmt.Sprintf(", couleur %q", color)
	}
	var errs []error
	for _, t := range a.Tariffs {
		if !t.appliesTo(color) {
			continue
		}
		if p, ok := prices[t.Label]; ok && p != t.PricePerKWh {
			errs = append(errs, fmt.Errorf("%s : libellé %q défini avec plusieurs prix", context, t.Label))
		}
		prices[t.Label] = t.PricePerKWh
		if t.IsDefault() {
			defaults = append(defaults, t.Label)
			continue
		}
		for _, r := range t.Ranges {
			for m := Minutes(0); m < minutesPerDay; m++ {
				if r.Contains(m) {
					counts[m]++
				}
			}
		}
	}
	if len(defaults) > 1 {
		errs = append(errs, fmt.Errorf("%s : plusieurs tarifs par défaut : %s", context, strings.Join(defaults, ", ")))
	}
	uncovered := ranges(counts[:], func(n int) bool { return n == 0 })
	switch {
	case len(defaults) == 0 && len(uncovered) > 0:
		errs = append(errs, fmt.Errorf("%s : plages horaires non couvertes : %s", context, strings.Join(uncovered, ", ")))
	case len(defaults) > 0 && len(uncovered) == 0:
		errs = append(errs, fmt.Errorf("%s : tarif par défaut %q jamais appliqué (les plages couvrent déjà 24 h)", context, defaults[0]))
	}
	if p := ranges(counts[:], func(n int) bool { return n > 1 }); len(p) > 0 {
		errs = append(errs, fmt.Errorf("%s : plages horaires qui se chevauchent : %s", context, strings.Join(p, ", ")))
	}
	return errs
}

// ranges lists the "HH:MM-HH:MM" ranges of consecutive minutes whose count is flagged by bad.
func ranges(counts []int, bad func(int) bool) []string {
	var out []string
	start := -1
	for m := 0; m <= len(counts); m++ {
		if m < len(counts) && bad(counts[m]) {
			if start < 0 {
				start = m
			}
		} else if start >= 0 {
			out = append(out, fmt.Sprintf("%s-%s", Minutes(start), Minutes(m)))
			start = -1
		}
	}
	return out
}

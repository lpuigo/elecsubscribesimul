package pricing

import (
	"time"

	"elecsubscribesimul/internal/subscription"
)

const daysPerYear = 365

// Day gathers the data of one civil day, shared by all subscriptions.
type Day struct {
	Date    time.Time
	KWh     float64
	MaxKW   float64
	Missing int
	Color   string // color of the civil day, empty if no subscription uses colors
}

// DayCost is the cost of one civil day for one subscription.
type DayCost struct {
	Variable   float64
	Fixed      float64
	KWhByLabel map[string]float64
}

// Total is the variable cost plus the fixed fee.
func (c DayCost) Total() float64 { return c.Variable + c.Fixed }

// SlotCost is the variable cost of a slot and its majority tariff label.
type SlotCost struct {
	Cost  float64
	Label string
}

// SubscriptionCost is the cost of one subscription over the whole period.
type SubscriptionCost struct {
	Subscription subscription.Subscription
	Days         []DayCost
	Slots        []SlotCost
}

// TotalVariable sums the variable costs of the period.
func (c SubscriptionCost) TotalVariable() float64 {
	t := 0.0
	for _, d := range c.Days {
		t += d.Variable
	}
	return t
}

// TotalFixed sums the fixed fees of the period.
func (c SubscriptionCost) TotalFixed() float64 {
	t := 0.0
	for _, d := range c.Days {
		t += d.Fixed
	}
	return t
}

// Total is the full cost of the period.
func (c SubscriptionCost) Total() float64 { return c.TotalVariable() + c.TotalFixed() }

// Result is the outcome of the computation for all subscriptions.
type Result struct {
	Days          []Day
	Subscriptions []SubscriptionCost
	AssumedColors []time.Time
}

// TotalKWh is the energy consumed over the period.
func (r Result) TotalKWh() float64 {
	t := 0.0
	for _, d := range r.Days {
		t += d.KWh
	}
	return t
}

// Missing counts the missing slots of the period.
func (r Result) Missing() int {
	n := 0
	for _, d := range r.Days {
		n += d.Missing
	}
	return n
}

// DailyAverage is the average cost per civil day; Extrapolation extends it over 365 days (§3.4).
func (r Result) DailyAverage(i int) float64 {
	return r.Subscriptions[i].Total() / float64(len(r.Days))
}

// Extrapolation extends the daily average cost of subscription i over 365 days.
func (r Result) Extrapolation(i int) float64 { return r.DailyAverage(i) * daysPerYear }

// Cheapest returns the index of the subscription with the lowest total cost.
func (r Result) Cheapest() int {
	best := 0
	for i := range r.Subscriptions {
		if r.Subscriptions[i].Total() < r.Subscriptions[best].Total() {
			best = i
		}
	}
	return best
}

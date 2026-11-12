package pricing

import (
	"math"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"elecsubscribesimul/internal/calendar"
	"elecsubscribesimul/internal/loadcurve"
	"elecsubscribesimul/internal/subscription"
)

var paris, _ = time.LoadLocation("Europe/Paris")

func tariff(color, label, start, end string, price float64) subscription.Tariff {
	s, _ := subscription.ParseMinutes(start)
	e, _ := subscription.ParseMinutes(end)
	return subscription.Tariff{Color: color, Label: label, Ranges: []subscription.TimeRange{{Start: s, End: e}}, PricePerKWh: price}
}

// series produces 30-minute slots at constant power (1 kW => 0.5 kWh per slot).
func series(start time.Time, n int, kw float64) loadcurve.Curve {
	c := loadcurve.Curve{}
	for i := 0; i < n; i++ {
		d := start.Add(time.Duration(i) * 30 * time.Minute)
		c.Measurements = append(c.Measurements, loadcurve.Measurement{Start: d, End: d.Add(30 * time.Minute), KW: kw})
	}
	return c
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

var (
	subBase = subscription.Subscription{Name: "Base", FixedFee: subscription.FixedFee{Amount: 365, Periodicity: subscription.Annual},
		Tariffs: []subscription.Tariff{tariff("", "Base", "00:00", "00:00", 0.20)}}
	subHPHC = subscription.Subscription{Name: "HPHC", FixedFee: subscription.FixedFee{Amount: 365, Periodicity: subscription.Annual},
		Tariffs: []subscription.Tariff{tariff("", "HC", "22:00", "06:00", 0.10), tariff("", "HP", "06:00", "22:00", 0.30)}}
	subTempo = subscription.Subscription{Name: "Tempo", FixedFee: subscription.FixedFee{Amount: 365, Periodicity: subscription.Annual},
		TariffDayStart: 6 * 60,
		Tariffs: []subscription.Tariff{
			tariff("bleu", "HC", "22:00", "06:00", 0.10), tariff("bleu", "HP", "06:00", "22:00", 0.20),
			tariff("rouge", "HC", "22:00", "06:00", 0.15), tariff("rouge", "HP", "06:00", "22:00", 0.70),
		}}
)

func TestBase(t *testing.T) {
	c := series(time.Date(2026, 9, 1, 0, 0, 0, 0, paris), 96, 1) // 2 days, 48 kWh
	res, err := Compute(c, []subscription.Subscription{subBase}, calendar.Calendar{})
	if err != nil {
		t.Fatal(err)
	}
	a := res.Subscriptions[0]
	if !approx(a.TotalVariable(), 48*0.20) || !approx(a.TotalFixed(), 2) || len(res.Days) != 2 {
		t.Errorf("variable=%v fixed=%v days=%d", a.TotalVariable(), a.TotalFixed(), len(res.Days))
	}
	if !approx(res.Extrapolation(0), (48*0.20+2)/2*365) {
		t.Errorf("extrapolation = %v", res.Extrapolation(0))
	}
}

func TestHPHCBoundaries(t *testing.T) {
	tests := []struct {
		hour  time.Time
		label string
	}{
		{time.Date(2026, 9, 1, 21, 30, 0, 0, paris), "HP"},
		{time.Date(2026, 9, 1, 22, 0, 0, 0, paris), "HC"},
		{time.Date(2026, 9, 1, 5, 30, 0, 0, paris), "HC"},
		{time.Date(2026, 9, 1, 6, 0, 0, 0, paris), "HP"},
	}
	for _, tt := range tests {
		res, err := Compute(series(tt.hour, 1, 1), []subscription.Subscription{subHPHC}, calendar.Calendar{})
		if err != nil {
			t.Fatal(err)
		}
		if got := res.Subscriptions[0].Slots[0].Label; got != tt.label {
			t.Errorf("%s: %s, want %s", tt.hour.Format("15:04"), got, tt.label)
		}
	}
}

// REQ-CALC-01: HC boundary at 22:15, slot 22:00-22:30 split 15 min / 15 min.
func TestProrataUnalignedBoundary(t *testing.T) {
	sub := subHPHC
	sub.Tariffs = []subscription.Tariff{tariff("", "HC", "22:15", "06:00", 0.10), tariff("", "HP", "06:00", "22:15", 0.30)}
	res, err := Compute(series(time.Date(2026, 9, 1, 22, 0, 0, 0, paris), 1, 2), []subscription.Subscription{sub}, calendar.Calendar{})
	if err != nil {
		t.Fatal(err)
	}
	want := 0.5 * (0.30 + 0.10) / 1 // 1 kWh in total, half HP, half HC
	if got := res.Subscriptions[0].Slots[0].Cost; !approx(got, want) {
		t.Errorf("cost = %v, want %v", got, want)
	}
}

// REQ-CALC-05: default HP tariff with two HC ranges; the 14:15 boundary is split pro rata.
func TestDefaultTariffWithRanges(t *testing.T) {
	r := func(start, end string) subscription.TimeRange {
		s, _ := subscription.ParseMinutes(start)
		e, _ := subscription.ParseMinutes(end)
		return subscription.TimeRange{Start: s, End: e}
	}
	sub := subHPHC
	sub.Tariffs = []subscription.Tariff{
		{Label: "HP", PricePerKWh: 0.30},
		{Label: "HC", Ranges: []subscription.TimeRange{r("01:00", "07:00"), r("14:15", "15:00")}, PricePerKWh: 0.10},
	}
	if err := sub.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		hour time.Time
		cost float64
	}{
		{time.Date(2026, 9, 1, 0, 30, 0, 0, paris), 0.30},
		{time.Date(2026, 9, 1, 1, 0, 0, 0, paris), 0.10},
		{time.Date(2026, 9, 1, 14, 0, 0, 0, paris), 0.5 * (0.30 + 0.10)},
		{time.Date(2026, 9, 1, 14, 30, 0, 0, paris), 0.10},
		{time.Date(2026, 9, 1, 15, 0, 0, 0, paris), 0.30},
	} {
		res, err := Compute(series(tt.hour, 1, 2), []subscription.Subscription{sub}, calendar.Calendar{}) // 1 kWh
		if err != nil {
			t.Fatal(err)
		}
		if got := res.Subscriptions[0].Slots[0].Cost; !approx(got, tt.cost) {
			t.Errorf("%s: cost %v, want %v", tt.hour.Format("15:04"), got, tt.cost)
		}
	}
}

// REQ-TIME-03: the slot of day D at 01:00 takes the color of D-1.
func TestTempoTariffDay(t *testing.T) {
	cal := calendar.New(map[string]string{"2026-09-01": "bleu", "2026-09-02": "rouge"}, "")
	// 02/09 01:00 (HC, tariff day of 01/09 = bleu) then 02/09 10:00 (HP, tariff day of 02/09 = rouge)
	for _, tt := range []struct {
		hour  time.Time
		price float64
	}{
		{time.Date(2026, 9, 2, 1, 0, 0, 0, paris), 0.10},
		{time.Date(2026, 9, 2, 10, 0, 0, 0, paris), 0.70},
	} {
		res, err := Compute(series(tt.hour, 1, 2), []subscription.Subscription{subTempo}, cal)
		if err != nil {
			t.Fatal(err)
		}
		if got := res.Subscriptions[0].Slots[0].Cost; !approx(got, tt.price) {
			t.Errorf("%s: cost %v, want %v", tt.hour.Format("02/01 15:04"), got, tt.price)
		}
	}
}

func TestTempoMissingColorsAndDefault(t *testing.T) {
	c := series(time.Date(2026, 9, 1, 8, 0, 0, 0, paris), 2, 1)
	_, err := Compute(c, []subscription.Subscription{subTempo}, calendar.Calendar{})
	if err == nil || !strings.Contains(err.Error(), "2026-09-01") {
		t.Fatalf("error = %v", err)
	}
	res, err := Compute(c, []subscription.Subscription{subTempo}, calendar.New(nil, "bleu"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.AssumedColors) != 1 {
		t.Errorf("assumed colors: %v", res.AssumedColors)
	}
}

// A subscription without colors does not depend on the calendar (REQ-IN-13).
func TestColorlessIgnoresCalendar(t *testing.T) {
	c := series(time.Date(2026, 9, 1, 8, 0, 0, 0, paris), 2, 1)
	if _, err := Compute(c, []subscription.Subscription{subBase, subHPHC}, calendar.Calendar{}); err != nil {
		t.Fatal(err)
	}
}

func TestMissingExcluded(t *testing.T) {
	c := series(time.Date(2026, 9, 1, 8, 0, 0, 0, paris), 4, 1)
	c.Measurements[1].Missing = true
	res, err := Compute(c, []subscription.Subscription{subBase}, calendar.Calendar{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Missing() != 1 || !approx(res.TotalKWh(), 1.5) || !approx(res.Subscriptions[0].TotalVariable(), 0.30) {
		t.Errorf("missing=%d kWh=%v cost=%v", res.Missing(), res.TotalKWh(), res.Subscriptions[0].TotalVariable())
	}
}

// Day of the switch to winter time: 50 slots, 25 kWh billed.
func TestWinterTimeChange(t *testing.T) {
	c := loadcurve.Curve{}
	day := time.Date(2026, 10, 25, 0, 0, 0, 0, paris)
	for d := day; d.Before(day.AddDate(0, 0, 1)); d = d.Add(30 * time.Minute) {
		c.Measurements = append(c.Measurements, loadcurve.Measurement{Start: d, End: d.Add(30 * time.Minute), KW: 1})
	}
	res, err := Compute(c, []subscription.Subscription{subBase}, calendar.Calendar{})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Measurements) != 50 || len(res.Days) != 1 || !approx(res.TotalKWh(), 25) {
		t.Errorf("slots=%d days=%d kWh=%v", len(c.Measurements), len(res.Days), res.TotalKWh())
	}
}

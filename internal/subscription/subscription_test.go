package subscription

import (
	"strings"
	"testing"
	"time"
)

func tariff(color, label, start, end string) Tariff {
	s, _ := ParseMinutes(start)
	e, _ := ParseMinutes(end)
	return Tariff{Color: color, Label: label, Ranges: []TimeRange{{Start: s, End: e}}, PricePerKWh: 0.2}
}

func defaultTariff(color, label string) Tariff {
	return Tariff{Color: color, Label: label, PricePerKWh: 0.3}
}

// multiRange builds a 0.1 €/kWh tariff over "HH:MM" start/end pairs.
func multiRange(color, label string, bounds ...string) Tariff {
	t := Tariff{Color: color, Label: label, PricePerKWh: 0.1}
	for i := 0; i+1 < len(bounds); i += 2 {
		s, _ := ParseMinutes(bounds[i])
		e, _ := ParseMinutes(bounds[i+1])
		t.Ranges = append(t.Ranges, TimeRange{Start: s, End: e})
	}
	return t
}

func TestValidate(t *testing.T) {
	base := FixedFee{Amount: 100, Periodicity: Annual}
	tests := []struct {
		name    string
		tariffs []Tariff
		wantErr string
	}{
		{"base", []Tariff{tariff("", "Base", "00:00", "00:00")}, ""},
		{"hphc", []Tariff{tariff("", "HC", "22:00", "06:00"), tariff("", "HP", "06:00", "22:00")}, ""},
		{"overlap", []Tariff{tariff("", "HC", "21:00", "06:00"), tariff("", "HP", "06:00", "22:00")}, "chevauchent : 21:00-22:00"},
		{"gap", []Tariff{tariff("", "HC", "22:00", "06:00"), tariff("", "HP", "07:00", "22:00")}, "non couvertes : 06:00-07:00"},
		{"incomplete color", []Tariff{
			tariff("bleu", "HC", "22:00", "06:00"), tariff("bleu", "HP", "06:00", "22:00"),
			tariff("rouge", "HP", "06:00", "22:00"),
		}, `couleur "rouge" : plages horaires non couvertes`},
		{"colorless tariff + colors", []Tariff{
			tariff("", "HC", "22:00", "06:00"), tariff("bleu", "HP", "06:00", "22:00"), tariff("rouge", "HP", "06:00", "22:00"),
		}, ""},
		// REQ-CALC-05: default tariff overridden by ranges
		{"default + ranges", []Tariff{defaultTariff("", "HP"), multiRange("", "HC", "01:00", "07:00", "14:00", "15:00")}, ""},
		{"default only", []Tariff{defaultTariff("", "Base")}, ""},
		{"two defaults", []Tariff{defaultTariff("", "HP"), defaultTariff("", "Base"), multiRange("", "HC", "01:00", "07:00")},
			"plusieurs tarifs par défaut : HP, Base"},
		{"overlapping ranges with default", []Tariff{defaultTariff("", "HP"), multiRange("", "HC", "01:00", "07:00", "06:00", "08:00")},
			"chevauchent : 06:00-07:00"},
		{"unused default", []Tariff{defaultTariff("", "HP"), multiRange("", "HC", "22:00", "06:00"), multiRange("", "HC", "06:00", "22:00")},
			`tarif par défaut "HP" jamais appliqué`},
		{"same label, two prices", []Tariff{tariff("", "HC", "22:00", "06:00"), defaultTariff("", "HC")}, `libellé "HC" défini avec plusieurs prix`},
		{"colorless default + colored ranges", []Tariff{
			defaultTariff("", "HP"), multiRange("bleu", "HC", "22:00", "06:00"), multiRange("rouge", "HC", "23:00", "07:00"),
		}, ""},
		{"colored default + colorless default", []Tariff{
			defaultTariff("", "HP"), defaultTariff("bleu", "HP"), multiRange("", "HC", "22:00", "06:00"),
		}, `couleur "bleu" : plusieurs tarifs par défaut`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Subscription{Name: "x", FixedFee: base, Tariffs: tt.tariffs}.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestTariffContains(t *testing.T) {
	hc := tariff("", "HC", "22:00", "06:00")
	hp := tariff("", "HP", "06:00", "22:00")
	// REQ: slots 21:30, 22:00, 05:30 and 06:00
	for _, c := range []struct {
		hour string
		hc   bool
	}{{"21:30", false}, {"22:00", true}, {"05:30", true}, {"06:00", false}} {
		m, _ := ParseMinutes(c.hour)
		if hc.Contains(m) != c.hc || hp.Contains(m) == c.hc {
			t.Errorf("%s: HC=%v HP=%v", c.hour, hc.Contains(m), hp.Contains(m))
		}
	}
}

// REQ-CALC-05: ranges take precedence over the default tariff.
func TestSchedule(t *testing.T) {
	sub := Subscription{Tariffs: []Tariff{defaultTariff("", "HP"), multiRange("", "HC", "01:00", "07:00", "14:00", "15:00")}}
	sch := sub.Schedule("")
	for hour, want := range map[string]string{"00:59": "HP", "01:00": "HC", "06:59": "HC", "07:00": "HP", "13:59": "HP", "14:00": "HC", "15:00": "HP"} {
		m, _ := ParseMinutes(hour)
		if got, ok := sch.At(m); !ok || got.Label != want {
			t.Errorf("%s: %q (ok=%v), want %s", hour, got.Label, ok, want)
		}
	}
	// Without a default tariff, an uncovered minute has no tariff.
	if _, ok := (Subscription{Tariffs: []Tariff{multiRange("", "HC", "01:00", "07:00")}}).Schedule("").At(0); ok {
		t.Error("tariff found outside every range")
	}
}

func TestDailyAmount(t *testing.T) {
	tests := []struct {
		name string
		fee  FixedFee
		day  time.Time
		want float64
	}{
		{"leap year", FixedFee{366, Annual}, time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC), 1},
		{"common year", FixedFee{365, Annual}, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), 1},
		{"28-day month", FixedFee{28, Monthly}, time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC), 1},
		{"30-day month", FixedFee{30, Monthly}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), 1},
		{"31-day month", FixedFee{31, Monthly}, time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), 1},
	}
	for _, tt := range tests {
		if got := tt.fee.DailyAmount(tt.day); got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

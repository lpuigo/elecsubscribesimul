package loadcurve

import (
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func statement(source, prm string, startSlot, n int, kw float64) Statement {
	s := Statement{Source: source, PRM: prm}
	for i := 0; i < n; i++ {
		d := t0.Add(time.Duration(startSlot+i) * 30 * time.Minute)
		s.Measurements = append(s.Measurements, Measurement{Start: d, End: d.Add(30 * time.Minute), KW: kw})
	}
	return s
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name         string
		statements   []Statement
		measurements int
		missing      int
		warning      string // expected substring, empty if none
		wantErr      string
	}{
		{"contiguous", []Statement{statement("a", "1", 0, 4, 1), statement("b", "1", 4, 4, 1)}, 8, 0, "", ""},
		{"overlap", []Statement{statement("a", "1", 0, 4, 1), statement("b", "1", 2, 4, 2)}, 6, 0, "2 créneau(x) de \"b\"", ""},
		{"gap", []Statement{statement("a", "1", 0, 2, 1), statement("b", "1", 5, 2, 1)}, 7, 3, "trou", ""},
		{"different PRMs", []Statement{statement("a", "1", 0, 2, 1), statement("b", "2", 2, 2, 1)}, 0, 0, "", "PRM différents"},
		{"no statement", nil, 0, 0, "", "aucun"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, warns, err := Merge(tt.statements)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Measurements) != tt.measurements {
				t.Errorf("%d measurements, want %d", len(c.Measurements), tt.measurements)
			}
			missing := 0
			for i, m := range c.Measurements {
				if m.Missing {
					missing++
				}
				if i > 0 && !m.Start.Equal(c.Measurements[i-1].End) {
					t.Errorf("series not continuous at index %d", i)
				}
			}
			if missing != tt.missing {
				t.Errorf("%d missing, want %d", missing, tt.missing)
			}
			if tt.warning == "" && len(warns) != 0 {
				t.Errorf("unexpected warnings: %v", warns)
			}
			if tt.warning != "" && !strings.Contains(strings.Join(warns, "\n"), tt.warning) {
				t.Errorf("warnings %v, want %q", warns, tt.warning)
			}
		})
	}
}

// REQ-IN-09: the last statement read wins, even when its value is missing.
func TestMergeLastWins(t *testing.T) {
	a := statement("a", "1", 0, 2, 1)
	b := statement("b", "1", 1, 2, 5)
	b.Measurements[0].Missing = true
	c, _, err := Merge([]Statement{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if c.Measurements[0].KW != 1 || !c.Measurements[1].Missing || c.Measurements[2].KW != 5 {
		t.Errorf("unexpected measurements: %+v", c.Measurements)
	}
}

package enedis

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

type testRow struct {
	start, end string
	value      any
}

// writeXLSX produces a file in the Enedis format: header on row 16, data from row 17, PRM in C8.
func writeXLSX(t *testing.T, path, prm string, rows []testRow) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Page d'accueil"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.NewSheet(preferredSheet); err != nil {
		t.Fatal(err)
	}
	set := func(cell string, v any) {
		if err := f.SetCellValue(preferredSheet, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	set("B8", "PRM")
	set("C8", prm)
	set("C16", "Début")
	set("D16", "Fin")
	set("E16", "Valeur (en kW)")
	for i, r := range rows {
		n := 17 + i
		set(cellName("C", n), r.start)
		set(cellName("D", n), r.end)
		set(cellName("E", n), r.value)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
}

func cellName(col string, row int) string {
	n, _ := excelize.CoordinatesToCellName(map[string]int{"C": 3, "D": 4, "E": 5}[col], row)
	return n
}

func TestRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.xlsx")
	writeXLSX(t, path, "12345678901234", []testRow{
		{"01/09/2026 00:00:00", "01/09/2026 00:30:00", 1.0},
		{"01/09/2026 00:30:00", "01/09/2026 01:00:00", "NA"},
		{"01/09/2026 01:00:00", "01/09/2026 01:30:00", 2.5},
	})
	s, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.PRM != "12345678901234" {
		t.Errorf("PRM = %q", s.PRM)
	}
	if len(s.Measurements) != 3 || !s.Measurements[1].Missing || s.Measurements[0].Missing {
		t.Fatalf("unexpected measurements: %+v", s.Measurements)
	}
	if got := s.Measurements[0].KWh() + s.Measurements[2].KWh(); got != 1.75 {
		t.Errorf("energy = %v, want 1.75 (kW × 0.5 h) (REQ-IN-05)", got)
	}
}

// REQ-TIME-02: the day of the switch to winter time has 50 slots, all kept.
func TestReadTimeChange(t *testing.T) {
	loc, _ := Timezone()
	tests := []struct {
		name  string
		day   time.Time
		slots int
	}{
		{"winter time", time.Date(2026, 10, 25, 0, 0, 0, 0, loc), 50},
		{"summer time", time.Date(2026, 3, 29, 0, 0, 0, 0, loc), 46},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			end := time.Date(tt.day.Year(), tt.day.Month(), tt.day.Day()+1, 0, 0, 0, 0, loc)
			var rows []testRow
			for d := tt.day; d.Before(end); d = d.Add(30 * time.Minute) {
				rows = append(rows, testRow{d.Format(enedisLayout), d.Add(30 * time.Minute).Format(enedisLayout), 1.0})
			}
			path := filepath.Join(t.TempDir(), "dst.xlsx")
			writeXLSX(t, path, "12345678901234", rows)
			s, err := Read(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Measurements) != tt.slots {
				t.Fatalf("%d measurements, want %d", len(s.Measurements), tt.slots)
			}
			total := 0.0
			for i, m := range s.Measurements {
				if m.Duration() != 30*time.Minute {
					t.Fatalf("measurement %d: duration %v", i, m.Duration())
				}
				if i > 0 && !m.Start.Equal(s.Measurements[i-1].End) {
					t.Fatalf("measurement %d not contiguous", i)
				}
				total += m.KWh()
			}
			if want := float64(tt.slots) * 0.5; total != want {
				t.Errorf("energy = %v, want %v", total, want)
			}
		})
	}
}

func TestListFiles(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b.xlsx", "a.XLSX", "~$a.xlsx", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	other := filepath.Join(t.TempDir(), "z.xlsx")
	if err := os.WriteFile(other, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListFiles([]string{other, dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{other, filepath.Join(dir, "a.XLSX"), filepath.Join(dir, "b.xlsx")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if _, err := ListFiles([]string{t.TempDir()}); err == nil {
		t.Error("a directory without .xlsx must be an error")
	}
}

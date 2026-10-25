package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

const testConfig = `
abonnements:
  - nom: "Base"
    part_fixe: { montant: 365, periodicite: annuelle }
    tarifs:
      - { libelle: Base, debut: "00:00", fin: "00:00", prix_kwh: 0.20 }
  - nom: "HPHC"
    part_fixe: { montant: 30, periodicite: mensuelle }
    tarifs:
      - { libelle: HC, debut: "22:00", fin: "06:00", prix_kwh: 0.10 }
      - { libelle: HP, debut: "06:00", fin: "22:00", prix_kwh: 0.30 }
`

// writeCurve writes a synthetic Enedis export of n 1 kW slots starting at start.
func writeCurve(t *testing.T, path string, start time.Time, n int) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Puissance moyenne soutirée")
	const sheet = "Puissance moyenne soutirée"
	f.SetCellValue(sheet, "C8", "12345678901234")
	f.SetCellValue(sheet, "C16", "Début")
	f.SetCellValue(sheet, "D16", "Fin")
	f.SetCellValue(sheet, "E16", "Valeur (en kW)")
	const layout = "02/01/2006 15:04:05"
	for i := range n {
		d := start.Add(time.Duration(i) * 30 * time.Minute)
		r := 17 + i
		f.SetCellValue(sheet, cell("C", r), d.Format(layout))
		f.SetCellValue(sheet, cell("D", r), d.Add(30*time.Minute).Format(layout))
		f.SetCellValue(sheet, cell("E", r), 1.0)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
}

func cell(col string, row int) string {
	c, _ := excelize.ColumnNameToNumber(col)
	n, _ := excelize.CoordinatesToCellName(c, row)
	return n
}

func TestEndToEnd(t *testing.T) {
	dir := t.TempDir()
	exportsDir := filepath.Join(dir, "exportsDir")
	if err := os.Mkdir(exportsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Europe/Paris")
	day1 := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	// Two days then two days with one in common: 3 distinct days, 1 duplicated day.
	writeCurve(t, filepath.Join(exportsDir, "a.xlsx"), day1, 96)
	writeCurve(t, filepath.Join(exportsDir, "b.xlsx"), day1.AddDate(0, 0, 1), 96)
	configPath := filepath.Join(dir, "abonnements.yaml")
	if err := os.WriteFile(configPath, []byte(testConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "resultat.xlsx")

	var out, errOut bytes.Buffer
	code := run([]string{"--courbe", exportsDir, "--abonnements", configPath, "--sortie", output}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "données en doublon") {
		t.Errorf("overlap warning missing: %q", errOut.String())
	}

	f, err := excelize.OpenFile(output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, s := range []string{"Synthèse journalière", "Détail 30 min", "Paramètres"} {
		if i, _ := f.GetSheetIndex(s); i < 0 {
			t.Errorf("sheet %q missing", s)
		}
	}

	// REQ-OUT-02: summary rows on top, daily detail below, frozen pane.
	if v, _ := f.GetCellValue("Synthèse journalière", "A6"); v != "01/09/2026" {
		t.Errorf("A6 = %q", v)
	}
	if v, _ := f.GetCellValue("Synthèse journalière", "A2"); v != "Total période" {
		t.Errorf("A2 = %q", v)
	}
	panes, err := f.GetPanes("Synthèse journalière")
	if err != nil || !panes.Freeze || panes.XSplit != 1 || panes.YSplit != 5 || panes.TopLeftCell != "B6" {
		t.Errorf("unexpected frozen pane: %+v (%v)", panes, err)
	}

	// 3 days × 24 kWh, Base tariff 0.20 € + fixed fee 3 × 1 €.
	raw, _ := f.GetCellValue("Synthèse journalière", "B2", excelize.Options{RawCellValue: true})
	if kwh := parse(t, raw); math.Abs(kwh-72) > 1e-6 {
		t.Errorf("consumption = %v, want 72", kwh)
	}
	raw, _ = f.GetCellValue("Synthèse journalière", "C2", excelize.Options{RawCellValue: true})
	if cout := parse(t, raw); math.Abs(cout-(72*0.20+3)) > 1e-6 {
		t.Errorf("Base cost = %v, want %v", cout, 72*0.20+3)
	}
	if !strings.Contains(out.String(), "Moins cher") {
		t.Errorf("incomplete console summary: %q", out.String())
	}
}

func TestInvalidOptions(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--abonnements", "x.yaml"}, &out, &errOut); code != 1 {
		t.Errorf("code %d, want 1 (no --courbe)", code)
	}
}

func parse(t *testing.T, s string) float64 {
	t.Helper()
	var v float64
	if _, err := fmt.Sscan(s, &v); err != nil {
		t.Fatalf("unreadable value %q", s)
	}
	return v
}

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

const configTest = `
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

// ecrireCourbe écrit un export Enedis synthétique de n créneaux d'1 kW à partir de debut.
func ecrireCourbe(t *testing.T, chemin string, debut time.Time, n int) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Puissance moyenne soutirée")
	const feuille = "Puissance moyenne soutirée"
	f.SetCellValue(feuille, "C8", "12345678901234")
	f.SetCellValue(feuille, "C16", "Début")
	f.SetCellValue(feuille, "D16", "Fin")
	f.SetCellValue(feuille, "E16", "Valeur (en kW)")
	const layout = "02/01/2006 15:04:05"
	for i := range n {
		d := debut.Add(time.Duration(i) * 30 * time.Minute)
		r := 17 + i
		f.SetCellValue(feuille, cell("C", r), d.Format(layout))
		f.SetCellValue(feuille, cell("D", r), d.Add(30*time.Minute).Format(layout))
		f.SetCellValue(feuille, cell("E", r), 1.0)
	}
	if err := f.SaveAs(chemin); err != nil {
		t.Fatal(err)
	}
}

func cell(col string, ligne int) string {
	c, _ := excelize.ColumnNameToNumber(col)
	n, _ := excelize.CoordinatesToCellName(c, ligne)
	return n
}

func TestBoutEnBout(t *testing.T) {
	dir := t.TempDir()
	exports := filepath.Join(dir, "exports")
	if err := os.Mkdir(exports, 0o755); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Europe/Paris")
	jour1 := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	// Deux jours puis deux jours dont un en commun : 3 jours distincts, 1 jour en doublon.
	ecrireCourbe(t, filepath.Join(exports, "a.xlsx"), jour1, 96)
	ecrireCourbe(t, filepath.Join(exports, "b.xlsx"), jour1.AddDate(0, 0, 1), 96)
	cfg := filepath.Join(dir, "abonnements.yaml")
	if err := os.WriteFile(cfg, []byte(configTest), 0o644); err != nil {
		t.Fatal(err)
	}
	sortie := filepath.Join(dir, "resultat.xlsx")

	var out, errOut bytes.Buffer
	code := run([]string{"--courbe", exports, "--abonnements", cfg, "--sortie", sortie}, &out, &errOut)
	if code != 0 {
		t.Fatalf("code %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "données en doublon") {
		t.Errorf("avertissement de chevauchement absent : %q", errOut.String())
	}

	f, err := excelize.OpenFile(sortie)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, s := range []string{"Synthèse journalière", "Détail 30 min", "Paramètres"} {
		if i, _ := f.GetSheetIndex(s); i < 0 {
			t.Errorf("feuille %q absente", s)
		}
	}

	// REQ-OUT-02 : synthèse à gauche, détail dès la colonne F, volet figé.
	if v, _ := f.GetCellValue("Synthèse journalière", "F1"); v != "01/09/2026" {
		t.Errorf("F1 = %q", v)
	}
	if v, _ := f.GetCellValue("Synthèse journalière", "B1"); v != "Total période" {
		t.Errorf("B1 = %q", v)
	}
	panes, err := f.GetPanes("Synthèse journalière")
	if err != nil || !panes.Freeze || panes.XSplit != 5 || panes.TopLeftCell != "F2" {
		t.Errorf("volet figé inattendu : %+v (%v)", panes, err)
	}

	// 3 jours × 24 kWh, tarif Base 0,20 € + part fixe 3 × 1 €.
	raw, _ := f.GetCellValue("Synthèse journalière", "B2", excelize.Options{RawCellValue: true})
	if kwh := parse(t, raw); math.Abs(kwh-72) > 1e-6 {
		t.Errorf("consommation = %v, attendu 72", kwh)
	}
	raw, _ = f.GetCellValue("Synthèse journalière", "B3", excelize.Options{RawCellValue: true})
	if cout := parse(t, raw); math.Abs(cout-(72*0.20+3)) > 1e-6 {
		t.Errorf("coût Base = %v, attendu %v", cout, 72*0.20+3)
	}
	if !strings.Contains(out.String(), "Moins cher") {
		t.Errorf("résumé console incomplet : %q", out.String())
	}
}

func TestOptionsInvalides(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--abonnements", "x.yaml"}, &out, &errOut); code != 1 {
		t.Errorf("code %d, attendu 1 (aucune --courbe)", code)
	}
}

func parse(t *testing.T, s string) float64 {
	t.Helper()
	var v float64
	if _, err := fmt.Sscan(s, &v); err != nil {
		t.Fatalf("valeur %q illisible", s)
	}
	return v
}

package enedis

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

type ligneTest struct {
	debut, fin string
	valeur     any
}

// ecrireXLSX produit un fichier au format Enedis : en-tête en ligne 16, données dès la ligne 17, PRM en C8.
func ecrireXLSX(t *testing.T, chemin, prm string, lignes []ligneTest) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Page d'accueil"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.NewSheet(feuillePreferee); err != nil {
		t.Fatal(err)
	}
	set := func(cell string, v any) {
		if err := f.SetCellValue(feuillePreferee, cell, v); err != nil {
			t.Fatal(err)
		}
	}
	set("B8", "PRM")
	set("C8", prm)
	set("C16", "Début")
	set("D16", "Fin")
	set("E16", "Valeur (en kW)")
	for i, l := range lignes {
		r := 17 + i
		set(cellule2("C", r), l.debut)
		set(cellule2("D", r), l.fin)
		set(cellule2("E", r), l.valeur)
	}
	if err := f.SaveAs(chemin); err != nil {
		t.Fatal(err)
	}
}

func cellule2(col string, ligne int) string {
	n, _ := excelize.CoordinatesToCellName(map[string]int{"C": 3, "D": 4, "E": 5}[col], ligne)
	return n
}

func TestLire(t *testing.T) {
	chemin := filepath.Join(t.TempDir(), "a.xlsx")
	ecrireXLSX(t, chemin, "12345678901234", []ligneTest{
		{"01/09/2026 00:00:00", "01/09/2026 00:30:00", 1.0},
		{"01/09/2026 00:30:00", "01/09/2026 01:00:00", "NA"},
		{"01/09/2026 01:00:00", "01/09/2026 01:30:00", 2.5},
	})
	r, err := Lire(chemin)
	if err != nil {
		t.Fatal(err)
	}
	if r.PRM != "12345678901234" {
		t.Errorf("PRM = %q", r.PRM)
	}
	if len(r.Mesures) != 3 || !r.Mesures[1].Manquante || r.Mesures[0].Manquante {
		t.Fatalf("mesures inattendues : %+v", r.Mesures)
	}
	if got := r.Mesures[0].KWh() + r.Mesures[2].KWh(); got != 1.75 {
		t.Errorf("énergie = %v, attendu 1.75 (kW × 0,5 h) (REQ-IN-05)", got)
	}
}

// REQ-TIME-02 : le jour du passage à l'heure d'hiver compte 50 créneaux, tous conservés.
func TestLireChangementHeure(t *testing.T) {
	loc, _ := Fuseau()
	tests := []struct {
		nom      string
		jour     time.Time
		creneaux int
	}{
		{"heure d'hiver", time.Date(2026, 10, 25, 0, 0, 0, 0, loc), 50},
		{"heure d'été", time.Date(2026, 3, 29, 0, 0, 0, 0, loc), 46},
	}
	for _, tt := range tests {
		t.Run(tt.nom, func(t *testing.T) {
			fin := time.Date(tt.jour.Year(), tt.jour.Month(), tt.jour.Day()+1, 0, 0, 0, 0, loc)
			var lignes []ligneTest
			for d := tt.jour; d.Before(fin); d = d.Add(30 * time.Minute) {
				lignes = append(lignes, ligneTest{d.Format(layoutEnedis), d.Add(30 * time.Minute).Format(layoutEnedis), 1.0})
			}
			chemin := filepath.Join(t.TempDir(), "dst.xlsx")
			ecrireXLSX(t, chemin, "12345678901234", lignes)
			r, err := Lire(chemin)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Mesures) != tt.creneaux {
				t.Fatalf("%d mesures, attendu %d", len(r.Mesures), tt.creneaux)
			}
			total := 0.0
			for i, m := range r.Mesures {
				if m.Duree() != 30*time.Minute {
					t.Fatalf("mesure %d : durée %v", i, m.Duree())
				}
				if i > 0 && !m.Debut.Equal(r.Mesures[i-1].Fin) {
					t.Fatalf("mesure %d non contiguë", i)
				}
				total += m.KWh()
			}
			if want := float64(tt.creneaux) * 0.5; total != want {
				t.Errorf("énergie = %v, attendu %v", total, want)
			}
		})
	}
}

func TestListerFichiers(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b.xlsx", "a.XLSX", "~$a.xlsx", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	autre := filepath.Join(t.TempDir(), "z.xlsx")
	if err := os.WriteFile(autre, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListerFichiers([]string{autre, dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{autre, filepath.Join(dir, "a.XLSX"), filepath.Join(dir, "b.xlsx")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if _, err := ListerFichiers([]string{t.TempDir()}); err == nil {
		t.Error("un répertoire sans .xlsx doit être une erreur")
	}
}

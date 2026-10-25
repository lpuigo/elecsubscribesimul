package courbe

import (
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func releve(source, prm string, debutCreneau, n int, kw float64) Releve {
	r := Releve{Source: source, PRM: prm}
	for i := 0; i < n; i++ {
		d := t0.Add(time.Duration(debutCreneau+i) * 30 * time.Minute)
		r.Mesures = append(r.Mesures, Mesure{Debut: d, Fin: d.Add(30 * time.Minute), KW: kw})
	}
	return r
}

func TestFusionner(t *testing.T) {
	tests := []struct {
		nom           string
		releves       []Releve
		mesures       int
		manquantes    int
		avertissement string // sous-chaîne attendue, vide si aucun
		erreur        string
	}{
		{"contigus", []Releve{releve("a", "1", 0, 4, 1), releve("b", "1", 4, 4, 1)}, 8, 0, "", ""},
		{"chevauchement", []Releve{releve("a", "1", 0, 4, 1), releve("b", "1", 2, 4, 2)}, 6, 0, "2 créneau(x) de \"b\"", ""},
		{"trou", []Releve{releve("a", "1", 0, 2, 1), releve("b", "1", 5, 2, 1)}, 7, 3, "trou", ""},
		{"PRM différents", []Releve{releve("a", "1", 0, 2, 1), releve("b", "2", 2, 2, 1)}, 0, 0, "", "PRM différents"},
		{"aucun relevé", nil, 0, 0, "", "aucun"},
	}
	for _, tt := range tests {
		t.Run(tt.nom, func(t *testing.T) {
			c, warns, err := Fusionner(tt.releves)
			if tt.erreur != "" {
				if err == nil || !strings.Contains(err.Error(), tt.erreur) {
					t.Fatalf("erreur = %v, attendu %q", err, tt.erreur)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Mesures) != tt.mesures {
				t.Errorf("%d mesures, attendu %d", len(c.Mesures), tt.mesures)
			}
			manq := 0
			for i, m := range c.Mesures {
				if m.Manquante {
					manq++
				}
				if i > 0 && !m.Debut.Equal(c.Mesures[i-1].Fin) {
					t.Errorf("série non continue à l'indice %d", i)
				}
			}
			if manq != tt.manquantes {
				t.Errorf("%d manquantes, attendu %d", manq, tt.manquantes)
			}
			if tt.avertissement == "" && len(warns) != 0 {
				t.Errorf("avertissements inattendus : %v", warns)
			}
			if tt.avertissement != "" && !strings.Contains(strings.Join(warns, "\n"), tt.avertissement) {
				t.Errorf("avertissements %v, attendu %q", warns, tt.avertissement)
			}
		})
	}
}

// REQ-IN-09 : le dernier relevé lu gagne, y compris quand sa valeur est manquante.
func TestFusionnerDernierGagne(t *testing.T) {
	a := releve("a", "1", 0, 2, 1)
	b := releve("b", "1", 1, 2, 5)
	b.Mesures[0].Manquante = true
	c, _, err := Fusionner([]Releve{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if c.Mesures[0].KW != 1 || !c.Mesures[1].Manquante || c.Mesures[2].KW != 5 {
		t.Errorf("mesures inattendues : %+v", c.Mesures)
	}
}

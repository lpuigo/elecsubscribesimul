package abonnement

import (
	"strings"
	"testing"
	"time"
)

func tarif(couleur, lib, debut, fin string) Tarif {
	d, _ := ParseMinutes(debut)
	f, _ := ParseMinutes(fin)
	return Tarif{Couleur: couleur, Libelle: lib, Debut: d, Fin: f, PrixKWh: 0.2}
}

func TestValidate(t *testing.T) {
	base := PartFixe{Montant: 100, Periodicite: Annuelle}
	tests := []struct {
		nom    string
		tarifs []Tarif
		erreur string
	}{
		{"base", []Tarif{tarif("", "Base", "00:00", "00:00")}, ""},
		{"hphc", []Tarif{tarif("", "HC", "22:00", "06:00"), tarif("", "HP", "06:00", "22:00")}, ""},
		{"chevauchement", []Tarif{tarif("", "HC", "21:00", "06:00"), tarif("", "HP", "06:00", "22:00")}, "chevauchent : 21:00-22:00"},
		{"trou", []Tarif{tarif("", "HC", "22:00", "06:00"), tarif("", "HP", "07:00", "22:00")}, "non couvertes : 06:00-07:00"},
		{"couleur incomplète", []Tarif{
			tarif("bleu", "HC", "22:00", "06:00"), tarif("bleu", "HP", "06:00", "22:00"),
			tarif("rouge", "HP", "06:00", "22:00"),
		}, `couleur "rouge" : plages horaires non couvertes`},
		{"tarif sans couleur + couleur", []Tarif{
			tarif("", "HC", "22:00", "06:00"), tarif("bleu", "HP", "06:00", "22:00"), tarif("rouge", "HP", "06:00", "22:00"),
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.nom, func(t *testing.T) {
			err := Abonnement{Nom: "x", PartFixe: base, Tarifs: tt.tarifs}.Validate()
			if tt.erreur == "" && err != nil {
				t.Fatalf("erreur inattendue : %v", err)
			}
			if tt.erreur != "" && (err == nil || !strings.Contains(err.Error(), tt.erreur)) {
				t.Fatalf("erreur = %v, attendu %q", err, tt.erreur)
			}
		})
	}
}

func TestTarifContient(t *testing.T) {
	hc := tarif("", "HC", "22:00", "06:00")
	hp := tarif("", "HP", "06:00", "22:00")
	// REQ : créneaux 21:30, 22:00, 05:30 et 06:00
	for _, c := range []struct {
		heure string
		hc    bool
	}{{"21:30", false}, {"22:00", true}, {"05:30", true}, {"06:00", false}} {
		m, _ := ParseMinutes(c.heure)
		if hc.Contient(m) != c.hc || hp.Contient(m) == c.hc {
			t.Errorf("%s : HC=%v HP=%v", c.heure, hc.Contient(m), hp.Contient(m))
		}
	}
}

func TestMontantJour(t *testing.T) {
	tests := []struct {
		nom  string
		pf   PartFixe
		jour time.Time
		want float64
	}{
		{"annuel bissextile", PartFixe{366, Annuelle}, time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC), 1},
		{"annuel normal", PartFixe{365, Annuelle}, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), 1},
		{"mensuel 28 jours", PartFixe{28, Mensuelle}, time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC), 1},
		{"mensuel 30 jours", PartFixe{30, Mensuelle}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), 1},
		{"mensuel 31 jours", PartFixe{31, Mensuelle}, time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), 1},
	}
	for _, tt := range tests {
		if got := tt.pf.MontantJour(tt.jour); got != tt.want {
			t.Errorf("%s : %v, attendu %v", tt.nom, got, tt.want)
		}
	}
}

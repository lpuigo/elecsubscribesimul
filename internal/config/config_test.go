package config

import (
	"strings"
	"testing"
)

func TestParser(t *testing.T) {
	const ok = `
abonnements:
  - nom: "Tempo"
    part_fixe: { montant: 210, periodicite: annuelle }
    debut_journee_tarifaire: "06:00"
    tarifs:
      - { couleur: Bleu, libelle: HC, debut: "22:00", fin: "06:00", prix_kwh: 0.13 }
      - { couleur: bleu, libelle: HP, debut: "06:00", fin: "22:00", prix_kwh: 0.16 }
`
	abos, err := Parser([]byte(ok))
	if err != nil {
		t.Fatal(err)
	}
	if len(abos) != 1 || !abos[0].UtiliseCouleurs() || abos[0].DebutJourneeTarifaire != 360 || abos[0].Tarifs[0].Couleur != "bleu" {
		t.Errorf("abonnement mal chargé : %+v", abos)
	}

	tests := []struct{ nom, yaml, erreur string }{
		{"champ inconnu", "abonnements:\n  - nom: x\n    inconnu: 1\n", "inconnu"},
		{"aucun abonnement", "abonnements: []\n", "aucun abonnement"},
		{"trou", `
abonnements:
  - nom: "X"
    part_fixe: { montant: 1, periodicite: annuelle }
    tarifs:
      - { libelle: HP, debut: "06:00", fin: "22:00", prix_kwh: 0.2 }
`, "non couvertes"},
	}
	for _, tt := range tests {
		if _, err := Parser([]byte(tt.yaml)); err == nil || !strings.Contains(err.Error(), tt.erreur) {
			t.Errorf("%s : erreur = %v, attendu %q", tt.nom, err, tt.erreur)
		}
	}
}

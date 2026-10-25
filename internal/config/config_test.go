package config

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	const ok = `
abonnements:
  - nom: "Tempo"
    part_fixe: { montant: 210, periodicite: annuelle }
    debut_journee_tarifaire: "06:00"
    tarifs:
      - { couleur: Bleu, libelle: HC, debut: "22:00", fin: "06:00", prix_kwh: 0.13 }
      - { couleur: bleu, libelle: HP, debut: "06:00", fin: "22:00", prix_kwh: 0.16 }
`
	subs, err := Parse([]byte(ok))
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || !subs[0].UsesColors() || subs[0].TariffDayStart != 360 || subs[0].Tariffs[0].Color != "bleu" {
		t.Errorf("subscription badly loaded: %+v", subs)
	}

	tests := []struct{ name, yaml, wantErr string }{
		{"unknown field", "abonnements:\n  - nom: x\n    inconnu: 1\n", "inconnu"},
		{"no subscription", "abonnements: []\n", "aucun abonnement"},
		{"gap", `
abonnements:
  - nom: "X"
    part_fixe: { montant: 1, periodicite: annuelle }
    tarifs:
      - { libelle: HP, debut: "06:00", fin: "22:00", prix_kwh: 0.2 }
`, "non couvertes"},
	}
	for _, tt := range tests {
		if _, err := Parse([]byte(tt.yaml)); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%s: error = %v, want %q", tt.name, err, tt.wantErr)
		}
	}
}

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

	// REQ-CALC-05: default tariff and several ranges.
	const multi = `
abonnements:
  - nom: "HP/HC"
    part_fixe: { montant: 20, periodicite: mensuelle }
    tarifs:
      - { libelle: HP, prix_kwh: 0.22 }
      - libelle: HC
        prix_kwh: 0.16
        plages: [ { debut: "01:00", fin: "07:00" }, { debut: "14:00", fin: "15:00" } ]
`
	subs, err = Parse([]byte(multi))
	if err != nil {
		t.Fatal(err)
	}
	if tt := subs[0].Tariffs; !tt[0].IsDefault() || len(tt[1].Ranges) != 2 || tt[1].Ranges[1].Start != 14*60 || tt[1].Ranges[1].End != 15*60 {
		t.Errorf("tariffs badly loaded: %+v", tt)
	}

	tests := []struct{ name, yaml, wantErr string }{
		{"debut/fin + plages", `
abonnements:
  - nom: "X"
    part_fixe: { montant: 1, periodicite: annuelle }
    tarifs:
      - { libelle: HC, debut: "01:00", fin: "07:00", plages: [ { debut: "14:00", fin: "15:00" } ], prix_kwh: 0.1 }
`, "ne peuvent pas être utilisés ensemble"},
		{"debut without fin", `
abonnements:
  - nom: "X"
    part_fixe: { montant: 1, periodicite: annuelle }
    tarifs:
      - { libelle: HC, debut: "01:00", prix_kwh: 0.1 }
`, "renseignés ensemble"},
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

package tarification

import (
	"math"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"elecsubscribesimul/internal/abonnement"
	"elecsubscribesimul/internal/calendrier"
	"elecsubscribesimul/internal/courbe"
)

var paris, _ = time.LoadLocation("Europe/Paris")

func tarif(couleur, lib, debut, fin string, prix float64) abonnement.Tarif {
	d, _ := abonnement.ParseMinutes(debut)
	f, _ := abonnement.ParseMinutes(fin)
	return abonnement.Tarif{Couleur: couleur, Libelle: lib, Debut: d, Fin: f, PrixKWh: prix}
}

// serie produit des créneaux de 30 min à puissance constante (1 kW => 0,5 kWh par créneau).
func serie(debut time.Time, n int, kw float64) courbe.Courbe {
	c := courbe.Courbe{}
	for i := 0; i < n; i++ {
		d := debut.Add(time.Duration(i) * 30 * time.Minute)
		c.Mesures = append(c.Mesures, courbe.Mesure{Debut: d, Fin: d.Add(30 * time.Minute), KW: kw})
	}
	return c
}

func proche(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

var (
	aboBase = abonnement.Abonnement{Nom: "Base", PartFixe: abonnement.PartFixe{Montant: 365, Periodicite: abonnement.Annuelle},
		Tarifs: []abonnement.Tarif{tarif("", "Base", "00:00", "00:00", 0.20)}}
	aboHPHC = abonnement.Abonnement{Nom: "HPHC", PartFixe: abonnement.PartFixe{Montant: 365, Periodicite: abonnement.Annuelle},
		Tarifs: []abonnement.Tarif{tarif("", "HC", "22:00", "06:00", 0.10), tarif("", "HP", "06:00", "22:00", 0.30)}}
	aboTempo = abonnement.Abonnement{Nom: "Tempo", PartFixe: abonnement.PartFixe{Montant: 365, Periodicite: abonnement.Annuelle},
		DebutJourneeTarifaire: 6 * 60,
		Tarifs: []abonnement.Tarif{
			tarif("bleu", "HC", "22:00", "06:00", 0.10), tarif("bleu", "HP", "06:00", "22:00", 0.20),
			tarif("rouge", "HC", "22:00", "06:00", 0.15), tarif("rouge", "HP", "06:00", "22:00", 0.70),
		}}
)

func TestBase(t *testing.T) {
	c := serie(time.Date(2026, 9, 1, 0, 0, 0, 0, paris), 96, 1) // 2 jours, 48 kWh
	res, err := Calculer(c, []abonnement.Abonnement{aboBase}, calendrier.Calendrier{})
	if err != nil {
		t.Fatal(err)
	}
	a := res.Abonnements[0]
	if !proche(a.TotalVariable(), 48*0.20) || !proche(a.TotalFixe(), 2) || len(res.Jours) != 2 {
		t.Errorf("variable=%v fixe=%v jours=%d", a.TotalVariable(), a.TotalFixe(), len(res.Jours))
	}
	if !proche(res.Extrapolation(0), (48*0.20+2)/2*365) {
		t.Errorf("extrapolation = %v", res.Extrapolation(0))
	}
}

func TestHPHCFrontieres(t *testing.T) {
	tests := []struct {
		heure   time.Time
		libelle string
	}{
		{time.Date(2026, 9, 1, 21, 30, 0, 0, paris), "HP"},
		{time.Date(2026, 9, 1, 22, 0, 0, 0, paris), "HC"},
		{time.Date(2026, 9, 1, 5, 30, 0, 0, paris), "HC"},
		{time.Date(2026, 9, 1, 6, 0, 0, 0, paris), "HP"},
	}
	for _, tt := range tests {
		res, err := Calculer(serie(tt.heure, 1, 1), []abonnement.Abonnement{aboHPHC}, calendrier.Calendrier{})
		if err != nil {
			t.Fatal(err)
		}
		if got := res.Abonnements[0].Creneaux[0].Libelle; got != tt.libelle {
			t.Errorf("%s : %s, attendu %s", tt.heure.Format("15:04"), got, tt.libelle)
		}
	}
}

// REQ-CALC-01 : frontière HC à 22:15, créneau 22:00-22:30 réparti 15 min / 15 min.
func TestProrataFrontiereNonAlignee(t *testing.T) {
	abo := aboHPHC
	abo.Tarifs = []abonnement.Tarif{tarif("", "HC", "22:15", "06:00", 0.10), tarif("", "HP", "06:00", "22:15", 0.30)}
	res, err := Calculer(serie(time.Date(2026, 9, 1, 22, 0, 0, 0, paris), 1, 2), []abonnement.Abonnement{abo}, calendrier.Calendrier{})
	if err != nil {
		t.Fatal(err)
	}
	want := 0.5 * (0.30 + 0.10) / 1 // 1 kWh au total, moitié HP, moitié HC
	if got := res.Abonnements[0].Creneaux[0].Cout; !proche(got, want) {
		t.Errorf("coût = %v, attendu %v", got, want)
	}
}

// REQ-TIME-03 : le créneau de J à 01:00 prend la couleur de J-1.
func TestTempoJourneeTarifaire(t *testing.T) {
	cal := calendrier.Nouveau(map[string]string{"2026-09-01": "bleu", "2026-09-02": "rouge"}, "")
	// 02/09 01:00 (HC, journée tarifaire du 01/09 = bleu) puis 02/09 10:00 (HP, journée du 02/09 = rouge)
	for _, tt := range []struct {
		heure time.Time
		prix  float64
	}{
		{time.Date(2026, 9, 2, 1, 0, 0, 0, paris), 0.10},
		{time.Date(2026, 9, 2, 10, 0, 0, 0, paris), 0.70},
	} {
		res, err := Calculer(serie(tt.heure, 1, 2), []abonnement.Abonnement{aboTempo}, cal)
		if err != nil {
			t.Fatal(err)
		}
		if got := res.Abonnements[0].Creneaux[0].Cout; !proche(got, tt.prix) {
			t.Errorf("%s : coût %v, attendu %v", tt.heure.Format("02/01 15:04"), got, tt.prix)
		}
	}
}

func TestTempoCouleursManquantesEtDefaut(t *testing.T) {
	c := serie(time.Date(2026, 9, 1, 8, 0, 0, 0, paris), 2, 1)
	_, err := Calculer(c, []abonnement.Abonnement{aboTempo}, calendrier.Calendrier{})
	if err == nil || !strings.Contains(err.Error(), "2026-09-01") {
		t.Fatalf("erreur = %v", err)
	}
	res, err := Calculer(c, []abonnement.Abonnement{aboTempo}, calendrier.Nouveau(nil, "bleu"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CouleursSupposees) != 1 {
		t.Errorf("couleurs supposées : %v", res.CouleursSupposees)
	}
}

// Un abonnement sans couleur ne dépend pas du calendrier (REQ-IN-13).
func TestSansCouleurIgnoreCalendrier(t *testing.T) {
	c := serie(time.Date(2026, 9, 1, 8, 0, 0, 0, paris), 2, 1)
	if _, err := Calculer(c, []abonnement.Abonnement{aboBase, aboHPHC}, calendrier.Calendrier{}); err != nil {
		t.Fatal(err)
	}
}

func TestManquantsExclus(t *testing.T) {
	c := serie(time.Date(2026, 9, 1, 8, 0, 0, 0, paris), 4, 1)
	c.Mesures[1].Manquante = true
	res, err := Calculer(c, []abonnement.Abonnement{aboBase}, calendrier.Calendrier{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Manquants() != 1 || !proche(res.TotalKWh(), 1.5) || !proche(res.Abonnements[0].TotalVariable(), 0.30) {
		t.Errorf("manquants=%d kWh=%v coût=%v", res.Manquants(), res.TotalKWh(), res.Abonnements[0].TotalVariable())
	}
}

// Jour du passage à l'heure d'hiver : 50 créneaux, 25 kWh facturés.
func TestChangementHeureHiver(t *testing.T) {
	c := courbe.Courbe{}
	jour := time.Date(2026, 10, 25, 0, 0, 0, 0, paris)
	for d := jour; d.Before(jour.AddDate(0, 0, 1)); d = d.Add(30 * time.Minute) {
		c.Mesures = append(c.Mesures, courbe.Mesure{Debut: d, Fin: d.Add(30 * time.Minute), KW: 1})
	}
	res, err := Calculer(c, []abonnement.Abonnement{aboBase}, calendrier.Calendrier{})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Mesures) != 50 || len(res.Jours) != 1 || !proche(res.TotalKWh(), 25) {
		t.Errorf("créneaux=%d jours=%d kWh=%v", len(c.Mesures), len(res.Jours), res.TotalKWh())
	}
}

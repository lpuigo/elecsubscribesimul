package tarification

import (
	"time"

	"elecsubscribesimul/internal/abonnement"
)

const joursParAn = 365

// Jour regroupe les données d'un jour civil, communes à tous les abonnements.
type Jour struct {
	Date      time.Time
	KWh       float64
	MaxKW     float64
	Manquants int
	Couleur   string // couleur du jour civil, vide si aucun abonnement n'utilise de couleurs
}

type CoutJour struct {
	Variable      float64
	Fixe          float64
	KWhParLibelle map[string]float64
}

func (c CoutJour) Total() float64 { return c.Variable + c.Fixe }

// CoutCreneau est le coût variable d'un créneau et le libellé de tarif majoritaire.
type CoutCreneau struct {
	Cout    float64
	Libelle string
}

type CoutAbonnement struct {
	Abonnement abonnement.Abonnement
	Jours      []CoutJour
	Creneaux   []CoutCreneau
}

func (c CoutAbonnement) TotalVariable() float64 {
	t := 0.0
	for _, j := range c.Jours {
		t += j.Variable
	}
	return t
}

func (c CoutAbonnement) TotalFixe() float64 {
	t := 0.0
	for _, j := range c.Jours {
		t += j.Fixe
	}
	return t
}

func (c CoutAbonnement) Total() float64 { return c.TotalVariable() + c.TotalFixe() }

type Resultat struct {
	Jours             []Jour
	Abonnements       []CoutAbonnement
	CouleursSupposees []time.Time
}

func (r Resultat) TotalKWh() float64 {
	t := 0.0
	for _, j := range r.Jours {
		t += j.KWh
	}
	return t
}

func (r Resultat) Manquants() int {
	n := 0
	for _, j := range r.Jours {
		n += j.Manquants
	}
	return n
}

// MoyenneJour est le coût moyen par jour civil ; Extrapolation le prolonge sur 365 jours (§3.4).
func (r Resultat) MoyenneJour(i int) float64 {
	return r.Abonnements[i].Total() / float64(len(r.Jours))
}

func (r Resultat) Extrapolation(i int) float64 { return r.MoyenneJour(i) * joursParAn }

// MoinsCher renvoie l'indice de l'abonnement au coût total le plus faible.
func (r Resultat) MoinsCher() int {
	best := 0
	for i := range r.Abonnements {
		if r.Abonnements[i].Total() < r.Abonnements[best].Total() {
			best = i
		}
	}
	return best
}

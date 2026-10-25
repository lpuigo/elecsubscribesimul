// Package courbe porte le modèle de la courbe de charge (mesures) et la fusion de plusieurs relevés.
package courbe

import "time"

// Mesure est la puissance moyenne soutirée sur un créneau.
type Mesure struct {
	Debut     time.Time
	Fin       time.Time
	KW        float64
	Manquante bool
}

func (m Mesure) Duree() time.Duration { return m.Fin.Sub(m.Debut) }

// KWh convertit la puissance moyenne en énergie (REQ-IN-05).
func (m Mesure) KWh() float64 { return m.KW * m.Duree().Hours() }

// Releve est le contenu d'un fichier source.
type Releve struct {
	Source  string
	PRM     string
	Mesures []Mesure
}

// Courbe est la série fusionnée, triée par instant de début.
type Courbe struct {
	PRM     string
	Sources []string
	Mesures []Mesure
}

func (c Courbe) Debut() time.Time { return c.Mesures[0].Debut }
func (c Courbe) Fin() time.Time   { return c.Mesures[len(c.Mesures)-1].Fin }

// Package abonnement définit les offres tarifaires et leur validation.
package abonnement

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Periodicite string

const (
	Annuelle  Periodicite = "annuelle"
	Mensuelle Periodicite = "mensuelle"
)

// Tarif est un prix du kWh sur une plage horaire, éventuellement limité à une couleur de jour.
// Couleur vide = tous les jours.
type Tarif struct {
	Couleur string
	Libelle string
	Debut   Minutes
	Fin     Minutes
	PrixKWh float64
}

// Contient indique si la minute m appartient à la plage ; debut == fin signifie toute la journée,
// fin < debut une plage à cheval sur minuit.
func (t Tarif) Contient(m Minutes) bool {
	fin := t.Fin % minutesParJour
	debut := t.Debut % minutesParJour
	switch {
	case debut == fin:
		return true
	case debut < fin:
		return m >= debut && m < fin
	default:
		return m >= debut || m < fin
	}
}

type PartFixe struct {
	Montant     float64
	Periodicite Periodicite
}

// MontantJour est la part fixe imputée à un jour civil (REQ-CALC-03).
func (p PartFixe) MontantJour(jour time.Time) float64 {
	if p.Periodicite == Mensuelle {
		return p.Montant / float64(joursDuMois(jour))
	}
	return p.Montant / float64(joursDeAnnee(jour.Year()))
}

func joursDuMois(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func joursDeAnnee(annee int) int {
	debut := time.Date(annee, 1, 1, 0, 0, 0, 0, time.UTC)
	return int(debut.AddDate(1, 0, 0).Sub(debut).Hours() / 24)
}

type Abonnement struct {
	Nom                   string
	PuissanceKVA          int
	PartFixe              PartFixe
	DebutJourneeTarifaire Minutes
	Tarifs                []Tarif
}

// Couleurs renvoie les couleurs explicitement tarifées, triées.
func (a Abonnement) Couleurs() []string {
	vues := map[string]bool{}
	var out []string
	for _, t := range a.Tarifs {
		if t.Couleur != "" && !vues[t.Couleur] {
			vues[t.Couleur] = true
			out = append(out, t.Couleur)
		}
	}
	sort.Strings(out)
	return out
}

func (a Abonnement) UtiliseCouleurs() bool { return len(a.Couleurs()) > 0 }

// Libelles renvoie les libellés de tarif dans l'ordre d'apparition.
func (a Abonnement) Libelles() []string {
	vus := map[string]bool{}
	var out []string
	for _, t := range a.Tarifs {
		if !vus[t.Libelle] {
			vus[t.Libelle] = true
			out = append(out, t.Libelle)
		}
	}
	return out
}

// Tarif cherche le tarif applicable à une couleur de jour (vide si l'abonnement n'en a pas) et une minute.
func (a Abonnement) Tarif(couleur string, m Minutes) (Tarif, bool) {
	for _, t := range a.Tarifs {
		if (t.Couleur == couleur || t.Couleur == "") && t.Contient(m) {
			return t, true
		}
	}
	return Tarif{}, false
}

// Validate vérifie la cohérence de l'abonnement et regroupe toutes les anomalies (REQ-CALC-02, REQ-IN-14).
func (a Abonnement) Validate() error {
	var errs []error
	if strings.TrimSpace(a.Nom) == "" {
		errs = append(errs, errors.New("abonnement sans nom"))
	}
	ajout := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("abonnement %q : "+format, append([]any{a.Nom}, args...)...))
	}
	if a.PartFixe.Montant < 0 {
		ajout("part fixe négative")
	}
	if a.PartFixe.Periodicite != Annuelle && a.PartFixe.Periodicite != Mensuelle {
		ajout("périodicité %q invalide (annuelle ou mensuelle)", a.PartFixe.Periodicite)
	}
	if a.DebutJourneeTarifaire < 0 || a.DebutJourneeTarifaire >= minutesParJour {
		ajout("debut_journee_tarifaire hors de la journée")
	}
	if len(a.Tarifs) == 0 {
		ajout("aucun tarif")
	}
	for _, t := range a.Tarifs {
		if t.PrixKWh < 0 {
			ajout("prix négatif pour le tarif %q", t.Libelle)
		}
		if strings.TrimSpace(t.Libelle) == "" {
			ajout("tarif sans libellé")
		}
	}
	if len(a.Tarifs) > 0 {
		couleurs := a.Couleurs()
		if len(couleurs) == 0 {
			couleurs = []string{""}
		}
		for _, c := range couleurs {
			errs = append(errs, a.verifierCouverture(c)...)
		}
	}
	return errors.Join(errs...)
}

func (a Abonnement) verifierCouverture(couleur string) []error {
	var comptes [minutesParJour]int
	for _, t := range a.Tarifs {
		if t.Couleur != couleur && t.Couleur != "" {
			continue
		}
		for m := Minutes(0); m < minutesParJour; m++ {
			if t.Contient(m) {
				comptes[m]++
			}
		}
	}
	contexte := fmt.Sprintf("abonnement %q", a.Nom)
	if couleur != "" {
		contexte += fmt.Sprintf(", couleur %q", couleur)
	}
	var errs []error
	if p := plages(comptes[:], func(n int) bool { return n == 0 }); len(p) > 0 {
		errs = append(errs, fmt.Errorf("%s : plages horaires non couvertes : %s", contexte, strings.Join(p, ", ")))
	}
	if p := plages(comptes[:], func(n int) bool { return n > 1 }); len(p) > 0 {
		errs = append(errs, fmt.Errorf("%s : plages horaires qui se chevauchent : %s", contexte, strings.Join(p, ", ")))
	}
	return errs
}

func plages(comptes []int, mauvais func(int) bool) []string {
	var out []string
	debut := -1
	for m := 0; m <= len(comptes); m++ {
		if m < len(comptes) && mauvais(comptes[m]) {
			if debut < 0 {
				debut = m
			}
		} else if debut >= 0 {
			out = append(out, fmt.Sprintf("%s-%s", Minutes(debut), Minutes(m)))
			debut = -1
		}
	}
	return out
}

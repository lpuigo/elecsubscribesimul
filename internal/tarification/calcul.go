// Package tarification est le moteur de calcul pur : mesures + abonnements + calendrier -> coûts.
package tarification

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"elecsubscribesimul/internal/abonnement"
	"elecsubscribesimul/internal/calendrier"
	"elecsubscribesimul/internal/courbe"
)

type couleurJour struct {
	couleur string
	supposee,
	ok bool
}

type calcul struct {
	cal       calendrier.Calendrier
	cache     map[string]couleurJour
	manquants map[string]bool
	supposees map[string]time.Time
}

// Calculer applique chaque abonnement à chaque créneau de la courbe.
func Calculer(c courbe.Courbe, abos []abonnement.Abonnement, cal calendrier.Calendrier) (Resultat, error) {
	if len(c.Mesures) == 0 {
		return Resultat{}, errors.New("courbe de charge vide")
	}
	k := &calcul{cal: cal, cache: map[string]couleurJour{}, manquants: map[string]bool{}, supposees: map[string]time.Time{}}

	jours, indexJour := agregerJours(c.Mesures)
	res := Resultat{Jours: jours}
	for _, a := range abos {
		res.Abonnements = append(res.Abonnements, k.coutAbonnement(a, c.Mesures, jours, indexJour))
	}
	if len(k.manquants) > 0 {
		return Resultat{}, fmt.Errorf("couleur de jour manquante pour : %s (fournir --couleurs ou --couleur-par-defaut)", listerJours(k.manquants))
	}
	k.couleursCiviles(&res)
	for _, t := range k.supposees {
		res.CouleursSupposees = append(res.CouleursSupposees, t)
	}
	sort.Slice(res.CouleursSupposees, func(i, j int) bool { return res.CouleursSupposees[i].Before(res.CouleursSupposees[j]) })
	return res, nil
}

func jourCivil(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func agregerJours(mesures []courbe.Mesure) ([]Jour, map[string]int) {
	var jours []Jour
	index := map[string]int{}
	for _, m := range mesures {
		d := jourCivil(m.Debut)
		cle := d.Format("2006-01-02")
		i, ok := index[cle]
		if !ok {
			i = len(jours)
			index[cle] = i
			jours = append(jours, Jour{Date: d})
		}
		if m.Manquante {
			jours[i].Manquants++
			continue
		}
		jours[i].KWh += m.KWh()
		if m.KW > jours[i].MaxKW {
			jours[i].MaxKW = m.KW
		}
	}
	return jours, index
}

func (k *calcul) coutAbonnement(a abonnement.Abonnement, mesures []courbe.Mesure, jours []Jour, indexJour map[string]int) CoutAbonnement {
	out := CoutAbonnement{Abonnement: a, Creneaux: make([]CoutCreneau, len(mesures)), Jours: make([]CoutJour, len(jours))}
	for i, j := range jours {
		out.Jours[i] = CoutJour{Fixe: a.PartFixe.MontantJour(j.Date), KWhParLibelle: map[string]float64{}}
	}
	for i, m := range mesures {
		if m.Manquante {
			continue
		}
		cout, parLibelle, dominant := k.coutCreneau(a, m)
		out.Creneaux[i] = CoutCreneau{Cout: cout, Libelle: dominant}
		jour := &out.Jours[indexJour[jourCivil(m.Debut).Format("2006-01-02")]]
		jour.Variable += cout
		for lib, kwh := range parLibelle {
			jour.KWhParLibelle[lib] += kwh
		}
	}
	return out
}

// coutCreneau répartit le créneau au prorata du temps passé dans chaque tarif (REQ-CALC-01).
func (k *calcul) coutCreneau(a abonnement.Abonnement, m courbe.Mesure) (float64, map[string]float64, string) {
	duree := m.Duree()
	kwh := m.KWh()
	cout := 0.0
	parLibelle := map[string]float64{}
	for t := m.Debut; t.Before(m.Fin); {
		pas := time.Minute
		if reste := m.Fin.Sub(t); reste < pas {
			pas = reste
		}
		part := kwh * float64(pas) / float64(duree)
		if tarif, ok := k.tarifA(a, t); ok {
			cout += part * tarif.PrixKWh
			parLibelle[tarif.Libelle] += part
		}
		t = t.Add(pas)
	}
	dominant, max := "", -1.0
	for _, lib := range a.Libelles() {
		if v, ok := parLibelle[lib]; ok && v > max {
			dominant, max = lib, v
		}
	}
	return cout, parLibelle, dominant
}

// tarifA trouve le tarif applicable à un instant, via la journée tarifaire (REQ-TIME-03).
func (k *calcul) tarifA(a abonnement.Abonnement, t time.Time) (abonnement.Tarif, bool) {
	couleur := ""
	if a.UtiliseCouleurs() {
		jour := k.resoudre(jourTarifaire(t, a.DebutJourneeTarifaire))
		couleur = jour.couleur
	}
	return a.Tarif(couleur, abonnement.Minutes(t.Hour()*60+t.Minute()))
}

// jourTarifaire renvoie la date de la journée tarifaire, en arithmétique d'heure murale
// (indépendante des changements d'heure).
func jourTarifaire(t time.Time, debut abonnement.Minutes) time.Time {
	d := jourCivil(t)
	if abonnement.Minutes(t.Hour()*60+t.Minute()) < debut {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

func (k *calcul) resoudre(jour time.Time) couleurJour {
	cle := jour.Format("2006-01-02")
	if v, ok := k.cache[cle]; ok {
		return v
	}
	couleur, supposee, ok := k.cal.Couleur(jour)
	v := couleurJour{couleur, supposee, ok}
	k.cache[cle] = v
	if !ok {
		k.manquants[cle] = true
	}
	if supposee {
		k.supposees[cle] = jour
	}
	return v
}

// couleursCiviles renseigne la couleur affichée pour chaque jour civil.
func (k *calcul) couleursCiviles(r *Resultat) {
	utilise := false
	for _, a := range r.Abonnements {
		utilise = utilise || a.Abonnement.UtiliseCouleurs()
	}
	if !utilise {
		return
	}
	for i := range r.Jours {
		if c, _, ok := k.cal.Couleur(r.Jours[i].Date); ok {
			r.Jours[i].Couleur = c
		}
	}
}

func listerJours(m map[string]bool) string {
	cles := make([]string, 0, len(m))
	for c := range m {
		cles = append(cles, c)
	}
	sort.Strings(cles)
	return strings.Join(cles, ", ")
}

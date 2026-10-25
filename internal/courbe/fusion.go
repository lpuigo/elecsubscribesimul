package courbe

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type paireSources struct{ nouveau, ancien int }

type chevauchement struct {
	nombre   int
	min, max time.Time
}

type entree struct {
	mesure Mesure
	source int
}

const formatAvertissement = "02/01/2006 15:04"

// Fusionner assemble les relevés dans l'ordre fourni : en cas de doublon (même instant de début),
// le dernier relevé lu écrase le précédent (REQ-IN-09, REQ-IN-10). Les chevauchements et les trous
// sont renvoyés sous forme d'avertissements ; des PRM différents sont une erreur (REQ-IN-12).
func Fusionner(releves []Releve) (Courbe, []string, error) {
	if len(releves) == 0 {
		return Courbe{}, nil, errors.New("aucun relevé à fusionner")
	}
	prm, err := prmCommun(releves)
	if err != nil {
		return Courbe{}, nil, err
	}

	index := map[int64]entree{}
	chevauchements := map[paireSources]*chevauchement{}
	sources := make([]string, len(releves))
	for i, r := range releves {
		sources[i] = r.Source
		for _, m := range r.Mesures {
			cle := m.Debut.Unix()
			if ancien, existe := index[cle]; existe && ancien.source != i {
				enregistrerChevauchement(chevauchements, paireSources{i, ancien.source}, m.Debut)
			}
			index[cle] = entree{m, i}
		}
	}
	if len(index) == 0 {
		return Courbe{}, nil, errors.New("aucune mesure dans les fichiers fournis")
	}

	mesures := make([]Mesure, 0, len(index))
	for _, e := range index {
		mesures = append(mesures, e.mesure)
	}
	sort.Slice(mesures, func(i, j int) bool { return mesures[i].Debut.Before(mesures[j].Debut) })

	avertissements := avertissementsChevauchement(chevauchements, sources)
	mesures, trous := combler(mesures)
	return Courbe{PRM: prm, Sources: sources, Mesures: mesures}, append(avertissements, trous...), nil
}

func prmCommun(releves []Releve) (string, error) {
	prm, source := "", ""
	for _, r := range releves {
		if r.PRM == "" {
			continue
		}
		if prm == "" {
			prm, source = r.PRM, r.Source
		} else if r.PRM != prm {
			return "", fmt.Errorf("PRM différents : %s (%s) et %s (%s)", prm, source, r.PRM, r.Source)
		}
	}
	return prm, nil
}

func enregistrerChevauchement(m map[paireSources]*chevauchement, p paireSources, debut time.Time) {
	c := m[p]
	if c == nil {
		c = &chevauchement{min: debut, max: debut}
		m[p] = c
	}
	c.nombre++
	if debut.Before(c.min) {
		c.min = debut
	}
	if debut.After(c.max) {
		c.max = debut
	}
}

func avertissementsChevauchement(m map[paireSources]*chevauchement, sources []string) []string {
	paires := make([]paireSources, 0, len(m))
	for p := range m {
		paires = append(paires, p)
	}
	sort.Slice(paires, func(i, j int) bool {
		if paires[i].nouveau != paires[j].nouveau {
			return paires[i].nouveau < paires[j].nouveau
		}
		return paires[i].ancien < paires[j].ancien
	})
	var out []string
	for _, p := range paires {
		c := m[p]
		out = append(out, fmt.Sprintf("données en doublon : %d créneau(x) de %q (du %s au %s) écrase(nt) ceux de %q",
			c.nombre, sources[p.nouveau], c.min.Format(formatAvertissement), c.max.Format(formatAvertissement), sources[p.ancien]))
	}
	return out
}

// combler insère des créneaux manquants dans les trous de la série (REQ-IN-12).
func combler(mesures []Mesure) ([]Mesure, []string) {
	out := make([]Mesure, 0, len(mesures))
	var avertissements []string
	for i, m := range mesures {
		if i > 0 {
			prec := mesures[i-1]
			pas := prec.Duree()
			if pas > 0 && m.Debut.After(prec.Fin) {
				n := 0
				for t := prec.Fin; t.Before(m.Debut); t = t.Add(pas) {
					fin := t.Add(pas)
					if fin.After(m.Debut) {
						fin = m.Debut
					}
					out = append(out, Mesure{Debut: t, Fin: fin, Manquante: true})
					n++
				}
				avertissements = append(avertissements, fmt.Sprintf("trou dans les données du %s au %s (%d créneau(x) manquant(s))",
					prec.Fin.Format(formatAvertissement), m.Debut.Format(formatAvertissement), n))
			}
		}
		out = append(out, m)
	}
	return out, avertissements
}

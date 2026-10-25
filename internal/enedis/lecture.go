// Package enedis lit les exports de courbe de charge Enedis (.xlsx).
package enedis

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/courbe"
)

const feuillePreferee = "Puissance moyenne soutirée"

var (
	regexpPRM     = regexp.MustCompile(`\b\d{14}\b`)
	maxLignesEnTe = 60
)

type colonnes struct{ debut, fin, valeur int }

// Lire charge un fichier Enedis (REQ-IN-01 à REQ-IN-05) sans coder de numéros de ligne en dur.
func Lire(chemin string) (courbe.Releve, error) {
	f, err := excelize.OpenFile(chemin)
	if err != nil {
		return courbe.Releve{}, fmt.Errorf("%s: %w", chemin, err)
	}
	defer f.Close()

	lignes, ligneEnTete, cols, err := trouverDonnees(f)
	if err != nil {
		return courbe.Releve{}, fmt.Errorf("%s: %w", chemin, err)
	}
	loc, err := Fuseau()
	if err != nil {
		return courbe.Releve{}, err
	}
	lecteur := nouveauLecteurHorodatage(loc)

	releve := courbe.Releve{Source: chemin, PRM: extrairePRM(lignes[:ligneEnTete])}
	for i := ligneEnTete + 1; i < len(lignes); i++ {
		ligne := lignes[i]
		cDebut := cellule(ligne, cols.debut)
		if strings.TrimSpace(cDebut) == "" {
			break // première ligne vide
		}
		m, err := lireMesure(lecteur, cDebut, cellule(ligne, cols.fin), cellule(ligne, cols.valeur))
		if err != nil {
			return courbe.Releve{}, fmt.Errorf("%s, ligne %d: %w", chemin, i+1, err)
		}
		releve.Mesures = append(releve.Mesures, m)
	}
	if len(releve.Mesures) == 0 {
		return courbe.Releve{}, fmt.Errorf("%s: aucune mesure trouvée", chemin)
	}
	return releve, nil
}

func lireMesure(lecteur *lecteurHorodatage, debut, fin, valeur string) (courbe.Mesure, error) {
	d, mural, err := lecteur.debut(debut)
	if err != nil {
		return courbe.Mesure{}, err
	}
	f, err := lecteur.fin(fin, d, mural)
	if err != nil {
		return courbe.Mesure{}, err
	}
	m := courbe.Mesure{Debut: d, Fin: f}
	kw, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(valeur), ",", "."), 64)
	if err != nil { // "NA", vide ou illisible : donnée indisponible (REQ-IN-03)
		m.Manquante = true
		return m, nil
	}
	m.KW = kw
	return m, nil
}

func trouverDonnees(f *excelize.File) (lignes [][]string, ligneEnTete int, cols colonnes, err error) {
	noms := f.GetSheetList()
	ordre := make([]string, 0, len(noms))
	for _, n := range noms {
		if strings.EqualFold(n, feuillePreferee) {
			ordre = append(ordre, n)
		}
	}
	for _, n := range noms {
		if !strings.EqualFold(n, feuillePreferee) {
			ordre = append(ordre, n)
		}
	}
	for _, nom := range ordre {
		lignes, err := f.GetRows(nom, excelize.Options{RawCellValue: true})
		if err != nil {
			continue
		}
		for i := 0; i < len(lignes) && i < maxLignesEnTe; i++ {
			if c, ok := chercherEnTete(lignes[i]); ok {
				return lignes, i, c, nil
			}
		}
	}
	return nil, 0, colonnes{}, errors.New("feuille de données introuvable (en-tête Début / Fin / Valeur absent)")
}

func chercherEnTete(ligne []string) (colonnes, bool) {
	c := colonnes{-1, -1, -1}
	for i, v := range ligne {
		s := strings.ToLower(strings.TrimSpace(v))
		switch {
		case s == "début" || s == "debut":
			c.debut = i
		case s == "fin":
			c.fin = i
		case strings.HasPrefix(s, "valeur"):
			c.valeur = i
		}
	}
	return c, c.debut >= 0 && c.fin >= 0 && c.valeur >= 0
}

func extrairePRM(lignes [][]string) string {
	for _, l := range lignes {
		if m := regexpPRM.FindString(strings.Join(l, " ")); m != "" {
			return m
		}
	}
	return ""
}

func cellule(ligne []string, i int) string {
	if i < len(ligne) {
		return ligne[i]
	}
	return ""
}

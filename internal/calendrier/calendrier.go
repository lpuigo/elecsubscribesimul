// Package calendrier associe une couleur à chaque jour (Tempo).
package calendrier

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const formatCle = "2006-01-02"

type Calendrier struct {
	couleurs map[string]string
	defaut   string
}

func Nouveau(couleurs map[string]string, defaut string) Calendrier {
	return Calendrier{couleurs: couleurs, defaut: strings.ToLower(defaut)}
}

// Charger lit un CSV `date;couleur` (en-tête facultative).
func Charger(chemin, defaut string) (Calendrier, error) {
	f, err := os.Open(chemin)
	if err != nil {
		return Calendrier{}, fmt.Errorf("lecture du calendrier des couleurs: %w", err)
	}
	defer f.Close()
	couleurs, err := Lire(f)
	if err != nil {
		return Calendrier{}, fmt.Errorf("%s: %w", chemin, err)
	}
	return Nouveau(couleurs, defaut), nil
}

func Lire(r io.Reader) (map[string]string, error) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	couleurs := map[string]string{}
	for ligne := 1; ; ligne++ {
		rec, err := cr.Read()
		if err == io.EOF {
			return couleurs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("ligne %d: %w", ligne, err)
		}
		if len(rec) < 2 {
			return nil, fmt.Errorf("ligne %d: format attendu date;couleur", ligne)
		}
		date := strings.TrimSpace(strings.TrimPrefix(rec[0], "\xef\xbb\xbf"))
		if _, err := time.Parse(formatCle, date); err != nil {
			if ligne == 1 {
				continue // en-tête
			}
			return nil, fmt.Errorf("ligne %d: date %q invalide (attendu AAAA-MM-JJ)", ligne, date)
		}
		couleurs[date] = strings.ToLower(strings.TrimSpace(rec[1]))
	}
}

// Couleur renvoie la couleur d'un jour ; supposee vaut true si la couleur par défaut a été utilisée.
func (c Calendrier) Couleur(jour time.Time) (couleur string, supposee, ok bool) {
	if v, trouve := c.couleurs[jour.Format(formatCle)]; trouve {
		return v, false, true
	}
	if c.defaut != "" {
		return c.defaut, true, true
	}
	return "", false, false
}

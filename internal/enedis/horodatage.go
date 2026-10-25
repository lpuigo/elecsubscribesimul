package enedis

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // fuseaux disponibles sur un poste Windows sans base tzdata

	"github.com/xuri/excelize/v2"
)

const layoutEnedis = "02/01/2006 15:04:05"

var layoutsAcceptes = []string{layoutEnedis, "02/01/2006 15:04", "2006-01-02 15:04:05", "2006-01-02T15:04:05"}

// Fuseau renvoie le fuseau des horodatages Enedis (heure légale française).
func Fuseau() (*time.Location, error) { return time.LoadLocation("Europe/Paris") }

// lecteurHorodatage convertit les horodatages locaux d'un fichier en instants absolus.
// Un horodatage ambigu (passage à l'heure d'hiver) est résolu selon l'ordre d'apparition :
// 1re occurrence = heure d'été, 2e = heure d'hiver (REQ-TIME-01).
type lecteurHorodatage struct {
	loc *time.Location
	vus map[string]int
}

func nouveauLecteurHorodatage(loc *time.Location) *lecteurHorodatage {
	return &lecteurHorodatage{loc: loc, vus: map[string]int{}}
}

// normaliser ramène une cellule (texte ou date Excel brute) à l'heure murale au format layoutEnedis.
func normaliser(cellule string) (string, error) {
	s := strings.TrimSpace(cellule)
	for _, l := range layoutsAcceptes {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format(layoutEnedis), nil
		}
	}
	if f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64); err == nil {
		t, err := excelize.ExcelDateToTime(f, false)
		if err == nil {
			return t.Round(time.Second).Format(layoutEnedis), nil
		}
	}
	return "", fmt.Errorf("horodatage %q invalide (attendu JJ/MM/AAAA HH:MM:SS)", cellule)
}

// candidats renvoie les instants possibles pour une heure murale, du plus ancien au plus récent.
func (l *lecteurHorodatage) candidats(mural string) ([]time.Time, error) {
	base, err := time.ParseInLocation(layoutEnedis, mural, l.loc)
	if err != nil {
		return nil, err
	}
	var out []time.Time
	for _, d := range []time.Duration{-time.Hour, 0, time.Hour} {
		if c := base.Add(d); c.Format(layoutEnedis) == mural {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		out = append(out, base)
	}
	return out, nil
}

func (l *lecteurHorodatage) debut(cellule string) (time.Time, string, error) {
	mural, err := normaliser(cellule)
	if err != nil {
		return time.Time{}, "", err
	}
	cands, err := l.candidats(mural)
	if err != nil {
		return time.Time{}, "", err
	}
	n := l.vus[mural]
	l.vus[mural]++
	if n >= len(cands) {
		n = len(cands) - 1
	}
	return cands[n], mural, nil
}

// fin déduit l'instant de fin d'un créneau. La durée retenue est la plus petite durée positive entre
// la différence d'heures murales et l'écart au prochain instant candidat, ce qui reste juste
// autour des changements d'heure quelle que soit la façon dont l'export étiquette la fin.
func (l *lecteurHorodatage) fin(cellule string, debut time.Time, debutMural string) (time.Time, error) {
	mural, err := normaliser(cellule)
	if err != nil {
		return time.Time{}, err
	}
	cands, err := l.candidats(mural)
	if err != nil {
		return time.Time{}, err
	}
	var duree time.Duration
	for _, c := range cands {
		if c.After(debut) {
			duree = c.Sub(debut)
			break
		}
	}
	tFin, err1 := time.Parse(layoutEnedis, mural)
	tDebut, err2 := time.Parse(layoutEnedis, debutMural)
	if err1 == nil && err2 == nil {
		if d := tFin.Sub(tDebut); d > 0 && (duree == 0 || d < duree) {
			duree = d
		}
	}
	if duree <= 0 {
		return time.Time{}, fmt.Errorf("la fin %q n'est pas postérieure au début", cellule)
	}
	return debut.Add(duree), nil
}

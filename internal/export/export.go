// Package export produit le classeur Excel de comparaison.
package export

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/courbe"
	"elecsubscribesimul/internal/tarification"
)

const (
	feuilleSynthese   = "Synthèse journalière"
	feuilleDetail     = "Détail 30 min"
	feuilleParametres = "Paramètres"

	maxColonnesExcel   = 16384
	colonnesSyntheseJ  = 5 // libellé + 4 colonnes de synthèse
	colonnesSynthese30 = 4 // libellé + 3 colonnes de synthèse
	vertClair          = "C6EFCE"
	formatDateHeure    = "02/01/2006 15:04"
	formatDateCourt    = "02/01/2006"
)

type Options struct {
	Detail         bool
	SansDetail30   bool
	Avertissements []string
	Genere         time.Time
}

// Ecrire génère le classeur et renvoie les avertissements propres à l'export.
func Ecrire(chemin string, c courbe.Courbe, r tarification.Resultat, opts Options) ([]string, error) {
	f := excelize.NewFile()
	defer f.Close()
	st := nouvelleFabrique(f)

	if err := f.SetSheetName("Sheet1", feuilleSynthese); err != nil {
		return nil, err
	}
	if err := ecrireSynthese(f, st, r, opts.Detail); err != nil {
		return nil, fmt.Errorf("feuille %s: %w", feuilleSynthese, err)
	}

	var avertissements []string
	switch {
	case opts.SansDetail30:
	case len(c.Mesures)+colonnesSynthese30 > maxColonnesExcel:
		avertissements = append(avertissements, fmt.Sprintf(
			"feuille %q non générée : %d créneaux dépassent la limite de colonnes d'Excel", feuilleDetail, len(c.Mesures)))
	default:
		if _, err := f.NewSheet(feuilleDetail); err != nil {
			return nil, err
		}
		if err := ecrireDetail(f, st, c, r, opts.Detail); err != nil {
			return nil, fmt.Errorf("feuille %s: %w", feuilleDetail, err)
		}
	}

	if _, err := f.NewSheet(feuilleParametres); err != nil {
		return nil, err
	}
	if err := ecrireParametres(f, st, c, r, opts, avertissements); err != nil {
		return nil, fmt.Errorf("feuille %s: %w", feuilleParametres, err)
	}
	f.SetActiveSheet(0)
	if err := f.SaveAs(chemin); err != nil {
		return nil, fmt.Errorf("écriture de %s: %w", chemin, err)
	}
	return avertissements, nil
}

// ecrireur simplifie l'écriture cellule par cellule en mémorisant la première erreur.
type ecrireur struct {
	f     *excelize.File
	feuil string
	err   error
}

func (e *ecrireur) val(col, ligne int, v any, style int) {
	if e.err != nil {
		return
	}
	cell, err := excelize.CoordinatesToCellName(col, ligne)
	if err != nil {
		e.err = err
		return
	}
	if err := e.f.SetCellValue(e.feuil, cell, v); err != nil {
		e.err = err
		return
	}
	if style != 0 {
		e.err = e.f.SetCellStyle(e.feuil, cell, cell, style)
	}
}

func (e *ecrireur) figer(colonnes int) {
	if e.err != nil {
		return
	}
	cell, _ := excelize.CoordinatesToCellName(colonnes+1, 2)
	e.err = e.f.SetPanes(e.feuil, &excelize.Panes{
		Freeze: true, XSplit: colonnes, YSplit: 1, TopLeftCell: cell, ActivePane: "bottomRight",
	})
}

func (e *ecrireur) largeur(debut, fin int, w float64) {
	if e.err != nil {
		return
	}
	a, _ := excelize.ColumnNumberToName(debut)
	b, _ := excelize.ColumnNumberToName(fin)
	e.err = e.f.SetColWidth(e.feuil, a, b, w)
}

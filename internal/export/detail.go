package export

import (
	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/courbe"
	"elecsubscribesimul/internal/tarification"
)

func ecrireDetail(f *excelize.File, st *fabriqueStyles, c courbe.Courbe, r tarification.Resultat, detail bool) error {
	e := &ecrireur{f: f, feuil: feuilleDetail}
	gras := st.id(specStyle{gras: true})
	premiere := colonnesSynthese30 + 1

	e.val(1, 1, "Début", gras)
	for i, h := range []string{"Total période", "Moyenne par créneau", "Maximum"} {
		e.val(2+i, 1, h, gras)
	}

	// Statistiques de synthèse sur les créneaux non manquants.
	nb, sommeKW, sommeKWh, maxKW := 0, 0.0, 0.0, 0.0
	for _, m := range c.Mesures {
		if m.Manquante {
			continue
		}
		nb++
		sommeKW += m.KW
		sommeKWh += m.KWh()
		if m.KW > maxKW {
			maxKW = m.KW
		}
	}
	moy := func(somme float64) float64 {
		if nb == 0 {
			return 0
		}
		return somme / float64(nb)
	}

	e.val(1, 2, "Puissance moyenne (kW)", gras)
	e.val(3, 2, moy(sommeKW), st.id(specStyle{format: formatKW}))
	e.val(4, 2, maxKW, st.id(specStyle{format: formatKW}))
	e.val(1, 3, "Énergie (kWh)", gras)
	e.val(2, 3, sommeKWh, st.id(specStyle{format: formatKWh}))
	e.val(3, 3, moy(sommeKWh), st.id(specStyle{format: formatKWh}))

	ligne := 4
	type rangCout struct{ row, abo int }
	var couts []rangCout
	for ia, a := range r.Abonnements {
		e.val(1, ligne, a.Abonnement.Nom+" (€)", gras)
		total, maxi := 0.0, 0.0
		for i, m := range c.Mesures {
			if !m.Manquante {
				total += a.Creneaux[i].Cout
				if a.Creneaux[i].Cout > maxi {
					maxi = a.Creneaux[i].Cout
				}
			}
		}
		e.val(2, ligne, total, st.id(specStyle{format: formatEuro}))
		e.val(3, ligne, moy(total), st.id(specStyle{format: formatEuro}))
		e.val(4, ligne, maxi, st.id(specStyle{format: formatEuro}))
		couts = append(couts, rangCout{ligne, ia})
		ligne++
		if detail {
			e.val(1, ligne, "   tarif appliqué : "+a.Abonnement.Nom, st.id(specStyle{}))
			couts = append(couts, rangCout{-ligne, ia})
			ligne++
		}
	}

	for i, m := range c.Mesures {
		col := premiere + i
		bord := i == 0
		e.val(col, 1, m.Debut.Format(formatDateHeure), st.id(specStyle{gras: true, bordureG: bord}))
		if m.Manquante {
			continue
		}
		e.val(col, 2, m.KW, st.id(specStyle{format: formatKW, bordureG: bord}))
		e.val(col, 3, m.KWh(), st.id(specStyle{format: formatKWh, bordureG: bord}))
		for _, rc := range couts {
			if rc.row > 0 {
				e.val(col, rc.row, r.Abonnements[rc.abo].Creneaux[i].Cout, st.id(specStyle{format: formatEuro, bordureG: bord}))
			} else {
				e.val(col, -rc.row, r.Abonnements[rc.abo].Creneaux[i].Libelle, st.id(specStyle{bordureG: bord}))
			}
		}
	}
	e.largeur(1, 1, 34)
	e.largeur(2, colonnesSynthese30, 18)
	e.largeur(premiere, premiere+len(c.Mesures)-1, 16)
	e.figer(colonnesSynthese30)
	return e.err
}

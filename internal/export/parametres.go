package export

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/courbe"
	"elecsubscribesimul/internal/tarification"
)

func ecrireParametres(f *excelize.File, st *fabriqueStyles, c courbe.Courbe, r tarification.Resultat, opts Options, avertExport []string) error {
	e := &ecrireur{f: f, feuil: feuilleParametres}
	gras := st.id(specStyle{gras: true})
	norm := st.id(specStyle{})
	ligne := 1
	ecrire := func(cle string, valeur any) {
		e.val(1, ligne, cle, gras)
		if valeur != nil {
			e.val(2, ligne, valeur, norm)
		}
		ligne++
	}

	ecrire("PRM", c.PRM)
	ecrire("Période", fmt.Sprintf("du %s au %s", c.Debut().Format(formatDateHeure), c.Fin().Format(formatDateHeure)))
	ecrire("Jours", len(r.Jours))
	ecrire("Généré le", opts.Genere.Format("02/01/2006 15:04:05"))
	for i, s := range c.Sources {
		cle := ""
		if i == 0 {
			cle = "Fichiers sources"
		}
		e.val(1, ligne, cle, gras)
		e.val(2, ligne, s, norm)
		ligne++
	}

	ligne++
	ecrire("Abonnements", nil)
	for _, a := range r.Abonnements {
		ab := a.Abonnement
		ecrire(ab.Nom, fmt.Sprintf("%d kVA, part fixe %s € %s, début de journée tarifaire %s",
			ab.PuissanceKVA, fr(ab.PartFixe.Montant), ab.PartFixe.Periodicite, ab.DebutJourneeTarifaire))
		for _, t := range ab.Tarifs {
			couleur := "toutes couleurs"
			if t.Couleur != "" {
				couleur = t.Couleur
			}
			e.val(2, ligne, fmt.Sprintf("%s — %s : %s → %s à %.4f €/kWh", couleur, t.Libelle, t.Debut, t.Fin, t.PrixKWh), norm)
			ligne++
		}
	}

	ligne++
	ecrire("Anomalies", nil)
	ecrire("Créneaux manquants", r.Manquants())
	if len(r.CouleursSupposees) > 0 {
		jours := make([]string, len(r.CouleursSupposees))
		for i, j := range r.CouleursSupposees {
			jours[i] = j.Format(formatDateCourt)
		}
		ecrire("Couleurs supposées (par défaut)", strings.Join(jours, ", "))
	}
	for _, a := range append(append([]string{}, opts.Avertissements...), avertExport...) {
		ecrire("Avertissement", a)
	}
	e.largeur(1, 1, 34)
	e.largeur(2, 2, 110)
	return e.err
}

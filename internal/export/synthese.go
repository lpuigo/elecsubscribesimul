package export

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/tarification"
)

type ligneSynthese struct {
	libelle  string
	format   string
	gras     bool
	total    *float64
	moyenne  *float64
	extrap   *float64
	ecart    string
	jours    func(i int) any
	surbrill bool
}

func ptr(v float64) *float64 { return &v }

func ecrireSynthese(f *excelize.File, st *fabriqueStyles, r tarification.Resultat, detail bool) error {
	e := &ecrireur{f: f, feuil: feuilleSynthese}
	nbJours := float64(len(r.Jours))
	e.val(1, 1, "", st.id(specStyle{gras: true}))
	for i, h := range []string{"Total période", "Moyenne / jour", "Extrapolation annuelle", "Écart vs moins cher"} {
		e.val(2+i, 1, h, st.id(specStyle{gras: true}))
	}
	for i, j := range r.Jours {
		e.val(colonnesSyntheseJ+1+i, 1, j.Date.Format(formatDateCourt), st.id(specStyle{gras: true, bordureG: i == 0}))
	}

	totalKWh := r.TotalKWh()
	lignes := []ligneSynthese{{
		libelle: "Consommation (kWh)", format: formatKWh, gras: true,
		total: ptr(totalKWh), moyenne: ptr(totalKWh / nbJours), extrap: ptr(totalKWh / nbJours * 365),
		jours: func(i int) any { return r.Jours[i].KWh },
	}}

	moins := r.MoinsCher()
	for ia, a := range r.Abonnements {
		ia, a := ia, a
		ecart := ""
		if ia != moins {
			diff := a.Total() - r.Abonnements[moins].Total()
			ecart = fmt.Sprintf("+%s € (+%s %%)", fr(diff), fr(diff/r.Abonnements[moins].Total()*100))
		}
		lignes = append(lignes, ligneSynthese{
			libelle: a.Abonnement.Nom + " (€)", format: formatEuro, gras: true,
			total: ptr(a.Total()), moyenne: ptr(r.MoyenneJour(ia)), extrap: ptr(r.Extrapolation(ia)),
			ecart: ecart, surbrill: ia == moins,
			jours: func(i int) any { return a.Jours[i].Total() },
		})
		if detail {
			lignes = append(lignes, sousLignes(r, ia, nbJours)...)
		}
	}

	if utiliseCouleurs(r) {
		lignes = append(lignes, ligneSynthese{libelle: "Couleur Tempo", jours: func(i int) any { return r.Jours[i].Couleur }})
	}
	manquants := 0
	for _, j := range r.Jours {
		manquants += j.Manquants
	}
	lignes = append(lignes, ligneSynthese{
		libelle: "Créneaux manquants", total: ptr(float64(manquants)),
		jours: func(i int) any { return r.Jours[i].Manquants },
	})

	for k, l := range lignes {
		row := 2 + k
		e.val(1, row, l.libelle, st.id(specStyle{gras: l.gras}))
		valeurs := []*float64{l.total, l.moyenne, l.extrap}
		for c, v := range valeurs {
			if v != nil {
				fill := ""
				if c == 0 && l.surbrill {
					fill = vertClair
				}
				e.val(2+c, row, *v, st.id(specStyle{format: l.format, gras: l.gras, remplissage: fill}))
			}
		}
		if l.ecart != "" {
			e.val(5, row, l.ecart, st.id(specStyle{}))
		}
		for i := range r.Jours {
			e.val(colonnesSyntheseJ+1+i, row, l.jours(i), st.id(specStyle{format: l.format, bordureG: i == 0}))
		}
	}
	e.largeur(1, 1, 34)
	e.largeur(2, 5, 20)
	e.largeur(colonnesSyntheseJ+1, colonnesSyntheseJ+len(r.Jours), 13)
	e.figer(colonnesSyntheseJ)
	return e.err
}

func sousLignes(r tarification.Resultat, ia int, nbJours float64) []ligneSynthese {
	a := r.Abonnements[ia]
	fixe, variable := a.TotalFixe(), a.TotalVariable()
	out := []ligneSynthese{
		{libelle: "   part fixe", format: formatEuro, total: ptr(fixe), moyenne: ptr(fixe / nbJours), extrap: ptr(fixe / nbJours * 365),
			jours: func(i int) any { return a.Jours[i].Fixe }},
		{libelle: "   coût variable", format: formatEuro, total: ptr(variable), moyenne: ptr(variable / nbJours), extrap: ptr(variable / nbJours * 365),
			jours: func(i int) any { return a.Jours[i].Variable }},
	}
	for _, lib := range a.Abonnement.Libelles() {
		lib := lib
		total := 0.0
		for _, j := range a.Jours {
			total += j.KWhParLibelle[lib]
		}
		out = append(out, ligneSynthese{
			libelle: "   kWh " + lib, format: formatKWh, total: ptr(total), moyenne: ptr(total / nbJours),
			jours: func(i int) any { return a.Jours[i].KWhParLibelle[lib] },
		})
	}
	return out
}

func utiliseCouleurs(r tarification.Resultat) bool {
	for _, a := range r.Abonnements {
		if a.Abonnement.UtiliseCouleurs() {
			return true
		}
	}
	return false
}

// fr formate un nombre à la française (virgule décimale).
func fr(v float64) string { return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1) }

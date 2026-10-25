// Commande simu-abonnement : compare le coût de plusieurs abonnements sur une courbe de charge Enedis.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"elecsubscribesimul/internal/calendrier"
	"elecsubscribesimul/internal/config"
	"elecsubscribesimul/internal/courbe"
	"elecsubscribesimul/internal/enedis"
	"elecsubscribesimul/internal/export"
	"elecsubscribesimul/internal/tarification"
)

const (
	codeOK         = 0
	codeErreurDonn = 1
	codeInattendue = 2
)

type listeFlag []string

func (l *listeFlag) String() string     { return strings.Join(*l, ",") }
func (l *listeFlag) Set(v string) error { *l = append(*l, v); return nil }

type options struct {
	courbes         listeFlag
	abonnements     string
	couleurs        string
	couleurDefaut   string
	detail          bool
	sansDetail30min bool
	sortie          string
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "erreur inattendue : %v\n", r)
			os.Exit(codeInattendue)
		}
	}()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := lireOptions(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return codeOK
		}
		fmt.Fprintln(stderr, "erreur :", err)
		return codeErreurDonn
	}
	if err := execute(opts, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "erreur :", err)
		return codeErreurDonn
	}
	return codeOK
}

func lireOptions(args []string, stderr io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("simu-abonnement", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Var(&o.courbes, "courbe", "fichier .xlsx ou répertoire de courbe(s) de charge Enedis (répétable)")
	fs.StringVar(&o.abonnements, "abonnements", "", "fichier YAML des abonnements")
	fs.StringVar(&o.couleurs, "couleurs", "", "calendrier CSV date;couleur (requis si un abonnement utilise des couleurs)")
	fs.StringVar(&o.couleurDefaut, "couleur-par-defaut", "", "couleur supposée pour les jours absents du calendrier")
	fs.BoolVar(&o.detail, "detail", false, "ajoute les sous-lignes de détail")
	fs.BoolVar(&o.sansDetail30min, "sans-detail-30min", false, "ne génère pas la feuille Détail 30 min")
	fs.StringVar(&o.sortie, "sortie", "resultat.xlsx", "fichier Excel produit")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if len(o.courbes) == 0 {
		return o, errors.New("au moins une option --courbe est requise")
	}
	if o.abonnements == "" {
		return o, errors.New("l'option --abonnements est requise")
	}
	return o, nil
}

func execute(o options, stdout, stderr io.Writer) error {
	fichiers, err := enedis.ListerFichiers(o.courbes)
	if err != nil {
		return err
	}
	releves := make([]courbe.Releve, 0, len(fichiers))
	for _, chemin := range fichiers {
		r, err := enedis.Lire(chemin)
		if err != nil {
			return err
		}
		releves = append(releves, r)
	}
	serie, avertissements, err := courbe.Fusionner(releves)
	if err != nil {
		return err
	}
	for _, a := range avertissements {
		fmt.Fprintln(stderr, "avertissement :", a)
	}

	abos, err := config.Charger(o.abonnements)
	if err != nil {
		return err
	}
	cal := calendrier.Nouveau(nil, o.couleurDefaut)
	if o.couleurs != "" {
		if cal, err = calendrier.Charger(o.couleurs, o.couleurDefaut); err != nil {
			return err
		}
	}

	res, err := tarification.Calculer(serie, abos, cal)
	if err != nil {
		return err
	}
	if len(res.CouleursSupposees) > 0 {
		avertissements = append(avertissements, fmt.Sprintf("%d jour(s) avec couleur supposée", len(res.CouleursSupposees)))
	}

	exportAvert, err := export.Ecrire(o.sortie, serie, res, export.Options{
		Detail: o.detail, SansDetail30: o.sansDetail30min, Avertissements: avertissements, Genere: time.Now(),
	})
	if err != nil {
		return err
	}
	for _, a := range exportAvert {
		fmt.Fprintln(stderr, "avertissement :", a)
	}
	afficherResume(stdout, serie, res, o.sortie)
	return nil
}

func afficherResume(w io.Writer, c courbe.Courbe, r tarification.Resultat, sortie string) {
	fmt.Fprintf(w, "Période : %s → %s (%d jours, %.3f kWh)\n",
		c.Debut().Format("02/01/2006"), c.Fin().Format("02/01/2006"), len(r.Jours), r.TotalKWh())
	moins := r.MoinsCher()
	for i, a := range r.Abonnements {
		marque := " "
		if i == moins {
			marque = "*"
		}
		fmt.Fprintf(w, " %s %-30s %10.2f €  (extrapolation annuelle %.2f €)\n", marque, a.Abonnement.Nom, a.Total(), r.Extrapolation(i))
	}
	fmt.Fprintf(w, "Moins cher : %s\nFichier : %s\n", r.Abonnements[moins].Abonnement.Nom, sortie)
}

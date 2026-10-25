// Command simu-abonnement compares the cost of several subscriptions over an Enedis load curve.
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

	"elecsubscribesimul/internal/calendar"
	"elecsubscribesimul/internal/config"
	"elecsubscribesimul/internal/enedis"
	"elecsubscribesimul/internal/export"
	"elecsubscribesimul/internal/loadcurve"
	"elecsubscribesimul/internal/pricing"
)

const (
	exitOK         = 0
	exitDataError  = 1
	exitUnexpected = 2
)

type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

type options struct {
	curves          listFlag
	subscriptions   string
	colors          string
	defaultColor    string
	detail          bool
	skipDetail30min bool
	output          string
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "erreur inattendue : %v\n", r)
			os.Exit(exitUnexpected)
		}
	}()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		fmt.Fprintln(stderr, "erreur :", err)
		return exitDataError
	}
	if err := execute(opts, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "erreur :", err)
		return exitDataError
	}
	return exitOK
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("simu-abonnement", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Var(&o.curves, "courbe", "fichier .xlsx ou répertoire de courbe(s) de charge Enedis (répétable)")
	fs.StringVar(&o.subscriptions, "abonnements", "", "fichier YAML des abonnements")
	fs.StringVar(&o.colors, "couleurs", "", "calendrier CSV date;couleur (requis si un abonnement utilise des couleurs)")
	fs.StringVar(&o.defaultColor, "couleur-par-defaut", "", "couleur supposée pour les jours absents du calendrier")
	fs.BoolVar(&o.detail, "detail", false, "ajoute les sous-lignes de détail")
	fs.BoolVar(&o.skipDetail30min, "sans-detail-30min", false, "ne génère pas la feuille Détail 30 min")
	fs.StringVar(&o.output, "sortie", "resultat.xlsx", "fichier Excel produit")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if len(o.curves) == 0 {
		return o, errors.New("au moins une option --courbe est requise")
	}
	if o.subscriptions == "" {
		return o, errors.New("l'option --abonnements est requise")
	}
	return o, nil
}

func execute(o options, stdout, stderr io.Writer) error {
	files, err := enedis.ListFiles(o.curves)
	if err != nil {
		return err
	}
	statements := make([]loadcurve.Statement, 0, len(files))
	for _, path := range files {
		r, err := enedis.Read(path)
		if err != nil {
			return err
		}
		statements = append(statements, r)
	}
	curve, warnings, err := loadcurve.Merge(statements)
	if err != nil {
		return err
	}
	for _, a := range warnings {
		fmt.Fprintln(stderr, "avertissement :", a)
	}

	subs, err := config.Load(o.subscriptions)
	if err != nil {
		return err
	}
	cal := calendar.New(nil, o.defaultColor)
	if o.colors != "" {
		if cal, err = calendar.Load(o.colors, o.defaultColor); err != nil {
			return err
		}
	}

	res, err := pricing.Compute(curve, subs, cal)
	if err != nil {
		return err
	}
	if len(res.AssumedColors) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d jour(s) avec couleur supposée", len(res.AssumedColors)))
	}

	exportWarnings, err := export.Write(o.output, curve, res, export.Options{
		Detail: o.detail, SkipDetail30: o.skipDetail30min, Warnings: warnings, Generated: time.Now(),
	})
	if err != nil {
		return err
	}
	for _, a := range exportWarnings {
		fmt.Fprintln(stderr, "avertissement :", a)
	}
	printSummary(stdout, curve, res, o.output)
	return nil
}

func printSummary(w io.Writer, c loadcurve.Curve, r pricing.Result, output string) {
	fmt.Fprintf(w, "Période : %s → %s (%d jours, %.3f kWh)\n",
		c.Start().Format("02/01/2006"), c.End().Format("02/01/2006"), len(r.Days), r.TotalKWh())
	cheapest := r.Cheapest()
	for i, a := range r.Subscriptions {
		mark := " "
		if i == cheapest {
			mark = "*"
		}
		fmt.Fprintf(w, " %s %-30s %10.2f €  (extrapolation annuelle %.2f €)\n", mark, a.Subscription.Name, a.Total(), r.Extrapolation(i))
	}
	fmt.Fprintf(w, "Moins cher : %s\nFichier : %s\n", r.Subscriptions[cheapest].Subscription.Name, output)
}

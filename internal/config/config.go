// Package config charge le fichier YAML des abonnements.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"elecsubscribesimul/internal/abonnement"
)

type fichier struct {
	Abonnements []abonnementYAML `yaml:"abonnements"`
}

type abonnementYAML struct {
	Nom                   string      `yaml:"nom"`
	PuissanceKVA          int         `yaml:"puissance_kva"`
	PartFixe              partFixeYML `yaml:"part_fixe"`
	DebutJourneeTarifaire string      `yaml:"debut_journee_tarifaire"`
	Tarifs                []tarifYAML `yaml:"tarifs"`
}

type partFixeYML struct {
	Montant     float64 `yaml:"montant"`
	Periodicite string  `yaml:"periodicite"`
}

type tarifYAML struct {
	Couleur string  `yaml:"couleur"`
	Libelle string  `yaml:"libelle"`
	Debut   string  `yaml:"debut"`
	Fin     string  `yaml:"fin"`
	PrixKWh float64 `yaml:"prix_kwh"`
}

// Charger lit et valide les abonnements ; toutes les anomalies sont regroupées dans l'erreur.
func Charger(chemin string) ([]abonnement.Abonnement, error) {
	data, err := os.ReadFile(chemin)
	if err != nil {
		return nil, fmt.Errorf("lecture de la configuration: %w", err)
	}
	abos, err := Parser(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", chemin, err)
	}
	return abos, nil
}

func Parser(data []byte) ([]abonnement.Abonnement, error) {
	var f fichier
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("configuration YAML invalide: %w", err)
	}
	if len(f.Abonnements) == 0 {
		return nil, errors.New("configuration : aucun abonnement défini")
	}
	var errs []error
	abos := make([]abonnement.Abonnement, 0, len(f.Abonnements))
	noms := map[string]bool{}
	for _, y := range f.Abonnements {
		a, err := convertir(y)
		if err != nil {
			errs = append(errs, err)
		}
		if err := a.Validate(); err != nil {
			errs = append(errs, err)
		}
		if noms[a.Nom] {
			errs = append(errs, fmt.Errorf("abonnement %q défini plusieurs fois", a.Nom))
		}
		noms[a.Nom] = true
		abos = append(abos, a)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return abos, nil
}

func convertir(y abonnementYAML) (abonnement.Abonnement, error) {
	var errs []error
	a := abonnement.Abonnement{
		Nom:          y.Nom,
		PuissanceKVA: y.PuissanceKVA,
		PartFixe: abonnement.PartFixe{
			Montant:     y.PartFixe.Montant,
			Periodicite: abonnement.Periodicite(strings.ToLower(y.PartFixe.Periodicite)),
		},
	}
	if y.DebutJourneeTarifaire != "" {
		m, err := abonnement.ParseMinutes(y.DebutJourneeTarifaire)
		if err != nil {
			errs = append(errs, fmt.Errorf("abonnement %q, debut_journee_tarifaire: %w", y.Nom, err))
		}
		a.DebutJourneeTarifaire = m
	}
	for _, t := range y.Tarifs {
		debut, err1 := abonnement.ParseMinutes(t.Debut)
		fin, err2 := abonnement.ParseMinutes(t.Fin)
		if err := errors.Join(err1, err2); err != nil {
			errs = append(errs, fmt.Errorf("abonnement %q, tarif %q: %w", y.Nom, t.Libelle, err))
		}
		couleur := strings.ToLower(strings.TrimSpace(t.Couleur))
		if couleur == "*" {
			couleur = ""
		}
		a.Tarifs = append(a.Tarifs, abonnement.Tarif{Couleur: couleur, Libelle: t.Libelle, Debut: debut, Fin: fin, PrixKWh: t.PrixKWh})
	}
	return a, errors.Join(errs...)
}

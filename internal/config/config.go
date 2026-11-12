// Package config loads the YAML file of subscriptions.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"elecsubscribesimul/internal/subscription"
)

type file struct {
	Subscriptions []yamlSubscription `yaml:"abonnements"`
}

type yamlSubscription struct {
	Name           string       `yaml:"nom"`
	PowerKVA       int          `yaml:"puissance_kva"`
	FixedFee       yamlFixedFee `yaml:"part_fixe"`
	TariffDayStart string       `yaml:"debut_journee_tarifaire"`
	Tariffs        []yamlTariff `yaml:"tarifs"`
}

type yamlFixedFee struct {
	Amount      float64 `yaml:"montant"`
	Periodicity string  `yaml:"periodicite"`
}

// yamlTariff takes its ranges from "plages", or from "debut"/"fin" as a shortcut for a single range;
// without either, it is the default tariff.
type yamlTariff struct {
	Color       string      `yaml:"couleur"`
	Label       string      `yaml:"libelle"`
	Start       string      `yaml:"debut"`
	End         string      `yaml:"fin"`
	Ranges      []yamlRange `yaml:"plages"`
	PricePerKWh float64     `yaml:"prix_kwh"`
}

type yamlRange struct {
	Start string `yaml:"debut"`
	End   string `yaml:"fin"`
}

// Load reads and validates the subscriptions; every anomaly is gathered in the error.
func Load(path string) ([]subscription.Subscription, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("lecture de la configuration: %w", err)
	}
	subs, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return subs, nil
}

// Parse decodes and validates YAML subscriptions.
func Parse(data []byte) ([]subscription.Subscription, error) {
	var f file
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("configuration YAML invalide: %w", err)
	}
	if len(f.Subscriptions) == 0 {
		return nil, errors.New("configuration : aucun abonnement défini")
	}
	var errs []error
	subs := make([]subscription.Subscription, 0, len(f.Subscriptions))
	names := map[string]bool{}
	for _, y := range f.Subscriptions {
		s, err := convert(y)
		if err != nil {
			errs = append(errs, err)
		}
		if err := s.Validate(); err != nil {
			errs = append(errs, err)
		}
		if names[s.Name] {
			errs = append(errs, fmt.Errorf("abonnement %q défini plusieurs fois", s.Name))
		}
		names[s.Name] = true
		subs = append(subs, s)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return subs, nil
}

func convert(y yamlSubscription) (subscription.Subscription, error) {
	var errs []error
	s := subscription.Subscription{
		Name:     y.Name,
		PowerKVA: y.PowerKVA,
		FixedFee: subscription.FixedFee{
			Amount:      y.FixedFee.Amount,
			Periodicity: subscription.Periodicity(strings.ToLower(y.FixedFee.Periodicity)),
		},
	}
	if y.TariffDayStart != "" {
		m, err := subscription.ParseMinutes(y.TariffDayStart)
		if err != nil {
			errs = append(errs, fmt.Errorf("abonnement %q, debut_journee_tarifaire: %w", y.Name, err))
		}
		s.TariffDayStart = m
	}
	for _, t := range y.Tariffs {
		ranges, err := convertRanges(t)
		if err != nil {
			errs = append(errs, fmt.Errorf("abonnement %q, tarif %q: %w", y.Name, t.Label, err))
		}
		color := strings.ToLower(strings.TrimSpace(t.Color))
		if color == "*" {
			color = ""
		}
		s.Tariffs = append(s.Tariffs, subscription.Tariff{Color: color, Label: t.Label, Ranges: ranges, PricePerKWh: t.PricePerKWh})
	}
	return s, errors.Join(errs...)
}

func convertRanges(t yamlTariff) ([]subscription.TimeRange, error) {
	yamlRanges := t.Ranges
	if t.Start != "" || t.End != "" {
		if len(t.Ranges) > 0 {
			return nil, errors.New("debut/fin et plages ne peuvent pas être utilisés ensemble")
		}
		yamlRanges = []yamlRange{{t.Start, t.End}}
	}
	var errs []error
	var out []subscription.TimeRange
	for _, r := range yamlRanges {
		if r.Start == "" || r.End == "" {
			errs = append(errs, errors.New("debut et fin doivent être renseignés ensemble"))
			continue
		}
		start, err1 := subscription.ParseMinutes(r.Start)
		end, err2 := subscription.ParseMinutes(r.End)
		if err := errors.Join(err1, err2); err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, subscription.TimeRange{Start: start, End: end})
	}
	return out, errors.Join(errs...)
}

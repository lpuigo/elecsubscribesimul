package loadcurve

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type sourcePair struct{ newer, older int }

type overlap struct {
	count    int
	min, max time.Time
}

type entry struct {
	measurement Measurement
	source      int
}

const warningFormat = "02/01/2006 15:04"

// Merge assembles the statements in the given order: on a duplicate (same start instant),
// the last statement read overwrites the previous one (REQ-IN-09, REQ-IN-10). Overlaps and gaps
// are returned as warnings; different PRMs are an error (REQ-IN-12).
func Merge(statements []Statement) (Curve, []string, error) {
	if len(statements) == 0 {
		return Curve{}, nil, errors.New("aucun relevé à fusionner")
	}
	prm, err := commonPRM(statements)
	if err != nil {
		return Curve{}, nil, err
	}

	index := map[int64]entry{}
	overlaps := map[sourcePair]*overlap{}
	sources := make([]string, len(statements))
	for i, s := range statements {
		sources[i] = s.Source
		for _, m := range s.Measurements {
			key := m.Start.Unix()
			if older, exists := index[key]; exists && older.source != i {
				recordOverlap(overlaps, sourcePair{i, older.source}, m.Start)
			}
			index[key] = entry{m, i}
		}
	}
	if len(index) == 0 {
		return Curve{}, nil, errors.New("aucune mesure dans les fichiers fournis")
	}

	measurements := make([]Measurement, 0, len(index))
	for _, e := range index {
		measurements = append(measurements, e.measurement)
	}
	sort.Slice(measurements, func(i, j int) bool { return measurements[i].Start.Before(measurements[j].Start) })

	warnings := overlapWarnings(overlaps, sources)
	measurements, gaps := fillGaps(measurements)
	return Curve{PRM: prm, Sources: sources, Measurements: measurements}, append(warnings, gaps...), nil
}

func commonPRM(statements []Statement) (string, error) {
	prm, source := "", ""
	for _, s := range statements {
		if s.PRM == "" {
			continue
		}
		if prm == "" {
			prm, source = s.PRM, s.Source
		} else if s.PRM != prm {
			return "", fmt.Errorf("PRM différents : %s (%s) et %s (%s)", prm, source, s.PRM, s.Source)
		}
	}
	return prm, nil
}

func recordOverlap(m map[sourcePair]*overlap, p sourcePair, start time.Time) {
	o := m[p]
	if o == nil {
		o = &overlap{min: start, max: start}
		m[p] = o
	}
	o.count++
	if start.Before(o.min) {
		o.min = start
	}
	if start.After(o.max) {
		o.max = start
	}
}

func overlapWarnings(m map[sourcePair]*overlap, sources []string) []string {
	pairs := make([]sourcePair, 0, len(m))
	for p := range m {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].newer != pairs[j].newer {
			return pairs[i].newer < pairs[j].newer
		}
		return pairs[i].older < pairs[j].older
	})
	var out []string
	for _, p := range pairs {
		o := m[p]
		out = append(out, fmt.Sprintf("données en doublon : %d créneau(x) de %q (du %s au %s) écrase(nt) ceux de %q",
			o.count, sources[p.newer], o.min.Format(warningFormat), o.max.Format(warningFormat), sources[p.older]))
	}
	return out
}

// fillGaps inserts missing slots into the gaps of the series (REQ-IN-12).
func fillGaps(measurements []Measurement) ([]Measurement, []string) {
	out := make([]Measurement, 0, len(measurements))
	var warnings []string
	for i, m := range measurements {
		if i > 0 {
			prev := measurements[i-1]
			step := prev.Duration()
			if step > 0 && m.Start.After(prev.End) {
				n := 0
				for t := prev.End; t.Before(m.Start); t = t.Add(step) {
					end := t.Add(step)
					if end.After(m.Start) {
						end = m.Start
					}
					out = append(out, Measurement{Start: t, End: end, Missing: true})
					n++
				}
				warnings = append(warnings, fmt.Sprintf("trou dans les données du %s au %s (%d créneau(x) manquant(s))",
					prev.End.Format(warningFormat), m.Start.Format(warningFormat), n))
			}
		}
		out = append(out, m)
	}
	return out, warnings
}

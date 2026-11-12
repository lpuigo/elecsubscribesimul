// Package calendar associates a color with each day (Tempo).
package calendar

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const keyFormat = "2006-01-02"

// Calendar gives the color of a day, with an optional default color for unknown days.
type Calendar struct {
	colors       map[string]string
	defaultColor string
}

// New builds a calendar from colors indexed by "YYYY-MM-DD" date.
func New(colors map[string]string, defaultColor string) Calendar {
	return Calendar{colors: colors, defaultColor: strings.ToLower(defaultColor)}
}

// Load reads a `date;color` CSV file (header optional).
func Load(path, defaultColor string) (Calendar, error) {
	f, err := os.Open(path)
	if err != nil {
		return Calendar{}, fmt.Errorf("lecture du calendrier des couleurs: %w", err)
	}
	defer f.Close()
	colors, err := ReadColors(f)
	if err != nil {
		return Calendar{}, fmt.Errorf("%s: %w", path, err)
	}
	return New(colors, defaultColor), nil
}

// ReadColors parses `date;color` CSV records into colors indexed by "YYYY-MM-DD" date.
func ReadColors(r io.Reader) (map[string]string, error) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	colors := map[string]string{}
	for line := 1; ; line++ {
		rec, err := cr.Read()
		if err == io.EOF {
			return colors, nil
		}
		if err != nil {
			return nil, fmt.Errorf("ligne %d: %w", line, err)
		}
		if len(rec) < 2 {
			return nil, fmt.Errorf("ligne %d: format attendu date;couleur", line)
		}
		date := strings.TrimSpace(strings.TrimPrefix(rec[0], "\xef\xbb\xbf"))
		if _, err := time.Parse(keyFormat, date); err != nil {
			if line == 1 {
				continue // header
			}
			return nil, fmt.Errorf("ligne %d: date %q invalide (attendu AAAA-MM-JJ)", line, date)
		}
		colors[date] = strings.ToLower(strings.TrimSpace(rec[1]))
	}
}

// Color returns the color of a day; assumed is true when the default color was used.
func (c Calendar) Color(day time.Time) (color string, assumed, ok bool) {
	if v, found := c.colors[day.Format(keyFormat)]; found {
		return v, false, true
	}
	if c.defaultColor != "" {
		return c.defaultColor, true, true
	}
	return "", false, false
}

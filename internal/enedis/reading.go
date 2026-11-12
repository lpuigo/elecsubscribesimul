// Package enedis reads the Enedis load curve exports (.xlsx).
package enedis

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/loadcurve"
)

const preferredSheet = "Puissance moyenne soutirée"

var (
	prmRegexp     = regexp.MustCompile(`\b\d{14}\b`)
	maxHeaderRows = 60
)

type columns struct{ start, end, value int }

// Read loads an Enedis file (REQ-IN-01 to REQ-IN-05) without hard-coding row numbers.
func Read(path string) (loadcurve.Statement, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return loadcurve.Statement{}, fmt.Errorf("%s: %w", path, err)
	}
	defer f.Close()

	rows, headerRow, cols, err := findData(f)
	if err != nil {
		return loadcurve.Statement{}, fmt.Errorf("%s: %w", path, err)
	}
	loc, err := Timezone()
	if err != nil {
		return loadcurve.Statement{}, err
	}
	reader := newTimestampReader(loc)

	statement := loadcurve.Statement{Source: path, PRM: extractPRM(rows[:headerRow])}
	for i := headerRow + 1; i < len(rows); i++ {
		row := rows[i]
		startCell := cell(row, cols.start)
		if strings.TrimSpace(startCell) == "" {
			break // first empty row
		}
		m, err := readMeasurement(reader, startCell, cell(row, cols.end), cell(row, cols.value))
		if err != nil {
			return loadcurve.Statement{}, fmt.Errorf("%s, ligne %d: %w", path, i+1, err)
		}
		statement.Measurements = append(statement.Measurements, m)
	}
	if len(statement.Measurements) == 0 {
		return loadcurve.Statement{}, fmt.Errorf("%s: aucune mesure trouvée", path)
	}
	return statement, nil
}

func readMeasurement(reader *timestampReader, start, end, value string) (loadcurve.Measurement, error) {
	s, wall, err := reader.start(start)
	if err != nil {
		return loadcurve.Measurement{}, err
	}
	e, err := reader.end(end, s, wall)
	if err != nil {
		return loadcurve.Measurement{}, err
	}
	m := loadcurve.Measurement{Start: s, End: e}
	kw, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
	if err != nil { // "NA", empty or unreadable: unavailable data (REQ-IN-03)
		m.Missing = true
		return m, nil
	}
	m.KW = kw
	return m, nil
}

// findData locates the data sheet and its header row, trying the preferred sheet first.
func findData(f *excelize.File) (rows [][]string, headerRow int, cols columns, err error) {
	names := f.GetSheetList()
	order := make([]string, 0, len(names))
	for _, n := range names {
		if strings.EqualFold(n, preferredSheet) {
			order = append(order, n)
		}
	}
	for _, n := range names {
		if !strings.EqualFold(n, preferredSheet) {
			order = append(order, n)
		}
	}
	for _, name := range order {
		rows, err := f.GetRows(name, excelize.Options{RawCellValue: true})
		if err != nil {
			continue
		}
		for i := 0; i < len(rows) && i < maxHeaderRows; i++ {
			if c, ok := findHeader(rows[i]); ok {
				return rows, i, c, nil
			}
		}
	}
	return nil, 0, columns{}, errors.New("feuille de données introuvable (en-tête Début / Fin / Valeur absent)")
}

func findHeader(row []string) (columns, bool) {
	c := columns{-1, -1, -1}
	for i, v := range row {
		s := strings.ToLower(strings.TrimSpace(v))
		switch {
		case s == "début" || s == "debut":
			c.start = i
		case s == "fin":
			c.end = i
		case strings.HasPrefix(s, "valeur"):
			c.value = i
		}
	}
	return c, c.start >= 0 && c.end >= 0 && c.value >= 0
}

func extractPRM(rows [][]string) string {
	for _, r := range rows {
		if m := prmRegexp.FindString(strings.Join(r, " ")); m != "" {
			return m
		}
	}
	return ""
}

func cell(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

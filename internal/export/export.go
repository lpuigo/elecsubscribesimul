// Package export produces the Excel comparison workbook.
package export

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"

	"elecsubscribesimul/internal/loadcurve"
	"elecsubscribesimul/internal/pricing"
)

// Sheet names are part of the user-facing output and stay in French.
const (
	summarySheet    = "Synthèse journalière"
	detailSheet     = "Détail 30 min"
	parametersSheet = "Paramètres"

	maxExcelColumns   = 16384
	summaryDayColumns = 5 // label + 4 summary columns
	summary30Columns  = 4 // label + 3 summary columns
	lightGreen        = "C6EFCE"
	dateTimeFormat    = "02/01/2006 15:04"
	shortDateFormat   = "02/01/2006"
)

// Options tunes the content of the workbook.
type Options struct {
	Detail       bool
	SkipDetail30 bool
	Warnings     []string
	Generated    time.Time
}

// Write generates the workbook and returns the warnings specific to the export.
func Write(path string, c loadcurve.Curve, r pricing.Result, opts Options) ([]string, error) {
	f := excelize.NewFile()
	defer f.Close()
	st := newStyleFactory(f)

	if err := f.SetSheetName("Sheet1", summarySheet); err != nil {
		return nil, err
	}
	if err := writeSummary(f, st, r, opts.Detail); err != nil {
		return nil, fmt.Errorf("feuille %s: %w", summarySheet, err)
	}

	var warnings []string
	switch {
	case opts.SkipDetail30:
	case len(c.Measurements)+summary30Columns > maxExcelColumns:
		warnings = append(warnings, fmt.Sprintf(
			"feuille %q non générée : %d créneaux dépassent la limite de colonnes d'Excel", detailSheet, len(c.Measurements)))
	default:
		if _, err := f.NewSheet(detailSheet); err != nil {
			return nil, err
		}
		if err := writeDetail(f, st, c, r, opts.Detail); err != nil {
			return nil, fmt.Errorf("feuille %s: %w", detailSheet, err)
		}
	}

	if _, err := f.NewSheet(parametersSheet); err != nil {
		return nil, err
	}
	if err := writeParameters(f, st, c, r, opts, warnings); err != nil {
		return nil, fmt.Errorf("feuille %s: %w", parametersSheet, err)
	}
	f.SetActiveSheet(0)
	if err := f.SaveAs(path); err != nil {
		return nil, fmt.Errorf("écriture de %s: %w", path, err)
	}
	return warnings, nil
}

// writer simplifies cell-by-cell writing by remembering the first error.
type writer struct {
	f     *excelize.File
	sheet string
	err   error
}

func (w *writer) val(col, row int, v any, style int) {
	if w.err != nil {
		return
	}
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		w.err = err
		return
	}
	if err := w.f.SetCellValue(w.sheet, cell, v); err != nil {
		w.err = err
		return
	}
	if style != 0 {
		w.err = w.f.SetCellStyle(w.sheet, cell, cell, style)
	}
}

// freeze freezes the first column(s) up to the given count and the header row.
func (w *writer) freeze(columns int) {
	if w.err != nil {
		return
	}
	cell, _ := excelize.CoordinatesToCellName(columns+1, 2)
	w.err = w.f.SetPanes(w.sheet, &excelize.Panes{
		Freeze: true, XSplit: columns, YSplit: 1, TopLeftCell: cell, ActivePane: "bottomRight",
	})
}

func (w *writer) width(from, to int, width float64) {
	if w.err != nil {
		return
	}
	a, _ := excelize.ColumnNumberToName(from)
	b, _ := excelize.ColumnNumberToName(to)
	w.err = w.f.SetColWidth(w.sheet, a, b, width)
}

package export

import "github.com/xuri/excelize/v2"

const (
	formatEuro = `#,##0.00 "€"`
	formatKWh  = `#,##0.000`
	formatKW   = `#,##0.000`
)

type specStyle struct {
	format      string
	gras        bool
	bordureG    bool
	remplissage string
}

// fabriqueStyles crée et mémorise les styles du classeur (police Arial, formats français).
type fabriqueStyles struct {
	f     *excelize.File
	cache map[specStyle]int
}

func nouvelleFabrique(f *excelize.File) *fabriqueStyles {
	return &fabriqueStyles{f: f, cache: map[specStyle]int{}}
}

func (s *fabriqueStyles) id(spec specStyle) int {
	if id, ok := s.cache[spec]; ok {
		return id
	}
	style := &excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 10, Bold: spec.gras}}
	if spec.format != "" {
		fmtCopie := spec.format
		style.CustomNumFmt = &fmtCopie
	}
	if spec.bordureG {
		style.Border = []excelize.Border{{Type: "left", Color: "000000", Style: 2}}
	}
	if spec.remplissage != "" {
		style.Fill = excelize.Fill{Type: "pattern", Color: []string{spec.remplissage}, Pattern: 1}
	}
	id, err := s.f.NewStyle(style)
	if err != nil {
		id = 0
	}
	s.cache[spec] = id
	return id
}

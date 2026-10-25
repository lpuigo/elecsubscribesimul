package export

import "github.com/xuri/excelize/v2"

const (
	euroFormat = `#,##0.00 "€"`
	kwhFormat  = `#,##0.000`
	kwFormat   = `#,##0.000`
)

type styleSpec struct {
	format    string
	bold      bool
	topBorder bool
	fill      string
}

// styleFactory creates and memoizes the workbook styles (Arial font, French formats).
type styleFactory struct {
	f     *excelize.File
	cache map[styleSpec]int
}

func newStyleFactory(f *excelize.File) *styleFactory {
	return &styleFactory{f: f, cache: map[styleSpec]int{}}
}

func (s *styleFactory) id(spec styleSpec) int {
	if id, ok := s.cache[spec]; ok {
		return id
	}
	style := &excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 10, Bold: spec.bold}}
	if spec.format != "" {
		formatCopy := spec.format
		style.CustomNumFmt = &formatCopy
	}
	if spec.topBorder {
		style.Border = []excelize.Border{{Type: "top", Color: "000000", Style: 2}}
	}
	if spec.fill != "" {
		style.Fill = excelize.Fill{Type: "pattern", Color: []string{spec.fill}, Pattern: 1}
	}
	id, err := s.f.NewStyle(style)
	if err != nil {
		id = 0
	}
	s.cache[spec] = id
	return id
}

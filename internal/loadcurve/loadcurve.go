// Package loadcurve holds the load curve model (measurements) and the merging of several statements.
package loadcurve

import "time"

// Measurement is the average power drawn over one slot.
type Measurement struct {
	Start   time.Time
	End     time.Time
	KW      float64
	Missing bool
}

// Duration is the length of the slot.
func (m Measurement) Duration() time.Duration { return m.End.Sub(m.Start) }

// KWh converts the average power into energy (REQ-IN-05).
func (m Measurement) KWh() float64 { return m.KW * m.Duration().Hours() }

// Statement is the content of one source file.
type Statement struct {
	Source       string
	PRM          string
	Measurements []Measurement
}

// Curve is the merged series, sorted by start instant.
type Curve struct {
	PRM          string
	Sources      []string
	Measurements []Measurement
}

// Start is the start instant of the first slot.
func (c Curve) Start() time.Time { return c.Measurements[0].Start }

// End is the end instant of the last slot.
func (c Curve) End() time.Time { return c.Measurements[len(c.Measurements)-1].End }

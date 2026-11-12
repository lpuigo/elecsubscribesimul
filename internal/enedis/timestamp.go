package enedis

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // time zones available on a Windows machine without a tzdata database

	"github.com/xuri/excelize/v2"
)

const enedisLayout = "02/01/2006 15:04:05"

var acceptedLayouts = []string{enedisLayout, "02/01/2006 15:04", "2006-01-02 15:04:05", "2006-01-02T15:04:05"}

// Timezone returns the time zone of Enedis timestamps (French legal time).
func Timezone() (*time.Location, error) { return time.LoadLocation("Europe/Paris") }

// timestampReader converts the local timestamps of a file into absolute instants.
// An ambiguous timestamp (switch to winter time) is resolved by order of appearance:
// 1st occurrence = summer time, 2nd = winter time (REQ-TIME-01).
type timestampReader struct {
	loc  *time.Location
	seen map[string]int
}

func newTimestampReader(loc *time.Location) *timestampReader {
	return &timestampReader{loc: loc, seen: map[string]int{}}
}

// normalize brings a cell (text or raw Excel date) back to wall-clock time in enedisLayout.
func normalize(cell string) (string, error) {
	s := strings.TrimSpace(cell)
	for _, l := range acceptedLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format(enedisLayout), nil
		}
	}
	if f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64); err == nil {
		t, err := excelize.ExcelDateToTime(f, false)
		if err == nil {
			return t.Round(time.Second).Format(enedisLayout), nil
		}
	}
	return "", fmt.Errorf("horodatage %q invalide (attendu JJ/MM/AAAA HH:MM:SS)", cell)
}

// candidates returns the possible instants for a wall-clock time, from oldest to newest.
func (r *timestampReader) candidates(wall string) ([]time.Time, error) {
	base, err := time.ParseInLocation(enedisLayout, wall, r.loc)
	if err != nil {
		return nil, err
	}
	var out []time.Time
	for _, d := range []time.Duration{-time.Hour, 0, time.Hour} {
		if c := base.Add(d); c.Format(enedisLayout) == wall {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		out = append(out, base)
	}
	return out, nil
}

// start resolves the start instant of a slot; it also returns the wall-clock time used by end.
func (r *timestampReader) start(cell string) (time.Time, string, error) {
	wall, err := normalize(cell)
	if err != nil {
		return time.Time{}, "", err
	}
	cands, err := r.candidates(wall)
	if err != nil {
		return time.Time{}, "", err
	}
	n := r.seen[wall]
	r.seen[wall]++
	if n >= len(cands) {
		n = len(cands) - 1
	}
	return cands[n], wall, nil
}

// end deduces the end instant of a slot. The retained duration is the smallest positive one between
// the wall-clock difference and the gap to the next candidate instant, which stays correct
// around time changes whatever way the export labels the end.
func (r *timestampReader) end(cell string, start time.Time, startWall string) (time.Time, error) {
	wall, err := normalize(cell)
	if err != nil {
		return time.Time{}, err
	}
	cands, err := r.candidates(wall)
	if err != nil {
		return time.Time{}, err
	}
	var duration time.Duration
	for _, c := range cands {
		if c.After(start) {
			duration = c.Sub(start)
			break
		}
	}
	tEnd, err1 := time.Parse(enedisLayout, wall)
	tStart, err2 := time.Parse(enedisLayout, startWall)
	if err1 == nil && err2 == nil {
		if d := tEnd.Sub(tStart); d > 0 && (duration == 0 || d < duration) {
			duration = d
		}
	}
	if duration <= 0 {
		return time.Time{}, fmt.Errorf("la fin %q n'est pas postérieure au début", cell)
	}
	return start.Add(duration), nil
}

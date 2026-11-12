package calendar

import (
	"strings"
	"testing"
	"time"
)

func TestReadColorsAndColor(t *testing.T) {
	m, err := ReadColors(strings.NewReader("date;couleur\n2026-09-01;Bleu\n2026-09-02;rouge\n"))
	if err != nil {
		t.Fatal(err)
	}
	cal := New(m, "blanc")
	day := func(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }
	if c, assumed, ok := cal.Color(day(1)); c != "bleu" || assumed || !ok {
		t.Errorf("01/09: %q %v %v", c, assumed, ok)
	}
	if c, assumed, ok := cal.Color(day(3)); c != "blanc" || !assumed || !ok {
		t.Errorf("03/09: %q %v %v", c, assumed, ok)
	}
	if _, _, ok := New(m, "").Color(day(3)); ok {
		t.Error("unknown day without default color: ok should be false")
	}
	if _, err := ReadColors(strings.NewReader("2026-09-01;bleu\nn'importe quoi;rouge\n")); err == nil {
		t.Error("invalid date should be an error")
	}
}

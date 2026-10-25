package calendrier

import (
	"strings"
	"testing"
	"time"
)

func TestLireEtCouleur(t *testing.T) {
	m, err := Lire(strings.NewReader("date;couleur\n2026-09-01;Bleu\n2026-09-02;rouge\n"))
	if err != nil {
		t.Fatal(err)
	}
	cal := Nouveau(m, "blanc")
	jour := func(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }
	if c, supposee, ok := cal.Couleur(jour(1)); c != "bleu" || supposee || !ok {
		t.Errorf("01/09 : %q %v %v", c, supposee, ok)
	}
	if c, supposee, ok := cal.Couleur(jour(3)); c != "blanc" || !supposee || !ok {
		t.Errorf("03/09 : %q %v %v", c, supposee, ok)
	}
	if _, _, ok := Nouveau(m, "").Couleur(jour(3)); ok {
		t.Error("jour absent sans couleur par défaut : ok attendu à false")
	}
	if _, err := Lire(strings.NewReader("2026-09-01;bleu\nn'importe quoi;rouge\n")); err == nil {
		t.Error("date invalide attendue en erreur")
	}
}

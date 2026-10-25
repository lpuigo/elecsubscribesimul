package abonnement

import (
	"fmt"
	"strconv"
	"strings"
)

// Minutes est une heure de la journée exprimée en minutes depuis minuit.
type Minutes int

const minutesParJour = 24 * 60

// ParseMinutes lit "HH:MM" ("24:00" accepté pour une fin de journée).
func ParseMinutes(s string) (Minutes, error) {
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return 0, fmt.Errorf("heure %q invalide (attendu HH:MM)", s)
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || mm < 0 || mm > 59 || hh*60+mm > minutesParJour {
		return 0, fmt.Errorf("heure %q invalide (attendu HH:MM)", s)
	}
	return Minutes(hh*60 + mm), nil
}

func (m Minutes) String() string { return fmt.Sprintf("%02d:%02d", int(m)/60, int(m)%60) }

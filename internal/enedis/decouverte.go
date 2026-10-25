package enedis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListerFichiers développe les sources (fichiers ou répertoires) en une liste ordonnée de fichiers .xlsx
// (REQ-IN-08). Un répertoire est parcouru sans récursion, par ordre alphabétique.
func ListerFichiers(sources []string) ([]string, error) {
	var fichiers []string
	for _, s := range sources {
		info, err := os.Stat(s)
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", s, err)
		}
		if !info.IsDir() {
			fichiers = append(fichiers, s)
			continue
		}
		entrees, err := os.ReadDir(s)
		if err != nil {
			return nil, fmt.Errorf("répertoire %q: %w", s, err)
		}
		n := 0
		for _, e := range entrees {
			nom := e.Name()
			if e.IsDir() || !strings.EqualFold(filepath.Ext(nom), ".xlsx") || strings.HasPrefix(nom, "~$") {
				continue
			}
			fichiers = append(fichiers, filepath.Join(s, nom))
			n++
		}
		if n == 0 {
			return nil, fmt.Errorf("répertoire %q: aucun fichier .xlsx", s)
		}
	}
	if len(fichiers) == 0 {
		return nil, fmt.Errorf("aucune source de courbe de charge fournie")
	}
	return fichiers, nil
}

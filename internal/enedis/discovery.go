package enedis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListFiles expands the sources (files or directories) into an ordered list of .xlsx files
// (REQ-IN-08). A directory is scanned without recursion, in alphabetical order.
func ListFiles(sources []string) ([]string, error) {
	var files []string
	for _, s := range sources {
		info, err := os.Stat(s)
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", s, err)
		}
		if !info.IsDir() {
			files = append(files, s)
			continue
		}
		entries, err := os.ReadDir(s)
		if err != nil {
			return nil, fmt.Errorf("répertoire %q: %w", s, err)
		}
		n := 0
		for _, e := range entries {
			name := e.Name()
			// "~$" prefix: Excel lock file of an open workbook
			if e.IsDir() || !strings.EqualFold(filepath.Ext(name), ".xlsx") || strings.HasPrefix(name, "~$") {
				continue
			}
			files = append(files, filepath.Join(s, name))
			n++
		}
		if n == 0 {
			return nil, fmt.Errorf("répertoire %q: aucun fichier .xlsx", s)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("aucune source de courbe de charge fournie")
	}
	return files, nil
}

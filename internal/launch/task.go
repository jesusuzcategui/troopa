package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/store"
)

func BranchName(prefix, typ, title string) string {
	prefix = strings.TrimSpace(prefix)
	typ = strings.TrimSpace(typ)
	if typ == "" {
		typ = store.TypeFeature
	}
	slug := store.Slugify(title)
	if slug == "" {
		slug = "tarea"
	}
	b := typ + "/" + slug
	if prefix == "" {
		return b
	}
	return strings.TrimSuffix(prefix, "/") + "/" + b
}

func WriteTASK(dir string, t store.Task) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", strings.TrimSpace(t.Title))
	if d := strings.TrimSpace(t.Description); d != "" {
		b.WriteString(d)
		b.WriteString("\n\n")
	}
	b.WriteString("## Criterios de aceptación\n\n")
	crit := strings.TrimSpace(t.AcceptanceCriteria)
	if crit == "" {
		b.WriteString("- Completar la tarea descrita.\n")
	} else {
		for _, line := range strings.Split(crit, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			line = strings.TrimPrefix(line, "- ")
			line = strings.TrimPrefix(line, "* ")
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}
	b.WriteString("\n## Reglas\n\n")
	b.WriteString("- Implementa solo lo necesario. No agregues abstracciones de más.\n")
	b.WriteString("- No toques tests solo para que pasen.\n")
	if t.Commits == store.CommitsNone {
		b.WriteString("- No hagas commits, push ni crees ramas. Deja los cambios sin commitear.\n")
	}
	b.WriteString("\nLee este archivo y cumple la tarea. Al terminar escribe RESUMEN.md.\n")
	return os.WriteFile(filepath.Join(dir, "TASK.md"), []byte(b.String()), 0o644)
}

func ReadSummary(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, "RESUMEN.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

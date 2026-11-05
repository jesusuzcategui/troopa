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
		slug = "task"
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
	b.WriteString("## Acceptance criteria\n\n")
	crit := strings.TrimSpace(t.AcceptanceCriteria)
	if crit == "" {
		b.WriteString("- Complete the task described above.\n")
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
	b.WriteString("\n## Rules\n\n")
	b.WriteString("- Implement only what is needed. Do not add extra abstractions.\n")
	b.WriteString("- Do not change tests just to make them pass.\n")
	if t.Commits == store.CommitsNone {
		b.WriteString("- Do not commit, push or create branches. Leave the changes uncommitted.\n")
	}
	b.WriteString("\nRead this file and complete the task. When you finish, write SUMMARY.md.\n")
	return os.WriteFile(filepath.Join(dir, "TASK.md"), []byte(b.String()), 0o644)
}

func ReadSummary(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, "SUMMARY.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

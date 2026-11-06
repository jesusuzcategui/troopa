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

// WriteTASK writes TASK.md. Project instructions come from troopa.toml at the
// root of the worktree, so every task of a project gets the same rules.
func WriteTASK(dir string, t store.Task, atts []store.Attachment) error {
	return writeTASK(dir, t, atts, "")
}

func writeTASK(dir string, t store.Task, atts []store.Attachment, projectCtx string) error {
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
	writeAttachmentsSection(&b, dir, atts)
	writeReferencedSection(&b, dir, t)
	if strings.TrimSpace(projectCtx) != "" {
		b.WriteString("\n")
		b.WriteString(projectCtx)
		if !strings.HasSuffix(projectCtx, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Rules\n\n")
	b.WriteString("- Implement only what is needed. Do not add extra abstractions.\n")
	b.WriteString("- Do not change tests just to make them pass.\n")
	if strings.TrimSpace(projectCtx) != "" {
		b.WriteString("- You may read the second brain files listed under Project context. Do not modify the second brain unless this task asks you to.\n")
	}
	if t.Commits == store.CommitsNone {
		b.WriteString("- Do not commit, push or create branches. Leave the changes uncommitted.\n")
	}
	b.WriteString("- In SUMMARY.md, start with a \"How to run\" section: setup (environment, dependencies), the command to run the change and how to test it.\n")
	if extra := LoadProjectInstructions(dir); extra != "" {
		b.WriteString("\n## Project instructions\n\n")
		b.WriteString(extra)
		b.WriteString("\n")
	}
	b.WriteString("\nRead this file and complete the task. When you finish, write SUMMARY.md.\n")
	return os.WriteFile(filepath.Join(dir, "TASK.md"), []byte(b.String()), 0o644)
}

func writeAttachmentsSection(b *strings.Builder, dir string, atts []store.Attachment) {
	if len(atts) == 0 {
		return
	}
	b.WriteString("\n## Attachments\n\n")
	for _, a := range atts {
		name := a.Name
		if name == "" {
			name = filepath.Base(a.Path)
		}
		rel := filepath.ToSlash(filepath.Join(workAttachmentsDir, name))
		fmt.Fprintf(b, "- `%s` (%s, %s)", rel, a.MIME, store.FormatSize(a.Size))
		if isAV(a.MIME) {
			note := "the agent may not be able to view video or audio; use this as a reference (name " + name
			if d := MediaDuration(filepath.Join(dir, filepath.FromSlash(rel))); d != "" {
				note += ", duration " + d
			} else if d := MediaDuration(a.Path); d != "" {
				note += ", duration " + d
			}
			note += ")"
			fmt.Fprintf(b, " — %s", note)
		}
		b.WriteString("\n")
	}
}

func writeReferencedSection(b *strings.Builder, dir string, t store.Task) {
	refs := ReferencedPaths(t.Description, t.AcceptanceCriteria)
	if len(refs) == 0 {
		return
	}
	b.WriteString("\n## Referenced files\n\n")
	for _, p := range refs {
		if WorktreeHas(dir, p) {
			fmt.Fprintf(b, "- `%s`\n", p)
			continue
		}
		fmt.Fprintf(b, "- `%s` (not found in the worktree)\n", p)
	}
}

func ReadSummary(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, "SUMMARY.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

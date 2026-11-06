// Package brain writes Troopa's second-brain markdown files.
package brain

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/store"
)

const sectionLimit = 1500

type Settings struct {
	Dir string
	Git bool
}

type SyncResult struct {
	Dir       string `json:"dir"`
	Projects  int    `json:"projects"`
	Tasks     int    `json:"tasks"`
	Committed bool   `json:"committed"`
}

func FromConfig(c config.Config) Settings {
	dir := c.Launch.Brain
	if dir == "" {
		dir = config.ResolveBrain(c.BrainRaw)
	}
	return Settings{Dir: dir, Git: c.BrainGit}
}

func ProjectFile(dir, slug string) string {
	return filepath.Join(dir, "projects", slug, "project.md")
}

func IndexFile(dir string) string {
	return filepath.Join(dir, "projects", "INDEX.md")
}

// TaskFile is projects/<project>/tasks/<id>-<slug>.md.
func TaskFile(dir, project string, t store.Task) string {
	slug := store.Slugify(t.Title)
	if slug == "" {
		slug = "task"
	}
	return filepath.Join(dir, "projects", project, "tasks", fmt.Sprintf("%d-%s.md", t.ID, slug))
}

// GuideFile is projects/<project>/guides/<day>-<slug>.md.
func GuideFile(dir, project, day, slug string) string {
	if slug == "" {
		slug = "task"
	}
	return filepath.Join(dir, "projects", project, "guides", day+"-"+slug+".md")
}

func Ensure(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("brain folder is empty")
	}
	if err := os.MkdirAll(filepath.Join(dir, "projects"), 0o755); err != nil {
		return err
	}
	files := map[string]string{
		filepath.Join(dir, "README.md"):            readmeBody,
		filepath.Join(dir, "inbox.md"):             "# Inbox\n\nCapture notes here.\n",
		filepath.Join(dir, "today.md"):             "# Today\n\n",
		filepath.Join(dir, "projects", "INDEX.md"): "# Projects\n\n| Slug | Name | Status | Stack |\n|------|------|--------|-------|\n",
	}
	for path, body := range files {
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := atomicWrite(path, body); err != nil {
			return err
		}
	}
	return nil
}

const readmeBody = `# Second brain

Troopa writes notes here from your projects and tasks. You edit **Current state**, **Next** and **Decisions** in each ` + "`project.md`" + `; Troopa only updates frontmatter, INDEX, History, tasks and guides.

` + "```" + `
.
├── README.md
├── inbox.md
├── today.md
├── projects/
│   ├── INDEX.md
│   └── <slug>/
│       ├── project.md
│       ├── tasks/
│       │   └── <id>-<slug>.md
│       └── guides/
│           └── <date>-<slug>.md
` + "```" + `
`

func OnProject(s Settings, projects []store.Project, changed store.Project) error {
	if err := Ensure(s.Dir); err != nil {
		return err
	}
	if err := WriteIndex(s.Dir, projects); err != nil {
		return err
	}
	if err := UpsertProject(s.Dir, changed); err != nil {
		return err
	}
	return Commit(s, "troopa: update "+slugOf(changed))
}

func OnTask(s Settings, p store.Project, t store.Task) error {
	if err := Ensure(s.Dir); err != nil {
		return err
	}
	if err := UpsertProject(s.Dir, p); err != nil {
		return err
	}
	if err := WriteTask(s.Dir, p, t); err != nil {
		return err
	}
	return Commit(s, "troopa: update "+slugOf(p))
}

func OnApprove(s Settings, p store.Project, t store.Task, run store.Run) error {
	if err := Ensure(s.Dir); err != nil {
		return err
	}
	line := fmt.Sprintf("- %s approved «%s»", dayOf(run.UpdatedAt), t.Title)
	if run.Branch != "" {
		line += " (`" + run.Branch + "`)"
	}
	if err := AppendHistory(s.Dir, slugOf(p), "troopa:approve:"+strconv.FormatInt(run.ID, 10), line); err != nil {
		return err
	}
	if err := WriteTask(s.Dir, p, t); err != nil {
		return err
	}
	return Commit(s, "troopa: update "+slugOf(p))
}

func OnMerge(s Settings, p store.Project, t store.Task, run store.Run) error {
	if err := Ensure(s.Dir); err != nil {
		return err
	}
	day := dayOf(run.UpdatedAt)
	line := fmt.Sprintf("- %s merged «%s»", day, t.Title)
	if run.Base != "" {
		line += " into `" + run.Base + "`"
	}
	if err := AppendHistory(s.Dir, slugOf(p), "troopa:merge:"+strconv.FormatInt(run.ID, 10), line); err != nil {
		return err
	}
	if err := WriteGuide(s.Dir, p, t, run, day); err != nil {
		return err
	}
	if err := WriteTask(s.Dir, p, t); err != nil {
		return err
	}
	return Commit(s, "troopa: update "+slugOf(p))
}

func Sync(s Settings, st *store.Store) (SyncResult, error) {
	out := SyncResult{Dir: s.Dir}
	if err := Ensure(s.Dir); err != nil {
		return out, err
	}
	projects, err := st.List(true)
	if err != nil {
		return out, err
	}
	if err := WriteIndex(s.Dir, projects); err != nil {
		return out, err
	}
	var nTasks int
	for _, p := range projects {
		if err := UpsertProject(s.Dir, p); err != nil {
			return out, err
		}
		tasks, err := st.ListTasks(p.ID, store.TaskFilter{})
		if err != nil {
			return out, err
		}
		for _, t := range tasks {
			if err := WriteTask(s.Dir, p, t); err != nil {
				return out, err
			}
			nTasks++
		}
	}
	runs, err := st.ListRuns("")
	if err != nil {
		return out, err
	}
	byID := map[int64]store.Project{}
	for _, p := range projects {
		byID[p.ID] = p
	}
	for _, run := range runs {
		p, ok := byID[run.ProjectID]
		if !ok {
			continue
		}
		t, err := st.GetTask(run.TaskID)
		if err != nil {
			continue
		}
		switch run.State {
		case store.RunApproved, store.RunMerged, store.RunCleaned:
			line := fmt.Sprintf("- %s approved «%s»", dayOf(run.UpdatedAt), t.Title)
			if run.Branch != "" {
				line += " (`" + run.Branch + "`)"
			}
			_ = AppendHistory(s.Dir, slugOf(p), "troopa:approve:"+strconv.FormatInt(run.ID, 10), line)
		}
		if run.State == store.RunMerged {
			line := fmt.Sprintf("- %s merged «%s»", dayOf(run.UpdatedAt), t.Title)
			if run.Base != "" {
				line += " into `" + run.Base + "`"
			}
			_ = AppendHistory(s.Dir, slugOf(p), "troopa:merge:"+strconv.FormatInt(run.ID, 10), line)
			_ = WriteGuide(s.Dir, p, t, run, dayOf(run.UpdatedAt))
		}
	}
	committed, err := commitResult(s, "troopa: sync")
	if err != nil {
		return out, err
	}
	out.Projects = len(projects)
	out.Tasks = nTasks
	out.Committed = committed
	return out, nil
}

func WriteIndex(dir string, projects []store.Project) error {
	var b strings.Builder
	b.WriteString("# Projects\n\n| Slug | Name | Status | Stack |\n|------|------|--------|-------|\n")
	for _, p := range projects {
		status := "active"
		if p.Archived {
			status = "archived"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", slugOf(p), escapePipe(p.Name), status, escapePipe(p.Stack))
	}
	return atomicWrite(IndexFile(dir), b.String())
}

func UpsertProject(dir string, p store.Project) error {
	slug := slugOf(p)
	if slug == "" {
		slug = "project"
	}
	path := ProjectFile(dir, slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := ""
	if raw, err := os.ReadFile(path); err == nil {
		_, body = splitFrontmatter(string(raw))
	} else if !os.IsNotExist(err) {
		return err
	}
	if strings.TrimSpace(body) == "" {
		body = defaultProjectBody(p)
	}
	fm := projectFrontmatter(p, slug)
	return atomicWrite(path, fm+"\n"+strings.TrimPrefix(body, "\n"))
}

func defaultProjectBody(p store.Project) string {
	return fmt.Sprintf("# %s\n\n## Current state\n\n## Next\n\n## Decisions\n\n## History\n\n", orName(p))
}

func projectFrontmatter(p store.Project, slug string) string {
	status := "active"
	if p.Archived {
		status = "archived"
	}
	return "---\n" +
		"name: " + strconv.Quote(p.Name) + "\n" +
		"slug: " + strconv.Quote(slug) + "\n" +
		"repo: " + strconv.Quote(p.RepoPath) + "\n" +
		"stack: " + strconv.Quote(p.Stack) + "\n" +
		"archived: " + strconv.FormatBool(p.Archived) + "\n" +
		"status: " + strconv.Quote(status) + "\n" +
		"---\n"
}

func WriteTask(dir string, p store.Project, t store.Task) error {
	path := TaskFile(dir, slugOf(p), t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %d\n", t.ID)
	fmt.Fprintf(&b, "title: %s\n", strconv.Quote(t.Title))
	fmt.Fprintf(&b, "status: %s\n", strconv.Quote(t.Status))
	fmt.Fprintf(&b, "priority: %s\n", strconv.Quote(t.Priority))
	fmt.Fprintf(&b, "project: %s\n", strconv.Quote(slugOf(p)))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", t.Title)
	if d := strings.TrimSpace(t.Description); d != "" {
		b.WriteString(d)
		b.WriteString("\n\n")
	}
	if c := strings.TrimSpace(t.AcceptanceCriteria); c != "" {
		b.WriteString("## Criteria\n\n")
		b.WriteString(c)
		b.WriteString("\n")
	}
	return atomicWrite(path, b.String())
}

func AppendHistory(dir, slug, id, line string) error {
	if slug == "" {
		return nil
	}
	path := ProjectFile(dir, slug)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	text := string(raw)
	marker := "<!-- " + id + " -->"
	if strings.Contains(text, marker) {
		return nil
	}
	entry := marker + "\n" + line + "\n"
	fm, body := splitFrontmatter(text)
	body = insertHistory(body, entry)
	out := text
	if fm != "" {
		out = "---\n" + fm + "\n---\n" + body
	} else {
		out = body
	}
	return atomicWrite(path, out)
}

func insertHistory(body, entry string) string {
	idx := indexHeading(body, "History")
	if idx < 0 {
		return strings.TrimRight(body, "\n") + "\n\n## History\n\n" + entry
	}
	rest := body[idx:]
	nl := strings.Index(rest, "\n")
	if nl < 0 {
		return body + "\n\n" + entry
	}
	after := rest[nl+1:]
	next := nextHeading(after)
	if next < 0 {
		return body[:idx+nl+1] + strings.TrimRight(after, "\n") + "\n" + entry
	}
	return body[:idx+nl+1] + after[:next] + entry + after[next:]
}

func WriteGuide(dir string, p store.Project, t store.Task, run store.Run, day string) error {
	if err := os.MkdirAll(filepath.Join(dir, "projects", slugOf(p), "guides"), 0o755); err != nil {
		return err
	}
	summary := strings.TrimSpace(run.Summary)
	how := HowToRun(summary)
	if how == "" && run.Worktree != "" {
		if raw, err := os.ReadFile(filepath.Join(run.Worktree, "SUMMARY.md")); err == nil {
			summary = strings.TrimSpace(string(raw))
			how = HowToRun(summary)
		}
	}
	slug := store.Slugify(t.Title)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", t.Title)
	fmt.Fprintf(&b, "Date: %s\nProject: %s\nTask: %d\n\n", day, slugOf(p), t.ID)
	b.WriteString("## How to run\n\n")
	if how == "" {
		b.WriteString("(not recorded)\n\n")
	} else {
		b.WriteString(how)
		b.WriteString("\n\n")
	}
	b.WriteString("## Summary\n\n")
	if summary == "" {
		b.WriteString(strings.TrimSpace(run.Note) + "\n")
	} else {
		b.WriteString(summary + "\n")
	}
	return atomicWrite(GuideFile(dir, slugOf(p), day, slug), b.String())
}

func HowToRun(summary string) string {
	idx := indexHeading(summary, "How to run")
	if idx < 0 {
		return ""
	}
	rest := summary[idx:]
	nl := strings.Index(rest, "\n")
	if nl < 0 {
		return ""
	}
	body := rest[nl+1:]
	if n := nextHeading(body); n >= 0 {
		body = body[:n]
	}
	return strings.TrimSpace(body)
}

func TASKContext(dir, slug string) string {
	if strings.TrimSpace(dir) == "" || slug == "" {
		return ""
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	proj := filepath.Join(absDir, "projects", slug, "project.md")
	index := filepath.Join(absDir, "projects", "INDEX.md")
	current, next, decisions := "", "", ""
	if raw, err := os.ReadFile(proj); err == nil {
		_, body := splitFrontmatter(string(raw))
		current = clip(section(body, "Current state"))
		next = clip(section(body, "Next"))
		decisions = clip(section(body, "Decisions"))
	}
	var b strings.Builder
	b.WriteString("## Project context\n\n")
	fmt.Fprintf(&b, "- Project file: `%s`\n", proj)
	fmt.Fprintf(&b, "- Index: `%s`\n\n", index)
	b.WriteString("### Current state\n\n")
	writeOrEmpty(&b, current)
	b.WriteString("### Next\n\n")
	writeOrEmpty(&b, next)
	b.WriteString("### Decisions\n\n")
	writeOrEmpty(&b, decisions)
	return b.String()
}

func writeOrEmpty(b *strings.Builder, s string) {
	if strings.TrimSpace(s) == "" {
		b.WriteString("(empty)\n\n")
		return
	}
	b.WriteString(s)
	b.WriteString("\n\n")
}

func section(body, title string) string {
	idx := indexHeading(body, title)
	if idx < 0 {
		return ""
	}
	rest := body[idx:]
	nl := strings.Index(rest, "\n")
	if nl < 0 {
		return ""
	}
	rest = rest[nl+1:]
	if n := nextHeading(rest); n >= 0 {
		rest = rest[:n]
	}
	return strings.TrimSpace(rest)
}

func clip(s string) string {
	r := []rune(s)
	if len(r) <= sectionLimit {
		return s
	}
	return string(r[:sectionLimit]) + "\n…"
}

func splitFrontmatter(s string) (yaml, body string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return "", s
	}
	rest := s[4:]
	i := strings.Index(rest, "\n---\n")
	if i < 0 {
		if strings.HasSuffix(rest, "\n---") {
			return rest[:len(rest)-4], ""
		}
		return "", s
	}
	return rest[:i], rest[i+5:]
}

func indexHeading(s, title string) int {
	needles := []string{"## " + title, "## " + title + "\n"}
	for _, n := range needles {
		if i := strings.Index(s, "## "+title); i >= 0 {
			before := ""
			if i > 0 {
				before = s[i-1 : i]
			}
			if i == 0 || before == "\n" {
				_ = n
				return i
			}
		}
	}
	return -1
}

func nextHeading(s string) int {
	off := 0
	for {
		i := strings.Index(s[off:], "\n## ")
		if i < 0 {
			return -1
		}
		return off + i + 1
	}
}

func slugOf(p store.Project) string {
	if s := strings.TrimSpace(p.Slug); s != "" {
		return s
	}
	s := store.Slugify(p.Name)
	if s == "" {
		return "project"
	}
	return s
}

func orName(p store.Project) string {
	if strings.TrimSpace(p.Name) != "" {
		return p.Name
	}
	return slugOf(p)
}

func dayOf(t time.Time) string {
	if t.IsZero() {
		return time.Now().Format("2006-01-02")
	}
	return t.Local().Format("2006-01-02")
}

func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func atomicWrite(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			os.Remove(tmp)
			return err
		}
	}
	return nil
}

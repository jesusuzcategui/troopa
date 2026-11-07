package brain

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Node struct {
	Rel   string
	Name  string
	Dir   bool
	Depth int
}

type Link struct {
	Label  string
	Target string // wiki name or markdown href as written
	Rel    string // resolved path inside the brain, slash-separated
}

type Hit struct {
	Rel  string `json:"rel"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

var (
	wikiLink = regexp.MustCompile(`\[\[([^\]\n]+)\]\]`)
	mdLink   = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
)

func ListNotes(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), ".md") {
			return nil
		}
		rel, ok := relInside(dir, path)
		if !ok {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out, err
}

func Tree(dir string, collapsed map[string]bool) ([]Node, error) {
	if collapsed == nil {
		collapsed = map[string]bool{}
	}
	var out []Node
	var walk func(rel string, depth int) error
	walk = func(rel string, depth int) error {
		abs := dir
		if rel != "" {
			abs = filepath.Join(dir, filepath.FromSlash(rel))
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool {
			return treeLess(entries[i], entries[j], rel)
		})
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			child := name
			if rel != "" {
				child = rel + "/" + name
			}
			if e.IsDir() {
				out = append(out, Node{Rel: child, Name: name, Dir: true, Depth: depth})
				if collapsed[child] {
					continue
				}
				if err := walk(child, depth+1); err != nil {
					return err
				}
				continue
			}
			if !strings.EqualFold(filepath.Ext(name), ".md") {
				continue
			}
			out = append(out, Node{Rel: child, Name: name, Depth: depth})
		}
		return nil
	}
	return out, walk("", 0)
}

func treeLess(a, b os.DirEntry, parent string) bool {
	ra, rb := treeRank(a.Name(), a.IsDir(), parent), treeRank(b.Name(), b.IsDir(), parent)
	if ra != rb {
		return ra < rb
	}
	return strings.ToLower(a.Name()) < strings.ToLower(b.Name())
}

func treeRank(name string, isDir bool, parent string) int {
	if parent == "" {
		switch name {
		case "inbox.md":
			return 0
		case "today.md":
			return 1
		case "README.md":
			return 2
		case "projects":
			return 100
		case "tasks":
			return 101
		case "guides":
			return 102
		}
	}
	if parent == "projects" && name == "INDEX.md" {
		return 0
	}
	if isDir {
		return 50
	}
	return 10
}

func Abs(dir, rel string) (string, bool) {
	return absInside(dir, rel)
}

func ReadNote(dir, rel string) (string, error) {
	path, ok := absInside(dir, rel)
	if !ok {
		return "", os.ErrNotExist
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func AppendInbox(dir, line string) error {
	if err := Ensure(dir); err != nil {
		return err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if !strings.HasPrefix(line, "- ") {
		line = "- " + line
	}
	path := filepath.Join(dir, "inbox.md")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n" + line + "\n")
	return err
}

func NoteLinks(dir, rel, body string) []Link {
	seen := map[string]bool{}
	var out []Link
	add := func(label, target string) {
		target = strings.TrimSpace(target)
		if target == "" || isExternal(target) {
			return
		}
		resolved := Resolve(dir, rel, target)
		if resolved == "" || seen[resolved] {
			return
		}
		seen[resolved] = true
		if label == "" {
			label = target
		}
		out = append(out, Link{Label: label, Target: target, Rel: resolved})
	}
	for _, m := range wikiLink.FindAllStringSubmatch(body, -1) {
		add(strings.TrimSpace(m[1]), m[1])
	}
	for _, m := range mdLink.FindAllStringSubmatch(body, -1) {
		add(strings.TrimSpace(m[1]), m[2])
	}
	return out
}

func Backlinks(dir, rel string) []Link {
	notes, err := ListNotes(dir)
	if err != nil {
		return nil
	}
	rel = filepath.ToSlash(rel)
	var out []Link
	for _, other := range notes {
		if other == rel {
			continue
		}
		body, err := ReadNote(dir, other)
		if err != nil {
			continue
		}
		for _, l := range NoteLinks(dir, other, body) {
			if l.Rel == rel {
				out = append(out, Link{Label: other, Target: other, Rel: other})
				break
			}
		}
	}
	return out
}

func Search(dir, q string) []Hit {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	notes, err := ListNotes(dir)
	if err != nil {
		return nil
	}
	var out []Hit
	for _, rel := range notes {
		body, err := ReadNote(dir, rel)
		if err != nil {
			continue
		}
		n := 0
		sc := bufio.NewScanner(strings.NewReader(body))
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			n++
			line := sc.Text()
			if strings.Contains(strings.ToLower(line), q) {
				out = append(out, Hit{Rel: rel, Line: n, Text: strings.TrimSpace(line)})
			}
		}
	}
	return out
}

func Resolve(dir, fromRel, target string) string {
	target = strings.TrimSpace(target)
	target = strings.Trim(target, "\"'")
	if i := strings.IndexAny(target, " \t#"); i >= 0 {
		target = target[:i]
	}
	if target == "" || isExternal(target) {
		return ""
	}
	target = strings.ReplaceAll(target, "\\", "/")
	fromDir := ""
	if i := strings.LastIndex(fromRel, "/"); i >= 0 {
		fromDir = fromRel[:i]
	}
	if strings.Contains(target, "/") || strings.HasPrefix(target, ".") {
		joined := filepath.Clean(filepath.Join(filepath.FromSlash(fromDir), filepath.FromSlash(target)))
		if rel, ok := relInside(dir, filepath.Join(dir, joined)); ok {
			if noteExists(dir, rel) {
				return rel
			}
			if noteExists(dir, rel+".md") {
				return rel + ".md"
			}
		}
	}
	if notes, err := ListNotes(dir); err == nil {
		want := strings.ToLower(strings.TrimSuffix(filepath.Base(target), ".md"))
		var hits []string
		for _, n := range notes {
			base := strings.TrimSuffix(strings.ToLower(filepath.Base(n)), ".md")
			if n == target || strings.ToLower(n) == strings.ToLower(target) || strings.ToLower(n) == strings.ToLower(target)+".md" {
				return n
			}
			if base == want {
				hits = append(hits, n)
			}
		}
		if len(hits) == 1 {
			return hits[0]
		}
		proj := "projects/" + strings.TrimSuffix(target, ".md") + "/project.md"
		for _, n := range notes {
			if n == proj {
				return n
			}
		}
		if len(hits) > 0 {
			return hits[0]
		}
	}
	return ""
}

func WikiToMarkdown(body string) string {
	return wikiLink.ReplaceAllStringFunc(body, func(s string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(s, "[["), "]]")
		return "[" + inner + "](" + inner + ")"
	})
}

func noteExists(dir, rel string) bool {
	path, ok := absInside(dir, rel)
	if !ok {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func isExternal(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	return strings.Contains(t, "://") || strings.HasPrefix(t, "mailto:")
}

func relInside(dir, path string) (string, bool) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absDir, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func absInside(dir, rel string) (string, bool) {
	if strings.TrimSpace(rel) == "" {
		return "", false
	}
	rel = filepath.FromSlash(rel)
	if filepath.IsAbs(rel) {
		return "", false
	}
	path := filepath.Join(dir, rel)
	if _, ok := relInside(dir, path); !ok {
		return "", false
	}
	return path, true
}

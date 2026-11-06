package brain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeOrderAndFold(t *testing.T) {
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "projects", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "projects", "app", "project.md"), []byte("# App\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, err := Tree(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(nodes))
	for i, n := range nodes {
		got[i] = n.Rel
	}
	wantHead := []string{"inbox.md", "today.md", "README.md", "projects", "projects/INDEX.md", "projects/app", "projects/app/project.md"}
	for i, w := range wantHead {
		if i >= len(got) || got[i] != w {
			t.Fatalf("tree[%d]=%q want %q\n%q", i, got[i], w, got)
		}
	}
	folded, err := Tree(dir, map[string]bool{"projects": true})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range folded {
		if strings.HasPrefix(n.Rel, "projects/") {
			t.Fatalf("folded projects still listed %s", n.Rel)
		}
	}
}

func TestResolveWikiAndRelative(t *testing.T) {
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "projects", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "projects", "app", "project.md"), []byte("see [[inbox]] and [today](../../today.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Resolve(dir, "projects/app/project.md", "inbox"); got != "inbox.md" {
		t.Fatalf("wiki inbox: %q", got)
	}
	if got := Resolve(dir, "projects/app/project.md", "../../today.md"); got != "today.md" {
		t.Fatalf("relative: %q", got)
	}
	if got := Resolve(dir, "inbox.md", "app"); got != "projects/app/project.md" {
		t.Fatalf("project slug: %q", got)
	}
	body, _ := ReadNote(dir, "projects/app/project.md")
	links := NoteLinks(dir, "projects/app/project.md", body)
	if len(links) != 2 {
		t.Fatalf("links %+v", links)
	}
	backs := Backlinks(dir, "inbox.md")
	if len(backs) != 1 || backs[0].Rel != "projects/app/project.md" {
		t.Fatalf("backlinks %+v", backs)
	}
}

func TestSearchAndInbox(t *testing.T) {
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "today.md"), []byte("# Today\n\nShip the brain view.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits := Search(dir, "brain view")
	if len(hits) != 1 || hits[0].Rel != "today.md" || hits[0].Line != 3 {
		t.Fatalf("hits %+v", hits)
	}
	if err := AppendInbox(dir, "remember milk"); err != nil {
		t.Fatal(err)
	}
	body, _ := ReadNote(dir, "inbox.md")
	if !strings.Contains(body, "- remember milk") {
		t.Fatalf("inbox %q", body)
	}
}

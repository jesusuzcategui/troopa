package brain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildGraph(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("inbox.md", "see [[project]]\n")
	write("projects/INDEX.md", "- [App](app/project.md)\n")
	write("projects/app/project.md", "# App\n")
	write("projects/app/tasks/1-login.md", "# Login\n")
	write("projects/app/guides/2026-11-06-login.md", "back to [project](../project.md)\n")
	write("loose.md", "nothing\n")
	g, err := BuildGraph(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 6 {
		t.Fatalf("nodes %d", len(g.Nodes))
	}
	p := g.Index("projects/app/project.md")
	if p < 0 || g.Nodes[p].Kind != KindProject || g.Nodes[p].Label != "app" {
		t.Fatalf("project node %+v", g.Nodes)
	}
	if got := len(g.Neighbors(p)); got != 4 { // inbox [[project]], INDEX, task (folder), guide
		t.Fatalf("project neighbors %d %v", got, g.Edges)
	}
	if g.Nodes[g.Index("projects/app/tasks/1-login.md")].Kind != KindTask {
		t.Fatal("task kind")
	}
	for _, n := range g.Nodes {
		if n.X < 0 || n.X > 1 || n.Y < 0 || n.Y > 1 {
			t.Fatalf("layout out of range: %+v", n)
		}
	}
	again, _ := BuildGraph(dir)
	if again.Nodes[p].X != g.Nodes[p].X {
		t.Fatal("layout should be stable")
	}
}

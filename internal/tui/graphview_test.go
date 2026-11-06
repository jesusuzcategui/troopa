package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func sampleBrain(t *testing.T, dir string) {
	t.Helper()
	write := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("inbox.md", "- idea for [[solar-system]]\n")
	write("today.md", "- [[project]]\n")
	write("README.md", "[Index](projects/INDEX.md)\n")
	idx := "# Projects\n"
	for _, p := range []string{"solar-system", "clicktalk", "troopa"} {
		idx += fmt.Sprintf("- [%s](%s/project.md)\n", p, p)
		write("projects/"+p+"/project.md", "# "+p+"\n")
		for i := 1; i <= 4; i++ {
			write(fmt.Sprintf("projects/%s/tasks/%d-task-%s-%d.md", p, i, p, i), fmt.Sprintf("# Task %d\n[project](../project.md)\n", i))
		}
		write("projects/"+p+"/guides/2026-11-06-run.md", "[project](../project.md)\n")
	}
	write("projects/INDEX.md", idx)
}

func TestBrainGraphViews(t *testing.T) {
	m := newTestModel(t)
	sampleBrain(t, m.cfg.Launch.Brain)
	m.width, m.height = 110, 34
	next, _ := m.openBrain(screenList, "projects/solar-system/project.md")
	m = next.(Model)
	m = press(t, m, "G")
	if m.brainGraph == nil {
		t.Fatalf("G should open the graph: %q %q", m.err, m.status)
	}
	local := ansi.Strip(m.View())
	for _, want := range []string{"Brain graph", "solar-system", "task-solar-system-1", "INDEX"} {
		if !strings.Contains(local, want) {
			t.Fatalf("local view missing %q:\n%s", want, local)
		}
	}
	if strings.Contains(local, "task-clicktalk-1") {
		t.Fatal("local view (1 ring) should not show other projects' tasks")
	}
	m = press(t, m, "right")
	first := m.brainGraph.sel
	if first == m.brainGraph.center {
		t.Fatal("right should select a neighbor")
	}
	m = press(t, m, "enter")
	if m.brainGraph.center != first {
		t.Fatal("enter should recenter on the selection")
	}
	m = press(t, m, "v")
	whole := ansi.Strip(m.View())
	if !strings.Contains(whole, "clicktalk") || !strings.ContainsAny(whole, "⠁⠂⠄⡀⠈⠐⠠⢀⣀⠉⠒⠤") {
		t.Fatalf("whole view:\n%s", whole)
	}
	if os.Getenv("TROOPA_SHOW_GRAPH") != "" {
		fmt.Println(local)
		fmt.Println(whole)
	}
	m = press(t, m, "esc")
	if m.brainGraph != nil || m.screen != screenBrain {
		t.Fatal("esc should go back to the brain tree")
	}
	_ = tea.KeyMsg{}
}

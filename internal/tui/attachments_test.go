package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
)

func TestTaskFormShowsAttachmentsAndAtComplete(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := seedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	t0, ok := got.selectedTask()
	if !ok {
		t.Fatal("task")
	}
	src := filepath.Join(t.TempDir(), "spec.md")
	if err := os.WriteFile(src, []byte("# spec"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, err := config.AttachmentsDir(t0.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := got.store.AttachFile(t0.ID, src, dir, 50<<20); err != nil {
		t.Fatal(err)
	}
	got.openTaskForm(t0)
	view := got.viewTaskForm()
	if !strings.Contains(view, "spec.md") {
		t.Fatalf("form missing attachment: %s", view)
	}
	q, start, ok := mentionFromTextarea("See @int", 0, len("See @int"))
	if !ok || q != "int" || start != 4 {
		t.Fatalf("mention %q %d %v", q, start, ok)
	}
}

func TestMentionCompleteInsertsPath(t *testing.T) {
	m := newTestModel(t)
	m.taskFocus = tfDesc
	m.taskDesc.SetValue("See @")
	m.taskDesc.SetCursor(5)
	m.repoFiles = []string{"internal/store/tasks.go"}
	m.refreshMentions()
	if len(m.mentionHits) != 1 {
		t.Fatalf("hits %v", m.mentionHits)
	}
	next, _ := m.applyMention()
	got := next
	if !strings.Contains(got.taskDesc.Value(), "@internal/store/tasks.go") {
		t.Fatalf("value %q", got.taskDesc.Value())
	}
}

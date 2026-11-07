package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cfg := config.Config{Theme: "dark", WelcomeShown: true, SetupDone: true, BrainRaw: filepath.Join(dir, "brain")}
	cfg.Launch.Brain = cfg.BrainRaw
	return New(s, cfg, filepath.Join(dir, "config.toml"))
}

func TestQuitKey(t *testing.T) {
	m := newTestModel(t)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	_ = next
}

func TestNewOpensForm(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := next.(Model)
	if got.screen != screenForm {
		t.Fatalf("screen = %v, want form", got.screen)
	}
}

func TestHelpKey(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := next.(Model)
	if got.screen != screenHelp {
		t.Fatalf("screen = %v, want help", got.screen)
	}
}

func TestMouseSelectsRow(t *testing.T) {
	m := newTestModel(t)
	if _, err := m.store.Create(store.Project{Name: "Uno", RepoPath: "/u"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.Create(store.Project{Name: "Dos", RepoPath: "/d"}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	m.listOriginY = 3
	m.listHeight = 10
	next, _ := m.Update(tea.MouseMsg{
		X: 1, Y: 4 + m.listLogoRows(),
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	got := next.(Model)
	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 (second row)", got.cursor)
	}
}

func TestListShowsLogoWhenTall(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	if v := m.View(); !strings.Contains(v, "▀█▀ █▀█") || !strings.Contains(v, "[◉_◉]") {
		t.Fatal("tall terminal should show the logo above the projects")
	}
	m.height = 18
	if strings.Contains(m.View(), "[◉_◉]") {
		t.Fatal("short terminal should hide the logo")
	}
}

func TestSyncReloadsBoardChangedElsewhere(t *testing.T) {
	m := newTestModel(t)
	p, _ := m.store.Create(store.Project{Name: "Sync", Slug: "sync", RepoPath: t.TempDir()})
	m.boardProject = p
	m.screen = screenBoard
	m.reloadTasks()
	m = m.syncFromStore() // first stamp, nothing to reload
	if _, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "From the bot", Status: store.StatusBacklog}); err != nil {
		t.Fatal(err)
	}
	m = m.syncFromStore()
	found := false
	for _, tk := range m.tasks {
		found = found || tk.Title == "From the bot"
	}
	if !found {
		t.Fatal("the board should pick up a task created by another process")
	}
}

package tui

import (
	"path/filepath"
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
	return New(s, config.Config{Theme: "dark", WelcomeShown: true}, filepath.Join(dir, "config.toml"))
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
		X: 1, Y: 4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	got := next.(Model)
	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 (second row)", got.cursor)
	}
}

package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func seedBrainNotes(t *testing.T, m Model) Model {
	t.Helper()
	dir := m.brainDir()
	if err := brain.Ensure(dir); err != nil {
		t.Fatal(err)
	}
	inbox := "# Inbox\n\nSee [[today]] and [index](projects/INDEX.md).\n"
	if err := os.WriteFile(filepath.Join(dir, "inbox.md"), []byte(inbox), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "today.md"), []byte("# Today\n\nShip the brain view.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBrainOpensFromListAndBoard(t *testing.T) {
	m := seedBrainNotes(t, newTestModel(t))
	m.width, m.height = 140, 30
	m = press(t, m, "B")
	if m.screen != screenBrain {
		t.Fatalf("screen %v", m.screen)
	}
	v := m.View()
	if !strings.Contains(v, "inbox.md") || !strings.Contains(v, "today.md") {
		t.Fatalf("tree missing:\n%s", v)
	}
	if !strings.Contains(v, "Inbox") && !strings.Contains(strings.ToLower(v), "inbox") {
		t.Fatalf("note missing:\n%s", v)
	}
	m = press(t, m, "esc")
	if m.screen != screenList {
		t.Fatalf("esc should leave brain, got %v", m.screen)
	}

	if _, err := m.store.Create(store.Project{Name: "App", Slug: "app", RepoPath: "/a"}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	m = press(t, m, "enter", "B")
	if m.screen != screenBrain || m.brainRel != "projects/app/project.md" {
		t.Fatalf("board B: screen=%v rel=%s", m.screen, m.brainRel)
	}
}

func TestBrainNarrowPanes(t *testing.T) {
	m := seedBrainNotes(t, newTestModel(t))
	m.width, m.height = 80, 24
	m = press(t, m, "B")
	v := m.View()
	if strings.Contains(v, "See") || strings.Contains(v, "Links") {
		t.Fatalf("narrow should start on the tree:\n%s", v)
	}
	// inbox.md is the first node
	m = press(t, m, "enter")
	if !m.brainNotePane {
		t.Fatal("enter should open the note pane")
	}
	if !strings.Contains(m.View(), "Links") {
		t.Fatalf("note pane missing links:\n%s", m.View())
	}
	m = press(t, m, "esc")
	if m.brainNotePane || m.screen != screenBrain {
		t.Fatalf("esc should return to the tree, pane=%v screen=%v", m.brainNotePane, m.screen)
	}
}

func TestBrainFollowLinkAndBack(t *testing.T) {
	m := seedBrainNotes(t, newTestModel(t))
	m.width, m.height = 140, 30
	m = press(t, m, "B")
	m = press(t, m, "tab")
	if m.brainLinkIdx < 0 {
		t.Fatal("tab should select a link")
	}
	m = press(t, m, "enter")
	if m.brainRel != "today.md" {
		t.Fatalf("follow wiki: %s", m.brainRel)
	}
	backs := m.View()
	if !strings.Contains(backs, "inbox.md") {
		t.Fatalf("backlinks missing:\n%s", backs)
	}
	m = press(t, m, "esc")
	if m.brainRel != "inbox.md" {
		t.Fatalf("history back: %s", m.brainRel)
	}
}

func TestBrainSearch(t *testing.T) {
	m := seedBrainNotes(t, newTestModel(t))
	m.width, m.height = 140, 30
	m = press(t, m, "B", "/")
	if !m.brainFind {
		t.Fatal("expected search")
	}
	for _, r := range "brain view" {
		m = press(t, m, string(r))
	}
	if len(m.brainHits) != 1 || m.brainHits[0].Rel != "today.md" || m.brainHits[0].Line != 3 {
		t.Fatalf("hits %+v", m.brainHits)
	}
	m = press(t, m, "enter")
	if m.brainRel != "today.md" || !strings.Contains(m.status, "today.md:3") {
		t.Fatalf("jump %s status %s", m.brainRel, m.status)
	}
}

func TestBrainInboxAndEditorReload(t *testing.T) {
	m := seedBrainNotes(t, newTestModel(t))
	m.width, m.height = 140, 30
	m = press(t, m, "B", "i")
	for _, r := range "remember milk" {
		m = press(t, m, string(r))
	}
	m = press(t, m, "enter")
	body, _ := brain.ReadNote(m.brainDir(), "inbox.md")
	if !strings.Contains(body, "- remember milk") {
		t.Fatalf("inbox %q", body)
	}
	path := filepath.Join(m.brainDir(), "inbox.md")
	if err := os.WriteFile(path, []byte("# Inbox\n\nedited outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(editorClosedMsg{})
	m = next.(Model)
	if !strings.Contains(m.brainBody, "edited outside") {
		t.Fatalf("reload %q", m.brainBody)
	}
}

func TestBrainPaletteAndGitCommit(t *testing.T) {
	m := seedBrainNotes(t, newTestModel(t))
	m.width, m.height = 140, 30
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = next.(Model)
	m.paletteInput.SetValue("second brain")
	m = press(t, m, "enter")
	if m.screen != screenBrain {
		t.Fatalf("palette: %v", m.screen)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	dir := m.brainDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	m.cfg.BrainGit = true
	if err := os.WriteFile(filepath.Join(dir, "today.md"), []byte("# dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.refreshBrainTree()
	if !m.brainDirty {
		t.Fatal("expected dirty")
	}
	if !strings.Contains(m.brainStatus(), "uncommitted") {
		t.Fatalf("status %s", m.brainStatus())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	if m.brainDirty {
		t.Fatal("should be clean after commit")
	}
}

func TestBrainMouseSelectsTree(t *testing.T) {
	m := seedBrainNotes(t, newTestModel(t))
	m.width, m.height = 140, 30
	m = press(t, m, "B")
	today := -1
	for i, n := range m.brainNodes {
		if n.Rel == "today.md" {
			today = i
			break
		}
	}
	if today < 0 {
		t.Fatal("today.md not in tree")
	}
	next, _ := m.Update(tea.MouseMsg{
		X: 1, Y: 2 + today, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	m = next.(Model)
	if m.brainRel != "today.md" {
		t.Fatalf("click opened %s", m.brainRel)
	}
}

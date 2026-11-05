package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestLaunchOnReadyShowsLogAndReview(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte("checks = [\"true\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho working\necho done > RESUMEN.md\necho n >> extra.txt\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	m := newTestModel(t)
	m.cfg.Launch.Worktrees = filepath.Join(t.TempDir(), "wt")
	m.cfg.Launch.Notify = false
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateTask(store.Task{
		ProjectID: p.ID, Title: "Boton", Agent: store.AgentClaude, Mode: store.ModeYolo, Status: store.StatusBacklog,
	}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	m.width, m.height = 120, 30
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	got.colIndex = indexOfStatus(store.StatusBacklog)
	got.cardIndex = 0
	got.clampBoardCursor()
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	got = next.(Model)
	if got.screen != screenLog {
		t.Fatalf("screen=%v err=%s", got.screen, got.err)
	}
	cmd, err := launch.AgentCommand(got.cfg.Launch, store.AgentClaude, "", got.runSess.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	msg := runAgent(cmd, got.runSess.LogPath, got.run.ID)()
	next, _ = got.Update(msg)
	got = next.(Model)
	if got.screen != screenReview {
		t.Fatalf("screen=%v want review err=%s", got.screen, got.err)
	}
	view := got.viewReview()
	if !strings.Contains(view, "Checks") {
		t.Fatalf("review missing checks: %s", view)
	}
	saved, err := got.store.GetRun(got.run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Agent != store.AgentClaude {
		t.Fatalf("agent=%q", saved.Agent)
	}
	if !saved.DurationMS.Valid {
		t.Fatal("expected duration")
	}
	if saved.TokensIn.Valid || saved.TokensContext.Valid {
		t.Fatalf("fake CLI must stay n/d: %+v", saved)
	}
}

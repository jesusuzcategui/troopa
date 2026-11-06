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

func TestRequestChangesRerunsInSameWorktree(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// The fake agent appends a line per run and records its prompt.
	script := "#!/bin/sh\necho \"$*\" > last_prompt.txt\necho run >> work.txt\necho done > SUMMARY.md\n"
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
	if _, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Hola", Agent: store.AgentClaude, Mode: store.ModeYolo, Status: store.StatusBacklog}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	m.width, m.height = 120, 30
	got := press(t, m, "enter")
	got.colIndex = indexOfStatus(store.StatusBacklog)
	got.cardIndex = 0
	got.clampBoardCursor()
	got = press(t, got, "]", "enter")
	if got.screen != screenLog {
		t.Fatalf("screen %v err %s", got.screen, got.err)
	}
	cmd, err := launch.AgentCommand(got.cfg.Launch, store.AgentClaude, "", got.runSess.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := got.Update(runAgent(cmd, got.runSess.LogPath, got.run.ID)())
	got = next.(Model)
	if got.screen != screenReview {
		t.Fatalf("want review, screen %v err %s", got.screen, got.err)
	}
	wt := got.runSess.Worktree
	runID := got.run.ID

	got = press(t, got, "x")
	if got.screen != screenChanges {
		t.Fatalf("x should open request changes, screen %v status %s", got.screen, got.status)
	}
	got.changesInput.SetValue("Add requirements.txt")
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	got = next.(Model)
	if got.screen != screenLog || got.run.ID != runID || got.run.State != store.RunRunning {
		t.Fatalf("rerun: screen %v run %d state %s err %s", got.screen, got.run.ID, got.run.State, got.err)
	}
	if raw, _ := os.ReadFile(filepath.Join(wt, "CHANGES.md")); !strings.Contains(string(raw), "Add requirements.txt") {
		t.Fatalf("CHANGES.md missing feedback: %s", raw)
	}
	cmd, err = launch.AgentCommandPrompt(got.cfg.Launch, store.AgentClaude, "", wt, launch.ChangesPrompt(wt, 1))
	if err != nil {
		t.Fatal(err)
	}
	next, _ = got.Update(runAgent(cmd, got.runSess.LogPath, got.run.ID)())
	got = next.(Model)
	if got.screen != screenReview {
		t.Fatalf("second round should end in review, screen %v", got.screen)
	}
	work, _ := os.ReadFile(filepath.Join(wt, "work.txt"))
	if strings.Count(string(work), "run") != 2 {
		t.Fatalf("the agent should work twice in the same worktree: %q", work)
	}
	prompt, _ := os.ReadFile(filepath.Join(wt, "last_prompt.txt"))
	if !strings.Contains(string(prompt), "CHANGES.md") {
		t.Fatalf("second prompt should point at CHANGES.md: %s", prompt)
	}
}

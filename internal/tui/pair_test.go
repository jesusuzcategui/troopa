package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestStartPairFinishAndPause(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX", "")

	prevOpen := openPairSession
	openPairSession = func(worktree, terminal string) (launch.PairProc, error) {
		pid := os.Getpid()
		_ = os.WriteFile(filepath.Join(worktree, "pair.pid"), []byte(strconv.Itoa(pid)+"\n"), 0o644)
		return launch.PairProc{Where: strconv.Itoa(pid)}, nil
	}
	t.Cleanup(func() { openPairSession = prevOpen })

	m := newTestModel(t)
	m.cfg.Launch.Worktrees = filepath.Join(t.TempDir(), "wt")
	m.cfg.Launch.Notify = false
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateTask(store.Task{
		ProjectID: p.ID, Title: "Boton", Agent: store.AgentClaude, Mode: store.ModePair, Status: store.StatusBacklog,
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
	if got.screen != screenPair {
		t.Fatalf("screen=%v err=%s", got.screen, got.err)
	}
	if _, err := os.Stat(filepath.Join(got.runSess.Worktree, "PAIR.md")); err != nil {
		t.Fatal("PAIR.md", err)
	}
	if _, err := os.Stat(filepath.Join(got.runSess.Worktree, "TASK.md")); err != nil {
		t.Fatal("TASK.md", err)
	}
	if err := os.WriteFile(filepath.Join(got.runSess.Worktree, "extra.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(got.runSess.Worktree, "RESUMEN.md"), []byte("listo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wt := got.runSess.Worktree
	next, _ = got.Update(pairClosedMsg{runID: got.run.ID})
	got = next.(Model)
	task, err := got.store.GetTask(got.run.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != store.StatusPairPaused {
		t.Fatalf("status=%s", task.Status)
	}
	run, err := got.store.LatestRun(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.State != store.RunPairPaused {
		t.Fatalf("run=%s", run.State)
	}
	if _, err := os.Stat(wt); err != nil {
		t.Fatal("worktree should remain")
	}

	got.colIndex = indexOfStatus(store.StatusPairPaused)
	got.cardIndex = 0
	got.clampBoardCursor()
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if got.screen != screenPair {
		t.Fatalf("resume screen=%v err=%s", got.screen, got.err)
	}
	if got.runSess.Worktree != wt {
		t.Fatalf("did not reuse worktree")
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	got = next.(Model)
	if got.screen != screenReview {
		t.Fatalf("finish screen=%v err=%s", got.screen, got.err)
	}
	task, _ = got.store.GetTask(got.run.TaskID)
	if task.Status != store.StatusReview {
		t.Fatalf("after finish status=%s", task.Status)
	}
	view := got.viewReview()
	if !strings.Contains(view, "Checks") {
		t.Fatalf("review: %s", view)
	}
}

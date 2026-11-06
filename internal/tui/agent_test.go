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
	script := "#!/bin/sh\necho working\necho done > SUMMARY.md\necho n >> extra.txt\n"
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
	if got.popup == nil || got.popup.kind != propConfirmLaunch {
		t.Fatalf("moving to ready should ask before launching, err=%s", got.err)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
	if !strings.Contains(view, "Rounds") || !strings.Contains(view, "Launch") {
		t.Fatalf("review missing rounds: %s", view)
	}
	rounds, err := got.store.ListRounds(got.run.ID)
	if err != nil || len(rounds) != 1 || rounds[0].Kind != store.RoundLaunch || rounds[0].State != store.RoundReview {
		t.Fatalf("rounds=%+v err=%v", rounds, err)
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

func TestReviewGMergesApprovedRun(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t)
	m.cfg.Launch.Worktrees = filepath.Join(t.TempDir(), "wt")
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	task, err := m.store.CreateTask(store.Task{
		ProjectID: p.ID, Title: "Boton", Agent: store.AgentClaude, Mode: store.ModeYolo,
		Status: store.StatusDone, Commits: store.CommitsAuto,
	})
	if err != nil {
		t.Fatal(err)
	}
	sess, err := launch.Prepare(m.cfg.Launch, p, task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "btn.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := launch.CommitAll(sess.Worktree, "troopa: "+task.Title); err != nil {
		t.Fatal(err)
	}
	run, err := m.store.CreateRun(store.Run{
		TaskID: task.ID, ProjectID: p.ID, Worktree: sess.Worktree, Branch: sess.Branch,
		Base: sess.Base, State: store.RunApproved, Agent: store.AgentClaude,
	})
	if err != nil {
		t.Fatal(err)
	}
	m.boardProject = p
	m.run = run
	m.runTask = task
	m.runSess = sess
	m.screen = screenReview
	m.width, m.height = 120, 30

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got := next.(Model)
	if got.popup == nil || got.popup.kind != propConfirmMerge {
		t.Fatalf("popup=%+v", got.popup)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got = next.(Model)
	if got.err != "" {
		t.Fatal(got.err)
	}
	saved, err := got.store.GetRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != store.RunMerged {
		t.Fatalf("state=%s", saved.State)
	}
	if launch.BranchExists(repo, sess.Branch) {
		t.Fatal("branch still exists")
	}
}

func approvedBoard(t *testing.T) (Model, string, launch.Session, store.Run) {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t)
	m.cfg.Launch.Worktrees = filepath.Join(t.TempDir(), "wt")
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	task, err := m.store.CreateTask(store.Task{
		ProjectID: p.ID, Title: "Boton", Agent: store.AgentClaude, Mode: store.ModeYolo,
		Status: store.StatusDone, Commits: store.CommitsAuto,
	})
	if err != nil {
		t.Fatal(err)
	}
	sess, err := launch.Prepare(m.cfg.Launch, p, task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "btn.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := launch.CommitAll(sess.Worktree, "troopa: "+task.Title); err != nil {
		t.Fatal(err)
	}
	run, err := m.store.CreateRun(store.Run{
		TaskID: task.ID, ProjectID: p.ID, Worktree: sess.Worktree, Branch: sess.Branch,
		Base: sess.Base, State: store.RunApproved, Agent: store.AgentClaude,
	})
	if err != nil {
		t.Fatal(err)
	}
	m.boardProject = p
	m.reloadTasks()
	m.selectTaskID(task.ID)
	m.screen = screenBoard
	m.width, m.height = 120, 40
	return m, repo, sess, run
}

func confirmPopup(t *testing.T, m Model) Model {
	t.Helper()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got := next.(Model)
	if got.err != "" {
		t.Fatal(got.err)
	}
	return got
}

func TestReviewCCleansWorktree(t *testing.T) {
	m, repo, sess, run := approvedBoard(t)
	m.screen = screenReview
	m.run = run
	m.runTask, _ = m.selectedTask()
	m.runSess = sess
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := next.(Model)
	if got.popup == nil || got.popup.kind != propConfirmClean {
		t.Fatalf("popup=%+v", got.popup)
	}
	got = confirmPopup(t, got)
	saved, err := got.store.GetRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != store.RunCleaned {
		t.Fatalf("state=%s", saved.State)
	}
	if _, err := os.Stat(sess.Worktree); !os.IsNotExist(err) {
		t.Fatal("worktree still exists")
	}
	if !launch.BranchExists(repo, sess.Branch) {
		t.Fatal("branch was deleted")
	}
}

func TestBoardGAndC(t *testing.T) {
	m, repo, sess, run := approvedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got := next.(Model)
	if got.popup == nil || got.popup.kind != propConfirmMerge {
		t.Fatalf("g popup=%+v", got.popup)
	}
	got.popup = nil

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got = next.(Model)
	if got.popup == nil || got.popup.kind != propConfirmClean {
		t.Fatalf("c popup=%+v", got.popup)
	}
	got = confirmPopup(t, got)
	saved, err := got.store.GetRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != store.RunCleaned {
		t.Fatalf("state=%s", saved.State)
	}
	if !launch.BranchExists(repo, sess.Branch) {
		t.Fatal("branch was deleted")
	}
}

func TestListCCleans(t *testing.T) {
	m, repo, sess, run := approvedBoard(t)
	m.cfg.BoardView = viewList
	if t0, ok := m.selectedTask(); ok {
		m.selectTaskID(t0.ID)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := next.(Model)
	if got.popup == nil || got.popup.kind != propConfirmClean {
		t.Fatalf("popup=%+v", got.popup)
	}
	got = confirmPopup(t, got)
	saved, err := got.store.GetRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != store.RunCleaned {
		t.Fatalf("state=%s", saved.State)
	}
	if !launch.BranchExists(repo, sess.Branch) {
		t.Fatal("branch was deleted")
	}
}

func TestDrawerGAndC(t *testing.T) {
	m, repo, sess, run := approvedBoard(t)
	m.drawerFocus = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got := next.(Model)
	if got.popup == nil || got.popup.kind != propConfirmMerge {
		t.Fatalf("g popup=%+v", got.popup)
	}
	got.popup = nil
	got.drawerFocus = true
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got = next.(Model)
	if got.popup == nil || got.popup.kind != propConfirmClean {
		t.Fatalf("c popup=%+v", got.popup)
	}
	got = confirmPopup(t, got)
	saved, err := got.store.GetRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != store.RunCleaned {
		t.Fatalf("state=%s", saved.State)
	}
	if !launch.BranchExists(repo, sess.Branch) {
		t.Fatal("branch was deleted")
	}
	if t0, ok := got.selectedTask(); ok {
		got.selectTaskID(t0.ID)
	}
	view := got.viewDrawer(50, 40)
	if !strings.Contains(view, store.RunCleaned) {
		t.Fatalf("drawer missing cleaned: %s", view)
	}
	if !strings.Contains(view, sess.Branch) {
		t.Fatalf("drawer missing branch: %s", view)
	}
}

func TestBoardGWithoutApprovedExplains(t *testing.T) {
	m := seedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = next.(Model)
	if !strings.Contains(got.status, "no approved") {
		t.Fatalf("status=%q", got.status)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got = next.(Model)
	if !strings.Contains(got.status, "no approved") {
		t.Fatalf("clean status=%q", got.status)
	}
}

func TestDiscardWarnsWhenApprovedCommitsAreUnmerged(t *testing.T) {
	m, _, sess, run := approvedBoard(t)
	m.screen = screenReview
	m.run = run
	m.runTask, _ = m.selectedTask()
	m.runSess = sess
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got := next.(Model)
	if got.popup == nil || got.popup.kind != propConfirmDiscard {
		t.Fatalf("popup=%+v", got.popup)
	}
	if !strings.Contains(got.popup.title, "will be lost") {
		t.Fatalf("missing unmerged warning: %q", got.popup.title)
	}
	if len(got.popup.options) < 3 || got.popup.options[0] != "merge" {
		t.Fatalf("want Merge instead first, got %+v", got.popup.options)
	}
}

func TestLegacyRunShowsOneRound(t *testing.T) {
	m := newTestModel(t)
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Old"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := m.store.CreateRun(store.Run{
		TaskID: task.ID, ProjectID: p.ID, State: store.RunReview, Summary: "## done\n",
		Checks: `[{"name":"true","ok":true}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	m.boardProject = p
	m.bindRun(task, run)
	m.loadRounds()
	m.screen = screenReview
	m.width, m.height = 80, 24
	view := m.viewReview()
	if !strings.Contains(view, "Launch") || !strings.Contains(view, "Rounds") {
		t.Fatalf("legacy run should show one round: %s", view)
	}
	if len(m.rounds) != 1 || m.rounds[0].ID != 0 {
		t.Fatalf("synthetic %+v", m.rounds)
	}
}

func TestRunFolderFallsBackToProject(t *testing.T) {
	m := newTestModel(t)
	m.boardProject.RepoPath = t.TempDir()
	m.run.Worktree = filepath.Join(t.TempDir(), "gone")
	if got := m.runFolder(); got != m.boardProject.RepoPath {
		t.Fatalf("runFolder=%q", got)
	}
}

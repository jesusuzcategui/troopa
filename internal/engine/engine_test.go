package engine

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func testEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	t.Cleanup(func() {
		// Let detached agents finish before the temp folders go away.
		runs, _ := s.ListRuns("")
		var wts []string
		for _, r := range runs {
			wts = append(wts, r.Worktree)
		}
		launch.WaitAgents(wts, 10*time.Second)
	})
	cfg := config.Config{WelcomeShown: true, SetupDone: true, BrainRaw: filepath.Join(dir, "brain")}
	cfg.Launch = config.DefaultLaunch(filepath.Join(dir, "data"))
	cfg.Launch.Brain = cfg.BrainRaw
	cfg.Launch.Notify = false
	return New(s, cfg, filepath.Join(dir, "config.toml")), s
}

func fakeClaude(t *testing.T, script string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func waitAgent(t *testing.T, worktree string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st, _ := launch.AgentStatus(worktree); st != launch.AgentRunning {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("agent still running")
}

func TestLaunchFinishWithFakeAgent(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte("checks = [\"true\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeClaude(t, "#!/bin/sh\necho working\necho done > SUMMARY.md\necho n >> extra.txt\n")
	e, s := testEngine(t)
	p, err := s.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(store.Task{
		ProjectID: p.ID, Title: "Button", Agent: store.AgentClaude, Mode: store.ModeYolo, Status: store.StatusBacklog,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Launch(task)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Watch || res.Run.State != store.RunRunning {
		t.Fatalf("%+v", res)
	}
	waitAgent(t, res.Run.Worktree)
	done, err := e.Finish(res.Run.ID, nil)
	if err != nil || !done.Won {
		t.Fatalf("finish: %+v err=%v", done, err)
	}
	if done.Run.State != store.RunReview {
		t.Fatalf("state=%s", done.Run.State)
	}
	tk, _ := s.GetTask(task.ID)
	if tk.Status != store.StatusReview {
		t.Fatalf("task %s", tk.Status)
	}
	rounds, err := s.ListRounds(res.Run.ID)
	if err != nil || len(rounds) != 1 || rounds[0].Kind != store.RoundLaunch || rounds[0].State != store.RoundReview {
		t.Fatalf("rounds=%+v err=%v", rounds, err)
	}
}

func TestFinishSameRunFromTwoEngines(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "db")
	s1, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s1.Close() })
	s2, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	cfg := config.Config{WelcomeShown: true, SetupDone: true, BrainRaw: filepath.Join(dir, "brain")}
	cfg.Launch = config.DefaultLaunch(filepath.Join(dir, "data"))
	cfg.Launch.Brain = cfg.BrainRaw
	cfg.Launch.Notify = false
	e1 := New(s1, cfg, filepath.Join(dir, "config.toml"))
	e2 := New(s2, cfg, filepath.Join(dir, "config.toml"))

	p, err := s1.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s1.CreateTask(store.Task{ProjectID: p.ID, Title: "Race", Agent: store.AgentClaude, Mode: store.ModeYolo, Status: store.StatusInProgress})
	if err != nil {
		t.Fatal(err)
	}
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, "SUMMARY.md"), []byte("one result\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(wt, "agent.log")
	_ = os.WriteFile(logPath, []byte("log\n"), 0o644)
	run, err := s1.CreateRun(store.Run{
		TaskID: task.ID, ProjectID: p.ID, State: store.RunRunning, Worktree: wt, LogPath: logPath, Agent: store.AgentClaude, Summary: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s1.CreateRound(store.Round{RunID: run.ID, Kind: store.RoundLaunch, State: store.RoundRunning}); err != nil {
		t.Fatal(err)
	}

	var (
		wg         sync.WaitGroup
		r1, r2     Result
		err1, err2 error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		r1, err1 = e1.Finish(run.ID, nil)
	}()
	go func() {
		defer wg.Done()
		r2, err2 = e2.Finish(run.ID, nil)
	}()
	wg.Wait()
	if err1 != nil || err2 != nil {
		t.Fatalf("err1=%v err2=%v", err1, err2)
	}
	won := 0
	for _, r := range []Result{r1, r2} {
		if r.Won {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("winners=%d r1=%+v r2=%+v", won, r1, r2)
	}
	got, err := s1.GetRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != store.RunReview || !strings.Contains(got.Summary, "one result") {
		t.Fatalf("%+v", got)
	}
}

func TestReconcileLostAndAlive(t *testing.T) {
	e, s := testEngine(t)
	p, err := s.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	lostTask, err := s.CreateTask(store.Task{ProjectID: p.ID, Title: "Lost", Agent: store.AgentClaude, Mode: store.ModeYolo, Status: store.StatusInProgress})
	if err != nil {
		t.Fatal(err)
	}
	wt := t.TempDir()
	logPath := filepath.Join(wt, "agent.log")
	_ = os.WriteFile(logPath, []byte("half way\n"), 0o644)
	lost, err := s.CreateRun(store.Run{TaskID: lostTask.ID, ProjectID: p.ID, State: store.RunRunning, Worktree: wt, LogPath: logPath, Agent: store.AgentClaude})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := e.Reconcile()
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Alive) != 0 {
		t.Fatalf("alive=%d", len(rec.Alive))
	}
	saved, _ := s.GetRun(lost.ID)
	if saved.State != store.RunBlocked || !strings.Contains(saved.Note, "x") {
		t.Fatalf("%+v", saved)
	}
}

func TestApproveRequestChangesDiscard(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte("checks = [\"true\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeClaude(t, "#!/bin/sh\necho done > SUMMARY.md\n")
	e, s := testEngine(t)
	p, err := s.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(store.Task{
		ProjectID: p.ID, Title: "Button", Agent: store.AgentClaude, Mode: store.ModeYolo,
		Status: store.StatusBacklog, Commits: store.CommitsNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Launch(task)
	if err != nil {
		t.Fatal(err)
	}
	waitAgent(t, res.Run.Worktree)
	done, err := e.Finish(res.Run.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.RequestChanges(done.Run.ID, "add a test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Watch || again.Run.State != store.RunRunning {
		t.Fatalf("%+v", again)
	}
	waitAgent(t, again.Run.Worktree)
	done, err = e.Finish(again.Run.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := e.Approve(done.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok.Run.State != store.RunApproved {
		t.Fatalf("state=%s", ok.Run.State)
	}
	drop, err := e.Discard(ok.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if drop.Run.State != store.RunDiscarded {
		t.Fatalf("state=%s", drop.Run.State)
	}
}

func TestStartPlanLaunchesFirstStep(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte("checks = [\"true\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeClaude(t, "#!/bin/sh\necho done > SUMMARY.md\n")
	e, s := testEngine(t)
	p, err := s.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Menu", Agent: store.AgentClaude, Auto: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Starters", "Desserts"} {
		if _, err := s.AddPlanStep(plan.ID, store.Task{Title: title, Commits: store.CommitsNone}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := e.StartPlan(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Watch {
		t.Fatalf("expected launch: %+v", res)
	}
	waitAgent(t, res.Run.Worktree)
	done, err := e.Finish(res.Run.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if done.Run.State != store.RunReview {
		t.Fatalf("state=%s", done.Run.State)
	}
	next, err := e.Approve(done.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !next.Watch || next.Run.TaskID == done.Run.TaskID {
		t.Fatalf("expected next step: %+v", next)
	}
}

func TestPausePlan(t *testing.T) {
	e, s := testEngine(t)
	p, err := s.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Menu", State: store.PlanRunning})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.PausePlan(plan.ID)
	if err != nil || !res.Won || res.Plan.State != store.PlanPaused {
		t.Fatalf("%+v err=%v", res, err)
	}
}

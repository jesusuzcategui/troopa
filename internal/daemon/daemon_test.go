package daemon

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/engine"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestAcquireRejectsSecondAndReplacesStale(t *testing.T) {
	dir := t.TempDir()
	unlock, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(dir); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("second acquire: %v", err)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	if _, ok := Running(dir); ok {
		t.Fatal("pid file should be gone")
	}

	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stale := cmd.Process.Pid
	_ = cmd.Wait()
	if err := os.WriteFile(PidPath(dir), []byte(strconv.Itoa(stale)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unlock, err = Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unlock() })
	pid, ok := Running(dir)
	if !ok || pid != os.Getpid() {
		t.Fatalf("running pid=%d ok=%v", pid, ok)
	}
}

func TestTickFinishesExitedAgentAndAdvancesPlan(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
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
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho done > SUMMARY.md\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cfg := config.Config{WelcomeShown: true, SetupDone: true, BrainRaw: filepath.Join(dir, "brain")}
	cfg.Launch = config.DefaultLaunch(filepath.Join(dir, "data"))
	cfg.Launch.Brain = cfg.BrainRaw
	cfg.Launch.Notify = false
	e := engine.New(s, cfg, filepath.Join(dir, "config.toml"))
	p, err := s.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Menu", Agent: store.AgentClaude, Auto: true, AutoApprove: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Starters", "Desserts"} {
		if _, err := s.AddPlanStep(plan.ID, store.Task{Title: title, Commits: store.CommitsNone}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := e.StartPlan(plan.ID)
	if err != nil || !res.Watch {
		t.Fatalf("start: %+v err=%v", res, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st, _ := launch.AgentStatus(res.Run.Worktree); st != launch.AgentRunning {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := Tick(e, log.New(io.Discard, "", 0)); err != nil {
		t.Fatal(err)
	}
	first, _ := s.GetRun(res.Run.ID)
	if first.State != store.RunApproved {
		t.Fatalf("first run %s", first.State)
	}
	steps, _ := s.PlanSteps(plan.ID)
	if steps[0].Status != store.StatusDone {
		t.Fatalf("step 1 %s", steps[0].Status)
	}
	if steps[1].Status != store.StatusInProgress {
		t.Fatalf("step 2 %s, want in-progress", steps[1].Status)
	}
	next, err := s.LatestRun(steps[1].ID)
	if err != nil || next.State != store.RunRunning {
		t.Fatalf("next run %+v err=%v", next, err)
	}
}

func TestStatusNotRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var buf bytes.Buffer
	if err := Status(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "not running") {
		t.Fatalf("%q", buf.String())
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	old := Interval
	Interval = 50 * time.Millisecond
	t.Cleanup(func() { Interval = old })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	data, _, _, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	for time.Now().Before(deadline) {
		if _, ok := Running(data); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, ok := Running(data); !ok {
		t.Fatal("daemon did not take the pid file")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return")
	}
	if _, ok := Running(data); ok {
		t.Fatal("pid file left behind")
	}
	raw, err := os.ReadFile(LogPath(data))
	if err != nil || !strings.Contains(string(raw), "started") {
		t.Fatalf("log: %s err=%v", raw, err)
	}
}

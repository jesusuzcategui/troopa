package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/engine"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func start(t *testing.T) (*mcpsdk.ClientSession, *store.Store, string) {
	t.Helper()
	session, s, brainDir, _ := startEng(t, nil)
	return session, s, brainDir
}

func startEng(t *testing.T, hooks map[string][]string) (*mcpsdk.ClientSession, *store.Store, string, *engine.Engine) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	s, err := store.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	brainDir := filepath.Join(dir, "brain")
	if err := os.MkdirAll(filepath.Join(brainDir, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{WelcomeShown: true, SetupDone: true, BrainRaw: brainDir}
	cfg.Launch = config.DefaultLaunch(filepath.Join(dir, "data"))
	cfg.Launch.Brain = brainDir
	cfg.Launch.Notify = false
	if hooks != nil {
		cfg.Launch.Hooks = hooks
	}
	eng := engine.New(s, cfg, filepath.Join(dir, "config.toml"))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := NewWithContext(ctx, eng).Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, s, brainDir, eng
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

func call(t *testing.T, session *mcpsdk.ClientSession, name string, args any) json.RawMessage {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("%s error: %v", name, res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatalf("%s: no content", name)
	}
	tc, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("%s content %T", name, res.Content[0])
	}
	return json.RawMessage(tc.Text)
}

func callErr(t *testing.T, session *mcpsdk.ClientSession, name string, args any) string {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return err.Error()
	}
	if !res.IsError {
		t.Fatalf("%s: want tool error", name)
	}
	if len(res.Content) == 0 {
		return "error"
	}
	tc, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		return "error"
	}
	return tc.Text
}

func TestInitializeAndToolsList(t *testing.T) {
	session, _, _ := start(t)
	got, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"projects_list", "project_show",
		"tasks_list", "task_show", "task_create", "task_update", "task_move",
		"plans_list", "plan_show", "plan_create", "plan_add_step", "plan_update", "plan_start", "plan_pause",
		"run_show", "run_diff", "run_log", "run_wait",
		"brain_search", "brain_read",
		"profiles_list",
	}
	have := map[string]bool{}
	for _, tool := range got.Tools {
		have[tool.Name] = true
		if strings.TrimSpace(tool.Description) == "" {
			t.Fatalf("%s: empty description", tool.Name)
		}
	}
	for _, name := range want {
		if !have[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
}

func TestReadTools(t *testing.T) {
	session, s, brainDir := start(t)
	p, err := s.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(store.Task{ProjectID: p.ID, Title: "Search", Status: store.StatusReady})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Ship"})
	if err != nil {
		t.Fatal(err)
	}
	step, err := s.AddPlanStep(plan.ID, store.Task{Title: "Cart"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(store.Profile{Name: "Ana"}); err != nil {
		t.Fatal(err)
	}

	wt := t.TempDir()
	logPath := filepath.Join(wt, "agent.log")
	if err := os.WriteFile(logPath, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run, err := s.CreateRun(store.Run{
		TaskID: task.ID, ProjectID: p.ID, State: store.RunReview, Worktree: wt, Branch: "feature/search",
		Base: "HEAD", LogPath: logPath, Summary: "done", Checks: `{"ok":true}`, Agent: store.AgentClaude,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRound(store.Round{RunID: run.ID, Kind: store.RoundLaunch, State: store.RoundReview, Summary: "done"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRun(store.Run{TaskID: step.ID, ProjectID: p.ID, State: store.RunReview, Agent: store.AgentClaude}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brainDir, "inbox.md"), []byte("# Inbox\nfind me here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("projects_list", func(t *testing.T) {
		var items []store.Project
		if err := json.Unmarshal(call(t, session, "projects_list", map[string]any{}), &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].Name != "App" {
			t.Fatalf("%+v", items)
		}
	})
	t.Run("project_show", func(t *testing.T) {
		var got store.Project
		if err := json.Unmarshal(call(t, session, "project_show", map[string]any{"id": p.ID}), &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != p.ID || got.Name != "App" {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("tasks_list", func(t *testing.T) {
		var items []store.Task
		if err := json.Unmarshal(call(t, session, "tasks_list", map[string]any{"project": p.ID, "status": store.StatusReady}), &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].Title != "Search" {
			t.Fatalf("%+v", items)
		}
		if err := json.Unmarshal(call(t, session, "tasks_list", map[string]any{"plan": plan.ID}), &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].Title != "Cart" {
			t.Fatalf("plan filter %+v", items)
		}
	})
	t.Run("task_show", func(t *testing.T) {
		var got store.Task
		if err := json.Unmarshal(call(t, session, "task_show", map[string]any{"id": task.ID}), &got); err != nil {
			t.Fatal(err)
		}
		if got.Title != "Search" {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("plans_list", func(t *testing.T) {
		var items []planOut
		if err := json.Unmarshal(call(t, session, "plans_list", map[string]any{}), &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].Title != "Ship" || items[0].Total != 1 {
			t.Fatalf("%+v", items)
		}
	})
	t.Run("plan_show", func(t *testing.T) {
		var got planOut
		if err := json.Unmarshal(call(t, session, "plan_show", map[string]any{"id": plan.ID}), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Steps) != 1 || got.Steps[0].Status == "" || got.Steps[0].LatestRun == nil {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("run_show", func(t *testing.T) {
		var got map[string]any
		if err := json.Unmarshal(call(t, session, "run_show", map[string]any{"id": run.ID}), &got); err != nil {
			t.Fatal(err)
		}
		if got["summary"] != "done" || got["state"] != store.RunReview || got["branch"] != "feature/search" {
			t.Fatalf("%v", got)
		}
		if _, ok := got["checks"]; !ok {
			t.Fatal("missing checks")
		}
		rounds, ok := got["rounds"].([]any)
		if !ok || len(rounds) != 1 {
			t.Fatalf("rounds %v", got["rounds"])
		}
	})
	t.Run("run_log", func(t *testing.T) {
		var got runLogOut
		if err := json.Unmarshal(call(t, session, "run_log", map[string]any{"id": run.ID, "lines": 2}), &got); err != nil {
			t.Fatal(err)
		}
		if got.Text != "two\nthree" || !got.Truncated {
			t.Fatalf("%+v", got)
		}
	})
	t.Run("profiles_list", func(t *testing.T) {
		var items []store.Profile
		if err := json.Unmarshal(call(t, session, "profiles_list", map[string]any{}), &items); err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].Name != "Ana" {
			t.Fatalf("%+v", items)
		}
	})
	t.Run("brain_search", func(t *testing.T) {
		var hits []brain.Hit
		if err := json.Unmarshal(call(t, session, "brain_search", map[string]any{"query": "find me"}), &hits); err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || hits[0].Rel != "inbox.md" {
			t.Fatalf("%+v", hits)
		}
	})
	t.Run("brain_read", func(t *testing.T) {
		var got noteOut
		if err := json.Unmarshal(call(t, session, "brain_read", map[string]any{"path": "inbox.md"}), &got); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got.Content, "find me here") {
			t.Fatalf("%+v", got)
		}
	})
}

func TestRunDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	session, s, _ := start(t)
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	p, err := s.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(store.Task{ProjectID: p.ID, Title: "Diff"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "extra.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run, err := s.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, State: store.RunReview, Worktree: repo, Base: "HEAD", Agent: store.AgentClaude})
	if err != nil {
		t.Fatal(err)
	}
	var got runDiffOut
	if err := json.Unmarshal(call(t, session, "run_diff", map[string]any{"id": run.ID}), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Files) == 0 {
		t.Fatal("expected files")
	}
	found := false
	for _, f := range got.Files {
		if f.Path == "extra.txt" && strings.Contains(f.Patch, "hello") {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", got)
	}
}

func TestBrainReadRejectsOutside(t *testing.T) {
	session, _, brainDir := start(t)
	secret := filepath.Join(filepath.Dir(brainDir), "secret.md")
	if err := os.WriteFile(secret, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg := callErr(t, session, "brain_read", map[string]any{"path": "../secret.md"})
	if !strings.Contains(strings.ToLower(msg), "outside") {
		t.Fatalf("got %q", msg)
	}
	msg = callErr(t, session, "brain_read", map[string]any{"path": "/etc/passwd"})
	if !strings.Contains(strings.ToLower(msg), "outside") {
		t.Fatalf("got %q", msg)
	}
}

func TestWriteTools(t *testing.T) {
	session, s, _, _ := startEng(t, nil)
	p, err := s.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	var task store.Task
	if err := json.Unmarshal(call(t, session, "task_create", map[string]any{
		"project": p.ID, "title": "Search", "description": "find", "criteria": "works",
	}), &task); err != nil {
		t.Fatal(err)
	}
	if task.Title != "Search" || task.ProjectID != p.ID {
		t.Fatalf("%+v", task)
	}
	if err := json.Unmarshal(call(t, session, "task_update", map[string]any{"id": task.ID, "title": "Search v2"}), &task); err != nil {
		t.Fatal(err)
	}
	if task.Title != "Search v2" {
		t.Fatalf("%+v", task)
	}
	if err := json.Unmarshal(call(t, session, "task_move", map[string]any{"id": task.ID, "status": store.StatusReady}), &task); err != nil {
		t.Fatal(err)
	}
	if task.Status != store.StatusReady {
		t.Fatalf("%+v", task)
	}
	var plan planOut
	if err := json.Unmarshal(call(t, session, "plan_create", map[string]any{
		"project": p.ID, "title": "Ship", "goal": "done", "agent": store.AgentClaude, "auto": true,
		"steps": []map[string]any{
			{"title": "One", "description": "first", "criteria": "ok"},
			{"title": "Two"},
			{"title": "Three"},
		},
	}), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Title != "Ship" || !plan.Auto || len(plan.Steps) != 3 || plan.Steps[0].Description != "first" {
		t.Fatalf("%+v", plan)
	}
	var step store.Task
	if err := json.Unmarshal(call(t, session, "plan_add_step", map[string]any{"id": plan.ID, "title": "Four"}), &step); err != nil {
		t.Fatal(err)
	}
	if step.PlanID != plan.ID || step.Title != "Four" {
		t.Fatalf("%+v", step)
	}
	if err := json.Unmarshal(call(t, session, "plan_update", map[string]any{"id": plan.ID, "title": "Ship it", "auto": false}), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Title != "Ship it" || plan.Auto {
		t.Fatalf("%+v", plan)
	}
}

func TestPlanStartWaitAndHooks(t *testing.T) {
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
	hookLog := filepath.Join(t.TempDir(), "hooks.log")
	appendHook := `echo "$TROOPA_EVENT" >> ` + hookLog
	fakeClaude(t, "#!/bin/sh\necho done > SUMMARY.md\n")
	session, s, _, _ := startEng(t, map[string][]string{
		launch.HookTaskCreated: {appendHook},
		launch.HookLaunch:      {appendHook},
		launch.HookReview:      {appendHook},
	})
	p, err := s.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	var created planOut
	if err := json.Unmarshal(call(t, session, "plan_create", map[string]any{
		"project": p.ID, "title": "Menu", "agent": store.AgentClaude, "auto": false,
		"steps": []map[string]any{{"title": "Starters"}, {"title": "Mains"}, {"title": "Dessert"}},
	}), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Steps) != 3 {
		t.Fatalf("steps %+v", created.Steps)
	}
	var started map[string]any
	if err := json.Unmarshal(call(t, session, "plan_start", map[string]any{"id": created.ID}), &started); err != nil {
		t.Fatal(err)
	}
	run, _ := started["run"].(map[string]any)
	if run == nil || run["state"] != store.RunRunning {
		t.Fatalf("start %+v", started)
	}
	msg, _ := started["message"].(string)
	if !strings.Contains(msg, "troopa daemon") {
		t.Fatalf("message %q", msg)
	}
	var waited runWaitOut
	if err := json.Unmarshal(call(t, session, "run_wait", map[string]any{"plan_id": created.ID, "timeout_seconds": 15}), &waited); err != nil {
		t.Fatal(err)
	}
	if waited.TimedOut || waited.State != store.RunReview {
		t.Fatalf("%+v", waited)
	}
	if waited.Summary == "" || waited.Checks == nil {
		t.Fatalf("summary/checks %+v", waited)
	}
	var paused map[string]any
	if err := json.Unmarshal(call(t, session, "plan_pause", map[string]any{"id": created.ID}), &paused); err != nil {
		t.Fatal(err)
	}
	if paused["state"] != store.PlanPaused {
		t.Fatalf("%+v", paused)
	}
	deadline := time.Now().Add(2 * time.Second)
	var seen string
	for time.Now().Before(deadline) {
		launch.WaitHooks()
		raw, _ := os.ReadFile(hookLog)
		seen = string(raw)
		if strings.Contains(seen, launch.HookTaskCreated) && strings.Contains(seen, launch.HookLaunch) && strings.Contains(seen, launch.HookReview) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("hooks %q", seen)
}

func TestRunWaitTimeout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	fakeClaude(t, "#!/bin/sh\nsleep 30\necho done > SUMMARY.md\n")
	session, s, _, _ := startEng(t, nil)
	p, err := s.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	var created planOut
	if err := json.Unmarshal(call(t, session, "plan_create", map[string]any{
		"project": p.ID, "title": "Wait", "agent": store.AgentClaude,
		"steps": []map[string]any{{"title": "Slow"}},
	}), &created); err != nil {
		t.Fatal(err)
	}
	var started map[string]any
	if err := json.Unmarshal(call(t, session, "plan_start", map[string]any{"id": created.ID}), &started); err != nil {
		t.Fatal(err)
	}
	run, _ := started["run"].(map[string]any)
	if run == nil {
		t.Fatalf("no run %+v", started)
	}
	worktree, _ := run["worktree"].(string)
	t.Cleanup(func() { launch.StopAgent(worktree) })
	runID, _ := run["id"].(float64)
	var waited runWaitOut
	if err := json.Unmarshal(call(t, session, "run_wait", map[string]any{"run_id": int64(runID), "timeout_seconds": 1}), &waited); err != nil {
		t.Fatal(err)
	}
	if !waited.TimedOut || waited.State != store.RunRunning {
		t.Fatalf("%+v", waited)
	}
}

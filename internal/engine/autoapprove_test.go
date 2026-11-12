package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// runAutoApproveStep runs the only step of an auto-approve plan in a repo
// whose troopa.toml holds the given content ("" means no troopa.toml).
func runAutoApproveStep(t *testing.T, troopaTOML string) (*store.Store, store.Run) {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if troopaTOML != "" {
		if err := os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte(troopaTOML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fakeClaude(t, "#!/bin/sh\necho done > SUMMARY.md\n")
	e, s := testEngine(t)
	p, err := s.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Menu", Agent: store.AgentClaude, Auto: true, AutoApprove: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddPlanStep(plan.ID, store.Task{Title: "Starters", Commits: store.CommitsNone}); err != nil {
		t.Fatal(err)
	}
	res, err := e.StartPlan(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Watch {
		t.Fatalf("expected launch: %+v", res)
	}
	waitAgent(t, res.Run.Worktree)
	if _, err := e.Finish(res.Run.ID, nil); err != nil {
		t.Fatal(err)
	}
	run, err := s.GetRun(res.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, run
}

func TestAutoApproveWithoutChecksWaitsForReview(t *testing.T) {
	s, run := runAutoApproveStep(t, "")
	if run.State != store.RunReview {
		t.Fatalf("state=%s, want review", run.State)
	}
	if run.Note != launch.NoChecksNote() {
		t.Fatalf("note=%q", run.Note)
	}
	task, _ := s.GetTask(run.TaskID)
	if task.Status != store.StatusReview {
		t.Fatalf("task %s, want review", task.Status)
	}
}

func TestAutoApproveWithBlankChecksWaitsForReview(t *testing.T) {
	_, run := runAutoApproveStep(t, "checks = [\"  \"]\n")
	if run.State != store.RunReview {
		t.Fatalf("state=%s, want review", run.State)
	}
}

func TestAutoApproveWithPassingChecksApproves(t *testing.T) {
	s, run := runAutoApproveStep(t, "checks = [\"true\"]\n")
	if run.State != store.RunApproved {
		t.Fatalf("state=%s note=%q, want approved", run.State, run.Note)
	}
	task, _ := s.GetTask(run.TaskID)
	if task.Status != store.StatusDone {
		t.Fatalf("task %s, want done", task.Status)
	}
}

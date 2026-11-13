package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteTaskRemovesEverything(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	task, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Keep"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.CreateRun(Run{TaskID: task.ID, ProjectID: p.ID, State: RunReview})
	if err != nil {
		t.Fatal(err)
	}
	round, err := s.CreateRound(Round{RunID: run.ID, Kind: RoundLaunch})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateComment(ReviewComment{RunID: run.ID, Round: 1, Path: "a.go", Line: 1, Side: "+", Body: "x"})
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(src, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "attachments", "1")
	a, err := s.AttachFile(task.ID, src, dir, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTask(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTask(task.ID); err == nil {
		t.Fatal("task should be gone")
	}
	if _, err := s.GetRun(run.ID); err == nil {
		t.Fatal("run should be gone")
	}
	if _, err := s.GetRound(round.ID); err == nil {
		t.Fatal("round should be gone")
	}
	if _, err := s.GetComment(c.ID); err == nil {
		t.Fatal("comment should be gone")
	}
	if _, err := s.GetAttachment(a.ID); err == nil {
		t.Fatal("attachment row should be gone")
	}
	if _, err := os.Stat(a.Path); !os.IsNotExist(err) {
		t.Fatal("attachment file should be gone")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("attachment folder should be gone")
	}
	if _, err := s.GetTask(other.ID); err != nil {
		t.Fatal("other task must stay")
	}
}

func TestDeleteTaskRefusesRunningRun(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	task, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRun(Run{TaskID: task.ID, ProjectID: p.ID, State: RunRunning}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTask(task.ID); !errors.Is(err, ErrTaskRunning) {
		t.Fatalf("want ErrTaskRunning, got %v", err)
	}
	if _, err := s.GetTask(task.ID); err != nil {
		t.Fatal("task must stay")
	}
}

func TestDeleteTaskRenumbersPlanSteps(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	plan, err := s.CreatePlan(Plan{ProjectID: p.ID, Title: "P"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, title := range []string{"A", "B", "C"} {
		st, err := s.AddPlanStep(plan.ID, Task{Title: title})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, st.ID)
	}
	if err := s.DeleteTask(ids[1]); err != nil {
		t.Fatal(err)
	}
	steps, err := s.PlanSteps(plan.ID)
	if err != nil || len(steps) != 2 || steps[0].PlanStep != 1 || steps[1].PlanStep != 2 || steps[1].ID != ids[2] {
		t.Fatalf("%+v %v", steps, err)
	}
}

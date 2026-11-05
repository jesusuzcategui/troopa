package store

import (
	"path/filepath"
	"testing"
)

func testProject(t *testing.T, s *Store) Project {
	t.Helper()
	p, err := s.Create(Project{Name: "App", RepoPath: "/tmp/app"})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	return p
}

func TestCreateTaskTitleOnlyDefaults(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)

	task, err := s.CreateTask(Task{ProjectID: p.ID, Title: "  Primera  "})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.ID == 0 || task.Title != "Primera" {
		t.Fatalf("task = %+v", task)
	}
	if task.Status != StatusInbox || task.Priority != PriorityMedium || task.Agent != AgentMe {
		t.Fatalf("defaults status=%s pri=%s agent=%s", task.Status, task.Priority, task.Agent)
	}
	if task.Mode != ModePair || task.Commits != CommitsNone || task.Type != TypeFeature {
		t.Fatalf("defaults mode=%s commits=%s type=%s", task.Mode, task.Commits, task.Type)
	}
}

func TestCreateTaskRequiresTitleAndProject(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	if _, err := s.CreateTask(Task{ProjectID: p.ID}); err == nil {
		t.Fatal("expected title error")
	}
	if _, err := s.CreateTask(Task{Title: "X"}); err == nil {
		t.Fatal("expected project error")
	}
}

func TestUpdateAndMoveTask(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	task, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Fix"})
	if err != nil {
		t.Fatal(err)
	}
	task.Title = "Fix login"
	task.Description = "# Qué\n\nFalla el login."
	task.AcceptanceCriteria = "Dado usuario\nCuando entra\nEntonces dashboard"
	task.Priority = PriorityHigh
	task.Agent = AgentClaude
	task.Model = "sonnet"
	task.Mode = ModeYolo
	task.Commits = CommitsAuto
	task.Type = TypeBugfix
	task.Status = StatusReady
	if err := s.UpdateTask(task); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Fix login" || got.Status != StatusReady || got.Priority != PriorityHigh {
		t.Fatalf("got %+v", got)
	}
	if got.Agent != AgentClaude || got.Mode != ModeYolo || got.Type != TypeBugfix {
		t.Fatalf("enums %+v", got)
	}
	if got.AcceptanceCriteria != "Dado usuario\nCuando entra\nEntonces dashboard" {
		t.Fatalf("criteria %q", got.AcceptanceCriteria)
	}
	if err := s.MoveTask(got.ID, StatusInProgress); err != nil {
		t.Fatalf("move: %v", err)
	}
	moved, err := s.GetTask(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Status != StatusInProgress {
		t.Fatalf("status = %s", moved.Status)
	}
}

func TestListTasksFilterSearchAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := testProject(t, s)
	if _, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Auth login", Status: StatusInbox, Priority: PriorityLow}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Board view", Status: StatusReady, Priority: PriorityHigh}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Auth refresh", Description: "tokens", Status: StatusReady, Priority: PriorityHigh}); err != nil {
		t.Fatal(err)
	}

	all, err := s.ListTasks(p.ID, TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("all = %d", len(all))
	}
	ready, err := s.ListTasks(p.ID, TaskFilter{Status: StatusReady})
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 2 {
		t.Fatalf("ready = %d", len(ready))
	}
	high, err := s.ListTasks(p.ID, TaskFilter{Priority: PriorityHigh})
	if err != nil {
		t.Fatal(err)
	}
	if len(high) != 2 {
		t.Fatalf("high = %d", len(high))
	}
	auth, err := s.ListTasks(p.ID, TaskFilter{Query: "auth"})
	if err != nil {
		t.Fatal(err)
	}
	if len(auth) != 2 {
		t.Fatalf("auth search = %d", len(auth))
	}
	_ = s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	again, err := s2.ListTasks(p.ID, TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 3 {
		t.Fatalf("persist = %d", len(again))
	}
}

func TestRejectInvalidEnums(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	if _, err := s.CreateTask(Task{ProjectID: p.ID, Title: "X", Status: "nope"}); err == nil {
		t.Fatal("expected status error")
	}
	if err := s.MoveTask(1, "nope"); err == nil {
		t.Fatal("expected move error")
	}
}

func TestTasksScopedToProject(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := testProject(t, s)
	b, err := s.Create(Project{Name: "Other", Slug: "other", RepoPath: "/tmp/other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(Task{ProjectID: a.ID, Title: "A1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(Task{ProjectID: b.ID, Title: "B1"}); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListTasks(a.ID, TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "A1" {
		t.Fatalf("scoped = %+v", list)
	}
}

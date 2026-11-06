package store

import (
	"path/filepath"
	"testing"
)

func TestReviewCommentsCRUDAndResolve(t *testing.T) {
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
	run, err := s.CreateRun(Run{TaskID: task.ID, ProjectID: p.ID, State: RunReview})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateComment(ReviewComment{
		RunID: run.ID, Round: 1, Path: "main.go", Line: 12, Side: "+", Quote: "return nil", Body: "handle the error",
	})
	if err != nil || c.ID == 0 || c.Resolved {
		t.Fatalf("%+v %v", c, err)
	}
	c.Body = "handle the error, then return"
	if err := s.UpdateComment(c); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetComment(c.ID)
	if err != nil || got.Body != c.Body {
		t.Fatalf("%+v %v", got, err)
	}
	pending, err := s.ListPendingComments(run.ID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("%+v %v", pending, err)
	}
	if err := s.ResolvePendingComments(run.ID, 2); err != nil {
		t.Fatal(err)
	}
	pending, err = s.ListPendingComments(run.ID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after resolve: %+v %v", pending, err)
	}
	byRound, err := s.ListCommentsByRound(run.ID, 2)
	if err != nil || len(byRound) != 1 || !byRound[0].Resolved || byRound[0].Round != 2 {
		t.Fatalf("%+v %v", byRound, err)
	}
	if err := s.DeleteComment(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetComment(c.ID); err == nil {
		t.Fatal("deleted comment should be gone")
	}
}

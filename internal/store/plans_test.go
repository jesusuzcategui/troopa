package store

import (
	"path/filepath"
	"testing"
)

func TestPlansAndProfiles(t *testing.T) {
	s := openTest(t)
	p, err := s.Create(Project{Name: "App", RepoPath: t.TempDir(), Account: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(p.ID); got.Account != "work" {
		t.Fatalf("account %q", got.Account)
	}
	prof, err := s.CreateProfile(Profile{Name: "Ana", Profession: "Chef", About: "Runs a bakery"})
	if err != nil {
		t.Fatal(err)
	}
	if prof.Label() != "Ana · Chef" {
		t.Fatal(prof.Label())
	}
	p.ProfileID = prof.ID
	if err := s.Update(p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(Profile{}); err == nil {
		t.Fatal("empty profile should fail")
	}
	plan, err := s.CreatePlan(Plan{ProjectID: p.ID, Title: "Checkout", Agent: AgentCodex, Auto: true})
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.AddPlanStep(plan.ID, Task{Title: "Cart"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.AddPlanStep(plan.ID, Task{Title: "Payment"})
	if a.PlanStep != 1 || b.PlanStep != 2 || a.Agent != AgentCodex || a.Mode != ModeYolo {
		t.Fatalf("steps %+v %+v", a, b)
	}
	if err := s.MovePlanStep(b.ID, -1); err != nil {
		t.Fatal(err)
	}
	next, ok, _ := s.NextPlanStep(plan.ID)
	if !ok || next.ID != b.ID {
		t.Fatalf("next %+v", next)
	}
	_ = s.MoveTask(b.ID, StatusDone)
	done, total, _ := s.PlanProgress(plan.ID)
	if done != 1 || total != 2 {
		t.Fatalf("progress %d/%d", done, total)
	}
	if err := s.DeleteProfile(prof.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(p.ID); got.ProfileID != 0 {
		t.Fatal("project keeps a deleted profile")
	}
	if err := s.DeletePlan(plan.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetTask(a.ID); got.PlanID != 0 {
		t.Fatal("task still in deleted plan")
	}
}

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

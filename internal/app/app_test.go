package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestServiceProjectOperations(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "troopa.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cfg := config.Config{
		Launch: config.Launch{
			Accounts: map[string]config.Account{
				"work":     {},
				"personal": {},
			},
		},
	}
	a := New(s, cfg, "")

	accounts := a.ListAccounts()
	if len(accounts) != 2 || accounts[0] != "personal" || accounts[1] != "work" {
		t.Fatalf("accounts = %v, want [personal work]", accounts)
	}

	repo := filepath.Join(t.TempDir(), "new-repo")
	if a.PathExists(repo) {
		t.Fatalf("expected %s not to exist yet", repo)
	}
	if err := a.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if !a.PathExists(repo) {
		t.Fatalf("expected %s to exist after InitRepo", repo)
	}

	c, err := a.CreateClient(store.Client{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	clients, err := a.ListClients(false)
	if err != nil || len(clients) != 1 || clients[0].ID != c.ID {
		t.Fatalf("clients = %+v, err = %v", clients, err)
	}

	profiles, err := a.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	_ = profiles

	p, err := a.CreateProject(store.Project{Name: "Demo", RepoPath: repo, Stack: "Go", ClientID: c.ID, Account: "work"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := a.ListProjects(false)
	if err != nil || len(list) != 1 || list[0].ID != p.ID {
		t.Fatalf("projects = %+v, err = %v", list, err)
	}

	p.Description = "Updated"
	if err := a.UpdateProject(p); err != nil {
		t.Fatal(err)
	}
	got, err := a.GetProject(p.ID)
	if err != nil || got.Description != "Updated" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}

	if err := a.ArchiveProject(p.ID); err != nil {
		t.Fatal(err)
	}
	active, err := a.ListProjects(false)
	if err != nil || len(active) != 0 {
		t.Fatalf("active projects after archive = %+v, err = %v", active, err)
	}
	all, err := a.ListProjects(true)
	if err != nil || len(all) != 1 || !all[0].Archived {
		t.Fatalf("all projects after archive = %+v, err = %v", all, err)
	}
}

func TestServiceTaskOperations(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "troopa.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	a := New(s, config.Config{}, "")
	p, err := a.CreateProject(store.Project{Name: "Demo", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	stamp0 := a.ChangeStamp()
	task, err := a.CreateTask(store.Task{ProjectID: p.ID, Title: "Shared operation"})
	if err != nil {
		t.Fatal(err)
	}
	if stamp1 := a.ChangeStamp(); stamp1 == "" || stamp1 == stamp0 {
		t.Fatalf("ChangeStamp after create = %q, before = %q", stamp1, stamp0)
	}
	list, err := a.ListTasks(p.ID, "")
	if err != nil || len(list) != 1 || list[0].ID != task.ID {
		t.Fatalf("ListTasks = %+v, err = %v", list, err)
	}
	got, err := a.GetTask(task.ID)
	if err != nil || got.Title != "Shared operation" {
		t.Fatalf("GetTask = %+v, err = %v", got, err)
	}
	if err := a.MoveTask(task.ID, store.StatusReady); err != nil {
		t.Fatal(err)
	}
	got, err = a.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusReady {
		t.Fatalf("status = %q, want %q", got.Status, store.StatusReady)
	}

	src := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	att, err := a.AttachTaskFile(task.ID, src)
	if err != nil {
		t.Fatal(err)
	}
	atts, err := a.ListAttachments(task.ID)
	if err != nil || len(atts) != 1 || atts[0].ID != att.ID {
		t.Fatalf("ListAttachments = %+v, err = %v", atts, err)
	}
	if err := a.DeleteAttachment(att.ID); err != nil {
		t.Fatal(err)
	}
	atts, err = a.ListAttachments(task.ID)
	if err != nil || len(atts) != 0 {
		t.Fatalf("attachments after delete = %+v, err = %v", atts, err)
	}
}

func TestServicePlanOperations(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "troopa.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	a := New(s, config.Config{}, "")
	p, err := a.CreateProject(store.Project{Name: "Demo", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := a.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Desktop plan", Agent: store.AgentClaude})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := a.ListPlans(p.ID)
	if err != nil || len(plans) != 1 || plans[0].ID != plan.ID {
		t.Fatalf("ListPlans = %+v, err = %v", plans, err)
	}
	step, err := a.AddPlanStep(plan.ID, store.Task{Title: "First step"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := a.PlanSteps(plan.ID)
	if err != nil || len(steps) != 1 || steps[0].ID != step.ID {
		t.Fatalf("PlanSteps = %+v, err = %v", steps, err)
	}
}

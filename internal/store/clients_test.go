package store

import (
	"path/filepath"
	"testing"
)

func TestCreateListUpdateArchiveClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	c, err := s.CreateClient(Client{Name: "  Acme  ", Contact: "a@acme.test", Notes: "VIP"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == 0 || c.Name != "Acme" {
		t.Fatalf("client = %+v", c)
	}

	list, err := s.ListClients(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Contact != "a@acme.test" {
		t.Fatalf("list = %+v", list)
	}

	c.Notes = "prioridad alta"
	if err := s.UpdateClient(c); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := s.ArchiveClient(c.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	active, err := s.ListClients(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("active = %+v", active)
	}
	_ = s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	again, err := s2.ListClients(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || again[0].Notes != "prioridad alta" || !again[0].Archived {
		t.Fatalf("persist = %+v", again)
	}
}

func TestCreateClientRequiresName(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateClient(Client{Contact: "x"}); err == nil {
		t.Fatal("expected name error")
	}
}

func TestAssignProjectToClientAndFilter(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	acme, err := s.CreateClient(Client{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	beta, err := s.CreateClient(Client{Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	p1, err := s.Create(Project{Name: "Web", RepoPath: "/web", ClientID: acme.ID})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.Create(Project{Name: "API", RepoPath: "/api"})
	if err != nil {
		t.Fatal(err)
	}
	p2.ClientID = beta.ID
	if err := s.Update(p2); err != nil {
		t.Fatal(err)
	}

	acmeProjects, err := s.ListFiltered(ProjectFilter{ClientID: acme.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(acmeProjects) != 1 || acmeProjects[0].ID != p1.ID {
		t.Fatalf("acme projects = %+v", acmeProjects)
	}

	if _, err := s.CreateTask(Task{ProjectID: p1.ID, Title: "Login"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(Task{ProjectID: p2.ID, Title: "Jobs"}); err != nil {
		t.Fatal(err)
	}

	acmeTasks, err := s.ListTasks(0, TaskFilter{ClientID: acme.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(acmeTasks) != 1 || acmeTasks[0].Title != "Login" {
		t.Fatalf("acme tasks = %+v", acmeTasks)
	}
	scoped, err := s.ListTasks(p1.ID, TaskFilter{ClientID: beta.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 0 {
		t.Fatalf("mismatched client filter = %+v", scoped)
	}
}

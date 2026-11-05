package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestClientsCRUDAndProjectFilter(t *testing.T) {
	m := newTestModel(t)
	acme, err := m.store.CreateClient(store.Client{Name: "Acme", Contact: "a@acme.test"})
	if err != nil {
		t.Fatal(err)
	}
	beta, err := m.store.CreateClient(store.Client{Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.Create(store.Project{Name: "Web", RepoPath: "/web", ClientID: acme.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.Create(store.Project{Name: "API", RepoPath: "/api", ClientID: beta.ID}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	if len(m.projects) != 2 {
		t.Fatalf("projects = %d", len(m.projects))
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := next.(Model)
	if got.screen != screenClients {
		t.Fatalf("screen = %v, want clients", got.screen)
	}
	if len(got.clients) != 2 {
		t.Fatalf("clients = %d", len(got.clients))
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	got = next.(Model)
	if got.screen != screenList {
		t.Fatalf("screen = %v after F", got.screen)
	}
	if got.filterClientID != acme.ID {
		t.Fatalf("filter = %d, want acme", got.filterClientID)
	}
	if len(got.projects) != 1 || got.projects[0].Name != "Web" {
		t.Fatalf("filtered projects = %+v", got.projects)
	}
	view := got.viewList()
	if !strings.Contains(view, "Acme") {
		t.Fatalf("list missing client label: %q", view)
	}
}

func TestClientBoardFiltersTasks(t *testing.T) {
	m := newTestModel(t)
	acme, err := m.store.CreateClient(store.Client{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	p1, err := m.store.Create(store.Project{Name: "Web", RepoPath: "/web", ClientID: acme.ID})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := m.store.Create(store.Project{Name: "Other", RepoPath: "/o"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateTask(store.Task{ProjectID: p1.ID, Title: "Login"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateTask(store.Task{ProjectID: p2.ID, Title: "Jobs"}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if got.screen != screenBoard {
		t.Fatalf("screen = %v", got.screen)
	}
	if len(got.tasks) != 1 || got.tasks[0].Title != "Login" {
		t.Fatalf("client tasks = %+v", got.tasks)
	}
}

func TestNewClientForm(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got = next.(Model)
	if got.screen != screenClientForm {
		t.Fatalf("screen = %v", got.screen)
	}
	got.clientInputs[cfName].SetValue("Gamma")
	got.clientInputs[cfContact].SetValue("g@x.test")
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if got.screen != screenClients {
		t.Fatalf("screen = %v after save", got.screen)
	}
	list, err := got.store.ListClients(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Gamma" {
		t.Fatalf("saved = %+v", list)
	}
}

func TestAssignClientOnProjectForm(t *testing.T) {
	m := newTestModel(t)
	c, err := m.store.CreateClient(store.Client{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p, err := m.store.Create(store.Project{Name: "Web", RepoPath: dir})
	if err != nil {
		t.Fatal(err)
	}
	m.reload()
	m.openForm(p)
	if m.focus != fieldName {
		t.Fatalf("focus = %v", m.focus)
	}
	for m.focus != fieldClient {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = next.(Model)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.pending.ClientID != c.ID {
		t.Fatalf("client id = %d, want %d", m.pending.ClientID, c.ID)
	}
	m.focus = fieldName
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	got, err := m.store.Get(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientID != c.ID {
		t.Fatalf("assigned = %d", got.ClientID)
	}
}

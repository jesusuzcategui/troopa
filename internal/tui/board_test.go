package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/store"
)

func seedBoard(t *testing.T) Model {
	t.Helper()
	m := newTestModel(t)
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: "/a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Uno"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Dos", Status: store.StatusReady}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	m.width = 120
	m.height = 30
	return m
}

func TestEnterOpensBoard(t *testing.T) {
	m := seedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.screen != screenBoard {
		t.Fatalf("screen = %v, want board", got.screen)
	}
	if len(got.tasks) != 2 {
		t.Fatalf("tasks = %d", len(got.tasks))
	}
}

func TestQuickCreateTask(t *testing.T) {
	m := seedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got = next.(Model)
	if got.screen != screenQuickCreate {
		t.Fatalf("screen = %v", got.screen)
	}
	got.quickInput.SetValue("Tres")
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if got.screen != screenBoard {
		t.Fatalf("screen = %v", got.screen)
	}
	list, err := got.store.ListTasks(got.boardProject.ID, store.TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("list = %d", len(list))
	}
	var created store.Task
	for _, tsk := range list {
		if tsk.Title == "Tres" {
			created = tsk
		}
	}
	if _, err := os.Stat(filepath.Join(got.cfg.Launch.Brain, "projects", got.boardProject.Slug, "tasks", fmt.Sprintf("%d-tres.md", created.ID))); err != nil {
		t.Fatal("brain task file", err)
	}
}

func TestMoveTaskWithKeys(t *testing.T) {
	m := seedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	got = next.(Model)
	t0, ok := got.selectedTask()
	if !ok {
		t.Fatal("expected selected")
	}
	if t0.Status != store.StatusBacklog {
		t.Fatalf("status = %s, want backlog", t0.Status)
	}
}

func TestSearchAndPriorityFilter(t *testing.T) {
	m := seedBoard(t)
	p := m.projects[0]
	if err := m.store.UpdateTask(store.Task{
		ID: m.mustTaskID(t, p.ID, "Dos"), Title: "Dos", ProjectID: p.ID,
		Status: store.StatusReady, Priority: store.PriorityHigh,
	}); err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got = next.(Model)
	got.searchInput.SetValue("Dos")
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if len(got.tasks) != 1 || got.tasks[0].Title != "Dos" {
		t.Fatalf("search tasks = %+v", got.tasks)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got = next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = next.(Model)
	if got.search != "" {
		t.Fatalf("search after esc: %q", got.search)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}) // all -> low
	got = next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}) // low -> medium
	got = next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}) // medium -> high
	got = next.(Model)
	if got.filterPriority != store.PriorityHigh {
		t.Fatalf("filter = %q", got.filterPriority)
	}
	if len(got.tasks) != 1 || got.tasks[0].Title != "Dos" {
		t.Fatalf("priority filter = %+v", got.tasks)
	}
}

func TestMouseDragMovesTask(t *testing.T) {
	m := seedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	got.width = 140
	got.height = 30
	slots, _, _ := got.boardSlots()
	// first card of inbox sits on the first content row of the first column
	press := tea.MouseMsg{X: slots[0].x + 2, Y: boardContentY, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	next, _ = got.Update(press)
	got = next.(Model)
	if !got.dragging {
		t.Fatal("expected drag start")
	}
	// drop on backlog = second visible column
	rel := tea.MouseMsg{X: slots[1].x + 2, Y: boardContentY + 1, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft}
	next, _ = got.Update(rel)
	got = next.(Model)
	t0, ok := got.selectedTask()
	if !ok {
		t.Fatal("expected task")
	}
	if t0.Status != store.StatusBacklog {
		t.Fatalf("status = %s after drag", t0.Status)
	}
}

func TestTaskFormMarkdownPreview(t *testing.T) {
	m := seedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	got = next.(Model)
	if got.screen != screenTaskForm {
		t.Fatalf("screen = %v", got.screen)
	}
	got.taskDesc.SetValue("# Hola")
	view := got.viewTaskForm()
	if !strings.Contains(view, "Hola") {
		t.Fatalf("preview missing heading: %q", view)
	}
	sel, _ := got.selectedTask()
	if err := got.store.UpdateTask(store.Task{
		ID: sel.ID, ProjectID: got.boardProject.ID, Title: sel.Title,
		Description: "# Tablero", Status: sel.Status, Priority: sel.Priority,
		Agent: sel.Agent, Mode: sel.Mode, Commits: sel.Commits, Type: sel.Type,
	}); err != nil {
		t.Fatal(err)
	}
	got.reloadTasks()
	got.screen = screenBoard
	board := got.viewBoard()
	if !strings.Contains(board, "Tablero") {
		t.Fatalf("board missing markdown: %q", board)
	}
}

func (m Model) mustTaskID(t *testing.T, projectID int64, title string) int64 {
	t.Helper()
	list, err := m.store.ListTasks(projectID, store.TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range list {
		if task.Title == title {
			return task.ID
		}
	}
	t.Fatalf("task %s not found", title)
	return 0
}

func TestDeleteTaskKeyAsksAndDeletes(t *testing.T) {
	m := seedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	sel, ok := got.selectedTask()
	if !ok {
		t.Fatal("expected a selected task")
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	got = next.(Model)
	if got.popup == nil || got.popup.kind != propConfirmDeleteTask || got.popup.options[got.popup.cursor] != "no" {
		t.Fatalf("want a confirm popup on No, got %+v", got.popup)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if _, err := got.store.GetTask(sel.ID); err != nil {
		t.Fatal("No must keep the task")
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	got = next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	got = next.(Model)
	if _, err := got.store.GetTask(sel.ID); err == nil {
		t.Fatal("Yes must delete the task")
	}
	if len(got.tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(got.tasks))
	}
}

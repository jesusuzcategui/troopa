package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jesusuzcategui/troopa/internal/store"
)

// A long agent answer must be readable on the summary tab: j/k scroll it
// instead of moving the diff cursor.
func TestRunSummaryTabScrollsLongAnswer(t *testing.T) {
	m := newTestModel(t)
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Go", Agent: store.AgentClaude})
	if err != nil {
		t.Fatal(err)
	}
	run, err := m.store.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, State: store.RunReview})
	if err != nil {
		t.Fatal(err)
	}
	var sum strings.Builder
	for i := 1; i <= 80; i++ {
		fmt.Fprintf(&sum, "line %d of the answer\n\n", i)
	}
	r, err := m.store.CreateRound(store.Round{RunID: run.ID, Kind: store.RoundLaunch, State: store.RoundReview, Summary: sum.String()})
	if err != nil {
		t.Fatal(err)
	}
	_ = r
	m.width, m.height = 100, 30
	next, _ := m.openRunView(task)
	got := next.(Model)
	view := func(m Model) string { return ansi.Strip(m.viewReview()) }
	if got.screen != screenReview || got.runTab != tabSummary {
		t.Fatalf("run view should open on the summary tab, screen %v tab %d", got.screen, got.runTab)
	}
	if !strings.Contains(view(got), "line 1 of the answer") {
		t.Fatalf("summary tab should show the start of the answer:\n%s", view(got))
	}
	if strings.Contains(view(got), "line 80 of the answer") {
		t.Fatal("a long answer should not fit in one screen")
	}
	got = press(t, got, "G")
	if !strings.Contains(view(got), "line 80 of the answer") {
		t.Fatal("G should scroll to the end of the answer")
	}
	got = press(t, got, "2")
	if got.runTab != tabDiff {
		t.Fatalf("2 should open the changes tab, got %d", got.runTab)
	}
	got = press(t, got, "tab")
	if got.runTab != tabLog {
		t.Fatalf("tab should move to the log tab, got %d", got.runTab)
	}
}

// A merged run opens read-only on its answer instead of the task form.
func TestFinishedRunOpensReadOnly(t *testing.T) {
	m := newTestModel(t)
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Go", Agent: store.AgentClaude, Status: store.StatusDone})
	if err != nil {
		t.Fatal(err)
	}
	run, err := m.store.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, State: store.RunMerged})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateRound(store.Round{RunID: run.ID, Kind: store.RoundLaunch, State: store.RoundReview, Summary: "all done"}); err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 100, 30
	next, _ := m.openRunView(task)
	got := next.(Model)
	if got.screen != screenReview || !strings.Contains(ansi.Strip(got.viewReview()), "all done") {
		t.Fatalf("merged run should show its answer, screen %v", got.screen)
	}
	got = press(t, got, "y")
	if got.run.State != store.RunMerged || got.screen != screenReview {
		t.Fatalf("y on a merged run must do nothing, state %s screen %v", got.run.State, got.screen)
	}
}

// ctrl+u must move the drawer up right away even after ctrl+d was pressed
// past the end of the content.
func TestDrawerScrollUpAfterEnd(t *testing.T) {
	m := newTestModel(t)
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	desc := strings.Repeat("a long line of the description\n\n", 40)
	if _, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Go", Description: desc, Status: store.StatusBacklog}); err != nil {
		t.Fatal(err)
	}
	m.boardProject = p
	m.screen = screenBoard
	m.width, m.height = 140, 30
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusBacklog)
	m.cardIndex = 0
	m.clampBoardCursor()
	task, ok := m.selectedTask()
	if !ok {
		t.Fatal("no task selected")
	}
	for i := 0; i < 30; i++ {
		m.scrollDrawer(task.ID, 10)
	}
	end := m.drawerOffset
	m.scrollDrawer(task.ID, -10)
	if m.drawerOffset != max(0, end-10) {
		t.Fatalf("ctrl+u should go up from the end: end %d now %d", end, m.drawerOffset)
	}
}

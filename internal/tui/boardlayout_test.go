package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/store"
)

func openSeededBoard(t *testing.T, w, h int) Model {
	t.Helper()
	m := seedBoard(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	got.width, got.height = w, h
	got.clampBoardCursor()
	return got
}

func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestNarrowBoardScrollsToSelectedColumn(t *testing.T) {
	m := openSeededBoard(t, 80, 24)
	_, _, right := m.boardSlots()
	if right == 0 {
		t.Fatal("80 columns should hide some statuses")
	}
	m = press(t, m, "l", "l", "l", "l", "l", "l", "l")
	slots, left, _ := m.boardSlots()
	if left == 0 {
		t.Fatal("expected hidden columns on the left after moving right")
	}
	last := slots[len(slots)-1]
	if last.status != store.StatusDone {
		t.Fatalf("selected column not visible, last=%s", last.status)
	}
	if !strings.Contains(m.View(), "◀") {
		t.Fatal("missing left scroll mark")
	}
}

func TestFoldColumnPersists(t *testing.T) {
	m := openSeededBoard(t, 100, 26)
	m = press(t, m, "z")
	if !m.isCollapsed(store.StatusInbox) {
		t.Fatal("inbox should be folded")
	}
	again := New(m.store, m.cfg, m.configPath)
	if !again.isCollapsed(store.StatusInbox) {
		t.Fatal("fold not kept in config")
	}
	m = press(t, m, "Z")
	for _, st := range []string{store.StatusInProgress, store.StatusBlocked} {
		if !m.isCollapsed(st) {
			t.Fatalf("%s should be folded as empty", st)
		}
	}
}

func TestListViewMoveAndFold(t *testing.T) {
	m := openSeededBoard(t, 100, 26)
	m = press(t, m, "v")
	if !m.listMode() {
		t.Fatal("expected list view")
	}
	sel, ok := m.selectedTask()
	if !ok || sel.Title != "Uno" {
		t.Fatalf("selection lost on view switch: %+v %v", sel, ok)
	}
	m = press(t, m, "]")
	sel, _ = m.selectedTask()
	if sel.Title != "Uno" || sel.Status != store.StatusBacklog {
		t.Fatalf("move in list: %+v", sel)
	}
	m = press(t, m, "h")
	if !m.isCollapsed(store.StatusBacklog) {
		t.Fatal("h should fold the group")
	}
	if _, ok := m.selectedTask(); ok {
		t.Fatal("cursor should sit on the folded header")
	}
	m = press(t, m, "l")
	if m.isCollapsed(store.StatusBacklog) {
		t.Fatal("l should unfold the group")
	}
}

func TestStatusPopupMovesTask(t *testing.T) {
	m := openSeededBoard(t, 100, 26)
	m = press(t, m, "m")
	if m.popup == nil {
		t.Fatal("expected popup")
	}
	if !strings.Contains(m.View(), "8  done") {
		t.Fatal("popup not drawn over the board")
	}
	m = press(t, m, "2")
	if m.popup != nil {
		t.Fatal("popup should close")
	}
	sel, _ := m.selectedTask()
	if sel.Title != "Uno" || sel.Status != store.StatusBacklog {
		t.Fatalf("popup move: %+v", sel)
	}
}

func TestPriorityPopupAndDrawer(t *testing.T) {
	m := openSeededBoard(t, 140, 30)
	m = press(t, m, "P", "3")
	sel, _ := m.selectedTask()
	if sel.Priority != store.PriorityHigh {
		t.Fatalf("priority popup: %s", sel.Priority)
	}
	m = press(t, m, "tab")
	if !m.drawerFocus {
		t.Fatal("tab should focus the panel")
	}
	m = press(t, m, "j", "j", "right") // agent: me -> claude (wraps)
	sel, _ = m.selectedTask()
	if sel.Agent == store.AgentMe {
		t.Fatalf("agent not changed from panel: %s", sel.Agent)
	}
	if !strings.Contains(m.View(), "Branch") {
		t.Fatal("panel missing")
	}
	m = press(t, m, "esc")
	if m.drawerFocus {
		t.Fatal("esc should leave the panel")
	}
}

func TestNarrowPanelIsFullScreen(t *testing.T) {
	m := openSeededBoard(t, 80, 24)
	m = press(t, m, "tab")
	v := m.View()
	if !strings.Contains(v, "Branch") || strings.Contains(v, "▾ inbox") {
		t.Fatalf("narrow panel should replace the board:\n%s", v)
	}
}

func TestOverlayKeepsBaseAroundBox(t *testing.T) {
	got := overlay("abcdefgh\nijklmnop", "XY", 3, 1)
	if got != "abcdefgh\nijk\x1b[0mXY\x1b[0mnop" {
		t.Fatalf("overlay = %q", got)
	}
}

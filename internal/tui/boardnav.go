package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// Board views.
const (
	viewKanban = "board"
	viewList   = "list"
)

func (m Model) listMode() bool { return m.cfg.BoardView == viewList }

func (m Model) isCollapsed(status string) bool {
	for _, s := range m.cfg.Collapsed {
		if s == status {
			return true
		}
	}
	return false
}

func (m *Model) setCollapsed(status string, folded bool) {
	out := make([]string, 0, len(m.cfg.Collapsed)+1)
	for _, s := range m.cfg.Collapsed {
		if s != status {
			out = append(out, s)
		}
	}
	if folded {
		out = append(out, status)
	}
	m.cfg.Collapsed = out
	m.saveLayout()
}

func (m *Model) saveLayout() {
	if err := config.Save(m.configPath, m.cfg); err != nil {
		m.err = i18n.T("Could not save the config: ") + err.Error()
	}
}

func (m *Model) toggleCollapse(status string) {
	folded := !m.isCollapsed(status)
	m.setCollapsed(status, folded)
	if folded {
		m.status = i18n.T("Collapsed: ") + status
	} else {
		m.status = i18n.T("Expanded: ") + status
	}
	m.clampBoardCursor()
}

// toggleEmptyCollapsed folds every empty status, or unfolds everything when
// all empty statuses are already folded.
func (m *Model) toggleEmptyCollapsed() {
	var empty []string
	allFolded := true
	for _, st := range store.Statuses {
		if len(m.columnTasks(st)) == 0 {
			empty = append(empty, st)
			if !m.isCollapsed(st) {
				allFolded = false
			}
		}
	}
	if allFolded {
		m.cfg.Collapsed = []string{}
		m.status = i18n.T("All statuses expanded")
	} else {
		keep := []string{}
		for _, st := range m.cfg.Collapsed {
			keep = append(keep, st)
		}
		for _, st := range empty {
			if !m.isCollapsed(st) {
				keep = append(keep, st)
			}
		}
		m.cfg.Collapsed = keep
		m.status = i18n.T("Empty statuses collapsed")
	}
	m.saveLayout()
	m.clampBoardCursor()
}

func (m *Model) toggleView() {
	sel, had := m.selectedTask()
	if m.listMode() {
		m.cfg.BoardView = viewKanban
		m.status = i18n.T("Board view")
	} else {
		m.cfg.BoardView = viewList
		m.status = i18n.T("List view")
	}
	m.saveLayout()
	if had {
		m.selectTaskID(sel.ID)
	} else {
		m.syncListToColumn()
	}
	m.clampBoardCursor()
}

func (m Model) columnTasks(status string) []store.Task {
	var out []store.Task
	for _, t := range m.tasks {
		if t.Status == status {
			out = append(out, t)
		}
	}
	return out
}

func indexOfStatus(st string) int {
	for i, s := range store.Statuses {
		if s == st {
			return i
		}
	}
	return 0
}

// listRow is one line of the list view: a status header or a task.
type listRow struct {
	status string
	task   store.Task
	header bool
	count  int
}

func (m Model) listRows() []listRow {
	var rows []listRow
	for _, st := range store.Statuses {
		tasks := m.columnTasks(st)
		rows = append(rows, listRow{status: st, header: true, count: len(tasks)})
		if m.isCollapsed(st) {
			continue
		}
		for _, t := range tasks {
			rows = append(rows, listRow{status: st, task: t})
		}
	}
	return rows
}

func (m Model) selectedTask() (store.Task, bool) {
	if m.listMode() {
		rows := m.listRows()
		if m.listCursor < 0 || m.listCursor >= len(rows) || rows[m.listCursor].header {
			return store.Task{}, false
		}
		return rows[m.listCursor].task, true
	}
	if m.colIndex < 0 || m.colIndex >= len(store.Statuses) {
		return store.Task{}, false
	}
	col := m.columnTasks(store.Statuses[m.colIndex])
	if m.cardIndex < 0 || m.cardIndex >= len(col) {
		return store.Task{}, false
	}
	return col[m.cardIndex], true
}

// selectTaskID points both views at the task, wherever it lives now.
func (m *Model) selectTaskID(id int64) {
	for _, t := range m.tasks {
		if t.ID != id {
			continue
		}
		m.colIndex = indexOfStatus(t.Status)
		for k, c := range m.columnTasks(t.Status) {
			if c.ID == id {
				m.cardIndex = k
			}
		}
		for i, r := range m.listRows() {
			if (!r.header && r.task.ID == id) || (r.header && r.status == t.Status && m.isCollapsed(t.Status)) {
				m.listCursor = i
				break
			}
		}
		m.syncListOffset()
		return
	}
}

// syncListToColumn moves the list cursor to the header of the current column.
func (m *Model) syncListToColumn() {
	for i, r := range m.listRows() {
		if r.header && r.status == store.Statuses[m.colIndex] {
			m.listCursor = i
			break
		}
	}
	m.syncListOffset()
}

func (m *Model) clampBoardCursor() {
	if m.colIndex < 0 {
		m.colIndex = 0
	}
	if m.colIndex >= len(store.Statuses) {
		m.colIndex = len(store.Statuses) - 1
	}
	n := len(m.columnTasks(store.Statuses[m.colIndex]))
	if m.cardIndex >= n {
		m.cardIndex = n - 1
	}
	if m.cardIndex < 0 {
		m.cardIndex = 0
	}
	rows := m.listRows()
	if m.listCursor >= len(rows) {
		m.listCursor = len(rows) - 1
	}
	if m.listCursor < 0 {
		m.listCursor = 0
	}
	if m.listMode() && m.listCursor < len(rows) {
		m.colIndex = indexOfStatus(rows[m.listCursor].status)
	}
	m.syncListOffset()
	m.syncBoardScroll()
}

func (m *Model) moveList(delta int) {
	m.listCursor += delta
	m.clampBoardCursor()
}

func (m Model) boardHints() string {
	if m.listMode() {
		return i18n.T("enter open · n new · e edit · m status · P priority · A agent · [ ] move · tab panel · space/z fold · v board view · w plans · / search · B brain · o editor · ? help")
	}
	return i18n.T("enter open · n new · e edit · m status · P priority · A agent · [ ] move · tab panel · z fold · Z fold empty · v list view · w plans · / search · B brain · o editor · ? help")
}

// hintLines is how many lines the wrapped board hints take below the tasks.
func (m Model) hintLines() int {
	return lipgloss.Height(m.theme.Muted.Width(max(10, m.mainWidth()-1)).Render(m.boardHints()))
}

func (m Model) updateBoardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.popup != nil {
		return m.updatePopupKey(msg)
	}
	if m.drawerFocus {
		return m.updateDrawerKey(msg)
	}
	key := msg.String()
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = screenList
		m.status = listStatus()
		m.reload()
		return m, nil
	case "B":
		return m.openBrainProject()
	case "?":
		m.helpFrom = screenBoard
		m.screen = screenHelp
	case "t":
		m.cycleTheme()
	case "C":
		m.cycleClientFilter(1)
		m.reloadTasks()
		m.status = i18n.T("Client filter: ") + m.clientFilterLabel()
	case "M":
		return m.openMetrics()
	case "S":
		m.openSettings(screenBoard)
		return m, nil
	case "w":
		if m.boardProject.ID == 0 {
			m.status = i18n.T("Open a project board to see its plans")
			return m, nil
		}
		m.openPlans()
		return m, nil
	case "x":
		return m.requestChangesFromTask()
	case "i":
		return m.openAbout()
	case "v":
		m.toggleView()
	case "z":
		m.toggleCollapse(store.Statuses[m.colIndex])
	case "Z":
		m.toggleEmptyCollapsed()
	case "tab":
		if t, ok := m.selectedTask(); ok {
			m.drawerFocus = true
			m.drawerField = 0
			m.loadAttachments(t.ID)
			m.status = i18n.T("j/k field  a attach  p path  d remove  e edit  o editor  tab/esc back")
		} else {
			m.status = i18n.T("No task selected")
		}
	case "n":
		if m.boardProject.ID == 0 {
			m.status = i18n.T("Open a project to create tasks")
			return m, nil
		}
		m.quickInput.SetValue("")
		m.quickInput.Focus()
		m.screen = screenQuickCreate
		m.status = i18n.T("Type a title and press enter to add it to inbox")
		return m, textinput.Blink
	case "e":
		if t, ok := m.selectedTask(); ok {
			m.openTaskForm(t)
			return m.withTaskFocus()
		}
	case "enter", " ":
		if m.listMode() {
			rows := m.listRows()
			if m.listCursor < len(rows) && rows[m.listCursor].header {
				m.toggleCollapse(rows[m.listCursor].status)
				return m, nil
			}
		}
		if key == " " {
			return m, nil
		}
		if t, ok := m.selectedTask(); ok {
			m.backToPlan = 0
			return m.openRunView(t)
		}
	case "o":
		return m.openEditor(m.taskFolder())
	case "g":
		return m.requestMergeFromTask()
	case "O":
		return m.requestPRFromTask()
	case "U":
		return m.pullBase()
	case "c":
		return m.requestCleanFromTask()
	case "m":
		return m.openFieldPopup(propStatus)
	case "P":
		return m.openFieldPopup(propPriority)
	case "A":
		return m.openFieldPopup(propAgent)
	case "h", "left":
		if m.listMode() {
			m.setCollapsed(store.Statuses[m.colIndex], true)
			m.syncListToColumn()
			m.clampBoardCursor()
			return m, nil
		}
		m.colIndex--
		m.cardIndex = 0
		m.clampBoardCursor()
	case "l", "right":
		if m.listMode() {
			m.setCollapsed(store.Statuses[m.colIndex], false)
			m.clampBoardCursor()
			return m, nil
		}
		m.colIndex++
		m.cardIndex = 0
		m.clampBoardCursor()
	case "j", "down":
		if m.listMode() {
			m.moveList(1)
			return m, nil
		}
		m.cardIndex++
		m.clampBoardCursor()
	case "k", "up":
		if m.listMode() {
			m.moveList(-1)
			return m, nil
		}
		m.cardIndex--
		m.clampBoardCursor()
	case "pgdown":
		if m.listMode() {
			m.moveList(max(1, m.listVisibleRows()))
		}
	case "pgup":
		if m.listMode() {
			m.moveList(-max(1, m.listVisibleRows()))
		}
	case "ctrl+d", "ctrl+u":
		if t, ok := m.selectedTask(); ok {
			d := 10
			if key == "ctrl+u" {
				d = -10
			}
			m.scrollDrawer(t.ID, d)
		}
	case "[", "H":
		return m.shiftTask(-1)
	case "]", "L":
		return m.shiftTask(1)
	case "/":
		m.searchInput.SetValue(m.search)
		m.searchInput.Focus()
		m.screen = screenSearch
		return m, textinput.Blink
	case "f":
		m.filterStatus = cycleFilter(m.filterStatus, append([]string{""}, store.Statuses...))
		m.reloadTasks()
		m.status = i18n.T("Status filter: ") + filterLabel(m.filterStatus)
	case "p":
		m.filterPriority = cycleFilter(m.filterPriority, append([]string{""}, store.Priorities...))
		m.reloadTasks()
		m.status = i18n.T("Priority filter: ") + filterLabel(m.filterPriority)
	}
	return m, nil
}

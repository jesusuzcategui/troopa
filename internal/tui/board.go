package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func (m *Model) openBoard(p store.Project) {
	m.boardProject = p
	m.screen = screenBoard
	m.colIndex = 0
	m.cardIndex = 0
	m.search = ""
	m.filterStatus = ""
	m.filterPriority = ""
	m.searchInput.SetValue("")
	m.dragging = false
	m.dragID = 0
	m.err = ""
	m.reloadTasks()
	m.status = i18n.T("n new  e edit  [ ] move  m menu  / search  f/p filter  M metrics  esc projects")
}

func (m *Model) reloadTasks() {
	if m.boardProject.ID == 0 && m.filterClientID == 0 {
		return
	}
	list, err := m.store.ListTasks(m.boardProject.ID, store.TaskFilter{
		Query:    m.search,
		Status:   m.filterStatus,
		Priority: m.filterPriority,
		ClientID: m.filterClientID,
	})
	if err != nil {
		m.err = err.Error()
		return
	}
	m.tasks = list
	m.clampBoardCursor()
}

func (m *Model) clampBoardCursor() {
	if m.colIndex < 0 {
		m.colIndex = 0
	}
	if m.colIndex >= len(store.Statuses) {
		m.colIndex = len(store.Statuses) - 1
	}
	n := len(m.columnTasks(store.Statuses[m.colIndex]))
	if n == 0 {
		m.cardIndex = 0
		return
	}
	if m.cardIndex >= n {
		m.cardIndex = n - 1
	}
	if m.cardIndex < 0 {
		m.cardIndex = 0
	}
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

func (m Model) selectedTask() (store.Task, bool) {
	if m.colIndex < 0 || m.colIndex >= len(store.Statuses) {
		return store.Task{}, false
	}
	col := m.columnTasks(store.Statuses[m.colIndex])
	if m.cardIndex < 0 || m.cardIndex >= len(col) {
		return store.Task{}, false
	}
	return col[m.cardIndex], true
}

func (m Model) visibleStatuses() []string {
	all := store.Statuses
	n := len(all)
	if m.width < 84 {
		n = 3
	}
	if m.width < 48 {
		n = 1
	}
	start := m.colIndex - n/2
	if start < 0 {
		start = 0
	}
	if start+n > len(all) {
		start = max(0, len(all)-n)
	}
	return all[start : start+n]
}

func (m Model) boardGeom() (originY, colW, cardH int, statuses []string) {
	originY = 3
	statuses = m.visibleStatuses()
	colW = max(14, m.width/max(1, len(statuses)))
	cardH = 3
	return
}

func (m Model) statusAt(x, y int) (string, bool) {
	originY, colW, _, statuses := m.boardGeom()
	if y < originY || len(statuses) == 0 {
		return "", false
	}
	col := x / colW
	if col < 0 || col >= len(statuses) {
		return "", false
	}
	return statuses[col], true
}

func (m Model) taskAt(x, y int) (store.Task, int, bool) {
	originY, colW, cardH, statuses := m.boardGeom()
	if y <= originY || len(statuses) == 0 {
		return store.Task{}, 0, false
	}
	col := x / colW
	if col < 0 || col >= len(statuses) {
		return store.Task{}, 0, false
	}
	status := statuses[col]
	cards := m.columnTasks(status)
	idx := (y - originY - 1) / cardH
	if idx < 0 || idx >= len(cards) {
		return store.Task{}, 0, false
	}
	return cards[idx], indexOfStatus(status), true
}

func indexOfStatus(st string) int {
	for i, s := range store.Statuses {
		if s == st {
			return i
		}
	}
	return 0
}

func (m Model) updateBoardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = screenList
		m.status = listStatus()
		m.reload()
		return m, nil
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
	case "enter":
		if t, ok := m.selectedTask(); ok {
			return m.openRunView(t)
		}
	case "m":
		if _, ok := m.selectedTask(); ok {
			m.moveMenuIndex = m.colIndex
			m.screen = screenMoveMenu
		}
	case "h", "left":
		m.colIndex--
		m.cardIndex = 0
		m.clampBoardCursor()
	case "l", "right":
		m.colIndex++
		m.cardIndex = 0
		m.clampBoardCursor()
	case "j", "down":
		m.cardIndex++
		m.clampBoardCursor()
	case "k", "up":
		m.cardIndex--
		m.clampBoardCursor()
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

func (m Model) shiftTask(dir int) (tea.Model, tea.Cmd) {
	t, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	i := m.colIndex + dir
	if i < 0 || i >= len(store.Statuses) {
		return m, nil
	}
	st := store.Statuses[i]
	return m.maybeLaunch(t, st)
}

func (m Model) updateQuickKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.screen = screenBoard
		m.status = i18n.T("Cancelled")
		return m, nil
	case "enter":
		title := strings.TrimSpace(m.quickInput.Value())
		if title == "" {
			m.err = i18n.T("Title is required")
			return m, nil
		}
		created, err := m.store.CreateTask(store.Task{ProjectID: m.boardProject.ID, Title: title})
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.screen = screenBoard
		m.err = ""
		m.status = i18n.T("Created: ") + created.Title
		m.colIndex = 0
		m.reloadTasks()
		col := m.columnTasks(store.StatusInbox)
		for k, c := range col {
			if c.ID == created.ID {
				m.cardIndex = k
				break
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.quickInput, cmd = m.quickInput.Update(msg)
	return m, cmd
}

func (m Model) updateSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.search = ""
		m.searchInput.SetValue("")
		m.screen = screenBoard
		m.reloadTasks()
		m.status = i18n.T("Search cleared")
		return m, nil
	case "enter":
		m.search = strings.TrimSpace(m.searchInput.Value())
		m.screen = screenBoard
		m.reloadTasks()
		m.status = i18n.T("Search: ") + filterLabel(m.search)
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.search = strings.TrimSpace(m.searchInput.Value())
	m.reloadTasks()
	return m, cmd
}

func (m Model) updateMoveMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.screen = screenBoard
		return m, nil
	case "j", "down":
		m.moveMenuIndex = (m.moveMenuIndex + 1) % len(store.Statuses)
	case "k", "up":
		m.moveMenuIndex = (m.moveMenuIndex - 1 + len(store.Statuses)) % len(store.Statuses)
	case "enter":
		return m.applyMoveMenu(store.Statuses[m.moveMenuIndex])
	case "1", "2", "3", "4", "5", "6", "7", "8":
		idx := int(msg.String()[0] - '1')
		if idx >= 0 && idx < len(store.Statuses) {
			return m.applyMoveMenu(store.Statuses[idx])
		}
	}
	return m, nil
}

func (m Model) applyMoveMenu(status string) (tea.Model, tea.Cmd) {
	t, ok := m.selectedTask()
	if !ok {
		m.screen = screenBoard
		return m, nil
	}
	m.screen = screenBoard
	return m.maybeLaunch(t, status)
}

func (m Model) updateBoardMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenMoveMenu:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			idx := msg.Y - 3
			if idx >= 0 && idx < len(store.Statuses) {
				return m.applyMoveMenu(store.Statuses[idx])
			}
		}
		return m, nil
	case screenQuickCreate, screenSearch:
		return m, nil
	}

	if msg.Button == tea.MouseButtonWheelUp {
		m.cardIndex--
		m.clampBoardCursor()
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		m.cardIndex++
		m.clampBoardCursor()
		return m, nil
	}

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		if t, col, ok := m.taskAt(msg.X, msg.Y); ok {
			m.colIndex = col
			cards := m.columnTasks(t.Status)
			for i, c := range cards {
				if c.ID == t.ID {
					m.cardIndex = i
					break
				}
			}
			m.dragging = true
			m.dragID = t.ID
			m.dragOver = t.Status
			return m, nil
		}
		if st, ok := m.statusAt(msg.X, msg.Y); ok {
			m.colIndex = indexOfStatus(st)
			m.cardIndex = 0
			m.clampBoardCursor()
		}
	case tea.MouseActionMotion:
		if m.dragging {
			if st, ok := m.statusAt(msg.X, msg.Y); ok {
				m.dragOver = st
			}
		}
	case tea.MouseActionRelease:
		if m.dragging && msg.Button == tea.MouseButtonLeft {
			if st, ok := m.statusAt(msg.X, msg.Y); ok {
				t, err := m.store.GetTask(m.dragID)
				if err == nil && t.Status != st {
					next, cmd := m.maybeLaunch(t, st)
					m = next.(Model)
					m.dragging = false
					m.dragID = 0
					m.dragOver = ""
					return m, cmd
				}
			}
			m.dragging = false
			m.dragID = 0
			m.dragOver = ""
		}
	}
	return m, nil
}

func (m Model) viewBoard() string {
	th := m.theme
	originY, colW, cardH, statuses := m.boardGeom()
	_ = originY
	title := th.Title.Render("Troopa") + th.Muted.Render("  ·  "+m.boardProject.Name)
	if m.search != "" {
		title += th.Muted.Render("  /" + m.search)
	}
	if m.filterStatus != "" {
		title += th.Muted.Render(i18n.T("  status:") + m.filterStatus)
	}
	if m.filterPriority != "" {
		title += th.Muted.Render(i18n.T("  pri:") + m.filterPriority)
	}
	if m.filterClientID != 0 {
		title += th.Muted.Render(i18n.T("  client:") + m.clientFilterLabel())
	}
	if m.dragging && m.dragOver != "" {
		title += th.Muted.Render(i18n.T("  drop:") + m.dragOver)
	}

	var cols []string
	sel, _ := m.selectedTask()
	for _, st := range statuses {
		headerStyle := th.Muted
		if st == store.Statuses[m.colIndex] {
			headerStyle = th.Title
		}
		head := headerStyle.Width(colW - 1).Render(st)
		var body strings.Builder
		body.WriteString(head)
		body.WriteString("\n")
		cards := m.columnTasks(st)
		if len(cards) == 0 {
			body.WriteString(th.Muted.Width(colW - 1).Render(i18n.T("  · empty")))
			body.WriteString("\n")
		}
		for _, c := range cards {
			line1 := truncate(c.Title, max(4, colW-3))
			line2 := truncate(c.Priority+" · "+c.Agent, max(4, colW-3))
			style := th.Normal
			if sel.ID == c.ID {
				style = th.Selected
			}
			if m.dragging && m.dragID == c.ID {
				style = th.Border
			}
			card := style.Width(colW - 1).Render(line1 + "\n" + line2)
			body.WriteString(card)
			body.WriteString("\n")
			_ = cardH
		}
		cols = append(cols, th.App.Width(colW).Render(body.String()))
	}
	board := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	preview := ""
	if sel.ID != 0 {
		var extra strings.Builder
		extra.WriteString("\n")
		meta := sel.Type + " · " + sel.Mode + " · " + sel.Commits
		if sel.Model != "" {
			meta += " · " + sel.Model
		}
		extra.WriteString(th.Muted.Render(meta))
		extra.WriteString("\n")
		if sel.Description != "" {
			extra.WriteString(m.renderMarkdown(sel.Description))
			extra.WriteString("\n")
		}
		if sel.AcceptanceCriteria != "" {
			extra.WriteString(th.Muted.Render(i18n.T("Criteria")))
			extra.WriteString("\n")
			for _, line := range strings.Split(sel.AcceptanceCriteria, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				extra.WriteString(th.Normal.Render("• " + line))
				extra.WriteString("\n")
			}
		}
		preview = extra.String()
	}
	hints := th.Muted.Render(i18n.T("n new  e edit  [ ] or drag  m menu  / search  f status  p priority  C client  M metrics  esc back"))
	return th.App.Render(title + "\n\n" + board + preview + "\n" + hints)
}

func (m Model) viewQuickCreate() string {
	th := m.theme
	return th.App.Render(th.Title.Render(i18n.T("New task")) + i18n.T("\n\n  Title  ") + m.quickInput.View() + "\n\n" +
		th.Muted.Render(i18n.T("enter create  esc cancel")))
}

func (m Model) viewSearch() string {
	th := m.theme
	return th.App.Render(th.Title.Render(i18n.T("Search")) + "\n\n  / " + m.searchInput.View() + "\n\n" +
		th.Muted.Render(i18n.T("type to filter  enter apply  esc clear")) + "\n\n" + m.viewBoard())
}

func (m Model) viewMoveMenu() string {
	th := m.theme
	t, _ := m.selectedTask()
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Move «") + t.Title + "»"))
	b.WriteString("\n\n")
	for i, st := range store.Statuses {
		line := fmt.Sprintf("  %d  %s", i+1, st)
		style := th.Normal
		if i == m.moveMenuIndex {
			style = th.Selected
		}
		b.WriteString(style.Width(max(0, m.width)).Render(line))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("1–8 or enter  click  esc cancel")))
	return th.App.Render(b.String())
}

func cycleFilter(cur string, opts []string) string {
	return cycleValue(cur, opts, 1)
}

func cycleValue(cur string, opts []string, dir int) string {
	if len(opts) == 0 {
		return cur
	}
	i := 0
	for k, o := range opts {
		if o == cur {
			i = k
			break
		}
	}
	n := len(opts)
	return opts[(i+dir%n+n)%n]
}

func filterLabel(v string) string {
	if v == "" {
		return i18n.T("all")
	}
	return v
}

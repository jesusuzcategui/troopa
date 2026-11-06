package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

const (
	boardHeaderY   = 2 // title, blank line, then column headers
	boardContentY  = 3
	minColumnWidth = 24
	foldedWidth    = 5
	scrollMarkW    = 2
	cardHeight     = 3
	drawerMinTotal = 120
)

// drawerVisible reports whether the task panel sits beside the board.
func (m Model) drawerVisible() bool { return m.width >= drawerMinTotal }

func (m Model) drawerWidth() int {
	return min(64, max(40, m.width*38/100))
}

func (m Model) mainWidth() int {
	if m.drawerVisible() {
		return m.width - m.drawerWidth()
	}
	return m.width
}

// contentRows is how many lines fit under the column headers.
func (m Model) contentRows() int { return max(3, m.height-5) }

type colSlot struct {
	status string
	x, w   int
	folded bool
}

func (m Model) slotWidth(st string) int {
	if m.isCollapsed(st) {
		return foldedWidth
	}
	return minColumnWidth
}

func (m Model) windowFits(s, e int) bool {
	n := len(store.Statuses)
	total := 0
	for i := s; i <= e; i++ {
		total += m.slotWidth(store.Statuses[i])
	}
	if s > 0 {
		total += scrollMarkW
	}
	if e < n-1 {
		total += scrollMarkW
	}
	return total <= m.mainWidth()
}

// boardWindow returns the first and last visible column. It keeps the
// selected column visible and shows as many columns as fit.
func (m Model) boardWindow() (int, int) {
	n := len(store.Statuses)
	s := min(max(0, m.boardScroll), n-1)
	sel := min(max(0, m.colIndex), n-1)
	if sel < s {
		s = sel
	}
	for s < sel && !m.windowFits(s, sel) {
		s++
	}
	e := s
	for e+1 < n && m.windowFits(s, e+1) {
		e++
	}
	if e < sel {
		e = sel
	}
	for s > 0 && m.windowFits(s-1, e) {
		s--
	}
	return s, e
}

func (m *Model) syncBoardScroll() {
	s, _ := m.boardWindow()
	m.boardScroll = s
}

// boardSlots lays out the visible columns, spreading spare width over the
// expanded ones.
func (m Model) boardSlots() (slots []colSlot, hiddenLeft, hiddenRight int) {
	n := len(store.Statuses)
	s, e := m.boardWindow()
	avail := m.mainWidth()
	x := 0
	if s > 0 {
		avail -= scrollMarkW
		x = scrollMarkW
	}
	if e < n-1 {
		avail -= scrollMarkW
	}
	used, open := 0, 0
	for i := s; i <= e; i++ {
		used += m.slotWidth(store.Statuses[i])
		if !m.isCollapsed(store.Statuses[i]) {
			open++
		}
	}
	extra := max(0, avail-used)
	for i := s; i <= e; i++ {
		st := store.Statuses[i]
		w := m.slotWidth(st)
		if !m.isCollapsed(st) && open > 0 {
			add := extra / open
			extra -= add
			open--
			w += add
		}
		slots = append(slots, colSlot{status: st, x: x, w: w, folded: m.isCollapsed(st)})
		x += w
	}
	return slots, s, n - 1 - e
}

func (m Model) slotAt(x int) (colSlot, bool) {
	slots, _, _ := m.boardSlots()
	for _, sl := range slots {
		if x >= sl.x && x < sl.x+sl.w {
			return sl, true
		}
	}
	return colSlot{}, false
}

func (m Model) cardsPerColumn() int { return max(1, (m.contentRows()-1)/cardHeight) }

// cardOffset is the first card shown in a column.
func (m Model) cardOffset(status string) int {
	if status != store.Statuses[m.colIndex] {
		return 0
	}
	return max(0, m.cardIndex-m.cardsPerColumn()+1)
}

func (m Model) taskAtPoint(x, y int) (store.Task, bool) {
	sl, ok := m.slotAt(x)
	if !ok || sl.folded || y < boardContentY {
		return store.Task{}, false
	}
	cards := m.columnTasks(sl.status)
	idx := m.cardOffset(sl.status) + (y-boardContentY)/cardHeight
	if idx < 0 || idx >= len(cards) {
		return store.Task{}, false
	}
	return cards[idx], true
}

func (m Model) listVisibleRows() int { return max(1, m.contentRows()) }

func (m *Model) syncListOffset() {
	h := m.listVisibleRows()
	if m.listCursor < m.listOffset {
		m.listOffset = m.listCursor
	}
	if m.listCursor >= m.listOffset+h {
		m.listOffset = m.listCursor - h + 1
	}
	if m.listOffset < 0 {
		m.listOffset = 0
	}
}

func (m Model) boardTitle() string {
	th := m.theme
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
	return title
}

func (m Model) viewBoard() string {
	th := m.theme
	if m.drawerFocus && !m.drawerVisible() {
		return m.viewDrawer(m.width, max(3, m.height-1))
	}
	mainW := m.mainWidth()
	var body string
	if m.listMode() {
		body = m.viewTaskList(mainW)
	} else {
		body = m.viewKanban(mainW)
	}
	hints := th.Muted.Render(truncate(m.boardHints(), max(0, mainW-1)))
	main := th.App.Width(mainW).Render(truncate(m.boardTitle(), mainW) + "\n\n" + body + "\n" + hints)
	if !m.drawerVisible() {
		return main
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, main, m.viewDrawer(m.drawerWidth(), max(3, m.height-1)))
}

func (m Model) viewKanban(mainW int) string {
	th := m.theme
	rows := m.contentRows()
	slots, hiddenLeft, hiddenRight := m.boardSlots()
	sel, _ := m.selectedTask()
	block := func(w int, lines []string) string {
		for len(lines) < rows+1 {
			lines = append(lines, "")
		}
		return lipgloss.NewStyle().Width(w).Height(rows + 1).MaxHeight(rows + 1).Background(th.App.GetBackground()).
			Render(strings.Join(lines[:rows+1], "\n"))
	}
	var cols []string
	if hiddenLeft > 0 {
		cols = append(cols, block(scrollMarkW, []string{th.Title.Render("◀"), th.Muted.Render(fmt.Sprint(hiddenLeft))}))
	}
	for _, sl := range slots {
		cards := m.columnTasks(sl.status)
		current := sl.status == store.Statuses[m.colIndex]
		head := th.Muted
		if current {
			head = th.Title
		}
		if sl.folded {
			lines := []string{head.Render(" ▸")}
			for _, r := range sl.status {
				lines = append(lines, head.Render(" "+string(r)))
			}
			lines = append(lines, "", th.Muted.Render(fmt.Sprintf(" %d", len(cards))))
			cols = append(cols, block(sl.w, lines))
			continue
		}
		lines := []string{head.Render(truncate(fmt.Sprintf("▾ %s %d", sl.status, len(cards)), sl.w-1))}
		if len(cards) == 0 {
			lines = append(lines, th.Muted.Render(i18n.T("  · empty")))
		}
		off := m.cardOffset(sl.status)
		shown := 0
		for i := off; i < len(cards) && shown < m.cardsPerColumn(); i++ {
			c := cards[i]
			style := th.Normal
			if sel.ID == c.ID && current {
				style = th.Selected
			}
			if m.dragging && m.dragID == c.ID {
				style = th.Border
			}
			card := style.Width(sl.w - 1).Render(truncate(c.Title, max(4, sl.w-3)) + "\n" +
				truncate(c.Priority+" · "+c.Agent, max(4, sl.w-3)))
			lines = append(lines, strings.Split(card, "\n")...)
			lines = append(lines, "")
			shown++
		}
		if more := len(cards) - off - shown; more > 0 {
			lines = append(lines, th.Muted.Render(fmt.Sprintf(i18n.T("  ↓ %d more"), more)))
		}
		cols = append(cols, block(sl.w, lines))
	}
	if hiddenRight > 0 {
		cols = append(cols, block(scrollMarkW, []string{th.Title.Render("▶"), th.Muted.Render(fmt.Sprint(hiddenRight))}))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m Model) viewTaskList(mainW int) string {
	th := m.theme
	rows := m.listRows()
	h := m.listVisibleRows()
	priW, agentW, dateW := 8, 10, 7
	titleW := max(8, mainW-priW-agentW-dateW-14)
	header := th.Muted.Render(truncate(fmt.Sprintf("    %-*s %-*s %-*s %s", titleW, i18n.T("Title"), priW, i18n.T("Priority"), agentW, i18n.T("Agent"), i18n.T("Updated")), mainW-1))
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	end := min(len(rows), m.listOffset+h-1)
	for i := m.listOffset; i < end; i++ {
		r := rows[i]
		var line string
		style := th.Normal
		if r.header {
			mark := "▾"
			if m.isCollapsed(r.status) {
				mark = "▸"
			}
			line = fmt.Sprintf("%s %s  %d", mark, r.status, r.count)
			style = th.Title
			if i != m.listCursor {
				line = " " + line
			}
		} else {
			line = fmt.Sprintf("   %-*s %-*s %-*s %s", titleW, truncate(r.task.Title, titleW), priW, r.task.Priority,
				agentW, truncate(r.task.Agent, agentW), r.task.UpdatedAt.Local().Format("01-02"))
		}
		if i == m.listCursor {
			style = th.Selected
		}
		b.WriteString(style.Width(mainW - 1).Render(truncate(line, mainW-3)))
		b.WriteString("\n")
	}
	if end < len(rows) {
		b.WriteString(th.Muted.Render(fmt.Sprintf(i18n.T("  ↓ %d more"), len(rows)-end)))
	}
	return lipgloss.NewStyle().Height(m.contentRows() + 1).MaxHeight(m.contentRows() + 1).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) updateBoardMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenQuickCreate, screenSearch:
		return m, nil
	}
	if m.popup != nil {
		return m.updatePopupMouse(msg)
	}
	if m.drawerVisible() && msg.X >= m.mainWidth() {
		return m.updateDrawerMouse(msg)
	}
	if m.listMode() {
		return m.updateListMouse(msg)
	}
	switch {
	case msg.Button == tea.MouseButtonWheelLeft || (msg.Shift && msg.Button == tea.MouseButtonWheelUp):
		m.colIndex--
		m.cardIndex = 0
		m.clampBoardCursor()
		return m, nil
	case msg.Button == tea.MouseButtonWheelRight || (msg.Shift && msg.Button == tea.MouseButtonWheelDown):
		m.colIndex++
		m.cardIndex = 0
		m.clampBoardCursor()
		return m, nil
	case msg.Button == tea.MouseButtonWheelUp:
		m.cardIndex--
		m.clampBoardCursor()
		return m, nil
	case msg.Button == tea.MouseButtonWheelDown:
		m.cardIndex++
		m.clampBoardCursor()
		return m, nil
	}
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		if msg.Y == boardHeaderY {
			if sl, ok := m.slotAt(msg.X); ok {
				m.colIndex = indexOfStatus(sl.status)
				m.toggleCollapse(sl.status)
			}
			return m, nil
		}
		if t, ok := m.taskAtPoint(msg.X, msg.Y); ok {
			m.selectTaskID(t.ID)
			m.clampBoardCursor()
			m.dragging = true
			m.dragID = t.ID
			m.dragOver = t.Status
			return m, nil
		}
		if sl, ok := m.slotAt(msg.X); ok {
			m.colIndex = indexOfStatus(sl.status)
			m.cardIndex = 0
			m.clampBoardCursor()
		}
	case tea.MouseActionMotion:
		if m.dragging {
			if sl, ok := m.slotAt(msg.X); ok {
				m.dragOver = sl.status
			}
		}
	case tea.MouseActionRelease:
		if m.dragging && msg.Button == tea.MouseButtonLeft {
			id := m.dragID
			m.dragging = false
			m.dragID = 0
			m.dragOver = ""
			if sl, ok := m.slotAt(msg.X); ok {
				if t, err := m.store.GetTask(id); err == nil && t.Status != sl.status {
					return m.maybeLaunch(t, sl.status)
				}
			}
		}
	}
	return m, nil
}

func (m Model) updateListMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.moveList(-1)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.moveList(1)
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	idx := m.listOffset + msg.Y - boardContentY
	rows := m.listRows()
	if msg.Y < boardContentY || idx < 0 || idx >= len(rows) {
		return m, nil
	}
	m.listCursor = idx
	m.clampBoardCursor()
	if rows[idx].header {
		m.toggleCollapse(rows[idx].status)
	}
	return m, nil
}

package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
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
	m.boardScroll = 0
	m.listCursor = 0
	m.listOffset = 0
	m.drawerFocus = false
	m.popup = nil
	m.status = ""
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
	prev, had := m.selectedTask()
	m.tasks = list
	if had {
		m.selectTaskID(prev.ID)
	}
	m.clampBoardCursor()
}

func (m Model) shiftTask(dir int) (tea.Model, tea.Cmd) {
	t, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	i := indexOfStatus(t.Status) + dir
	if i < 0 || i >= len(store.Statuses) {
		return m, nil
	}
	return m.maybeLaunch(t, store.Statuses[i])
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
		created, err := m.store.CreateTask(store.Task{ProjectID: m.boardProject.ID, Title: title, Agent: m.cfg.DefaultAgent, Model: m.cfg.DefaultModel})
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.rememberTask(created)
		m.fireTaskHook(launch.HookTaskCreated, created)
		m.screen = screenBoard
		m.err = ""
		m.status = i18n.T("Created: ") + created.Title
		m.reloadTasks()
		m.selectTaskID(created.ID)
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

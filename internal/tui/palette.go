package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

type paletteItem struct {
	ID    string
	Title string
	Hint  string
	Keys  string
}

func (m Model) allCommands() []paletteItem {
	return []paletteItem{
		{ID: "palette", Title: i18n.T("Command palette"), Hint: i18n.T("this palette"), Keys: "ctrl+k"},
		{ID: "help", Title: i18n.T("Help"), Hint: i18n.T("shortcuts"), Keys: "?"},
		{ID: "settings", Title: i18n.T("Settings"), Hint: i18n.T("language, agents, editor, merge"), Keys: "S"},
		{ID: "setup", Title: i18n.T("Run the setup again"), Hint: i18n.T("first-run wizard")},
		{ID: "about", Title: i18n.T("About Troopa"), Keys: "i"},
		{ID: "quit", Title: i18n.T("Quit"), Hint: i18n.T("close Troopa"), Keys: "q"},
		{ID: "projects", Title: i18n.T("Go to projects"), Hint: i18n.T("main list"), Keys: "esc"},
		{ID: "new-project", Title: i18n.T("New project"), Keys: "n"},
		{ID: "edit-project", Title: i18n.T("Edit project"), Keys: "e"},
		{ID: "archive-project", Title: i18n.T("Archive project"), Keys: "a"},
		{ID: "open-board", Title: i18n.T("Open board"), Keys: "enter"},
		{ID: "plans", Title: i18n.T("Plans"), Hint: i18n.T("steps that run one after the other"), Keys: "w"},
		{ID: "profiles", Title: i18n.T("Profiles"), Hint: i18n.T("who the agents work for")},
		{ID: "brain", Title: i18n.T("Open second brain"), Hint: i18n.T("notes tree and markdown"), Keys: "B"},
		{ID: "brain-project", Title: i18n.T("Open project.md"), Hint: i18n.T("this project's note")},
		{ID: "toggle-archived", Title: i18n.T("Show or hide archived"), Keys: "tab"},
		{ID: "clients", Title: i18n.T("Clients view"), Keys: "c"},
		{ID: "new-client", Title: i18n.T("New client"), Keys: "n"},
		{ID: "edit-client", Title: i18n.T("Edit client"), Keys: "e"},
		{ID: "archive-client", Title: i18n.T("Archive client"), Keys: "a"},
		{ID: "client-board", Title: i18n.T("Client board"), Keys: "enter"},
		{ID: "filter-client", Title: i18n.T("Filter by client"), Keys: "F"},
		{ID: "metrics", Title: i18n.T("Metrics"), Keys: "M"},
		{ID: "new-task", Title: i18n.T("New task"), Keys: "n"},
		{ID: "edit-task", Title: i18n.T("Edit task"), Keys: "e"},
		{ID: "attach-file", Title: i18n.T("Attach a file"), Keys: "a"},
		{ID: "search-tasks", Title: i18n.T("Search tasks"), Keys: "/"},
		{ID: "status-menu", Title: i18n.T("Change status"), Keys: "m"},
		{ID: "set-priority", Title: i18n.T("Change priority"), Keys: "P"},
		{ID: "set-agent", Title: i18n.T("Change agent"), Keys: "A"},
		{ID: "delete-task", Title: i18n.T("Delete task"), Keys: "D"},
		{ID: "toggle-view", Title: i18n.T("Switch board / list view"), Keys: "v"},
		{ID: "fold-status", Title: i18n.T("Collapse or expand status"), Keys: "z"},
		{ID: "fold-empty", Title: i18n.T("Collapse empty statuses"), Keys: "Z"},
		{ID: "task-panel", Title: i18n.T("Task panel"), Keys: "tab"},
		{ID: "move-left", Title: i18n.T("Move task left"), Keys: "["},
		{ID: "move-right", Title: i18n.T("Move task right"), Keys: "]"},
		{ID: "filter-status", Title: i18n.T("Status filter"), Keys: "f"},
		{ID: "filter-priority", Title: i18n.T("Priority filter"), Keys: "p"},
		{ID: "cycle-theme", Title: i18n.T("Next theme"), Keys: "t"},
		{ID: "pick-theme", Title: i18n.T("Pick theme"), Hint: i18n.T("list of TOML palettes")},
		{ID: "new-theme", Title: i18n.T("Create custom theme"), Hint: i18n.T("copies the current palette to a TOML file")},
		{ID: "approve", Title: i18n.T("Approve review"), Keys: "y"},
		{ID: "merge", Title: i18n.T("Merge into base"), Keys: "g"},
		{ID: "open-pr", Title: i18n.T("Push and open a PR"), Hint: i18n.T("needs a remote"), Keys: "O"},
		{ID: "pull", Title: i18n.T("Pull the base branch"), Hint: i18n.T("fast-forward it from its remote before the next task"), Keys: "U"},
		{ID: "clean", Title: i18n.T("Clean worktree (keep branch)"), Keys: "c"},
		{ID: "discard", Title: i18n.T("Discard worktree"), Keys: "d"},
		{ID: "request-changes", Title: i18n.T("Request changes"), Hint: i18n.T("send the task back to its agent"), Keys: "x"},
		{ID: "comment-line", Title: i18n.T("Comment on this line"), Keys: "enter"},
		{ID: "comment-list", Title: i18n.T("Pending comments"), Keys: "C"},
		{ID: "finish-pair", Title: i18n.T("Finish pair"), Keys: "f"},
		{ID: "reopen-agent", Title: i18n.T("Reopen agent"), Keys: "r"},
		{ID: "open-editor", Title: i18n.T("Open in editor"), Hint: i18n.T("worktree or project folder"), Keys: "o"},
	}
}

func (m Model) filteredCommands() []paletteItem {
	q := strings.ToLower(strings.TrimSpace(m.paletteInput.Value()))
	all := m.allCommands()
	if q == "" {
		return all
	}
	var out []paletteItem
	for _, c := range all {
		blob := strings.ToLower(c.ID + " " + c.Title + " " + c.Hint + " " + c.Keys)
		if strings.Contains(blob, q) {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) openPalette() (tea.Model, tea.Cmd) {
	m.paletteFrom = m.screen
	m.screen = screenPalette
	m.paletteCursor = 0
	m.paletteInput.SetValue("")
	m.paletteInput.Focus()
	m.status = i18n.T("Ctrl+K palette · enter run · esc close")
	return m, textinput.Blink
}

func (m Model) closePalette() Model {
	if m.paletteFrom == screenPalette || m.paletteFrom == screenWelcome {
		m.paletteFrom = screenList
	}
	m.screen = m.paletteFrom
	return m
}

func (m Model) updatePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m = m.closePalette()
		return m, nil
	case "up", "k", "shift+tab":
		if m.paletteCursor > 0 {
			m.paletteCursor--
		}
		return m, nil
	case "down", "j", "tab":
		n := len(m.filteredCommands())
		if n > 0 && m.paletteCursor < n-1 {
			m.paletteCursor++
		}
		return m, nil
	case "enter":
		items := m.filteredCommands()
		if m.paletteCursor < 0 || m.paletteCursor >= len(items) {
			return m, nil
		}
		return m.runCommand(items[m.paletteCursor].ID)
	}
	var cmd tea.Cmd
	m.paletteInput, cmd = m.paletteInput.Update(msg)
	m.paletteCursor = 0
	return m, cmd
}

func (m Model) runCommand(id string) (tea.Model, tea.Cmd) {
	from := m.paletteFrom
	m = m.closePalette()
	if boardOnlyCommand(id) && m.boardProject.ID == 0 && m.filterClientID == 0 {
		m.screen = from
		m.status = i18n.T("Open a project to create tasks")
		return m, nil
	}
	switch id {
	case "palette":
		return m.openPalette()
	case "help":
		m.helpFrom = m.screen
		m.screen = screenHelp
		return m, nil
	case "settings":
		m.openSettings(from)
		return m, nil
	case "plans":
		if m.boardProject.ID == 0 {
			m.screen = from
			m.status = i18n.T("Open a project board to see its plans")
			return m, nil
		}
		m.openPlans()
		return m, nil
	case "profiles":
		m.openProfiles(from)
		return m, nil
	case "request-changes":
		if from == screenReview {
			m.screen = screenReview
			return m.openChanges(screenReview)
		}
		m.screen = screenBoard
		return m.requestChangesFromTask()
	case "comment-line":
		if from != screenReview && m.screen != screenReview {
			m.status = i18n.T("No review open")
			return m, nil
		}
		m.screen = screenReview
		return m.openCommentOnCursor()
	case "comment-list":
		if from != screenReview && m.screen != screenReview {
			m.status = i18n.T("No review open")
			return m, nil
		}
		m.screen = screenReview
		return m.openCommentList()
	case "setup":
		m.startSetup(screenList)
		return m, nil
	case "about":
		m.screen = from
		return m.openAbout()
	case "quit":
		return m, tea.Quit
	case "projects":
		m.screen = screenList
		m.status = listStatus()
		m.reload()
		return m, nil
	case "new-project":
		m.openForm(store.Project{})
		return m, nil
	case "edit-project":
		if p, ok := m.selected(); ok {
			m.openForm(p)
		} else {
			m.status = i18n.T("No project selected")
		}
		return m, nil
	case "archive-project":
		if p, ok := m.selected(); ok {
			if p.Archived {
				m.status = i18n.T("Already archived")
			} else {
				m.pending = p
				m.screen = screenConfirmArchive
			}
		} else {
			m.status = i18n.T("No project selected")
		}
		return m, nil
	case "open-board":
		if p, ok := m.selected(); ok {
			m.openBoard(p)
		} else {
			m.status = i18n.T("No project selected")
		}
		return m, nil
	case "brain":
		rel := "inbox.md"
		if from == screenBoard && m.boardProject.ID != 0 {
			return m.openBrainProject()
		}
		return m.openBrain(from, rel)
	case "brain-project":
		return m.openBrainProject()
	case "toggle-archived":
		m.showArchived = !m.showArchived
		m.cursor = 0
		m.offset = 0
		m.reload()
		return m, nil
	case "clients":
		m.openClients()
		return m, nil
	case "new-client":
		m.openClients()
		m.openClientForm(store.Client{})
		return m, nil
	case "edit-client":
		m.openClients()
		if c, ok := m.selectedClient(); ok {
			m.openClientForm(c)
		} else {
			m.status = i18n.T("No client selected")
		}
		return m, nil
	case "archive-client":
		m.openClients()
		if c, ok := m.selectedClient(); ok {
			if c.Archived {
				m.status = i18n.T("Already archived")
			} else {
				m.pendingClient = c
				m.screen = screenConfirmArchiveClient
			}
		} else {
			m.status = i18n.T("No client selected")
		}
		return m, nil
	case "client-board":
		m.openClients()
		if c, ok := m.selectedClient(); ok {
			m.filterClientID = c.ID
			m.openClientBoard(c)
		} else {
			m.status = i18n.T("No client selected")
		}
		return m, nil
	case "filter-client":
		m.cycleClientFilter(1)
		if from == screenBoard {
			m.screen = screenBoard
			m.reloadTasks()
		}
		m.status = i18n.T("Client filter: ") + m.clientFilterLabel()
		return m, nil
	case "metrics":
		return m.openMetrics()
	case "new-task":
		if m.boardProject.ID == 0 {
			if p, ok := m.selected(); ok {
				m.openBoard(p)
			}
		}
		if m.boardProject.ID == 0 {
			m.status = i18n.T("Open a project to create tasks")
			return m, nil
		}
		m.quickInput.SetValue("")
		m.quickInput.Focus()
		m.screen = screenQuickCreate
		m.status = i18n.T("Type a title and press enter to add it to inbox")
		return m, textinput.Blink
	case "edit-task":
		if t, ok := m.selectedTask(); ok {
			m.openTaskForm(t)
			return m.withTaskFocus()
		}
		m.status = i18n.T("No task selected")
		return m, nil
	case "attach-file":
		if t, ok := m.selectedTask(); ok {
			m.taskDraft = t
			m.loadAttachments(t.ID)
			return m.openFilePicker()
		}
		if m.screen == screenTaskForm {
			return m.openFilePicker()
		}
		m.status = i18n.T("No task selected")
		return m, nil
	case "search-tasks":
		if m.boardProject.ID == 0 {
			if p, ok := m.selected(); ok {
				m.openBoard(p)
			}
		}
		m.searchInput.SetValue(m.search)
		m.searchInput.Focus()
		m.screen = screenSearch
		return m, textinput.Blink
	case "status-menu":
		m.screen = screenBoard
		return m.openFieldPopup(propStatus)
	case "set-priority":
		m.screen = screenBoard
		return m.openFieldPopup(propPriority)
	case "set-agent":
		m.screen = screenBoard
		return m.openFieldPopup(propAgent)
	case "delete-task":
		m.screen = screenBoard
		return m.openDeleteTaskPopup()
	case "toggle-view":
		m.screen = screenBoard
		m.toggleView()
		return m, nil
	case "fold-status":
		m.screen = screenBoard
		m.toggleCollapse(store.Statuses[m.colIndex])
		return m, nil
	case "fold-empty":
		m.screen = screenBoard
		m.toggleEmptyCollapsed()
		return m, nil
	case "task-panel":
		m.screen = screenBoard
		if _, ok := m.selectedTask(); ok {
			m.drawerFocus = true
			m.drawerField = 0
		} else {
			m.status = i18n.T("No task selected")
		}
		return m, nil
	case "move-left":
		return m.shiftTask(-1)
	case "move-right":
		return m.shiftTask(1)
	case "filter-status":
		m.filterStatus = cycleFilter(m.filterStatus, append([]string{""}, store.Statuses...))
		m.reloadTasks()
		m.status = i18n.T("Status filter: ") + filterLabel(m.filterStatus)
		return m, nil
	case "filter-priority":
		m.filterPriority = cycleFilter(m.filterPriority, append([]string{""}, store.Priorities...))
		m.reloadTasks()
		m.status = i18n.T("Priority filter: ") + filterLabel(m.filterPriority)
		return m, nil
	case "cycle-theme":
		m.cycleTheme()
		return m, nil
	case "pick-theme":
		m.themeCursor = 0
		for i, t := range m.themes {
			if t.Name == m.theme.Name {
				m.themeCursor = i
				break
			}
		}
		m.screen = screenTheme
		m.status = i18n.T("j/k choose · enter apply · esc back")
		return m, nil
	case "new-theme":
		m.createUserTheme()
		return m, nil
	case "approve":
		if m.screen != screenReview && from != screenReview {
			m.status = i18n.T("No review open")
			return m, nil
		}
		m.screen = screenReview
		return m.approveRun()
	case "open-pr":
		if from == screenReview || m.screen == screenReview {
			m.screen = screenReview
			return m.requestPR()
		}
		m.screen = screenBoard
		return m.requestPRFromTask()
	case "pull":
		m.screen = from
		return m.pullBase()
	case "merge":
		if from == screenReview || m.screen == screenReview {
			m.screen = screenReview
			return m.requestMerge()
		}
		if from == screenBoard || m.boardProject.ID != 0 {
			m.screen = screenBoard
			return m.requestMergeFromTask()
		}
		m.screen = from
		m.status = i18n.T("This task has no approved run to merge")
		return m, nil
	case "clean":
		if from == screenReview || m.screen == screenReview {
			m.screen = screenReview
			return m.requestClean()
		}
		if from == screenBoard || m.boardProject.ID != 0 {
			m.screen = screenBoard
			return m.requestCleanFromTask()
		}
		m.screen = from
		m.status = i18n.T("This task has no approved run to clean")
		return m, nil
	case "discard":
		if from != screenReview && m.screen != screenReview {
			m.status = i18n.T("No review open")
			return m, nil
		}
		m.screen = screenReview
		return m.discardRun()
	case "finish-pair":
		if from != screenPair && m.screen != screenPair {
			m.status = i18n.T("No pair session")
			return m, nil
		}
		m.screen = screenPair
		return m.finishPair()
	case "open-editor":
		switch {
		case from == screenPair || from == screenReview || from == screenLog:
			m.screen = from
			return m.openEditor(m.runFolder())
		case from == screenBoard:
			m.screen = screenBoard
			return m.openEditor(m.taskFolder())
		default:
			if p, ok := m.selected(); ok {
				return m.openEditor(p.RepoPath)
			}
			m.status = i18n.T("No project selected")
			return m, nil
		}
	case "reopen-agent":
		if from != screenPair && m.screen != screenPair {
			m.status = i18n.T("No pair session")
			return m, nil
		}
		m.screen = screenPair
		return m.startPair(m.runTask)
	}
	return m, nil
}

func (m Model) viewPalette() string {
	th := m.theme
	items := m.filteredCommands()
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Commands")) + th.Muted.Render("  Ctrl+K"))
	b.WriteString("\n")
	b.WriteString(th.Normal.Render("> " + m.paletteInput.View()))
	b.WriteString("\n\n")
	if len(items) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("  No matches")))
		return th.App.Render(b.String())
	}
	cur := m.paletteCursor
	if cur >= len(items) {
		cur = len(items) - 1
	}
	h := max(5, m.height-8)
	start := 0
	if cur >= h {
		start = cur - h + 1
	}
	end := min(len(items), start+h)
	for i := start; i < end; i++ {
		c := items[i]
		line := c.Title
		if c.Keys != "" {
			line += "  " + c.Keys
		}
		st := th.Normal
		if i == cur {
			st = th.Selected
		}
		b.WriteString(st.Width(max(0, m.width)).Render(truncate(line, max(1, m.width-2))))
		b.WriteString("\n")
	}
	return th.App.Render(b.String())
}

func (m Model) updateThemeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.screen = screenList
		m.status = listStatus()
		return m, nil
	case "j", "down":
		if m.themeCursor < len(m.themes)-1 {
			m.themeCursor++
		}
	case "k", "up":
		if m.themeCursor > 0 {
			m.themeCursor--
		}
	case "enter":
		if m.themeCursor >= 0 && m.themeCursor < len(m.themes) {
			m.applyTheme(m.themes[m.themeCursor].Name)
			m.screen = screenList
		}
	}
	return m, nil
}

func (m Model) viewThemePicker() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Themes")))
	b.WriteString(th.Muted.Render(i18n.T("  ·  TOML in ") + m.themeDir))
	b.WriteString("\n\n")
	for i, t := range m.themes {
		line := t.Name
		if t.Name == m.theme.Name {
			line += i18n.T("  (active)")
		}
		st := th.Normal
		if i == m.themeCursor {
			st = th.Selected
		}
		b.WriteString(st.Width(max(0, m.width)).Render(truncate(line, max(1, m.width-2))))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("enter choose  t cycle  Ctrl+K → Create custom theme  esc back")))
	return th.App.Render(b.String())
}

func boardOnlyCommand(id string) bool {
	switch id {
	case "status-menu", "set-priority", "set-agent", "delete-task", "toggle-view", "fold-status", "fold-empty", "task-panel":
		return true
	}
	return false
}

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

type taskField int

const (
	tfTitle taskField = iota
	tfDesc
	tfCriteria
	tfAttach
	tfPriority
	tfAgent
	tfModel
	tfMode
	tfCommits
	tfType
	tfStatus
	tfCount
)

func newTaskInputs() (title, model textinput.Model, desc, crit textarea.Model) {
	title = textinput.New()
	title.Placeholder = i18n.T("Title")
	title.CharLimit = 200
	title.Width = 48
	title.Prompt = ""
	model = textinput.New()
	model.Placeholder = i18n.T("model (optional)")
	model.CharLimit = 80
	model.Width = 32
	model.Prompt = ""
	desc = textarea.New()
	desc.Placeholder = i18n.T("Description in Markdown")
	desc.SetHeight(5)
	desc.SetWidth(56)
	desc.ShowLineNumbers = false
	desc.Prompt = ""
	crit = textarea.New()
	crit.Placeholder = i18n.T("One acceptance criterion per line")
	crit.SetHeight(4)
	crit.SetWidth(56)
	crit.ShowLineNumbers = false
	crit.Prompt = ""
	return
}

func (m *Model) openTaskForm(t store.Task) {
	if t.ID == 0 {
		t.ProjectID = m.boardProject.ID
		t.Status = store.StatusInbox
		t.Priority = store.PriorityMedium
		t.Agent = store.AgentMe
		t.Mode = store.ModePair
		t.Commits = store.CommitsNone
		t.Type = store.TypeFeature
	}
	m.taskDraft = t
	m.taskTitle.SetValue(t.Title)
	m.taskModelIn.SetValue(t.Model)
	m.taskDesc.SetValue(t.Description)
	m.taskCrit.SetValue(t.AcceptanceCriteria)
	m.taskFocus = tfTitle
	m.pendingFiles = nil
	m.mentionHits = nil
	m.repoFiles = launch.ListRepoFiles(m.boardProject.RepoPath)
	m.loadAttachments(t.ID)
	m.err = ""
	m.screen = screenTaskForm
	m.status = i18n.T("ctrl+s save  tab field  a attach  @ files  esc cancel")
}

func (m Model) withTaskFocus() (Model, tea.Cmd) {
	m.taskTitle.Blur()
	m.taskModelIn.Blur()
	m.taskDesc.Blur()
	m.taskCrit.Blur()
	switch m.taskFocus {
	case tfTitle:
		m.taskTitle.Focus()
		return m, textinput.Blink
	case tfModel:
		m.taskModelIn.Focus()
		return m, textinput.Blink
	case tfDesc:
		m.taskDesc.Focus()
		return m, textarea.Blink
	case tfCriteria:
		m.taskCrit.Focus()
		return m, textarea.Blink
	}
	return m, nil
}

func (m Model) updateTaskFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.taskFocus == tfAttach {
		switch msg.String() {
		case "a":
			return m.openFilePicker()
		case "p":
			return m.openAttachPath()
		case "d":
			return m.confirmDetach()
		case "o", "enter":
			return m.openSelectedAttachment()
		case "j", "down":
			m.moveAttachCursor(1)
			return m, nil
		case "k", "up":
			m.moveAttachCursor(-1)
			return m, nil
		}
	}
	if len(m.mentionHits) > 0 && (m.taskFocus == tfDesc || m.taskFocus == tfCriteria) {
		switch msg.String() {
		case "esc":
			m.mentionHits = nil
			return m, nil
		case "down":
			m.mentionCursor = (m.mentionCursor + 1) % len(m.mentionHits)
			return m, nil
		case "up":
			m.mentionCursor = (m.mentionCursor - 1 + len(m.mentionHits)) % len(m.mentionHits)
			return m, nil
		case "tab", "enter":
			return m.applyMention()
		}
	}
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+s":
		return m.saveTaskForm()
	case "esc":
		m.screen = screenBoard
		m.status = i18n.T("Cancelled")
		return m, nil
	case "?":
		m.helpFrom = screenTaskForm
		m.screen = screenHelp
		return m, nil
	case "tab", "shift+tab":
		dir := 1
		if msg.String() == "shift+tab" {
			dir = -1
		}
		m.taskFocus = taskField((int(m.taskFocus) + dir + int(tfCount)) % int(tfCount))
		m.mentionHits = nil
		return m.withTaskFocus()
	case "enter":
		if m.taskFocus != tfDesc && m.taskFocus != tfCriteria && m.taskFocus != tfAttach {
			return m.saveTaskForm()
		}
	case "left":
		if m.cycleEnum(-1) {
			return m, nil
		}
	case "right":
		if m.cycleEnum(1) {
			return m, nil
		}
	}
	mod, cmd := m.updateTaskInputs(msg)
	got := mod.(Model)
	got.refreshMentions()
	return got, cmd
}

func (m *Model) cycleEnum(dir int) bool {
	switch m.taskFocus {
	case tfPriority:
		m.taskDraft.Priority = cycleValue(m.taskDraft.Priority, store.Priorities, dir)
	case tfAgent:
		m.taskDraft.Agent = cycleValue(m.taskDraft.Agent, store.Agents, dir)
	case tfMode:
		m.taskDraft.Mode = cycleValue(m.taskDraft.Mode, store.Modes, dir)
	case tfCommits:
		m.taskDraft.Commits = cycleValue(m.taskDraft.Commits, store.CommitModes, dir)
	case tfType:
		m.taskDraft.Type = cycleValue(m.taskDraft.Type, store.Types, dir)
	case tfStatus:
		m.taskDraft.Status = cycleValue(m.taskDraft.Status, store.Statuses, dir)
	default:
		return false
	}
	return true
}

func (m Model) updateTaskInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.taskFocus {
	case tfTitle:
		m.taskTitle, cmd = m.taskTitle.Update(msg)
	case tfModel:
		m.taskModelIn, cmd = m.taskModelIn.Update(msg)
	case tfDesc:
		m.taskDesc, cmd = m.taskDesc.Update(msg)
	case tfCriteria:
		m.taskCrit, cmd = m.taskCrit.Update(msg)
	}
	return m, cmd
}

func (m Model) saveTaskForm() (tea.Model, tea.Cmd) {
	t := m.taskDraft
	t.Title = strings.TrimSpace(m.taskTitle.Value())
	t.Model = strings.TrimSpace(m.taskModelIn.Value())
	t.Description = strings.TrimSpace(m.taskDesc.Value())
	t.AcceptanceCriteria = strings.TrimSpace(m.taskCrit.Value())
	if m.boardProject.ID != 0 {
		t.ProjectID = m.boardProject.ID
	}
	if t.Title == "" {
		m.err = i18n.T("Title is required")
		return m, nil
	}
	if t.ProjectID == 0 {
		m.err = i18n.T("Open a project to create tasks")
		return m, nil
	}
	prevStatus := ""
	if t.ID != 0 {
		if cur, err := m.store.GetTask(t.ID); err == nil {
			prevStatus = cur.Status
		}
	}
	var err error
	if t.ID == 0 {
		var created store.Task
		created, err = m.store.CreateTask(t)
		if err == nil {
			t = created
			m.rememberTask(t)
			m.fireTaskHook(launch.HookTaskCreated, t)
			m.status = i18n.T("Created: ") + created.Title
		}
	} else {
		err = m.store.UpdateTask(t)
		if err == nil {
			m.rememberTask(t)
			m.status = i18n.T("Saved: ") + t.Title
		}
	}
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.taskDraft = t
	if m, err = m.flushPendingFiles(t.ID); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.colIndex = indexOfStatus(t.Status)
	m.screen = screenBoard
	m.err = ""
	m.reloadTasks()
	col := m.columnTasks(t.Status)
	for i, c := range col {
		if c.ID == t.ID {
			m.cardIndex = i
			break
		}
	}
	if t.Status == store.StatusReady && prevStatus != store.StatusReady && t.Agent != store.AgentMe {
		return m.startAgentFor(t)
	}
	return m, nil
}

func (m Model) updateTaskFormMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	idx := msg.Y - 3
	if idx >= 0 && idx < int(tfCount) {
		m.taskFocus = taskField(idx)
		return m.withTaskFocus()
	}
	return m, nil
}

func (m Model) viewTaskForm() string {
	th := m.theme
	title := i18n.T("New task")
	if m.taskDraft.ID != 0 {
		title = i18n.T("Edit task")
	}
	var b strings.Builder
	b.WriteString(th.Title.Render(title))
	b.WriteString("\n\n")
	rows := []struct {
		label string
		value string
		field taskField
	}{
		{i18n.T("Title"), m.taskTitle.View(), tfTitle},
		{i18n.T("Description"), m.taskDesc.View(), tfDesc},
		{i18n.T("Criteria"), m.taskCrit.View(), tfCriteria},
		{i18n.T("Attachments"), m.renderAttachList(), tfAttach},
		{i18n.T("Priority"), m.taskDraft.Priority + "  ←/→", tfPriority},
		{i18n.T("Agent"), m.agentDisplay(m.taskDraft.Agent) + "  ←/→", tfAgent},
		{i18n.T("Model"), m.taskModelIn.View(), tfModel},
		{i18n.T("Mode"), m.taskDraft.Mode + "  ←/→", tfMode},
		{i18n.T("Commits"), m.taskDraft.Commits + "  ←/→", tfCommits},
		{i18n.T("Type"), m.taskDraft.Type + "  ←/→", tfType},
		{i18n.T("Status"), m.taskDraft.Status + "  ←/→", tfStatus},
	}
	for _, r := range rows {
		marker := "  "
		if r.field == m.taskFocus {
			marker = th.Border.Render("> ")
		}
		b.WriteString(marker + th.Label.Render(r.label) + " " + r.value + "\n")
	}
	if hits := m.viewMentions(); hits != "" {
		b.WriteString("\n")
		b.WriteString(hits)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("Markdown preview")))
	b.WriteString("\n")
	b.WriteString(m.renderMarkdown(m.taskDesc.Value()))
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("ctrl+s save  tab field  a attach  p path  d remove  @ files  esc cancel")))
	return th.App.Render(b.String())
}

func (m Model) renderMarkdown(src string) string {
	return m.renderMarkdownW(src, max(24, min(m.width-4, 72)))
}

func (m Model) renderMarkdownW(src string, wrap int) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return m.theme.Muted.Render(i18n.T("  (empty)"))
	}
	style := "dark"
	if m.theme.IsLight() {
		style = "light"
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(wrap),
	)
	if err != nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return strings.TrimSpace(out)
}

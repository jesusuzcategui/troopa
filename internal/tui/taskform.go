package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/jesusuzcategui/troopa/internal/store"
)

type taskField int

const (
	tfTitle taskField = iota
	tfDesc
	tfCriteria
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
	title.Placeholder = "Título"
	title.CharLimit = 200
	title.Width = 48
	title.Prompt = ""
	model = textinput.New()
	model.Placeholder = "modelo (opcional)"
	model.CharLimit = 80
	model.Width = 32
	model.Prompt = ""
	desc = textarea.New()
	desc.Placeholder = "Descripción en Markdown"
	desc.SetHeight(5)
	desc.SetWidth(56)
	desc.ShowLineNumbers = false
	desc.Prompt = ""
	crit = textarea.New()
	crit.Placeholder = "Un criterio de aceptación por línea"
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
	m.err = ""
	m.screen = screenTaskForm
	m.status = "ctrl+s guardar  tab campo  ←/→ enums  esc cancelar"
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
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+s":
		return m.saveTaskForm()
	case "esc":
		m.screen = screenBoard
		m.status = "Cancelado"
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
		return m.withTaskFocus()
	case "enter":
		if m.taskFocus != tfDesc && m.taskFocus != tfCriteria {
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
	return m.updateTaskInputs(msg)
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
		m.err = "El título es obligatorio"
		return m, nil
	}
	if t.ProjectID == 0 {
		m.err = "Abre un proyecto para crear tareas"
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
			m.status = "Creada: " + created.Title
		}
	} else {
		err = m.store.UpdateTask(t)
		if err == nil {
			m.status = "Guardada: " + t.Title
		}
	}
	if err != nil {
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
	title := "Nueva tarea"
	if m.taskDraft.ID != 0 {
		title = "Editar tarea"
	}
	var b strings.Builder
	b.WriteString(th.Title.Render(title))
	b.WriteString("\n\n")
	rows := []struct {
		label string
		value string
		field taskField
	}{
		{"Título", m.taskTitle.View(), tfTitle},
		{"Descripción", m.taskDesc.View(), tfDesc},
		{"Criterios", m.taskCrit.View(), tfCriteria},
		{"Prioridad", m.taskDraft.Priority + "  ←/→", tfPriority},
		{"Agente", m.taskDraft.Agent + "  ←/→", tfAgent},
		{"Modelo", m.taskModelIn.View(), tfModel},
		{"Modo", m.taskDraft.Mode + "  ←/→", tfMode},
		{"Commits", m.taskDraft.Commits + "  ←/→", tfCommits},
		{"Tipo", m.taskDraft.Type + "  ←/→", tfType},
		{"Estado", m.taskDraft.Status + "  ←/→", tfStatus},
	}
	for _, r := range rows {
		marker := "  "
		if r.field == m.taskFocus {
			marker = th.Border.Render("> ")
		}
		b.WriteString(marker + th.Label.Render(r.label) + " " + r.value + "\n")
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render("Vista previa Markdown"))
	b.WriteString("\n")
	b.WriteString(m.renderMarkdown(m.taskDesc.Value()))
	b.WriteString("\n")
	b.WriteString(th.Muted.Render("ctrl+s guardar  tab campo  esc cancelar"))
	return th.App.Render(b.String())
}

func (m Model) renderMarkdown(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return m.theme.Muted.Render("  (vacía)")
	}
	style := "dark"
	if m.theme.Name == "light" {
		style = "light"
	}
	wrap := max(24, min(m.width-4, 72))
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

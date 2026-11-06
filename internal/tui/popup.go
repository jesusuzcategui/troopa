package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// prop is an editable task property shown in the panel and in popups.
type prop int

const (
	propStatus prop = iota
	propPriority
	propAgent
	propMode
	propCommits
	propType
	propCount
	propConfirmDiscard // not a task field: yes/no before discarding a run
	propConfirmMerge   // not a task field: yes/no before merging a local branch
	propConfirmClean   // not a task field: yes/no before cleaning a worktree
)

func (p prop) label() string {
	switch p {
	case propStatus:
		return i18n.T("Status")
	case propPriority:
		return i18n.T("Priority")
	case propAgent:
		return i18n.T("Agent")
	case propMode:
		return i18n.T("Mode")
	case propCommits:
		return "Commits"
	case propType:
		return i18n.T("Type")
	}
	return ""
}

func (p prop) options() []string {
	switch p {
	case propStatus:
		return store.Statuses
	case propPriority:
		return store.Priorities
	case propAgent:
		return store.Agents
	case propMode:
		return store.Modes
	case propCommits:
		return store.CommitModes
	case propType:
		return store.Types
	case propConfirmDiscard:
		return []string{"yes", "no"}
	case propConfirmMerge:
		return []string{"yes", "no"}
	case propConfirmClean:
		return []string{"yes", "no"}
	}
	return nil
}

func propValue(t store.Task, p prop) string {
	switch p {
	case propStatus:
		return t.Status
	case propPriority:
		return t.Priority
	case propAgent:
		return t.Agent
	case propMode:
		return t.Mode
	case propCommits:
		return t.Commits
	case propType:
		return t.Type
	}
	return ""
}

type popup struct {
	kind    prop
	title   string
	options []string
	labels  []string
	cursor  int
	taskID  int64
}

func (m Model) openFieldPopup(p prop) (tea.Model, tea.Cmd) {
	t, ok := m.selectedTask()
	if !ok {
		m.status = i18n.T("No task selected")
		return m, nil
	}
	opts := p.options()
	cur := 0
	for i, o := range opts {
		if o == propValue(t, p) {
			cur = i
		}
	}
	m.popup = &popup{kind: p, title: p.label() + " · " + truncate(t.Title, 32), options: opts, labels: opts, cursor: cur, taskID: t.ID}
	return m, nil
}

func (m Model) openDiscardPopup() (tea.Model, tea.Cmd) {
	title := i18n.T("Discard the worktree and branch?")
	opts := []string{"yes", "no"}
	labels := []string{i18n.T("Yes, discard"), i18n.T("No")}
	cur := 1
	if m.discardWouldLoseCommits() {
		title = i18n.T("Discard? Approved commits on this local branch have not been merged and will be lost.")
		opts = []string{"merge", "yes", "no"}
		labels = []string{i18n.T("Merge instead"), i18n.T("Yes, discard"), i18n.T("No")}
		cur = 2
	}
	m.popup = &popup{
		kind:    propConfirmDiscard,
		title:   title,
		options: opts,
		labels:  labels,
		cursor:  cur,
	}
	return m, nil
}

func (m Model) openCleanPopup() (tea.Model, tea.Cmd) {
	m.popup = &popup{
		kind:    propConfirmClean,
		title:   i18n.T("Clean the worktree and keep the branch?"),
		options: []string{"yes", "no"},
		labels:  []string{i18n.T("Yes, clean"), i18n.T("No")},
		cursor:  1,
	}
	return m, nil
}

func (m Model) openMergePopup() (tea.Model, tea.Cmd) {
	kind, _ := launch.NormalizeMergeStrategy(m.cfg.Launch.Merge)
	m.popup = &popup{
		kind:    propConfirmMerge,
		title:   i18n.T("Merge %s into %s (%s)?", m.runSess.Branch, m.runSess.Base, kind),
		options: []string{"yes", "no"},
		labels:  []string{i18n.T("Yes, merge"), i18n.T("No")},
		cursor:  1,
	}
	return m, nil
}

func (m Model) updatePopupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.popup
	switch k := msg.String(); k {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.popup = nil
	case "j", "down", "tab":
		p.cursor = (p.cursor + 1) % len(p.options)
	case "k", "up", "shift+tab":
		p.cursor = (p.cursor - 1 + len(p.options)) % len(p.options)
	case "enter", " ":
		return m.applyPopup(p.options[p.cursor])
	default:
		if len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
			if i := int(k[0] - '1'); i < len(p.options) {
				return m.applyPopup(p.options[i])
			}
		}
	}
	return m, nil
}

func (m Model) applyPopup(value string) (tea.Model, tea.Cmd) {
	p := m.popup
	m.popup = nil
	if p.kind == propConfirmDiscard {
		switch value {
		case "yes":
			return m.discardRun()
		case "merge":
			return m.mergeRun()
		}
		return m, nil
	}
	if p.kind == propConfirmMerge {
		if value == "yes" {
			return m.mergeRun()
		}
		return m, nil
	}
	if p.kind == propConfirmClean {
		if value == "yes" {
			return m.cleanRun()
		}
		return m, nil
	}
	t, err := m.store.GetTask(p.taskID)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	return m.setTaskProp(t, p.kind, value)
}

// setTaskProp changes one property. Status goes through maybeLaunch so moving
// to ready still starts the agent.
func (m Model) setTaskProp(t store.Task, p prop, value string) (tea.Model, tea.Cmd) {
	switch p {
	case propStatus:
		if t.Status == value {
			return m, nil
		}
		return m.maybeLaunch(t, value)
	case propPriority:
		t.Priority = value
	case propAgent:
		t.Agent = value
	case propMode:
		t.Mode = value
	case propCommits:
		t.Commits = value
	case propType:
		t.Type = value
	}
	if err := m.store.UpdateTask(t); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.err = ""
	m.reloadTasks()
	m.selectTaskID(t.ID)
	m.status = fmt.Sprintf("%s: %s", p.label(), value)
	return m, nil
}

// popupBox renders the popup and where it goes on screen.
func (m Model) popupBox() (box string, x, y int) {
	th := m.theme
	p := m.popup
	var b strings.Builder
	b.WriteString(th.Title.Render(p.title))
	b.WriteString("\n\n")
	w := lipgloss.Width(p.title)
	for i, l := range p.labels {
		line := fmt.Sprintf(" %d  %s ", i+1, l)
		w = max(w, lipgloss.Width(line))
		style := th.Normal
		if i == p.cursor {
			style = th.Selected
		}
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
	hint := i18n.T("j/k  enter  1–9  esc")
	w = max(w, lipgloss.Width(hint))
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(hint))
	box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Title.GetForeground()).
		BorderBackground(th.App.GetBackground()).
		Background(th.App.GetBackground()).
		Padding(0, 1).
		Width(w + 4).
		Render(b.String())
	x = max(0, (m.width-lipgloss.Width(box))/2)
	y = max(0, (m.height-1-lipgloss.Height(box))/2)
	return box, x, y
}

func (m Model) updatePopupMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	box, x, y := m.popupBox()
	if msg.X < x || msg.X >= x+lipgloss.Width(box) || msg.Y < y || msg.Y >= y+lipgloss.Height(box) {
		m.popup = nil
		return m, nil
	}
	idx := msg.Y - y - 3 // border, title, blank line
	if idx >= 0 && idx < len(m.popup.options) {
		return m.applyPopup(m.popup.options[idx])
	}
	return m, nil
}

// overlay draws top over base starting at column x, row y.
func overlay(base, top string, x, y int) string {
	lines := strings.Split(base, "\n")
	for i, tl := range strings.Split(top, "\n") {
		row := y + i
		if row < 0 || row >= len(lines) {
			continue
		}
		bl := lines[row]
		left := ansi.Truncate(bl, x, "")
		if pad := x - ansi.StringWidth(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}
		right := ansi.TruncateLeft(bl, x+ansi.StringWidth(tl), "")
		lines[row] = left + "\x1b[0m" + tl + "\x1b[0m" + right
	}
	return strings.Join(lines, "\n")
}

package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

const drawerLabelW = 10

// drawerHead renders the lines above the properties; drawerPropTop uses its
// height to map mouse clicks to rows.
func (m Model) drawerHead(t store.Task, inner int) string {
	th := m.theme
	id := th.Muted.Render(fmt.Sprintf("#%d · %s", t.ID, t.Status))
	title := th.Title.Width(inner).Render(t.Title)
	return id + "\n" + title + "\n"
}

// drawerPropTop is the screen row of the first property (border included).
func (m Model) drawerPropTop(t store.Task, inner int) int {
	return 1 + lipgloss.Height(m.drawerHead(t, inner))
}

func (m Model) viewDrawer(w, h int) string {
	th := m.theme
	inner := max(10, w-4)
	t, ok := m.selectedTask()
	var b strings.Builder
	if !ok {
		b.WriteString(th.Muted.Render(i18n.T("No task selected")))
	} else {
		b.WriteString(m.drawerHead(t, inner))
		for i := prop(0); i < propCount; i++ {
			val := propValue(t, i)
			if val == "" {
				val = "—"
			}
			label := th.Muted.Render(fmt.Sprintf("%-*s", drawerLabelW, i.label()))
			if m.drawerFocus && int(i) == m.drawerField {
				b.WriteString(th.Selected.UnsetPadding().Width(inner).Render(fmt.Sprintf("%-*s‹ %s ›", drawerLabelW, i.label(), val)))
			} else {
				b.WriteString(label + th.Normal.Render(val))
			}
			b.WriteString("\n")
		}
		model := t.Model
		if model == "" {
			model = "—"
		}
		b.WriteString(th.Muted.Render(fmt.Sprintf("%-*s", drawerLabelW, i18n.T("Model"))) + th.Normal.Render(model) + "\n")
		run, runErr := m.store.LatestRun(t.ID)
		branch := launch.BranchName(m.cfg.Launch.BranchPrefix, t.Type, t.Title)
		if runErr == nil && run.Branch != "" {
			branch = run.Branch
		}
		b.WriteString(th.Muted.Render(fmt.Sprintf("%-*s", drawerLabelW, i18n.T("Branch"))) + th.Normal.Render(truncate(branch, inner-drawerLabelW)) + "\n")
		b.WriteString("\n" + th.Muted.Render(i18n.T("Description")) + "\n")
		b.WriteString(m.renderMarkdownW(t.Description, inner))
		b.WriteString("\n")
		if strings.TrimSpace(t.AcceptanceCriteria) != "" {
			b.WriteString("\n" + th.Muted.Render(i18n.T("Criteria")) + "\n")
			for _, line := range strings.Split(t.AcceptanceCriteria, "\n") {
				if line = strings.TrimSpace(line); line != "" {
					b.WriteString(th.Normal.Width(inner).Render("• "+line) + "\n")
				}
			}
		}
		if runErr == nil {
			b.WriteString("\n" + th.Muted.Render(i18n.T("Last run")) + "\n")
			info := run.Agent + " · " + run.State
			if run.DurationMS.Valid {
				info += " · " + (time.Duration(run.DurationMS.Int64) * time.Millisecond).Round(time.Second).String()
			}
			b.WriteString(th.Normal.Width(inner).Render(info) + "\n")
			if extra := lastRunBranchLine(m.boardProject.RepoPath, run); extra != "" {
				b.WriteString(th.Normal.Width(inner).Render(extra) + "\n")
			}
			if run.Note != "" {
				b.WriteString(th.Muted.Width(inner).Render(run.Note) + "\n")
			}
		}
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	room := max(1, h-3)
	if len(lines) > room {
		lines = append(lines[:room-1], th.Muted.Render("…"))
	}
	hint := i18n.T("tab panel  m P A change")
	if m.drawerFocus {
		hint = i18n.T("j/k  ←/→  enter  e edit  esc")
	}
	for len(lines) < room {
		lines = append(lines, "")
	}
	lines = append(lines, th.Muted.Render(truncate(hint, inner)))
	border := th.Muted.GetForeground()
	if m.drawerFocus {
		border = th.Title.GetForeground()
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(th.App.GetBackground()).
		Background(th.App.GetBackground()).
		Padding(0, 1).
		Width(w - 2).
		Height(h - 2).
		MaxHeight(h).
		Render(strings.Join(lines, "\n"))
}

func (m Model) updateDrawerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	t, ok := m.selectedTask()
	if !ok {
		m.drawerFocus = false
		return m, nil
	}
	p := prop(m.drawerField)
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "tab":
		m.drawerFocus = false
		m.status = ""
	case "j", "down":
		m.drawerField = (m.drawerField + 1) % int(propCount)
	case "k", "up":
		m.drawerField = (m.drawerField - 1 + int(propCount)) % int(propCount)
	case "enter", " ":
		return m.openFieldPopup(p)
	case "left", "h", "right", "l":
		if p == propStatus {
			return m.openFieldPopup(p)
		}
		dir := 1
		if s := msg.String(); s == "left" || s == "h" {
			dir = -1
		}
		return m.setTaskProp(t, p, cycleValue(propValue(t, p), p.options(), dir))
	case "m":
		return m.openFieldPopup(propStatus)
	case "P":
		return m.openFieldPopup(propPriority)
	case "A":
		return m.openFieldPopup(propAgent)
	case "e":
		m.drawerFocus = false
		m.openTaskForm(t)
		return m.withTaskFocus()
	case "o":
		return m.openEditor(m.taskFolder())
	case "g":
		return m.requestMergeFromTask()
	case "c":
		return m.requestCleanFromTask()
	}
	return m, nil
}

func lastRunBranchLine(repo string, run store.Run) string {
	if run.Branch == "" {
		return ""
	}
	if root, err := launch.RepoRoot(repo); err == nil {
		repo = root
	}
	if repo == "" || !launch.BranchExists(repo, run.Branch) {
		return ""
	}
	line := run.Branch
	if run.Base == "" {
		return line
	}
	ahead, err := launch.BranchAheadOfBase(repo, run.Base, run.Branch)
	if err != nil {
		return line
	}
	if ahead {
		return line + " · " + i18n.T("not merged into %s", run.Base)
	}
	return line + " · " + i18n.T("merged into %s", run.Base)
}

func (m Model) updateDrawerMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	t, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	idx := msg.Y - m.drawerPropTop(t, max(10, m.drawerWidth()-4))
	if idx >= 0 && idx < int(propCount) {
		m.drawerFocus = true
		m.drawerField = idx
		return m.openFieldPopup(prop(idx))
	}
	m.drawerFocus = true
	return m, nil
}

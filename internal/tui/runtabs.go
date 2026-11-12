package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// The run screen has three tabs: what the agent answered (summary), the diff
// it left (changes) and its raw log. Each tab scrolls on its own.
const (
	tabSummary = iota
	tabDiff
	tabLog
	runTabCount
)

func runTabLabel(t int) string {
	switch t {
	case tabDiff:
		return i18n.T("Changes")
	case tabLog:
		return i18n.T("Log")
	default:
		return i18n.T("Summary")
	}
}

func (m *Model) setRunTab(t int) {
	m.runTab = (t + runTabCount) % runTabCount
	m.status = m.reviewBar()
}

// runIsFinal reports whether a run can no longer be approved or sent back:
// its branch was merged, cleaned or discarded.
func runIsFinal(state string) bool {
	switch state {
	case store.RunReview, store.RunBlocked, store.RunApproved, store.RunRunning:
		return false
	}
	return true
}

func (m Model) runTabBar() string {
	th := m.theme
	var parts []string
	for t := 0; t < runTabCount; t++ {
		label := fmt.Sprintf(" %d %s ", t+1, runTabLabel(t))
		if t == tabDiff && len(m.diffFiles) > 0 {
			label = fmt.Sprintf(" %d %s (%d) ", t+1, runTabLabel(t), len(m.diffFiles))
		}
		if t == m.runTab {
			parts = append(parts, th.Selected.UnsetPadding().Render(label))
		} else {
			parts = append(parts, th.Muted.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

// paneHeight is the number of lines left for the active tab below the title,
// the tab bar and above the help line.
func (m Model) paneHeight() int {
	return max(4, m.height-6)
}

func (m Model) summaryLines() []string {
	inner := max(20, m.width-2)
	rounds, _ := m.renderRounds(inner, false)
	return strings.Split(rounds, "\n")
}

func (m *Model) scrollSummary(delta int) {
	lines := len(m.summaryLines())
	m.sumOffset += delta
	if limit := lines - (m.paneHeight() - 1); m.sumOffset > limit {
		m.sumOffset = limit
	}
	if m.sumOffset < 0 {
		m.sumOffset = 0
	}
}

func (m Model) logLines() []string {
	raw, _ := os.ReadFile(m.run.LogPath)
	if r, ok := m.selectedRound(); ok {
		raw = roundLogSlice(raw, r)
	}
	return strings.Split(string(raw), "\n")
}

func (m *Model) scrollLog(delta int) {
	lines := len(m.logLines())
	m.logOffset += delta
	if limit := lines - (m.paneHeight() - 1); m.logOffset > limit {
		m.logOffset = limit
	}
	if m.logOffset < 0 {
		m.logOffset = 0
	}
}

// window cuts lines[off:off+h] and adds a muted marker when there is more.
func (m Model) window(lines []string, off, h int) string {
	if off > len(lines) {
		off = max(0, len(lines)-1)
	}
	if off < 0 {
		off = 0
	}
	end := min(len(lines), off+h)
	body := strings.Join(lines[off:end], "\n")
	if rest := len(lines) - end; rest > 0 {
		body += "\n" + m.theme.Muted.Render(i18n.T("↓ %d more lines (j/k, PgDn)", rest))
	}
	return body
}

func (m Model) viewSummaryPane() string {
	lines := m.summaryLines()
	return m.window(lines, m.sumOffset, m.paneHeight()-1)
}

func (m Model) viewLogPane() string {
	if _, err := os.Stat(m.run.LogPath); err != nil {
		return m.theme.Muted.Render(i18n.T("No log: the worktree of this run was removed."))
	}
	lines := m.logLines()
	return m.window(lines, m.logOffset, m.paneHeight()-1)
}

func runTabHelp(tab int, state string) string {
	nav := i18n.T("tab/1 2 3 view")
	var pane string
	switch tab {
	case tabDiff:
		pane = i18n.T("j/k line  n/p file  enter comment  e edit  D delete  C comments")
	case tabLog:
		pane = i18n.T("j/k scroll  PgUp/PgDn page  [ ] round")
	default:
		pane = i18n.T("j/k scroll  PgUp/PgDn page  r fold  [ ] round")
	}
	var acts string
	switch {
	case state == store.RunApproved:
		acts = i18n.T("g merge  O PR  x request changes  c clean  d discard")
	case runIsFinal(state):
		acts = ""
	default:
		acts = i18n.T("y approve  x request changes  d discard")
	}
	out := nav + "  ·  " + pane
	if acts != "" {
		out += "  ·  " + acts
	}
	return out + "  ·  " + i18n.T("o editor  esc board")
}

// updateRunPaneKey scrolls the summary or the log tab.
func (m Model) updateRunPaneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	page := max(1, m.paneHeight()-2)
	scroll := m.scrollSummary
	if m.runTab == tabLog {
		scroll = m.scrollLog
	}
	switch msg.String() {
	case "j", "down":
		scroll(1)
	case "k", "up":
		scroll(-1)
	case "pgdown", "ctrl+d", " ", "f":
		scroll(page)
	case "pgup", "ctrl+u":
		scroll(-page)
	case "home":
		scroll(-1 << 30)
	case "end", "G":
		scroll(1 << 30)
	case "C":
		return m.openCommentList()
	}
	return m, nil
}

// leaveRunView goes back to where the run was opened from: the plan when it
// came from a plan step, the board otherwise.
func (m Model) leaveRunView() (tea.Model, tea.Cmd) {
	planID := m.backToPlan
	m.backToPlan = 0
	if planID != 0 {
		m.openPlans()
		for i, p := range m.plans.list {
			if p.ID == planID {
				m.plans.cursor = i
				m.plans.view = planDetail
				m.loadPlan(planID)
				m.status = planDetailHelp()
				return m, nil
			}
		}
	}
	m.screen = screenBoard
	m.reloadTasks()
	m.status = ""
	return m, nil
}

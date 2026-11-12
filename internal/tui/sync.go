package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/launch"
)

// The daemon, the Telegram bot, MCP and the CLI change the same store while
// the TUI is open. Every couple of seconds the TUI checks a cheap stamp of the
// store and, when it moved, reloads what the current screen shows.

const syncEvery = 2 * time.Second

type syncTickMsg struct{}

func syncTick() tea.Cmd {
	return tea.Tick(syncEvery, func(time.Time) tea.Msg { return syncTickMsg{} })
}

func (m Model) syncFromStore() Model {
	stamp := m.store.ChangeStamp()
	if stamp == m.stamp {
		return m
	}
	first := m.stamp == ""
	m.stamp = stamp
	if first {
		return m
	}
	switch m.screen {
	case screenList:
		m.reload()
	case screenBoard:
		m.reloadTasks()
	case screenPlans:
		if m.plans != nil {
			switch m.plans.view {
			case planList:
				m.reloadPlans()
			case planDetail:
				m.reloadPlans()
				m.loadPlan(m.plans.plan.ID)
			}
		}
	case screenReview, screenChanges:
		if m.run.ID != 0 {
			if r, err := m.store.GetRun(m.run.ID); err == nil {
				changed := r.State != m.run.State
				m.run = r
				if t, err := m.store.GetTask(r.TaskID); err == nil {
					m.runTask = t
				}
				if changed && dirExists(r.Worktree) {
					m.diffFiles, _ = launch.WorktreeDiff(r.Worktree, r.Base)
					if m.diffIndex >= len(m.diffFiles) {
						m.diffIndex, m.diffOffset, m.diffCursor = 0, 0, 0
					}
				}
			}
			m.loadRounds()
			m.loadComments()
			if m.screen == screenReview {
				m.status = m.reviewBar()
			}
		}
	}
	return m
}

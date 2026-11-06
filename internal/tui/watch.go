package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// Agents run detached (see launch.StartAgent), so they keep working when
// Troopa closes. Troopa polls their state and finalizes the run when the
// agent exits, whether or not its screen is open.

const watchEvery = time.Second

// startAgent starts the agent of a run in the background and returns the
// command that watches it.
func startAgent(cmd *exec.Cmd, run store.Run) (tea.Cmd, error) {
	if err := launch.StartAgent(cmd, run.Worktree, run.LogPath); err != nil {
		return nil, err
	}
	return watchAgent(run.ID, run.Worktree), nil
}

// watchAgent checks the agent every second until it stops.
func watchAgent(runID int64, worktree string) tea.Cmd {
	return tea.Tick(watchEvery, func(time.Time) tea.Msg { return agentStatusMsg(runID, worktree) })
}

type agentWatchMsg struct {
	runID    int64
	worktree string
}

func agentStatusMsg(runID int64, worktree string) tea.Msg {
	switch state, code := launch.AgentStatus(worktree); state {
	case launch.AgentRunning:
		return agentWatchMsg{runID: runID, worktree: worktree}
	case launch.AgentExited:
		var err error
		if code != 0 {
			err = fmt.Errorf("%s %d", i18n.T("The agent exited with code"), code)
		}
		return agentDoneMsg{runID: runID, err: err}
	default:
		return agentDoneMsg{runID: runID, err: errors.New(errAgentLost())}
	}
}

func errAgentLost() string {
	return i18n.T("The agent stopped before finishing (Troopa, the terminal or the computer closed). Press x to send it back and continue.")
}

// reconcileRuns looks at the runs left running from an earlier session:
// agents still working get a watcher, the ones that finished or died are
// finalized right away.
func reconcileRuns(s *store.Store) []tea.Cmd {
	runs, err := s.ListRuns(store.RunRunning)
	if err != nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, run := range runs {
		t, err := s.GetTask(run.TaskID)
		if err != nil || t.Mode == store.ModePair {
			continue
		}
		run := run
		if state, _ := launch.AgentStatus(run.Worktree); state == launch.AgentRunning {
			cmds = append(cmds, watchAgent(run.ID, run.Worktree))
			continue
		}
		cmds = append(cmds, func() tea.Msg { return agentStatusMsg(run.ID, run.Worktree) })
	}
	return cmds
}

// finishBackgroundRun finalizes a run whose screen is not open and leaves the
// current screen as it is.
func (m Model) finishBackgroundRun(msg agentDoneMsg) (tea.Model, tea.Cmd) {
	run, err := m.store.GetRun(msg.runID)
	if err != nil {
		return m, nil
	}
	t, err := m.store.GetTask(run.TaskID)
	if err != nil {
		return m, nil
	}
	c := m
	c.bindRun(t, run)
	if r, err := launch.RepoFromWorktree(run.Worktree); err == nil {
		c.runSess.Repo = r
	}
	res, cmd := c.finishAgent(msg)
	done := res.(Model)
	if m.screen == screenBoard {
		m.reloadTasks()
	}
	m.err = ""
	m.status = fmt.Sprintf("%s · %s", t.Title, done.run.State)
	if done.run.ID != run.ID {
		// Auto-approve started the next plan step: keep watching it.
		m.status = done.status
		if m.screen == screenBoard {
			m.reloadTasks()
		}
		return m, cmd
	}
	return m, nil
}

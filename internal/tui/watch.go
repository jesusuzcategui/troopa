package tui

import (
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/engine"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
)

// Agents run detached (see launch.StartAgent), so they keep working when
// Troopa closes. Troopa polls their state and finalizes the run when the
// agent exits, whether or not its screen is open.

const watchEvery = time.Second

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
func reconcileRuns(e *engine.Engine) []tea.Cmd {
	if e == nil {
		return nil
	}
	rec, err := e.Reconcile()
	if err != nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, run := range rec.Alive {
		run := run
		cmds = append(cmds, watchAgent(run.ID, run.Worktree))
	}
	return cmds
}

// finishBackgroundRun finalizes a run whose screen is not open and leaves the
// current screen as it is.
func (m Model) finishBackgroundRun(msg agentDoneMsg) (tea.Model, tea.Cmd) {
	res, err := m.eng().Finish(msg.runID, msg.err)
	if err != nil || !res.Won {
		return m, nil
	}
	return m.applyEngineFinish(res, false)
}

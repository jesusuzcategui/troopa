package engine

import (
	"fmt"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// Reconcile looks at runs left running from an earlier session: agents still
// working are returned so the caller can watch them; the ones that finished
// or died are finalized here.
func (e *Engine) Reconcile() (ReconcileReport, error) {
	runs, err := e.Store.ListRuns(store.RunRunning)
	if err != nil {
		return ReconcileReport{}, err
	}
	var alive []store.Run
	for _, run := range runs {
		t, err := e.Store.GetTask(run.TaskID)
		if err != nil || t.Mode == store.ModePair {
			continue
		}
		switch state, code := launch.AgentStatus(run.Worktree); state {
		case launch.AgentRunning:
			alive = append(alive, run)
		case launch.AgentExited:
			var exitErr error
			if code != 0 {
				exitErr = fmt.Errorf("%s %d", i18n.T("The agent exited with code"), code)
			}
			_, _ = e.Finish(run.ID, exitErr)
		default:
			_, _ = e.Finish(run.ID, fmt.Errorf("%s", errAgentLost()))
		}
	}
	return ReconcileReport{Alive: alive}, nil
}

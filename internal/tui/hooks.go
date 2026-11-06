package tui

import (
	"path/filepath"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// fireHook runs the [hooks] of an event, global and from the project's
// troopa.toml, in the background.
func (m Model) fireHook(ev launch.HookEvent, p store.Project) {
	if ev.Project == "" {
		ev.Project = p.Name
	}
	repo := p.RepoPath
	if r, err := launch.RepoRoot(p.RepoPath); err == nil {
		repo = r
	}
	if ev.Repo == "" {
		ev.Repo = repo
	}
	project := launch.LoadProjectHooks(repo)
	if len(m.cfg.Launch.Hooks[ev.Event]) == 0 && len(project[ev.Event]) == 0 {
		return
	}
	launch.RunHooks(m.cfg.Launch.Hooks, project, ev, hooksLog())
}

func hooksLog() string {
	dataDir, _, _, err := config.Paths()
	if err != nil {
		return ""
	}
	return filepath.Join(dataDir, "hooks.log")
}

func (m Model) fireTaskHook(event string, t store.Task) {
	m.fireHook(launch.HookEvent{Event: event, TaskID: t.ID, Task: t.Title, Status: t.Status, Agent: t.Agent, PlanID: t.PlanID}, m.taskProject(t))
}

func (m Model) fireRunHook(event string, t store.Task, run store.Run, msg string) {
	m.fireHook(launch.HookEvent{
		Event: event, TaskID: t.ID, Task: t.Title, Status: t.Status, Agent: run.Agent,
		RunID: run.ID, Branch: run.Branch, Worktree: run.Worktree, PlanID: t.PlanID, Message: msg,
	}, m.taskProject(t))
}

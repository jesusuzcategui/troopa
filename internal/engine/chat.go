package engine

import (
	"os/exec"

	"github.com/jesusuzcategui/troopa/internal/launch"
)

// ChatCommand builds the command for one turn of a conversation with an
// agent: the project's folder as working directory and the project's account,
// like a launch, but no worktree, no run and no review.
func (e *Engine) ChatCommand(projectID int64, agent, prompt string, images ...string) (*exec.Cmd, error) {
	project, err := e.project(projectID)
	if err != nil {
		return nil, err
	}
	lc, err := e.launchCfg(project)
	if err != nil {
		return nil, err
	}
	dir := project.RepoPath
	if root, err := launch.RepoRoot(project.RepoPath); err == nil {
		dir = root
	}
	cmd, err := launch.AgentCommandPrompt(lc, agent, "", dir, prompt)
	if err != nil {
		return nil, err
	}
	launch.AddImages(cmd, agent, images)
	return cmd, nil
}

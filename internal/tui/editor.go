package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
)

type editorClosedMsg struct{ err error }

// openEditor suspends the TUI and opens dir in the editor. Terminal editors
// (nvim) take over the screen; GUI editors return right away.
func (m Model) openEditor(dir string) (tea.Model, tea.Cmd) {
	if dir == "" {
		m.err = i18n.T("Nothing to open: the task has no worktree and the project has no folder")
		return m, nil
	}
	cmd, err := launch.EditorCommand(m.cfg.Launch.Editor, dir)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.err = ""
	m.status = i18n.T("Opening in the editor: ") + dir
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorClosedMsg{err: err} })
}

func (m Model) handleEditorClosed(msg editorClosedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = i18n.T("Editor exited with an error: ") + msg.err.Error()
	}
	return m, nil
}

// taskFolder is the worktree of the task's latest run if it still exists,
// otherwise the project folder.
func (m Model) taskFolder() string {
	t, ok := m.selectedTask()
	if !ok {
		return m.boardProject.RepoPath
	}
	if run, err := m.store.LatestRun(t.ID); err == nil && run.Worktree != "" && dirExists(run.Worktree) {
		return run.Worktree
	}
	return m.boardProject.RepoPath
}

// runFolder is the worktree of the run shown on screen.
func (m Model) runFolder() string {
	wt := m.run.Worktree
	if wt == "" {
		wt = m.runSess.Worktree
	}
	if dirExists(wt) {
		return wt
	}
	if m.boardProject.RepoPath != "" {
		return m.boardProject.RepoPath
	}
	return m.runSess.Repo
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

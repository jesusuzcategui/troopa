package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// canRequestChanges reports whether the run on screen can go back to its agent.
func canRequestChanges(state string) bool {
	switch state {
	case store.RunReview, store.RunBlocked, store.RunApproved:
		return true
	}
	return false
}

func (m Model) requestChangesFromTask() (tea.Model, tea.Cmd) {
	t, ok := m.selectedTask()
	if !ok {
		m.status = i18n.T("No task selected")
		return m, nil
	}
	run, err := m.store.LatestRun(t.ID)
	if err != nil || !canRequestChanges(run.State) {
		m.err = ""
		m.status = i18n.T("This task has no run to send back to its agent")
		return m, nil
	}
	m.bindRun(t, run)
	return m.openChanges(screenBoard)
}

func (m Model) openChanges(from screen) (tea.Model, tea.Cmd) {
	if !canRequestChanges(m.run.State) {
		m.status = i18n.T("This task has no run to send back to its agent")
		return m, nil
	}
	if m.runTask.Agent == store.AgentMe || m.runTask.Agent == "" {
		m.status = i18n.T("This task has no agent: make the changes yourself with o")
		return m, nil
	}
	if !dirExists(m.runSess.Worktree) {
		m.err = i18n.T("The worktree is gone: discard the run and launch the task again")
		return m, nil
	}
	in := textarea.New()
	in.Placeholder = i18n.T("What should change? For example: add requirements.txt and explain how to create the venv")
	in.SetWidth(max(30, min(80, m.width-6)))
	in.SetHeight(6)
	in.ShowLineNumbers = false
	in.Prompt = ""
	in.Focus()
	m.changesInput = in
	m.changesFrom = from
	m.screen = screenChanges
	m.err = ""
	m.status = i18n.T("ctrl+s send to the agent · esc cancel")
	return m, textarea.Blink
}

func (m Model) updateChangesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.screen = m.changesFrom
		m.status = ""
		return m, nil
	case "ctrl+s":
		return m.sendChanges(m.changesInput.Value())
	}
	var cmd tea.Cmd
	m.changesInput, cmd = m.changesInput.Update(msg)
	return m, cmd
}

// sendChanges writes the feedback to CHANGES.md and starts the same agent
// again in the same worktree, keeping what it already did.
func (m Model) sendChanges(feedback string) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(feedback) == "" {
		m.err = i18n.T("Write what should change")
		return m, nil
	}
	if _, err := m.store.ActiveRun(); err == nil {
		m.err = i18n.T("An agent is already running")
		return m, nil
	}
	t := m.runTask
	if !launch.AgentInstalled(m.cfg.Launch, t.Agent) {
		m.err = i18n.T("%s is not installed. Install it or pick another agent with A.", launch.AgentName(t.Agent))
		return m, nil
	}
	wt := m.runSess.Worktree
	round, err := launch.RequestChanges(wt, feedback)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	note := i18n.T("Changes requested (round %d)", round)
	if t.Mode == store.ModePair {
		m.run.Note = note
		_ = m.store.UpdateRun(m.run)
		m.status = note
		return m.startPair(t)
	}
	cmd, err := launch.AgentCommandPrompt(m.cfg.Launch, t.Agent, t.Model, wt, launch.ChangesPrompt(wt, round))
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	if f, err := os.OpenFile(m.run.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintf(f, "\n\n── Troopa · %s ──\n%s\n\n", note, strings.TrimSpace(feedback))
		f.Close()
	}
	m.run.State = store.RunRunning
	m.run.Note = note
	if err := m.store.UpdateRun(m.run); err != nil {
		m.err = err.Error()
		return m, nil
	}
	if err := m.store.MoveTask(t.ID, store.StatusInProgress); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.logOffset = 0
	m.screen = screenLog
	m.err = ""
	m.status = note + " · " + launch.AgentName(t.Agent)
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusInProgress)
	return m, tea.Batch(runAgent(cmd, m.run.LogPath, m.run.ID), tickLog())
}

func (m Model) viewChanges() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Request changes")))
	b.WriteString(th.Muted.Render("  ·  " + truncate(m.runTask.Title, 40) + "  ·  " + launch.AgentName(m.runTask.Agent)))
	b.WriteString("\n\n")
	if r := launch.ChangeRounds(m.runSess.Worktree); r > 0 {
		b.WriteString(th.Muted.Render(fmt.Sprintf(i18n.T("Rounds so far: %d (in CHANGES.md)"), r)) + "\n\n")
	}
	b.WriteString(th.Muted.Render(i18n.T("The agent keeps its work in the same worktree and applies what you write here.")) + "\n\n")
	b.WriteString(m.changesInput.View())
	b.WriteString("\n\n")
	b.WriteString(th.Muted.Render(i18n.T("ctrl+s send to the agent · esc cancel")))
	return th.App.Render(b.String())
}

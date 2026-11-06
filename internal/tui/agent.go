package tui

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

type logTickMsg struct{}
type agentDoneMsg struct {
	runID int64
	err   error
}
type pairClosedMsg struct {
	runID int64
}

var openPairSession = launch.OpenPairSession

func (m Model) maybeLaunch(t store.Task, dest string) (tea.Model, tea.Cmd) {
	if dest != store.StatusReady || t.Agent == store.AgentMe || t.Agent == "" {
		return m.applyMove(t, dest)
	}
	if t.Status == store.StatusReady {
		return m.applyMove(t, dest)
	}
	return m.startAgentFor(t)
}

func (m Model) startAgentFor(t store.Task) (tea.Model, tea.Cmd) {
	if t.Mode == store.ModePair {
		return m.startPair(t)
	}
	return m.launchAgent(t)
}

func (m Model) applyMove(t store.Task, dest string) (tea.Model, tea.Cmd) {
	if err := m.store.MoveTask(t.ID, dest); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.colIndex = indexOfStatus(dest)
	m.reloadTasks()
	col := m.columnTasks(dest)
	for k, c := range col {
		if c.ID == t.ID {
			m.cardIndex = k
			break
		}
	}
	m.status = i18n.T("Moved to ") + dest
	m.err = ""
	return m, nil
}

func (m Model) startPair(t store.Task) (tea.Model, tea.Cmd) {
	if active, err := m.store.ActiveRun(); err == nil && active.TaskID != t.ID {
		m.err = i18n.T("An agent is already running")
		return m, nil
	}
	if m.cfg.Launch.Agents == nil {
		m.err = i18n.T("No agents in config.toml")
		return m, nil
	}
	prev := launch.Session{}
	if run, err := m.store.LatestRun(t.ID); err == nil && (run.State == store.RunPairPaused || run.State == store.RunRunning) {
		prev = launch.Session{Worktree: run.Worktree, Branch: run.Branch, Base: run.Base, LogPath: run.LogPath, Repo: m.boardProject.RepoPath}
		if r, err := launch.RepoFromWorktree(run.Worktree); err == nil {
			prev.Repo = r
		}
	}
	sess, err := launch.PreparePair(m.cfg.Launch, m.boardProject, t, prev)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	if err := launch.SetupPairAgent(m.cfg.Launch, t.Agent, t.Model, sess.Worktree); err != nil {
		if prev.Worktree == "" {
			_ = launch.Discard(sess)
		}
		m.err = err.Error()
		return m, nil
	}
	proc, err := openPairSession(sess.Worktree, m.cfg.Launch.Terminal)
	if err != nil {
		if prev.Worktree == "" {
			_ = launch.Discard(sess)
		}
		m.err = err.Error()
		return m, nil
	}
	run := store.Run{}
	if existing, err := m.store.LatestRun(t.ID); err == nil && (existing.State == store.RunPairPaused || existing.State == store.RunRunning) && existing.Worktree == sess.Worktree {
		existing.State = store.RunRunning
		existing.Note = proc.Where
		existing.Worktree = sess.Worktree
		existing.Branch = sess.Branch
		existing.Base = sess.Base
		existing.LogPath = sess.LogPath
		if existing.Agent == "" {
			existing.Agent = t.Agent
		}
		if err := m.store.UpdateRun(existing); err != nil {
			m.err = err.Error()
			return m, nil
		}
		run = existing
	} else {
		created, err := m.store.CreateRun(store.Run{
			TaskID:    t.ID,
			ProjectID: t.ProjectID,
			Worktree:  sess.Worktree,
			Branch:    sess.Branch,
			Base:      sess.Base,
			LogPath:   sess.LogPath,
			State:     store.RunRunning,
			Note:      proc.Where,
			Agent:     t.Agent,
		})
		if err != nil {
			if prev.Worktree == "" {
				_ = launch.Discard(sess)
			}
			m.err = err.Error()
			return m, nil
		}
		run = created
	}
	if err := m.store.MoveTask(t.ID, store.StatusInProgress); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.run = run
	m.runSess = sess
	m.runTask = t
	m.runPair = proc
	m.screen = screenPair
	m.status = i18n.T("Pair ") + t.Agent + " · " + sess.Branch
	m.err = ""
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusInProgress)
	return m, watchPair(run.ID, proc)
}

func watchPair(runID int64, proc launch.PairProc) tea.Cmd {
	if proc.Wait == nil {
		return nil
	}
	return func() tea.Msg {
		_ = proc.Wait()
		return pairClosedMsg{runID: runID}
	}
}

func (m Model) handlePairClosed(msg pairClosedMsg) (tea.Model, tea.Cmd) {
	if m.run.ID != 0 && m.run.ID != msg.runID {
		return m, nil
	}
	if m.run.ID == 0 {
		var err error
		m.run, err = m.store.GetRun(msg.runID)
		if err != nil {
			return m, nil
		}
	}
	if m.run.State != store.RunRunning {
		return m, nil
	}
	return m.pausePair()
}

func (m Model) pausePair() (tea.Model, tea.Cmd) {
	if m.runPair.Stop != nil {
		m.runPair.Stop()
	}
	m.run.State = store.RunPairPaused
	m.run.Note = i18n.T("Pair session paused. Press enter to resume.")
	_ = m.store.UpdateRun(m.run)
	_ = m.store.MoveTask(m.run.TaskID, store.StatusPairPaused)
	m.screen = screenBoard
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusPairPaused)
	m.status = i18n.T("pair-paused · enter to resume")
	m.err = ""
	m.runPair = launch.PairProc{}
	return m, nil
}

func (m Model) finishPair() (tea.Model, tea.Cmd) {
	if m.runPair.Stop != nil {
		m.runPair.Stop()
	}
	repo := m.runSess.Repo
	if repo == "" {
		repo = m.boardProject.RepoPath
	}
	summary, checksJSON, ok := launch.FinishPair(m.runSess, repo)
	if summary == "" {
		if raw, err := os.ReadFile(m.run.LogPath); err == nil {
			summary = strings.TrimSpace(string(raw))
			if len(summary) > 2000 {
				summary = summary[len(summary)-2000:]
			}
		}
	}
	m.run.Summary = summary
	m.run.Checks = checksJSON
	dest := store.StatusReview
	m.run.State = store.RunReview
	if !ok {
		dest = store.StatusBlocked
		m.run.State = store.RunBlocked
	}
	applyRunMetrics(&m.run, m.run.LogPath, m.runTask.Agent, m.cfg.Launch.PriceFor(m.runTask.Agent))
	_ = m.store.UpdateRun(m.run)
	_ = m.store.MoveTask(m.run.TaskID, dest)
	m.diffFiles, _ = launch.WorktreeDiff(m.runSess.Worktree, m.run.Base)
	m.diffIndex = 0
	m.diffOffset = 0
	m.reloadTasks()
	m.colIndex = indexOfStatus(dest)
	m.screen = screenReview
	m.status = dest
	m.err = ""
	m.runPair = launch.PairProc{}
	if m.cfg.Launch.Notify {
		launch.Notify("Troopa: "+dest, m.runTask.Title)
	}
	return m, nil
}

func (m Model) updatePairKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = screenBoard
		m.status = i18n.T("Pair is still running in the terminal")
		return m, nil
	case "f":
		return m.finishPair()
	case "r":
		return m.startPair(m.runTask)
	case "o":
		return m.openEditor(m.runFolder())
	}
	return m, nil
}

func (m Model) viewPair() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Pair")))
	b.WriteString(th.Muted.Render("  ·  " + m.run.Branch + "  ·  " + m.run.State))
	b.WriteString("\n\n")
	b.WriteString("Worktree  " + m.run.Worktree + "\n")
	if m.run.Note != "" && !isPausedNote(m.run.Note) {
		b.WriteString(i18n.T("Session   ") + m.run.Note + "\n")
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("Read TASK.md and PAIR.md (TDD: red, green, refactor only when asked).")))
	b.WriteString("\n\n")
	b.WriteString(th.Muted.Render(i18n.T("f finish pair  r reopen agent  o editor  esc board")))
	return th.App.Render(b.String())
}

func (m Model) launchAgent(t store.Task) (tea.Model, tea.Cmd) {
	if _, err := m.store.ActiveRun(); err == nil {
		m.err = i18n.T("An agent is already running")
		return m, nil
	}
	if m.cfg.Launch.Agents == nil {
		m.err = i18n.T("No agents in config.toml")
		return m, nil
	}
	sess, err := launch.Prepare(m.cfg.Launch, m.boardProject, t)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	cmd, err := launch.AgentCommand(m.cfg.Launch, t.Agent, t.Model, sess.Worktree)
	if err != nil {
		_ = launch.Discard(sess)
		m.err = err.Error()
		return m, nil
	}
	run, err := m.store.CreateRun(store.Run{
		TaskID:    t.ID,
		ProjectID: t.ProjectID,
		Worktree:  sess.Worktree,
		Branch:    sess.Branch,
		Base:      sess.Base,
		LogPath:   sess.LogPath,
		State:     store.RunRunning,
		Agent:     t.Agent,
	})
	if err != nil {
		_ = launch.Discard(sess)
		m.err = err.Error()
		return m, nil
	}
	if err := m.store.MoveTask(t.ID, store.StatusInProgress); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.run = run
	m.runSess = sess
	m.runTask = t
	m.logOffset = 0
	m.screen = screenLog
	m.status = i18n.T("Agent ") + t.Agent + " · " + sess.Branch
	m.err = ""
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusInProgress)
	return m, tea.Batch(runAgent(cmd, sess.LogPath, run.ID), tickLog())
}

func runAgent(cmd *exec.Cmd, logPath string, runID int64) tea.Cmd {
	return func() tea.Msg {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return agentDoneMsg{runID: runID, err: err}
		}
		defer f.Close()
		cmd.Stdout = f
		cmd.Stderr = f
		return agentDoneMsg{runID: runID, err: cmd.Run()}
	}
}

func tickLog() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg { return logTickMsg{} })
}

func (m Model) finishAgent(msg agentDoneMsg) (tea.Model, tea.Cmd) {
	if m.run.ID != 0 && m.run.ID != msg.runID {
		return m, nil
	}
	run := m.run
	if run.ID == 0 {
		var err error
		run, err = m.store.GetRun(msg.runID)
		if err != nil {
			return m, nil
		}
	}
	sess := m.runSess
	if sess.Worktree == "" {
		repo := m.boardProject.RepoPath
		if r, err := launch.RepoRoot(run.Worktree); err == nil {
			repo = r
		}
		sess = launch.Session{Worktree: run.Worktree, Branch: run.Branch, Base: run.Base, LogPath: run.LogPath, Repo: repo}
		m.runSess = sess
	}
	summary := launch.ReadSummary(sess.Worktree)
	if summary == "" {
		if raw, err := os.ReadFile(run.LogPath); err == nil {
			summary = strings.TrimSpace(string(raw))
			if len(summary) > 2000 {
				summary = summary[len(summary)-2000:]
			}
		}
	}
	repo := sess.Repo
	if repo == "" {
		repo = m.boardProject.RepoPath
	}
	checks := launch.RunChecks(sess.Worktree, launch.LoadProjectChecks(repo))
	raw, _ := json.Marshal(checks)
	run.Summary = summary
	run.Checks = string(raw)
	ok := msg.err == nil && launch.ChecksOK(checks)
	dest := store.StatusReview
	run.State = store.RunReview
	if !ok {
		dest = store.StatusBlocked
		run.State = store.RunBlocked
		if msg.err != nil {
			run.Note = msg.err.Error()
		}
	}
	applyRunMetrics(&run, run.LogPath, m.runTask.Agent, m.cfg.Launch.PriceFor(m.runTask.Agent))
	_ = m.store.UpdateRun(run)
	_ = m.store.MoveTask(run.TaskID, dest)
	m.run = run
	m.diffFiles, _ = launch.WorktreeDiff(sess.Worktree, run.Base)
	m.diffIndex = 0
	m.diffOffset = 0
	m.reloadTasks()
	m.colIndex = indexOfStatus(dest)
	m.screen = screenReview
	m.status = dest
	if m.cfg.Launch.Notify {
		title := "Troopa: " + dest
		launch.Notify(title, m.runTask.Title)
	}
	return m, nil
}

func (m Model) openRunView(t store.Task) (tea.Model, tea.Cmd) {
	run, err := m.store.LatestRun(t.ID)
	if err != nil {
		if t.Mode == store.ModePair && t.Agent != store.AgentMe && t.Agent != "" {
			return m.startPair(t)
		}
		m.openTaskForm(t)
		return m.withTaskFocus()
	}
	m.run = run
	m.runTask = t
	m.runSess = launch.Session{Worktree: run.Worktree, Branch: run.Branch, Base: run.Base, LogPath: run.LogPath, Repo: m.boardProject.RepoPath}
	if r, err := launch.RepoFromWorktree(run.Worktree); err == nil {
		m.runSess.Repo = r
	}
	switch run.State {
	case store.RunPairPaused:
		return m.startPair(t)
	case store.RunRunning:
		if t.Mode == store.ModePair {
			if !launch.PairStillOpen(run.Note, run.Worktree) {
				return m.pausePair()
			}
			m.screen = screenPair
			m.status = i18n.T("f finish pair  r reopen  o editor  esc board")
			return m, nil
		}
		m.screen = screenLog
		m.status = i18n.T("Agent running · ") + run.Branch
		return m, tickLog()
	case store.RunReview, store.RunBlocked:
		m.diffFiles, _ = launch.WorktreeDiff(run.Worktree, run.Base)
		m.diffIndex = 0
		m.diffOffset = 0
		m.screen = screenReview
		m.status = i18n.T("y approve  d discard  n/p file  o editor  esc board")
		return m, nil
	default:
		m.openTaskForm(t)
		return m.withTaskFocus()
	}
}

func (m Model) approveRun() (tea.Model, tea.Cmd) {
	if m.run.State != store.RunReview {
		m.err = i18n.T("Only a run in review can be approved")
		return m, nil
	}
	sess := m.runSess
	msg, err := launch.Approve(sess, m.runTask, m.run.Summary)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.run.State = store.RunApproved
	m.run.Note = msg
	_ = m.store.UpdateRun(m.run)
	_ = m.store.MoveTask(m.run.TaskID, store.StatusDone)
	m.screen = screenBoard
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusDone)
	m.status = msg
	m.err = ""
	return m, nil
}

func (m Model) discardRun() (tea.Model, tea.Cmd) {
	if err := launch.Discard(m.runSess); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.run.State = store.RunDiscarded
	_ = m.store.UpdateRun(m.run)
	_ = m.store.MoveTask(m.run.TaskID, store.StatusBacklog)
	m.screen = screenBoard
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusBacklog)
	m.status = i18n.T("Discarded: worktree and branch removed")
	m.err = ""
	return m, nil
}

func (m Model) updateLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = screenBoard
		m.status = i18n.T("Agent is still running")
		return m, nil
	case "o":
		return m.openEditor(m.runFolder())
	case "j", "down":
		m.logOffset++
	case "k", "up":
		if m.logOffset > 0 {
			m.logOffset--
		}
	}
	return m, nil
}

func (m Model) updateReviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.popup != nil {
		return m.updatePopupKey(msg)
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = screenBoard
		return m, nil
	case "y", "a":
		return m.approveRun()
	case "d":
		return m.openDiscardPopup()
	case "o":
		return m.openEditor(m.runFolder())
	case "n", "l", "right":
		if m.diffIndex < len(m.diffFiles)-1 {
			m.diffIndex++
			m.diffOffset = 0
		}
	case "p", "h", "left":
		if m.diffIndex > 0 {
			m.diffIndex--
			m.diffOffset = 0
		}
	case "j", "down":
		m.diffOffset++
	case "k", "up":
		if m.diffOffset > 0 {
			m.diffOffset--
		}
	}
	return m, nil
}

func (m Model) viewLog() string {
	th := m.theme
	raw, _ := os.ReadFile(m.run.LogPath)
	lines := strings.Split(string(raw), "\n")
	h := max(4, m.height-6)
	off := m.logOffset
	if off > len(lines) {
		off = max(0, len(lines)-1)
	}
	end := min(len(lines), off+h)
	body := strings.Join(lines[off:end], "\n")
	title := th.Title.Render(i18n.T("Agent")) + th.Muted.Render("  ·  "+m.run.Branch+"  ·  "+m.run.State)
	return th.App.Render(title + "\n\n" + body + "\n\n" + th.Muted.Render(i18n.T("j/k scroll  o editor  esc board")))
}

func (m Model) viewReview() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Review")))
	b.WriteString(th.Muted.Render("  ·  " + m.run.Branch + "  ·  " + m.run.State))
	b.WriteString("\n\n")
	if m.run.Summary != "" {
		b.WriteString(th.Muted.Render(i18n.T("Summary")))
		b.WriteString("\n")
		b.WriteString(truncate(m.run.Summary, 800))
		b.WriteString("\n\n")
	}
	b.WriteString(th.Muted.Render("Checks"))
	b.WriteString("\n")
	b.WriteString(launch.FormatChecks(decodeChecks(m.run.Checks)))
	b.WriteString("\n\n")
	if len(m.diffFiles) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("No diff")))
	} else {
		f := m.diffFiles[m.diffIndex]
		b.WriteString(th.Title.Render(f.Path))
		b.WriteString(th.Muted.Render(fmtIndex(m.diffIndex+1, len(m.diffFiles))))
		b.WriteString("\n")
		h := max(4, m.height-16)
		start := m.diffOffset
		if start > len(f.Lines) {
			start = max(0, len(f.Lines)-1)
		}
		end := min(len(f.Lines), start+h)
		for _, line := range f.Lines[start:end] {
			text := truncate(line.Text, max(8, m.width-2))
			switch line.Kind {
			case '+':
				b.WriteString(th.DiffAdd.Render(text))
			case '-':
				b.WriteString(th.DiffDel.Render(text))
			case '@':
				b.WriteString(th.DiffHunk.Render(text))
			default:
				b.WriteString(th.Muted.Render(text))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("y approve  d discard  n/p file  j/k scroll  o editor  esc board")))
	return th.App.Render(b.String())
}

func decodeChecks(raw string) []launch.CheckResult {
	var rs []launch.CheckResult
	if raw == "" {
		return rs
	}
	_ = json.Unmarshal([]byte(raw), &rs)
	return rs
}

func fmtIndex(i, n int) string {
	return "  (" + strconv.Itoa(i) + "/" + strconv.Itoa(n) + ")"
}

// isPausedNote reports whether a run note is the "pair session paused" note,
// in any language it may have been saved in.
func isPausedNote(note string) bool {
	return strings.HasPrefix(note, "Pair session") || strings.HasPrefix(note, "Sesión")
}

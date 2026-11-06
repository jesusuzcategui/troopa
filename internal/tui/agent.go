package tui

import (
	"encoding/json"
	"fmt"
	"os"
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
	if !launch.AgentInstalled(m.cfg.Launch, t.Agent) {
		m.err = i18n.T("%s is not installed. Install it or pick another agent with A.", launch.AgentName(t.Agent))
		return m, nil
	}
	return m.openLaunchPopup(t)
}

func (m Model) startAgentFor(t store.Task) (tea.Model, tea.Cmd) {
	if t.Mode == store.ModePair {
		return m.startPair(t)
	}
	return m.launchAgent(t)
}

func (m Model) applyMove(t store.Task, dest string) (tea.Model, tea.Cmd) {
	if err := m.moveTask(t.ID, dest); err != nil {
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
	if err := m.stageTask(sess.Worktree, t); err != nil {
		if prev.Worktree == "" {
			_ = launch.Discard(sess)
		}
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
	m.run = run
	if err := m.ensureRound(store.RoundLaunch, ""); err != nil {
		m.err = err.Error()
		return m, nil
	}
	if err := m.moveTask(t.ID, store.StatusInProgress); err != nil {
		m.err = err.Error()
		return m, nil
	}
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
	_ = m.moveTask(m.run.TaskID, store.StatusPairPaused)
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
	price := m.cfg.Launch.PriceFor(m.runTask.Agent)
	applyRunMetrics(&m.run, m.run.LogPath, m.runTask.Agent, price)
	m.closeOpenRound(m.run.State, summary, checksJSON, m.run.LogPath, price)
	_ = m.store.UpdateRun(m.run)
	_ = m.moveTask(m.run.TaskID, dest)
	m.loadRounds()
	m.diffFiles, _ = launch.WorktreeDiff(m.runSess.Worktree, m.run.Base)
	m.diffIndex = 0
	m.diffOffset = 0
	m.diffCursor = 0
	m.loadComments()
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
	if err := m.stageTask(sess.Worktree, t); err != nil {
		_ = launch.Discard(sess)
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
	m.run = run
	if err := m.beginRound(store.RoundLaunch, ""); err != nil {
		m.err = err.Error()
		return m, nil
	}
	if err := m.moveTask(t.ID, store.StatusInProgress); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.runSess = sess
	m.runTask = t
	m.logOffset = 0
	m.screen = screenLog
	m.status = i18n.T("Agent ") + t.Agent + " · " + sess.Branch
	m.err = ""
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusInProgress)
	watch, err := startAgent(cmd, run)
	if err != nil {
		return m.finishAgent(agentDoneMsg{runID: run.ID, err: err})
	}
	return m, tea.Batch(watch, tickLog())
}

func tickLog() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg { return logTickMsg{} })
}

func (m Model) finishAgent(msg agentDoneMsg) (tea.Model, tea.Cmd) {
	if m.run.ID != msg.runID {
		return m.finishBackgroundRun(msg)
	}
	// Finalize once: a run already out of "running" was handled before.
	if fresh, err := m.store.GetRun(msg.runID); err != nil || fresh.State != store.RunRunning {
		return m, nil
	}
	run := m.run
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
	price := m.cfg.Launch.PriceFor(m.runTask.Agent)
	applyRunMetrics(&run, run.LogPath, m.runTask.Agent, price)
	m.run = run
	m.closeOpenRound(run.State, summary, string(raw), run.LogPath, price)
	_ = m.store.UpdateRun(run)
	_ = m.moveTask(run.TaskID, dest)
	m.loadRounds()
	m.diffFiles, _ = launch.WorktreeDiff(sess.Worktree, run.Base)
	m.diffIndex = 0
	m.diffOffset = 0
	m.diffCursor = 0
	m.loadComments()
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
	m.bindRun(t, run)
	m.loadRounds()
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
		if st, _ := launch.AgentStatus(run.Worktree); st != launch.AgentRunning {
			return m.finishAgent(agentStatusMsg(run.ID, run.Worktree).(agentDoneMsg))
		}
		m.screen = screenLog
		m.status = i18n.T("Agent running · ") + run.Branch
		return m, tickLog()
	case store.RunReview, store.RunBlocked, store.RunApproved:
		m.diffFiles, _ = launch.WorktreeDiff(run.Worktree, run.Base)
		m.diffIndex = 0
		m.diffOffset = 0
		m.diffCursor = 0
		m.loadComments()
		m.screen = screenReview
		m.status = m.reviewBar()
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
	_ = m.moveTask(m.run.TaskID, store.StatusDone)
	m.rememberApprove(m.boardProject, m.runTask, m.run)
	m.screen = screenBoard
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusDone)
	m.status = msg
	m.err = ""
	return m, nil
}

func (m *Model) bindRun(t store.Task, run store.Run) {
	m.run = run
	m.runTask = t
	m.runSess = launch.Session{Worktree: run.Worktree, Branch: run.Branch, Base: run.Base, LogPath: run.LogPath, Repo: m.boardProject.RepoPath}
	if root, err := launch.RepoRoot(m.boardProject.RepoPath); err == nil {
		m.runSess.Repo = root
	} else if r, err := launch.RepoFromWorktree(run.Worktree); err == nil {
		m.runSess.Repo = r
	}
}

func (m Model) requestMergeFromTask() (tea.Model, tea.Cmd) {
	t, ok := m.selectedTask()
	if !ok {
		m.status = i18n.T("No task selected")
		m.err = ""
		return m, nil
	}
	run, err := m.store.LatestRun(t.ID)
	if err != nil || run.State != store.RunApproved {
		m.status = i18n.T("This task has no approved run to merge")
		m.err = ""
		return m, nil
	}
	m.bindRun(t, run)
	return m.requestMerge()
}

func (m Model) requestCleanFromTask() (tea.Model, tea.Cmd) {
	t, ok := m.selectedTask()
	if !ok {
		m.status = i18n.T("No task selected")
		m.err = ""
		return m, nil
	}
	run, err := m.store.LatestRun(t.ID)
	if err != nil || run.State != store.RunApproved {
		m.status = i18n.T("This task has no approved run to clean")
		m.err = ""
		return m, nil
	}
	m.bindRun(t, run)
	return m.openCleanPopup()
}

func (m Model) requestClean() (tea.Model, tea.Cmd) {
	if m.run.State != store.RunApproved {
		m.status = i18n.T("This task has no approved run to clean")
		m.err = ""
		return m, nil
	}
	return m.openCleanPopup()
}

func (m Model) mergeBlockReason() string {
	if m.run.State != store.RunApproved || m.runTask.Commits != store.CommitsAuto {
		return i18n.T("Only an approved run without remote can be merged")
	}
	repo := m.runSess.Repo
	if repo == "" {
		repo = m.runSess.Worktree
	}
	if launch.HasRemote(repo) || (m.runSess.Worktree != "" && launch.HasRemote(m.runSess.Worktree)) {
		return i18n.T("Only an approved run without remote can be merged")
	}
	return ""
}

func (m Model) requestMerge() (tea.Model, tea.Cmd) {
	if reason := m.mergeBlockReason(); reason != "" {
		m.status = reason
		m.err = ""
		return m, nil
	}
	return m.openMergePopup()
}

func (m Model) mergeRun() (tea.Model, tea.Cmd) {
	if reason := m.mergeBlockReason(); reason != "" {
		m.status = reason
		m.err = ""
		return m, nil
	}
	kind, err := launch.NormalizeMergeStrategy(m.cfg.Launch.Merge)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	msg, err := launch.MergeLocal(m.runSess, m.runTask, kind)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.run.State = store.RunMerged
	m.run.Note = msg
	_ = m.store.UpdateRun(m.run)
	_ = m.moveTask(m.run.TaskID, store.StatusDone)
	m.rememberMerge(m.boardProject, m.runTask, m.run)
	m.screen = screenBoard
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusDone)
	m.status = msg
	m.err = ""
	return m, nil
}

func (m Model) discardWouldLoseCommits() bool {
	if m.run.State != store.RunApproved {
		return false
	}
	if m.runSess.Repo == "" || m.runSess.Branch == "" || m.runSess.Base == "" {
		return false
	}
	ahead, err := launch.BranchAheadOfBase(m.runSess.Repo, m.runSess.Base, m.runSess.Branch)
	return err == nil && ahead
}

func (m Model) discardRun() (tea.Model, tea.Cmd) {
	if err := launch.Discard(m.runSess); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.run.State = store.RunDiscarded
	_ = m.store.UpdateRun(m.run)
	_ = m.moveTask(m.run.TaskID, store.StatusBacklog)
	m.screen = screenBoard
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusBacklog)
	m.status = i18n.T("Discarded: worktree and branch removed")
	m.err = ""
	return m, nil
}

func (m Model) cleanRun() (tea.Model, tea.Cmd) {
	if m.run.State != store.RunApproved {
		m.status = i18n.T("This task has no approved run to clean")
		m.err = ""
		return m, nil
	}
	if err := launch.CleanWorktree(m.runSess); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.run.State = store.RunCleaned
	m.run.Note = launch.CleanedNote(m.runSess.Branch)
	_ = m.store.UpdateRun(m.run)
	_ = m.moveTask(m.run.TaskID, store.StatusDone)
	m.screen = screenBoard
	m.reloadTasks()
	m.colIndex = indexOfStatus(store.StatusDone)
	m.status = m.run.Note
	m.err = ""
	return m, nil
}

func (m Model) updateLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = screenBoard
		m.status = i18n.T("Agent is still running (it keeps going if you close Troopa)")
		return m, nil
	case "o":
		return m.openEditor(m.runFolder())
	case "[":
		m.moveRound(-1)
		m.logOffset = 0
	case "]", "tab":
		m.moveRound(1)
		m.logOffset = 0
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
	case "g":
		return m.requestMerge()
	case "c":
		return m.requestClean()
	case "d":
		return m.openDiscardPopup()
	case "x":
		return m.openChanges(screenReview)
	case "r":
		m.toggleSelectedRound()
		return m, nil
	case "[":
		m.moveRound(-1)
		return m, nil
	case "]", "tab":
		m.moveRound(1)
		return m, nil
	case "o":
		return m.openEditor(m.runFolder())
	case "enter":
		return m.openCommentOnCursor()
	case "e":
		return m.editCommentOnCursor()
	case "D":
		return m.confirmDeleteCommentOnCursor()
	case "C":
		return m.openCommentList()
	case "n", "l", "right":
		if m.diffIndex < len(m.diffFiles)-1 {
			m.diffIndex++
			m.diffOffset = 0
			m.diffCursor = 0
		}
	case "p", "h", "left":
		if m.diffIndex > 0 {
			m.diffIndex--
			m.diffOffset = 0
			m.diffCursor = 0
		}
	case "j", "down":
		m.moveDiffCursor(1)
	case "k", "up":
		m.moveDiffCursor(-1)
	}
	return m, nil
}

func (m Model) viewLog() string {
	th := m.theme
	raw, _ := os.ReadFile(m.run.LogPath)
	if r, ok := m.selectedRound(); ok {
		raw = roundLogSlice(raw, r)
	}
	lines := strings.Split(string(raw), "\n")
	h := max(4, m.height-7)
	off := m.logOffset
	if off > len(lines) {
		off = max(0, len(lines)-1)
	}
	end := min(len(lines), off+h)
	body := strings.Join(lines[off:end], "\n")
	title := th.Title.Render(i18n.T("Agent")) + th.Muted.Render("  ·  "+m.run.Branch+"  ·  "+m.run.State)
	if r, ok := m.selectedRound(); ok {
		title += th.Muted.Render(fmt.Sprintf("  ·  %s %d/%d", i18n.T("Round"), r.Number, len(m.rounds)))
	}
	return th.App.Render(title + "\n\n" + body + "\n\n" + th.Muted.Render(i18n.T("j/k scroll  [ ] round  o editor  esc board")))
}

func (m Model) viewReview() string {
	th := m.theme
	inner := max(20, m.width-2)
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Review")))
	b.WriteString(th.Muted.Render("  ·  " + m.run.Branch + "  ·  " + m.run.State))
	if n := m.pendingCommentCount(); n == 1 {
		b.WriteString(th.Muted.Render("  ·  " + i18n.T("1 pending comment")))
	} else if n > 1 {
		b.WriteString(th.Muted.Render("  ·  " + i18n.T("%d pending comments", n)))
	}
	b.WriteString("\n\n")
	rounds, _ := m.renderRounds(inner, false)
	b.WriteString(rounds)
	b.WriteString("\n")
	if len(m.diffFiles) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("No diff")))
	} else {
		f := m.diffFiles[m.diffIndex]
		b.WriteString(th.Title.Render(f.Path))
		b.WriteString(th.Muted.Render(fmtIndex(m.diffIndex+1, len(m.diffFiles))))
		b.WriteString("\n")
		used := lipglossHeight(b.String())
		h := max(4, m.height-used-3)
		start := m.diffOffset
		if start > len(f.Lines) {
			start = max(0, len(f.Lines)-1)
		}
		shown := 0
		for i := start; i < len(f.Lines); i++ {
			line := f.Lines[i]
			need := m.diffLineHeight(f.Path, line)
			if shown > 0 && shown+need > h {
				break
			}
			mark := " "
			notes := m.commentsOnLine(f.Path, line)
			if len(notes) > 0 {
				mark = "●"
			}
			num, _ := line.CommentTarget()
			gutter := "    "
			if num > 0 {
				gutter = fmt.Sprintf("%4d", num)
			}
			prefix := mark + gutter + " "
			text := truncate(prefix+line.Text, max(8, m.width-2))
			style := th.Muted
			switch line.Kind {
			case '+':
				style = th.DiffAdd
			case '-':
				style = th.DiffDel
			case '@':
				style = th.DiffHunk
			}
			if i == m.diffCursor {
				b.WriteString(th.Selected.UnsetPadding().Width(max(8, m.width-2)).Render(text))
			} else {
				b.WriteString(style.Render(text))
			}
			b.WriteString("\n")
			shown++
			for _, c := range notes {
				b.WriteString(th.Muted.Render(truncate("    │ "+firstLine(c.Body), max(8, m.width-2))) + "\n")
				shown++
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(m.reviewBar()))
	return th.App.Render(b.String())
}

func reviewHelp(state string) string {
	if state == store.RunApproved {
		return i18n.T("g merge  x request changes  C comments  enter comment  e edit  D delete  c clean  d discard  r fold  [ ] round  n/p file  j/k line  o editor  esc board")
	}
	return i18n.T("y approve  x request changes  C comments  enter comment  e edit  D delete  d discard  r fold  [ ] round  n/p file  j/k line  o editor  esc board")
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

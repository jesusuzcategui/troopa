// Package engine is the shared run and plan lifecycle used by the TUI, a
// daemon and the MCP server.
package engine

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
	"github.com/jesusuzcategui/troopa/internal/termimg"
)

type Engine struct {
	Store      *store.Store
	Cfg        config.Config
	ConfigPath string
}

func New(s *store.Store, cfg config.Config, configPath string) *Engine {
	return &Engine{Store: s, Cfg: cfg, ConfigPath: configPath}
}

// Result is what a lifecycle method did. Watch means the caller should poll
// the detached agent. Pair means the TUI should open a pair session instead.
type Result struct {
	Run     store.Run
	Task    store.Task
	Plan    store.Plan
	Session launch.Session
	Message string
	Watch   bool
	Pair    bool
	Won     bool
}

type ReconcileReport struct {
	Alive []store.Run
}

func (e *Engine) project(id int64) (store.Project, error) {
	return e.Store.Get(id)
}

func (e *Engine) projectProfile(p store.Project) *store.Profile {
	id := p.ProfileID
	if id == 0 {
		id = e.Cfg.DefaultProfile
	}
	if id == 0 {
		return nil
	}
	prof, err := e.Store.GetProfile(id)
	if err != nil {
		return nil
	}
	return &prof
}

func (e *Engine) projectAccount(p store.Project) string {
	if p.Account != "" {
		return p.Account
	}
	return e.Cfg.DefaultAccount
}

func (e *Engine) launchCfg(p store.Project) (config.Launch, error) {
	return e.Cfg.Launch.ForAccount(e.ConfigPath, e.projectAccount(p))
}

func (e *Engine) sessionFor(run store.Run, project store.Project) launch.Session {
	sess := launch.Session{Worktree: run.Worktree, Branch: run.Branch, Base: run.Base, LogPath: run.LogPath, Repo: project.RepoPath}
	if root, err := launch.RepoRoot(project.RepoPath); err == nil {
		sess.Repo = root
	} else if r, err := launch.RepoFromWorktree(run.Worktree); err == nil {
		sess.Repo = r
	}
	return sess
}

func (e *Engine) stageTask(worktree string, t store.Task, project store.Project) error {
	atts, err := e.Store.ListAttachments(t.ID)
	if err != nil {
		return err
	}
	return launch.StageTaskContext(worktree, t, atts, e.Cfg.Launch.Brain, project.Slug)
}

func (e *Engine) moveTask(id int64, status string) error {
	if err := e.Store.MoveTask(id, status); err != nil {
		return err
	}
	t, err := e.Store.GetTask(id)
	if err != nil {
		return nil
	}
	p, err := e.Store.Get(t.ProjectID)
	if err != nil {
		return nil
	}
	_ = brain.OnTask(brain.FromConfig(e.Cfg), p, t)
	return nil
}

func (e *Engine) beginRound(run store.Run, kind, feedback string) error {
	_, err := e.Store.CreateRound(store.Round{
		RunID:    run.ID,
		Kind:     kind,
		Feedback: feedback,
		State:    store.RoundRunning,
		LogFrom:  logSize(run.LogPath),
	})
	return err
}

func (e *Engine) closeOpenRound(run store.Run, state, summary, checksJSON, logPath string, price config.TokenPrice) {
	r, err := e.Store.OpenRound(run.ID)
	if err != nil {
		return
	}
	r.State = state
	r.Summary = summary
	r.Checks = checksJSON
	r.LogTo = sql.NullInt64{Int64: logSize(logPath), Valid: true}
	r.FinishedAt = time.Now().UTC()
	applyRoundMetrics(&r, logPath, price)
	_ = e.Store.UpdateRound(r)
}

func (e *Engine) fireHook(ev launch.HookEvent, p store.Project) {
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
	if len(e.Cfg.Launch.Hooks[ev.Event]) == 0 && len(project[ev.Event]) == 0 {
		return
	}
	launch.RunHooks(e.Cfg.Launch.Hooks, project, ev, hooksLog())
}

func (e *Engine) FireTaskHook(event string, t store.Task) {
	p, err := e.project(t.ProjectID)
	if err != nil {
		return
	}
	e.fireHook(launch.HookEvent{Event: event, TaskID: t.ID, Task: t.Title, Status: t.Status, Agent: t.Agent, PlanID: t.PlanID}, p)
}

func (e *Engine) fireRunHook(event string, t store.Task, run store.Run, msg string, p store.Project) {
	e.fireHook(launch.HookEvent{
		Event: event, TaskID: t.ID, Task: t.Title, Status: t.Status, Agent: run.Agent,
		RunID: run.ID, Branch: run.Branch, Worktree: run.Worktree, PlanID: t.PlanID, Message: msg,
	}, p)
}

func hooksLog() string {
	dataDir, _, _, err := config.Paths()
	if err != nil {
		return ""
	}
	return filepath.Join(dataDir, "hooks.log")
}

func applyRunMetrics(run *store.Run, logPath, agent string, price config.TokenPrice) {
	if run.Agent == "" {
		run.Agent = agent
	}
	if !run.CreatedAt.IsZero() {
		ms := time.Since(run.CreatedAt).Milliseconds()
		if ms < 0 {
			ms = 0
		}
		run.DurationMS = sql.NullInt64{Int64: ms, Valid: true}
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		raw = nil
	}
	u := launch.ParseUsage(string(raw))
	run.TokensIn = nullI64(u.Input)
	run.TokensOut = nullI64(u.Output)
	run.TokensCache = nullI64(u.Cache)
	run.TokensContext = nullI64(u.InitialContext)
	if cost, ok := launch.EstimateCost(u, price); ok {
		run.CostUSD = sql.NullFloat64{Float64: cost, Valid: true}
	}
}

func applyRoundMetrics(r *store.Round, logPath string, price config.TokenPrice) {
	if !r.CreatedAt.IsZero() {
		ms := time.Since(r.CreatedAt).Milliseconds()
		if ms < 0 {
			ms = 0
		}
		r.DurationMS = sql.NullInt64{Int64: ms, Valid: true}
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		raw = nil
	}
	from := r.LogFrom
	to := int64(len(raw))
	if r.LogTo.Valid {
		to = r.LogTo.Int64
	}
	if from < 0 {
		from = 0
	}
	if from > int64(len(raw)) {
		from = int64(len(raw))
	}
	if to > int64(len(raw)) {
		to = int64(len(raw))
	}
	if to < from {
		to = from
	}
	u := launch.ParseUsage(string(raw[from:to]))
	r.TokensIn = nullI64(u.Input)
	r.TokensOut = nullI64(u.Output)
	r.TokensCache = nullI64(u.Cache)
	r.TokensContext = nullI64(u.InitialContext)
	if cost, ok := launch.EstimateCost(u, price); ok {
		r.CostUSD = sql.NullFloat64{Float64: cost, Valid: true}
	}
}

func logSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func nullI64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func errAgentLost() string {
	return i18n.T("The agent stopped before finishing (Troopa, the terminal or the computer closed). Press x to send it back and continue.")
}

func marshalChecks(checks []launch.CheckResult) string {
	raw, err := json.Marshal(checks)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func summaryFrom(sess launch.Session, logPath string) string {
	summary := launch.ReadSummary(sess.Worktree)
	if summary != "" {
		return summary
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		return ""
	}
	summary = strings.TrimSpace(string(raw))
	if len(summary) > 2000 {
		summary = summary[len(summary)-2000:]
	}
	return summary
}

func (e *Engine) result(run store.Run, task store.Task, sess launch.Session, msg string, watch bool) Result {
	return Result{Run: run, Task: task, Session: sess, Message: msg, Watch: watch, Won: true}
}

func (e *Engine) attachRoundImages(taskID int64, paths []string) ([]store.Attachment, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	dir, err := config.AttachmentsDir(taskID)
	if err != nil {
		return nil, err
	}
	var added []store.Attachment
	for _, src := range paths {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		if !termimg.IsImagePath(src) {
			return nil, fmt.Errorf("%s", i18n.T("Not an image: %s", src))
		}
		a, err := e.Store.AttachFile(taskID, src, dir, e.Cfg.MaxAttachmentBytes())
		if err != nil {
			return nil, err
		}
		added = append(added, a)
	}
	return added, nil
}

// Package mcp is Troopa's MCP server: read, write and run tools over stdio.
package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jesusuzcategui/troopa/internal/app"
	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/daemon"
	"github.com/jesusuzcategui/troopa/internal/engine"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxDiffBytes = 64 << 10
const defaultLogLines = 200
const maxLogLines = 2000

type handler struct {
	store     *store.Store
	app       *app.Service
	eng       *engine.Engine
	brainDir  string
	ctx       context.Context
	watchOnce sync.Once
}

type idIn struct {
	ID int64 `json:"id" jsonschema:"positive id"`
}

type tasksListIn struct {
	Project int64  `json:"project,omitempty" jsonschema:"project id"`
	Status  string `json:"status,omitempty" jsonschema:"task status"`
	Plan    int64  `json:"plan,omitempty" jsonschema:"plan id"`
}

type runLogIn struct {
	ID    int64 `json:"id" jsonschema:"run id"`
	Lines int   `json:"lines,omitempty" jsonschema:"trailing log lines"`
}

type brainSearchIn struct {
	Query string `json:"query" jsonschema:"text to find in notes"`
}

type brainReadIn struct {
	Path string `json:"path" jsonschema:"path inside the brain"`
}

type planStepOut struct {
	store.Task
	LatestRun *store.Run `json:"latest_run"`
}

type planOut struct {
	store.Plan
	Done  int           `json:"done"`
	Total int           `json:"total"`
	Steps []planStepOut `json:"steps"`
}

type runDiffFile struct {
	Path  string `json:"path"`
	Patch string `json:"patch"`
}

type runDiffOut struct {
	Files     []runDiffFile `json:"files"`
	Truncated bool          `json:"truncated"`
}

type runLogOut struct {
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}

type noteOut struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// New builds the MCP server with Troopa's read, write and run tools.
func New(e *engine.Engine) *mcpsdk.Server {
	return NewWithContext(context.Background(), e)
}

// NewWithContext is New with a lifetime context for background reconcile.
func NewWithContext(ctx context.Context, e *engine.Engine) *mcpsdk.Server {
	if ctx == nil {
		ctx = context.Background()
	}
	h := &handler{store: e.Store, app: &app.Service{Store: e.Store, Engine: e}, eng: e, brainDir: brain.FromConfig(e.Cfg).Dir, ctx: ctx}
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "troopa", Version: "dev"}, nil)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "projects_list", Description: "Lists projects. Returns the project JSON array."}, h.projectsList)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "project_show", Description: "Shows one project. Returns the project JSON object."}, h.projectShow)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "tasks_list", Description: "Lists tasks. Returns the task JSON array. Filters: project, status, plan."}, h.tasksList)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "task_show", Description: "Shows one task. Returns the task JSON object."}, h.taskShow)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "plans_list", Description: "Lists plans. Returns plan JSON with done and total."}, h.plansList)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "plan_show", Description: "Shows a plan. Returns steps with status and latest run."}, h.planShow)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "run_show", Description: "Shows a run. Returns summary, checks, state, branch and rounds."}, h.runShow)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "run_diff", Description: "Shows a run diff. Returns files and patch text, size-limited."}, h.runDiff)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "run_log", Description: "Shows a run log tail. Returns trailing agent.log text."}, h.runLog)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "brain_search", Description: "Searches the second brain. Returns matching note lines."}, h.brainSearch)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "brain_read", Description: "Reads a note inside the second brain. Returns path and content."}, h.brainRead)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "profiles_list", Description: "Lists profiles. Returns the profile JSON array."}, h.profilesList)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "task_create", Description: "Creates a task. Returns the task JSON object."}, h.taskCreate)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "task_update", Description: "Updates a task. Returns the task JSON object."}, h.taskUpdate)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "task_move", Description: "Moves a task to a status. Returns the task JSON object."}, h.taskMove)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "task_delete", Description: "Deletes a task with its runs, comments and attachments; removes its worktree and branch. Refuses a task with an agent running. Returns id and deleted."}, h.taskDelete)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "plan_create", Description: "Creates a plan and its steps in one call. Returns the plan with steps."}, h.planCreate)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "plan_add_step", Description: "Adds a step to a plan. Returns the task JSON object."}, h.planAddStep)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "plan_update", Description: "Updates a plan. Returns the plan with steps."}, h.planUpdate)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "plan_start", Description: "Starts a plan through the engine. Returns the plan, run if launched, and a message."}, h.planStart)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "plan_pause", Description: "Pauses a running plan through the engine. Returns the plan."}, h.planPause)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "run_wait", Description: "Waits until a run leaves running, or a plan's current step changes. Returns state, summary and checks."}, h.runWait)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "review_request_changes", Description: "Requests changes on a run: new round, CHANGES.md, same agent continues. Same as x in the TUI."}, h.reviewRequestChanges)
	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name: "review_approve", Description: "Approves a run in review. Requires [mcp] allow_approve = true. There is no merge tool."}, h.reviewApprove)
	return srv
}

// Run serves MCP over stdin and stdout until the client disconnects.
func Run(ctx context.Context) error {
	_, configPath, dbPath, err := config.Paths()
	if err != nil {
		return err
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	i18n.Apply(cfg.Language)
	srv := NewWithContext(ctx, engine.New(s, cfg, configPath))
	return srv.Run(ctx, &mcpsdk.StdioTransport{})
}

func (h *handler) rememberTask(t store.Task) {
	p, err := h.store.Get(t.ProjectID)
	if err != nil {
		return
	}
	_ = brain.OnTask(brain.FromConfig(h.eng.Cfg), p, t)
}

func (h *handler) watcherRunning() bool {
	dataDir, _, _, err := config.Paths()
	if err != nil {
		return false
	}
	_, ok := daemon.Running(dataDir)
	return ok
}

func (h *handler) watchFromMCP() {
	h.watchOnce.Do(func() {
		go h.watchLoop()
	})
}

func (h *handler) watchLoop() {
	tick := time.NewTicker(daemon.Interval)
	defer tick.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-tick.C:
			h.tickPlans()
		}
	}
}

func (h *handler) tickPlans() {
	_, _ = h.eng.Reconcile()
	plans, err := h.store.ListPlans(0)
	if err != nil {
		return
	}
	busy := i18n.T("An agent is already running")
	for _, p := range plans {
		if p.State != store.PlanRunning || !(p.Auto || p.AutoApprove) {
			continue
		}
		if _, err := h.eng.AdvancePlan(p.ID); err != nil && err.Error() != busy {
			continue
		}
	}
}

func (h *handler) projectsList(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
	items, err := h.store.ListFiltered(store.ProjectFilter{})
	if err != nil {
		return fail(err)
	}
	return jsonOut(notNil(items))
}

func (h *handler) projectShow(_ context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.ID); err != nil {
		return fail(err)
	}
	p, err := h.store.Get(in.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(errors.New("project not found"))
	}
	if err != nil {
		return fail(err)
	}
	return jsonOut(p)
}

func (h *handler) tasksList(_ context.Context, _ *mcpsdk.CallToolRequest, in tasksListIn) (*mcpsdk.CallToolResult, any, error) {
	items, err := h.store.ListTasks(in.Project, store.TaskFilter{Status: in.Status, PlanID: in.Plan})
	if err != nil {
		return fail(err)
	}
	return jsonOut(notNil(items))
}

func (h *handler) taskShow(_ context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.ID); err != nil {
		return fail(err)
	}
	t, err := h.store.GetTask(in.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(errors.New("task not found"))
	}
	if err != nil {
		return fail(err)
	}
	return jsonOut(t)
}

func (h *handler) plansList(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
	list, err := h.store.ListPlans(0)
	if err != nil {
		return fail(err)
	}
	out := make([]planOut, 0, len(list))
	for _, p := range list {
		d, err := h.planDetail(p, false)
		if err != nil {
			return fail(err)
		}
		out = append(out, d)
	}
	return jsonOut(out)
}

func (h *handler) planShow(_ context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.ID); err != nil {
		return fail(err)
	}
	p, err := h.store.GetPlan(in.ID)
	if err != nil {
		return fail(errors.New("plan not found"))
	}
	d, err := h.planDetail(p, true)
	if err != nil {
		return fail(err)
	}
	return jsonOut(d)
}

func (h *handler) runShow(_ context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	r, err := h.getRun(in.ID)
	if err != nil {
		return fail(err)
	}
	rounds, err := h.store.RoundsForRun(r)
	if err != nil {
		return fail(err)
	}
	out, err := withField(r, "rounds", notNil(rounds))
	if err != nil {
		return fail(err)
	}
	return jsonOut(out)
}

func (h *handler) runDiff(_ context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	r, err := h.getRun(in.ID)
	if err != nil {
		return fail(err)
	}
	files, err := launch.WorktreeDiff(r.Worktree, r.Base)
	if err != nil {
		files = nil
	}
	out := runDiffOut{Files: []runDiffFile{}}
	used := 0
	for _, fd := range files {
		patch := filePatch(fd)
		if used+len(patch) > maxDiffBytes {
			out.Truncated = true
			remain := maxDiffBytes - used
			if remain > 0 {
				out.Files = append(out.Files, runDiffFile{Path: fd.Path, Patch: patch[:remain]})
			}
			break
		}
		out.Files = append(out.Files, runDiffFile{Path: fd.Path, Patch: patch})
		used += len(patch)
	}
	return jsonOut(out)
}

func (h *handler) runLog(_ context.Context, _ *mcpsdk.CallToolRequest, in runLogIn) (*mcpsdk.CallToolResult, any, error) {
	r, err := h.getRun(in.ID)
	if err != nil {
		return fail(err)
	}
	n := in.Lines
	if n <= 0 {
		n = defaultLogLines
	}
	if n > maxLogLines {
		n = maxLogLines
	}
	raw, err := os.ReadFile(r.LogPath)
	if err != nil {
		return jsonOut(runLogOut{})
	}
	text, truncated := tailLines(string(raw), n)
	return jsonOut(runLogOut{Text: text, Truncated: truncated})
}

func (h *handler) brainSearch(_ context.Context, _ *mcpsdk.CallToolRequest, in brainSearchIn) (*mcpsdk.CallToolResult, any, error) {
	hits := brain.Search(h.brainDir, in.Query)
	return jsonOut(notNil(hits))
}

func (h *handler) brainRead(_ context.Context, _ *mcpsdk.CallToolRequest, in brainReadIn) (*mcpsdk.CallToolResult, any, error) {
	if _, ok := brain.Abs(h.brainDir, in.Path); !ok {
		return fail(errors.New("path is outside the brain"))
	}
	body, err := brain.ReadNote(h.brainDir, in.Path)
	if err != nil {
		return fail(errors.New("note not found"))
	}
	return jsonOut(noteOut{Path: in.Path, Content: body})
}

func (h *handler) profilesList(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
	items, err := h.store.ListProfiles()
	if err != nil {
		return fail(err)
	}
	return jsonOut(notNil(items))
}

func (h *handler) getRun(id int64) (store.Run, error) {
	if err := requireID(id); err != nil {
		return store.Run{}, err
	}
	r, err := h.store.GetRun(id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Run{}, errors.New("run not found")
	}
	return r, err
}

func (h *handler) planDetail(p store.Plan, withRuns bool) (planOut, error) {
	steps, err := h.store.PlanSteps(p.ID)
	if err != nil {
		return planOut{}, err
	}
	d := planOut{Plan: p, Total: len(steps), Steps: []planStepOut{}}
	for _, t := range steps {
		if t.Status == store.StatusDone {
			d.Done++
		}
		if !withRuns {
			continue
		}
		step := planStepOut{Task: t}
		if r, err := h.store.LatestRun(t.ID); err == nil {
			step.LatestRun = &r
		}
		d.Steps = append(d.Steps, step)
	}
	return d, nil
}

func requireID(id int64) error {
	if id < 1 {
		return errors.New("id must be a positive integer")
	}
	return nil
}

func jsonOut(v any) (*mcpsdk.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(b)}},
	}, nil, nil
}

func fail(err error) (*mcpsdk.CallToolResult, any, error) {
	return nil, nil, err
}

func notNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func withField(v any, key string, extra any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	m[key] = extra
	return m, nil
}

func filePatch(fd launch.FileDiff) string {
	var b strings.Builder
	for _, ln := range fd.Lines {
		b.WriteString(ln.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

func tailLines(s string, n int) (string, bool) {
	s = strings.TrimSuffix(s, "\n")
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s, false
	}
	return strings.Join(lines[len(lines)-n:], "\n"), true
}

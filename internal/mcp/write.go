package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxWaitSeconds = 600

type taskWriteIn struct {
	ID          int64  `json:"id,omitempty" jsonschema:"task id"`
	Project     int64  `json:"project,omitempty" jsonschema:"project id"`
	Title       string `json:"title,omitempty" jsonschema:"task title"`
	Description string `json:"description,omitempty" jsonschema:"task description"`
	Criteria    string `json:"criteria,omitempty" jsonschema:"acceptance criteria"`
	Status      string `json:"status,omitempty" jsonschema:"task status"`
	Priority    string `json:"priority,omitempty" jsonschema:"task priority"`
	Agent       string `json:"agent,omitempty" jsonschema:"agent name"`
	Model       string `json:"model,omitempty" jsonschema:"model name"`
	Mode        string `json:"mode,omitempty" jsonschema:"yolo or pair"`
	Commits     string `json:"commits,omitempty" jsonschema:"auto or none"`
	Type        string `json:"type,omitempty" jsonschema:"feature, bugfix or chore"`
}

type taskMoveIn struct {
	ID     int64  `json:"id" jsonschema:"task id"`
	Status string `json:"status" jsonschema:"destination status"`
}

type planStepIn struct {
	Title       string `json:"title" jsonschema:"step title"`
	Description string `json:"description,omitempty" jsonschema:"step description"`
	Criteria    string `json:"criteria,omitempty" jsonschema:"acceptance criteria"`
}

type planCreateIn struct {
	Project int64        `json:"project" jsonschema:"project id"`
	Title   string       `json:"title" jsonschema:"plan title"`
	Goal    string       `json:"goal,omitempty" jsonschema:"plan goal"`
	Agent   string       `json:"agent,omitempty" jsonschema:"default agent for steps"`
	Model   string       `json:"model,omitempty" jsonschema:"default model for steps"`
	Auto    *bool        `json:"auto,omitempty" jsonschema:"launch the next step when one is approved"`
	Steps   []planStepIn `json:"steps,omitempty" jsonschema:"steps with title, description and criteria"`
}

type planUpdateIn struct {
	ID    int64  `json:"id" jsonschema:"plan id"`
	Title string `json:"title,omitempty" jsonschema:"plan title"`
	Goal  string `json:"goal,omitempty" jsonschema:"plan goal"`
	Agent string `json:"agent,omitempty" jsonschema:"default agent"`
	Model string `json:"model,omitempty" jsonschema:"default model"`
	Auto  *bool  `json:"auto,omitempty" jsonschema:"auto-advance"`
}

type planAddStepIn struct {
	ID          int64  `json:"id" jsonschema:"plan id"`
	Title       string `json:"title" jsonschema:"step title"`
	Description string `json:"description,omitempty" jsonschema:"step description"`
	Criteria    string `json:"criteria,omitempty" jsonschema:"acceptance criteria"`
	Agent       string `json:"agent,omitempty" jsonschema:"agent name"`
	Model       string `json:"model,omitempty" jsonschema:"model name"`
}

type runWaitIn struct {
	RunID          int64 `json:"run_id,omitempty" jsonschema:"run id"`
	PlanID         int64 `json:"plan_id,omitempty" jsonschema:"plan id"`
	TimeoutSeconds int   `json:"timeout_seconds,omitempty" jsonschema:"seconds to wait, at most 600"`
}

type planActionOut struct {
	planOut
	Run     *store.Run `json:"run,omitempty"`
	Message string     `json:"message,omitempty"`
}

type runWaitOut struct {
	State      string `json:"state"`
	Summary    string `json:"summary"`
	Checks     any    `json:"checks"`
	TimedOut   bool   `json:"timed_out"`
	RunID      int64  `json:"run_id,omitempty"`
	PlanID     int64  `json:"plan_id,omitempty"`
	TaskID     int64  `json:"task_id,omitempty"`
	TaskStatus string `json:"task_status,omitempty"`
}

func (h *handler) taskCreate(_ context.Context, _ *mcpsdk.CallToolRequest, in taskWriteIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.Project); err != nil {
		return fail(errors.New("project must be a positive integer"))
	}
	if _, err := h.store.Get(in.Project); err != nil {
		return fail(errors.New("project not found"))
	}
	t, err := h.store.CreateTask(store.Task{
		ProjectID: in.Project, Title: in.Title, Description: in.Description, AcceptanceCriteria: in.Criteria,
		Status: in.Status, Priority: in.Priority, Agent: in.Agent, Model: in.Model, Mode: in.Mode, Commits: in.Commits, Type: in.Type,
	})
	if err != nil {
		return fail(err)
	}
	h.rememberTask(t)
	h.eng.FireTaskHook(launch.HookTaskCreated, t)
	return jsonOut(t)
}

func (h *handler) taskUpdate(_ context.Context, _ *mcpsdk.CallToolRequest, in taskWriteIn) (*mcpsdk.CallToolResult, any, error) {
	t, err := h.getTask(in.ID)
	if err != nil {
		return fail(err)
	}
	if in.Title != "" {
		t.Title = in.Title
	}
	if in.Description != "" {
		t.Description = in.Description
	}
	if in.Criteria != "" {
		t.AcceptanceCriteria = in.Criteria
	}
	if in.Status != "" {
		t.Status = in.Status
	}
	if in.Priority != "" {
		t.Priority = in.Priority
	}
	if in.Agent != "" {
		t.Agent = in.Agent
	}
	if in.Model != "" {
		t.Model = in.Model
	}
	if in.Mode != "" {
		t.Mode = in.Mode
	}
	if in.Commits != "" {
		t.Commits = in.Commits
	}
	if in.Type != "" {
		t.Type = in.Type
	}
	if err := h.store.UpdateTask(t); err != nil {
		return fail(err)
	}
	t, err = h.store.GetTask(t.ID)
	if err != nil {
		return fail(err)
	}
	h.rememberTask(t)
	return jsonOut(t)
}

func (h *handler) taskMove(_ context.Context, _ *mcpsdk.CallToolRequest, in taskMoveIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.ID); err != nil {
		return fail(err)
	}
	if err := h.store.MoveTask(in.ID, in.Status); err != nil {
		return fail(err)
	}
	t, err := h.store.GetTask(in.ID)
	if err != nil {
		return fail(err)
	}
	h.rememberTask(t)
	return jsonOut(t)
}

func (h *handler) planCreate(_ context.Context, _ *mcpsdk.CallToolRequest, in planCreateIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.Project); err != nil {
		return fail(errors.New("project must be a positive integer"))
	}
	if _, err := h.store.Get(in.Project); err != nil {
		return fail(errors.New("project not found"))
	}
	auto := true
	if in.Auto != nil {
		auto = *in.Auto
	}
	p, err := h.store.CreatePlan(store.Plan{
		ProjectID: in.Project, Title: in.Title, Goal: in.Goal, Agent: in.Agent, Model: in.Model, Auto: auto,
	})
	if err != nil {
		return fail(err)
	}
	for _, step := range in.Steps {
		t, err := h.store.AddPlanStep(p.ID, store.Task{
			Title: step.Title, Description: step.Description, AcceptanceCriteria: step.Criteria,
		})
		if err != nil {
			return fail(err)
		}
		h.rememberTask(t)
		h.eng.FireTaskHook(launch.HookTaskCreated, t)
	}
	d, err := h.planDetail(p, true)
	if err != nil {
		return fail(err)
	}
	return jsonOut(d)
}

func (h *handler) planAddStep(_ context.Context, _ *mcpsdk.CallToolRequest, in planAddStepIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.ID); err != nil {
		return fail(err)
	}
	if _, err := h.store.GetPlan(in.ID); err != nil {
		return fail(errors.New("plan not found"))
	}
	t, err := h.store.AddPlanStep(in.ID, store.Task{
		Title: in.Title, Description: in.Description, AcceptanceCriteria: in.Criteria, Agent: in.Agent, Model: in.Model,
	})
	if err != nil {
		return fail(err)
	}
	h.rememberTask(t)
	h.eng.FireTaskHook(launch.HookTaskCreated, t)
	return jsonOut(t)
}

func (h *handler) planUpdate(_ context.Context, _ *mcpsdk.CallToolRequest, in planUpdateIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.ID); err != nil {
		return fail(err)
	}
	p, err := h.store.GetPlan(in.ID)
	if err != nil {
		return fail(errors.New("plan not found"))
	}
	if in.Title != "" {
		p.Title = in.Title
	}
	if in.Goal != "" {
		p.Goal = in.Goal
	}
	if in.Agent != "" {
		p.Agent = in.Agent
	}
	if in.Model != "" {
		p.Model = in.Model
	}
	if in.Auto != nil {
		p.Auto = *in.Auto
	}
	if err := h.store.UpdatePlan(p); err != nil {
		return fail(err)
	}
	p, err = h.store.GetPlan(in.ID)
	if err != nil {
		return fail(err)
	}
	d, err := h.planDetail(p, true)
	if err != nil {
		return fail(err)
	}
	return jsonOut(d)
}

func (h *handler) planStart(_ context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.ID); err != nil {
		return fail(err)
	}
	res, err := h.eng.StartPlan(in.ID)
	if err != nil {
		return fail(err)
	}
	alone := !h.watcherRunning()
	if alone {
		h.watchFromMCP()
	}
	p, err := h.store.GetPlan(in.ID)
	if err != nil {
		return fail(errors.New("plan not found"))
	}
	d, err := h.planDetail(p, true)
	if err != nil {
		return fail(err)
	}
	out := planActionOut{planOut: d, Message: res.Message}
	if res.Run.ID != 0 {
		out.Run = &res.Run
	}
	if alone {
		hint := i18n.T("`troopa daemon` keeps plans moving")
		if out.Message == "" {
			out.Message = hint
		} else {
			out.Message = out.Message + " " + hint
		}
	}
	return jsonOut(out)
}

func (h *handler) planPause(_ context.Context, _ *mcpsdk.CallToolRequest, in idIn) (*mcpsdk.CallToolResult, any, error) {
	if err := requireID(in.ID); err != nil {
		return fail(err)
	}
	res, err := h.eng.PausePlan(in.ID)
	if err != nil {
		return fail(err)
	}
	d, err := h.planDetail(res.Plan, true)
	if err != nil {
		return fail(err)
	}
	return jsonOut(planActionOut{planOut: d, Message: res.Message})
}

func (h *handler) runWait(ctx context.Context, _ *mcpsdk.CallToolRequest, in runWaitIn) (*mcpsdk.CallToolResult, any, error) {
	if in.RunID < 1 && in.PlanID < 1 {
		return fail(errors.New("run_id or plan_id is required"))
	}
	if in.TimeoutSeconds > maxWaitSeconds {
		return fail(errors.New("timeout_seconds must be at most 600"))
	}
	timeout := in.TimeoutSeconds
	if timeout <= 0 {
		timeout = maxWaitSeconds
	}
	snap, err := h.waitSnapshot(in.RunID, in.PlanID)
	if err != nil {
		return fail(err)
	}
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		_, _ = h.eng.Reconcile()
		cur, err := h.waitSnapshot(in.RunID, in.PlanID)
		if err != nil {
			return fail(err)
		}
		if waitDone(in, snap, cur) {
			return jsonOut(cur.out(false))
		}
		if !time.Now().Before(deadline) {
			return jsonOut(cur.out(true))
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fail(ctx.Err())
		case <-timer.C:
		}
	}
}

type waitSnap struct {
	runID      int64
	planID     int64
	taskID     int64
	runState   string
	taskStatus string
	summary    string
	checks     any
}

func (s waitSnap) out(timedOut bool) runWaitOut {
	return runWaitOut{
		State: s.runState, Summary: s.summary, Checks: s.checks, TimedOut: timedOut,
		RunID: s.runID, PlanID: s.planID, TaskID: s.taskID, TaskStatus: s.taskStatus,
	}
}

func (h *handler) waitSnapshot(runID, planID int64) (waitSnap, error) {
	if runID > 0 {
		r, err := h.getRun(runID)
		if err != nil {
			return waitSnap{}, err
		}
		t, err := h.store.GetTask(r.TaskID)
		if err != nil {
			return waitSnap{}, err
		}
		return waitSnap{
			runID: r.ID, planID: t.PlanID, taskID: t.ID, runState: r.State,
			taskStatus: t.Status, summary: r.Summary, checks: parseChecks(r.Checks),
		}, nil
	}
	if _, err := h.store.GetPlan(planID); err != nil {
		return waitSnap{}, errors.New("plan not found")
	}
	step, ok, err := h.store.NextPlanStep(planID)
	if err != nil {
		return waitSnap{}, err
	}
	s := waitSnap{planID: planID}
	if !ok {
		s.runState = store.PlanDone
		return s, nil
	}
	s.taskID = step.ID
	s.taskStatus = step.Status
	if r, err := h.store.LatestRun(step.ID); err == nil {
		s.runID = r.ID
		s.runState = r.State
		s.summary = r.Summary
		s.checks = parseChecks(r.Checks)
	} else {
		s.runState = step.Status
	}
	return s, nil
}

func waitDone(in runWaitIn, before, after waitSnap) bool {
	if in.RunID > 0 {
		return after.runState != store.RunRunning
	}
	if after.taskID != before.taskID || after.taskStatus != before.taskStatus || after.runState != before.runState || after.runID != before.runID {
		return true
	}
	return false
}

func (h *handler) getTask(id int64) (store.Task, error) {
	if err := requireID(id); err != nil {
		return store.Task{}, err
	}
	t, err := h.store.GetTask(id)
	if err != nil {
		return store.Task{}, errors.New("task not found")
	}
	return t, nil
}

func parseChecks(raw string) any {
	if raw == "" {
		return nil
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) == nil {
		return v
	}
	return raw
}

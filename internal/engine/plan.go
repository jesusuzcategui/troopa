package engine

import (
	"fmt"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func (e *Engine) PausePlan(id int64) (Result, error) {
	plan, err := e.Store.GetPlan(id)
	if err != nil {
		return Result{}, err
	}
	if plan.State != store.PlanRunning {
		return Result{Plan: plan, Won: false}, nil
	}
	plan.State = store.PlanPaused
	won, err := e.Store.UpdatePlanIfState(plan, store.PlanRunning)
	if err != nil {
		return Result{}, err
	}
	if !won {
		fresh, _ := e.Store.GetPlan(id)
		return Result{Plan: fresh, Won: false}, nil
	}
	return Result{Plan: plan, Message: i18n.T("Plan paused. The running step finishes; the next one waits for s."), Won: true}, nil
}

func (e *Engine) StartPlan(id int64) (Result, error) {
	plan, err := e.Store.GetPlan(id)
	if err != nil {
		return Result{}, err
	}
	switch plan.State {
	case store.PlanDone, store.PlanPROpen, store.PlanMerged, store.PlanCancelled:
		return Result{Plan: plan, Message: i18n.T("This plan is finished")}, nil
	}
	project, err := e.project(plan.ProjectID)
	if err != nil {
		return Result{}, err
	}
	if plan.Base == "" {
		repo, err := launch.RepoRoot(project.RepoPath)
		if err != nil {
			return Result{}, err
		}
		base, err := launch.HeadBranch(repo)
		if err != nil {
			return Result{}, err
		}
		plan.Base = base
	}
	wasDraft := plan.State == store.PlanDraft
	from := plan.State
	plan.State = store.PlanRunning
	if _, err := e.Store.UpdatePlanIfState(plan, from); err != nil {
		return Result{}, err
	}
	if wasDraft {
		e.fireHook(launch.HookEvent{Event: launch.HookPlanStart, PlanID: plan.ID, Plan: plan.Title}, project)
	}
	return e.AdvancePlan(plan.ID)
}

func (e *Engine) markPlanStarted(t store.Task, base string, project store.Project) {
	if t.PlanID == 0 {
		return
	}
	plan, err := e.Store.GetPlan(t.PlanID)
	if err != nil || plan.State != store.PlanDraft {
		return
	}
	if plan.Base == "" {
		plan.Base = base
	}
	plan.State = store.PlanRunning
	ok, err := e.Store.UpdatePlanIfState(plan, store.PlanDraft)
	if err == nil && ok {
		e.fireHook(launch.HookEvent{Event: launch.HookPlanStart, PlanID: plan.ID, Plan: plan.Title}, project)
	}
}

func (e *Engine) AdvancePlan(id int64) (Result, error) {
	plan, err := e.Store.GetPlan(id)
	if err != nil {
		return Result{}, err
	}
	if plan.State != store.PlanRunning {
		return Result{Plan: plan, Won: false}, nil
	}
	next, ok, err := e.Store.NextPlanStep(id)
	if err != nil {
		return Result{}, err
	}
	project, err := e.project(plan.ProjectID)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		plan.State = store.PlanDone
		won, err := e.Store.UpdatePlanIfState(plan, store.PlanRunning)
		if err != nil {
			return Result{}, err
		}
		if !won {
			fresh, _ := e.Store.GetPlan(id)
			return Result{Plan: fresh, Won: false}, nil
		}
		e.fireHook(launch.HookEvent{Event: launch.HookPlanDone, PlanID: plan.ID, Plan: plan.Title}, project)
		if e.Cfg.Launch.Notify {
			launch.Notify("Troopa: "+i18n.T("plan done"), plan.Title)
		}
		return Result{Plan: plan, Message: i18n.T("Plan %s is done. Open it with w and press g to merge it.", plan.Title), Won: true}, nil
	}
	switch next.Status {
	case store.StatusInProgress, store.StatusReview, store.StatusBlocked, store.StatusPairPaused:
		return Result{Plan: plan, Task: next, Message: i18n.T("Step %d (%s) is %s: finish it to continue the plan", next.PlanStep, next.Title, next.Status), Won: true}, nil
	}
	if next.Agent == "" || next.Agent == store.AgentMe {
		return Result{Plan: plan, Task: next, Message: i18n.T("Step %d (%s) has no agent: give it one with A, or do it yourself and move it to done", next.PlanStep, next.Title), Won: true}, nil
	}
	if next.Mode != store.ModeYolo {
		next.Mode = store.ModeYolo
		_ = e.Store.UpdateTask(next)
	}
	res, err := e.Launch(next)
	if err != nil {
		return Result{}, err
	}
	res.Plan = plan
	return res, nil
}

func (e *Engine) planStepBase(t store.Task) (string, error) {
	if t.PlanID == 0 {
		return "", nil
	}
	plan, err := e.Store.GetPlan(t.PlanID)
	if err != nil {
		return "", nil
	}
	steps, err := e.Store.PlanSteps(t.PlanID)
	if err != nil {
		return "", err
	}
	base := plan.Base
	for _, s := range steps {
		if s.ID == t.ID {
			break
		}
		run, err := e.Store.LatestRun(s.ID)
		if err != nil || (run.State != store.RunApproved && run.State != store.RunMerged) {
			if s.Status == store.StatusDone {
				continue
			}
			return "", fmt.Errorf("%s", i18n.T("Step %d (%s) is not approved yet", s.PlanStep, s.Title))
		}
		if run.Branch != "" {
			base = run.Branch
		}
	}
	return base, nil
}

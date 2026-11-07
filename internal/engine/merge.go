package engine

import (
	"fmt"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// MergeBlockReason says why an approved run cannot be merged on its own, or
// "" when it can: plan steps merge with their plan, and repos with a remote
// get their PR when the run is approved.
func MergeBlockReason(run store.Run, task store.Task, sess launch.Session) string {
	if task.PlanID != 0 {
		return i18n.T("This task is a plan step: merge the whole plan from its plan (w, then g)")
	}
	if run.State != store.RunApproved || task.Commits != store.CommitsAuto {
		return i18n.T("Only an approved run without remote can be merged")
	}
	repo := sess.Repo
	if repo == "" {
		repo = sess.Worktree
	}
	if launch.HasRemote(repo) || (sess.Worktree != "" && launch.HasRemote(sess.Worktree)) {
		return i18n.T("Only an approved run without remote can be merged")
	}
	return ""
}

// CanMerge reports whether MergeTask would accept the run.
func (e *Engine) CanMerge(runID int64) (string, bool) {
	run, err := e.Store.GetRun(runID)
	if err != nil {
		return err.Error(), false
	}
	task, err := e.Store.GetTask(run.TaskID)
	if err != nil {
		return err.Error(), false
	}
	project, err := e.project(task.ProjectID)
	if err != nil {
		return err.Error(), false
	}
	reason := MergeBlockReason(run, task, e.sessionFor(run, project))
	return reason, reason == ""
}

// MergeTask merges an approved run into the branch it started from.
func (e *Engine) MergeTask(runID int64) (Result, error) {
	run, err := e.Store.GetRun(runID)
	if err != nil {
		return Result{}, err
	}
	task, err := e.Store.GetTask(run.TaskID)
	if err != nil {
		return Result{}, err
	}
	project, err := e.project(task.ProjectID)
	if err != nil {
		return Result{}, err
	}
	sess := e.sessionFor(run, project)
	if reason := MergeBlockReason(run, task, sess); reason != "" {
		return Result{}, fmt.Errorf("%s", reason)
	}
	kind, err := launch.NormalizeMergeStrategy(e.Cfg.Launch.Merge)
	if err != nil {
		return Result{}, err
	}
	msg, err := launch.MergeLocal(sess, task, kind)
	if err != nil {
		return Result{}, err
	}
	run.State = store.RunMerged
	run.Note = msg
	if _, err := e.Store.UpdateRunIfState(run, store.RunApproved); err != nil {
		return Result{}, err
	}
	_ = e.moveTask(run.TaskID, store.StatusDone)
	if fresh, err := e.Store.GetTask(task.ID); err == nil {
		task = fresh
	}
	_ = brain.OnMerge(brain.FromConfig(e.Cfg), project, task, run)
	e.fireRunHook(launch.HookMerge, task, run, msg, project)
	return e.result(run, task, sess, msg, false), nil
}

// PlanLastRun is the approved run of the last finished step: its branch holds
// the whole plan.
func (e *Engine) PlanLastRun(planID int64) (store.Run, store.Task, bool) {
	steps, err := e.Store.PlanSteps(planID)
	if err != nil {
		return store.Run{}, store.Task{}, false
	}
	for i := len(steps) - 1; i >= 0; i-- {
		run, err := e.Store.LatestRun(steps[i].ID)
		if err == nil && run.State == store.RunApproved && run.Branch != "" {
			return run, steps[i], true
		}
	}
	return store.Run{}, store.Task{}, false
}

// MergePlan merges the last branch of a finished plan into the branch the plan
// started from, or pushes it and opens a PR when the repo has a remote. On a
// plan whose PR is open it closes the plan and removes the worktrees.
func (e *Engine) MergePlan(id int64) (Result, error) {
	plan, err := e.Store.GetPlan(id)
	if err != nil {
		return Result{}, err
	}
	project, err := e.project(plan.ProjectID)
	if err != nil {
		return Result{}, err
	}
	repo, err := launch.RepoRoot(project.RepoPath)
	if err != nil {
		return Result{}, err
	}
	if plan.State == store.PlanPROpen {
		e.cleanPlanWorktrees(id, 0, repo)
		plan.State = store.PlanMerged
		_ = e.Store.UpdatePlan(plan)
		return Result{Plan: plan, Won: true, Message: i18n.T("Plan closed: its worktrees and branches were removed. Pull %s to get the merged PR.", plan.Base)}, nil
	}
	if plan.State != store.PlanDone {
		return Result{}, fmt.Errorf("%s", i18n.T("Finish every step before merging the plan"))
	}
	run, _, ok := e.PlanLastRun(id)
	if !ok {
		return Result{}, fmt.Errorf("%s", i18n.T("No approved step to merge"))
	}
	sess := launch.Session{Repo: repo, Worktree: run.Worktree, Branch: run.Branch, Base: plan.Base, LogPath: run.LogPath}
	var msg string
	state := store.PlanMerged
	if launch.HasRemote(repo) {
		state = store.PlanPROpen
		if err := launch.PushBranch(run.Worktree, run.Branch); err != nil {
			return Result{}, err
		}
		body := plan.Goal
		if steps, err := e.Store.PlanSteps(id); err == nil {
			body += "\n\n"
			for _, s := range steps {
				body += fmt.Sprintf("%d. %s\n", s.PlanStep, s.Title)
			}
		}
		url, err := launch.CreatePR(run.Worktree, run.Branch, plan.Base, plan.Title, strings.TrimSpace(body)+"\n\nMade with Troopa.")
		if err != nil {
			msg = i18n.T("Branch %s pushed. ", run.Branch) + err.Error()
		} else {
			msg = i18n.T("PR opened: ") + url
		}
		msg += " · " + i18n.T("merge it on the remote, then press g again to clean up")
	} else {
		kind, err := launch.NormalizeMergeStrategy(e.Cfg.Launch.Merge)
		if err != nil {
			return Result{}, err
		}
		msg, err = launch.MergeLocal(sess, store.Task{Title: plan.Title, Commits: store.CommitsAuto}, kind)
		if err != nil {
			return Result{}, err
		}
		e.cleanPlanWorktrees(id, run.ID, repo)
	}
	plan.State = state
	_ = e.Store.UpdatePlan(plan)
	e.fireHook(launch.HookEvent{Event: launch.HookMerge, PlanID: plan.ID, Plan: plan.Title, Branch: run.Branch, Message: msg}, project)
	return Result{Plan: plan, Run: run, Won: true, Message: msg}, nil
}

// cleanPlanWorktrees removes the worktrees and branches of the steps once the
// plan is merged; the last one was already removed by the merge.
func (e *Engine) cleanPlanWorktrees(planID, mergedRun int64, repo string) {
	steps, err := e.Store.PlanSteps(planID)
	if err != nil {
		return
	}
	for _, s := range steps {
		run, err := e.Store.LatestRun(s.ID)
		if err != nil {
			continue
		}
		if run.ID != mergedRun {
			_ = launch.Discard(launch.Session{Repo: repo, Worktree: run.Worktree, Branch: run.Branch})
		}
		run.State = store.RunMerged
		_ = e.Store.UpdateRun(run)
	}
}

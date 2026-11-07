package engine

import (
	"fmt"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// MergeBlockReason says why an approved run cannot be merged into its base
// branch on this machine, or "" when it can. Plan steps merge with their plan.
// A run approved without committing is committed first.
func MergeBlockReason(run store.Run, task store.Task, sess launch.Session) string {
	if task.PlanID != 0 {
		return i18n.T("This task is a plan step: merge the whole plan from its plan (w, then g)")
	}
	if run.State != store.RunApproved {
		return i18n.T("Only an approved run can be merged")
	}
	if task.Commits != store.CommitsNone && task.Commits != store.CommitsAuto {
		return i18n.T("Only an approved run can be merged")
	}
	return ""
}

// PRBlockReason says why the run cannot become a pull request, or "".
func PRBlockReason(run store.Run, task store.Task, sess launch.Session) string {
	if reason := MergeBlockReason(run, task, sess); reason != "" {
		return reason
	}
	if !HasRemote(sess) {
		return i18n.T("This repo has no remote: merge it locally instead")
	}
	if strings.Contains(run.Note, "http") && strings.Contains(run.Note, i18n.T("PR opened: ")) {
		return i18n.T("The PR is already open: %s", run.Note)
	}
	return ""
}

// MergeOptions says which ways out an approved run has: a local merge and,
// with a remote, a pull request.
func (e *Engine) MergeOptions(runID int64) (merge, pr bool) {
	run, task, sess, err := e.runContext(runID)
	if err != nil {
		return false, false
	}
	return MergeBlockReason(run, task, sess) == "", PRBlockReason(run, task, sess) == ""
}

func (e *Engine) runContext(runID int64) (store.Run, store.Task, launch.Session, error) {
	run, err := e.Store.GetRun(runID)
	if err != nil {
		return store.Run{}, store.Task{}, launch.Session{}, err
	}
	task, err := e.Store.GetTask(run.TaskID)
	if err != nil {
		return store.Run{}, store.Task{}, launch.Session{}, err
	}
	project, err := e.project(task.ProjectID)
	if err != nil {
		return store.Run{}, store.Task{}, launch.Session{}, err
	}
	return run, task, e.sessionFor(run, project), nil
}

// commitIfNeeded commits a run approved without committing, the way an auto
// task does at approval.
func (e *Engine) commitIfNeeded(task *store.Task, sess launch.Session) error {
	if task.Commits != store.CommitsNone {
		return nil
	}
	if err := launch.CommitAll(sess.Worktree, "troopa: "+task.Title); err != nil {
		return err
	}
	task.Commits = store.CommitsAuto
	return e.Store.UpdateTask(*task)
}

// OpenTaskPR pushes the run's branch and opens a pull request to its base.
func (e *Engine) OpenTaskPR(runID int64) (Result, error) {
	run, task, sess, err := e.runContext(runID)
	if err != nil {
		return Result{}, err
	}
	if reason := PRBlockReason(run, task, sess); reason != "" {
		return Result{}, fmt.Errorf("%s", reason)
	}
	if err := e.commitIfNeeded(&task, sess); err != nil {
		return Result{}, err
	}
	if err := launch.PushBranch(sess.Worktree, sess.Branch); err != nil {
		return Result{}, err
	}
	body := run.Summary
	if body == "" {
		body = "Made with Troopa."
	}
	url, err := launch.CreatePR(sess.Worktree, sess.Branch, sess.Base, task.Title, body)
	msg := i18n.T("PR opened: ") + url
	if err != nil {
		msg = i18n.T("Branch %s pushed. ", sess.Branch) + err.Error()
	}
	run.Note = msg
	_ = e.Store.UpdateRun(run)
	return e.result(run, task, sess, msg, false), nil
}

// Pull brings the project's checked-out branch (usually main) up to date with
// its remote, fast-forward only, so the next task starts from fresh code.
func (e *Engine) Pull(projectID int64) (string, error) {
	project, err := e.project(projectID)
	if err != nil {
		return "", err
	}
	repo, err := launch.RepoRoot(project.RepoPath)
	if err != nil {
		return "", err
	}
	branch, err := launch.Git(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	branch = strings.TrimSpace(branch)
	if !launch.HasRemote(repo) {
		return "", fmt.Errorf("%s", i18n.T("%s has no remote to pull from", project.Name))
	}
	before, _ := launch.Git(repo, "rev-parse", "--short", "HEAD")
	if _, err := launch.Git(repo, "pull", "--ff-only"); err != nil {
		return "", fmt.Errorf("%s: %v", i18n.T("Could not update %s (only fast-forward pulls; commit or stash local changes first)", branch), err)
	}
	after, _ := launch.Git(repo, "rev-parse", "--short", "HEAD")
	if strings.TrimSpace(before) == strings.TrimSpace(after) {
		return i18n.T("%s · %s was already up to date", project.Name, branch), nil
	}
	return i18n.T("%s · %s updated: %s → %s", project.Name, branch, strings.TrimSpace(before), strings.TrimSpace(after)), nil
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
	if err := e.commitIfNeeded(&task, sess); err != nil {
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
// started from, on this computer. On a plan whose PR is open (OpenPlanPR) it
// closes the plan and removes the worktrees.
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
	kind, err := launch.NormalizeMergeStrategy(e.Cfg.Launch.Merge)
	if err != nil {
		return Result{}, err
	}
	msg, err := launch.MergeLocal(sess, store.Task{Title: plan.Title, Commits: store.CommitsAuto}, kind)
	if err != nil {
		return Result{}, err
	}
	e.cleanPlanWorktrees(id, run.ID, repo)
	plan.State = store.PlanMerged
	_ = e.Store.UpdatePlan(plan)
	e.fireHook(launch.HookEvent{Event: launch.HookMerge, PlanID: plan.ID, Plan: plan.Title, Branch: run.Branch, Message: msg}, project)
	return Result{Plan: plan, Run: run, Won: true, Message: msg}, nil
}

// OpenPlanPR pushes the last branch of a finished plan and opens a PR to the
// branch the plan started from. Merge it on the remote, then MergePlan cleans up.
func (e *Engine) OpenPlanPR(id int64) (Result, error) {
	plan, err := e.Store.GetPlan(id)
	if err != nil {
		return Result{}, err
	}
	if plan.State != store.PlanDone {
		return Result{}, fmt.Errorf("%s", i18n.T("Finish every step before merging the plan"))
	}
	project, err := e.project(plan.ProjectID)
	if err != nil {
		return Result{}, err
	}
	repo, err := launch.RepoRoot(project.RepoPath)
	if err != nil {
		return Result{}, err
	}
	if !launch.HasRemote(repo) {
		return Result{}, fmt.Errorf("%s", i18n.T("This repo has no remote: merge it locally instead"))
	}
	run, _, ok := e.PlanLastRun(id)
	if !ok {
		return Result{}, fmt.Errorf("%s", i18n.T("No approved step to merge"))
	}
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
	var msg string
	url, err := launch.CreatePR(run.Worktree, run.Branch, plan.Base, plan.Title, strings.TrimSpace(body)+"\n\nMade with Troopa.")
	if err != nil {
		msg = i18n.T("Branch %s pushed. ", run.Branch) + err.Error()
	} else {
		msg = i18n.T("PR opened: ") + url
	}
	msg += " · " + i18n.T("merge it on the remote, then press g again to clean up")
	plan.State = store.PlanPROpen
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

// HasRemote reports whether the run's repo pushes somewhere, so merging
// becomes a PR.
func HasRemote(sess launch.Session) bool {
	repo := sess.Repo
	if repo == "" {
		repo = sess.Worktree
	}
	return launch.HasRemote(repo) || (sess.Worktree != "" && launch.HasRemote(sess.Worktree))
}

// MergePreview says what merging (or opening the PR of) the run will do.
func (e *Engine) MergePreview(runID int64, pr bool) string {
	run, task, sess, err := e.runContext(runID)
	if err != nil {
		return ""
	}
	var text string
	if pr {
		text = i18n.T("Push %s and open a PR from #%d “%s” to %s?", sess.Branch, task.ID, task.Title, run.Base)
	} else {
		text = i18n.T("Merge #%d “%s” into %s on this computer?", task.ID, task.Title, run.Base)
	}
	if task.Commits == store.CommitsNone {
		text = i18n.T("It was approved without committing: Troopa commits it first.") + " " + text
	}
	return text
}

package engine

import (
	"fmt"
	"os"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func (e *Engine) Launch(task store.Task) (Result, error) {
	if _, err := e.Store.ActiveRun(); err == nil {
		return Result{}, fmt.Errorf("%s", i18n.T("An agent is already running"))
	}
	if e.Cfg.Launch.Agents == nil {
		return Result{}, fmt.Errorf("%s", i18n.T("No agents in config.toml"))
	}
	project, err := e.project(task.ProjectID)
	if err != nil {
		return Result{}, err
	}
	lc, err := e.launchCfg(project)
	if err != nil {
		return Result{}, err
	}
	base, err := e.planStepBase(task)
	if err != nil {
		return Result{}, err
	}
	sess, err := launch.PrepareWith(lc, project, task, launch.PrepareOpts{Base: base, Profile: e.projectProfile(project)})
	if err != nil {
		return Result{}, err
	}
	e.markPlanStarted(task, sess.Base, project)
	if err := e.stageTask(sess.Worktree, task, project); err != nil {
		_ = launch.Discard(sess)
		return Result{}, err
	}
	cmd, err := launch.AgentCommand(lc, task.Agent, task.Model, sess.Worktree)
	if err != nil {
		_ = launch.Discard(sess)
		return Result{}, err
	}
	run, err := e.Store.CreateRun(store.Run{
		TaskID:    task.ID,
		ProjectID: task.ProjectID,
		Worktree:  sess.Worktree,
		Branch:    sess.Branch,
		Base:      sess.Base,
		LogPath:   sess.LogPath,
		State:     store.RunRunning,
		Agent:     task.Agent,
	})
	if err != nil {
		_ = launch.Discard(sess)
		return Result{}, err
	}
	if err := e.beginRound(run, store.RoundLaunch, ""); err != nil {
		return Result{}, err
	}
	if err := e.moveTask(task.ID, store.StatusInProgress); err != nil {
		return Result{}, err
	}
	if err := launch.StartAgent(cmd, run.Worktree, run.LogPath); err != nil {
		return e.Finish(run.ID, err)
	}
	e.fireRunHook(launch.HookLaunch, task, run, "", project)
	return e.result(run, task, sess, i18n.T("Agent ")+task.Agent+" · "+sess.Branch, true), nil
}

func (e *Engine) Finish(runID int64, exitErr error) (Result, error) {
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
		project = store.Project{RepoPath: run.Worktree}
	}
	sess := e.sessionFor(run, project)
	if run.State != store.RunRunning {
		return Result{Run: run, Task: task, Session: sess, Won: false}, nil
	}
	summary := summaryFrom(sess, run.LogPath)
	repo := sess.Repo
	if repo == "" {
		repo = project.RepoPath
	}
	checks := launch.RunChecks(sess.Worktree, launch.LoadProjectChecks(repo))
	checksJSON := marshalChecks(checks)
	run.Summary = summary
	run.Checks = checksJSON
	ok := exitErr == nil && launch.ChecksOK(checks)
	dest := store.StatusReview
	run.State = store.RunReview
	if !ok {
		dest = store.StatusBlocked
		run.State = store.RunBlocked
		if exitErr != nil {
			run.Note = exitErr.Error()
		}
	}
	price := e.Cfg.Launch.PriceFor(task.Agent)
	applyRunMetrics(&run, run.LogPath, task.Agent, price)
	won, err := e.Store.UpdateRunIfState(run, store.RunRunning)
	if err != nil {
		return Result{}, err
	}
	if !won {
		fresh, err := e.Store.GetRun(runID)
		if err != nil {
			return Result{}, err
		}
		return Result{Run: fresh, Task: task, Session: sess, Won: false}, nil
	}
	e.closeOpenRound(run, run.State, summary, checksJSON, run.LogPath, price)
	_ = e.moveTask(run.TaskID, dest)
	if dest == store.StatusReview {
		e.fireRunHook(launch.HookReview, task, run, summary, project)
	} else {
		e.fireRunHook(launch.HookBlocked, task, run, run.Note, project)
	}
	if dest == store.StatusReview && task.PlanID != 0 {
		if plan, err := e.Store.GetPlan(task.PlanID); err == nil && plan.State == store.PlanRunning && plan.AutoApprove {
			return e.Approve(run.ID)
		}
	}
	if e.Cfg.Launch.Notify {
		launch.Notify("Troopa: "+dest, task.Title)
	}
	return e.result(run, task, sess, dest, false), nil
}

func (e *Engine) Approve(runID int64) (Result, error) {
	run, err := e.Store.GetRun(runID)
	if err != nil {
		return Result{}, err
	}
	if run.State != store.RunReview {
		return Result{}, fmt.Errorf("%s", i18n.T("Only a run in review can be approved"))
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
	var msg string
	if task.PlanID != 0 {
		err = launch.CommitAll(sess.Worktree, "troopa: "+task.Title)
		msg = i18n.T("Step approved and committed on %s", sess.Branch)
	} else {
		msg, err = launch.Approve(sess, task, run.Summary)
	}
	if err != nil {
		return Result{}, err
	}
	run.State = store.RunApproved
	run.Note = msg
	won, err := e.Store.UpdateRunIfState(run, store.RunReview)
	if err != nil {
		return Result{}, err
	}
	if !won {
		fresh, err := e.Store.GetRun(runID)
		if err != nil {
			return Result{}, err
		}
		return Result{Run: fresh, Task: task, Session: sess, Won: false}, nil
	}
	_ = e.moveTask(run.TaskID, store.StatusDone)
	if fresh, err := e.Store.GetTask(task.ID); err == nil {
		task = fresh
	}
	_ = brain.OnApprove(brain.FromConfig(e.Cfg), project, task, run)
	e.fireRunHook(launch.HookApprove, task, run, msg, project)
	res := e.result(run, task, sess, msg, false)
	if task.PlanID != 0 {
		if plan, err := e.Store.GetPlan(task.PlanID); err == nil && plan.State == store.PlanRunning && (plan.Auto || plan.AutoApprove) {
			return e.afterPlanAdvance(run, task, sess, msg, plan.ID)
		} else if err == nil && plan.State == store.PlanRunning {
			if _, more, _ := e.Store.NextPlanStep(plan.ID); !more {
				return e.afterPlanAdvance(run, task, sess, msg, plan.ID)
			}
			res.Message = msg + " · " + i18n.T("open the plan with w and press s for the next step")
			res.Plan = plan
		}
	}
	return res, nil
}

func (e *Engine) afterPlanAdvance(run store.Run, task store.Task, sess launch.Session, msg string, planID int64) (Result, error) {
	adv, err := e.AdvancePlan(planID)
	if err != nil {
		return Result{}, err
	}
	if adv.Watch {
		return adv, nil
	}
	if adv.Run.ID == 0 {
		adv.Run = run
		adv.Task = task
		adv.Session = sess
	}
	if adv.Message == "" {
		adv.Message = msg
	}
	adv.Won = true
	return adv, nil
}

func (e *Engine) Discard(runID int64) (Result, error) {
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
	from := run.State
	if err := launch.Discard(sess); err != nil {
		return Result{}, err
	}
	run.State = store.RunDiscarded
	won, err := e.Store.UpdateRunIfState(run, from)
	if err != nil {
		return Result{}, err
	}
	if !won {
		fresh, err := e.Store.GetRun(runID)
		if err != nil {
			return Result{}, err
		}
		return Result{Run: fresh, Task: task, Session: sess, Won: false}, nil
	}
	e.fireRunHook(launch.HookDiscard, task, run, "", project)
	_ = e.moveTask(run.TaskID, store.StatusBacklog)
	return e.result(run, task, sess, i18n.T("Discarded: worktree and branch removed"), false), nil
}

func (e *Engine) RequestChanges(runID int64, feedback string, comments []store.ReviewComment) (Result, error) {
	run, err := e.Store.GetRun(runID)
	if err != nil {
		return Result{}, err
	}
	switch run.State {
	case store.RunReview, store.RunBlocked, store.RunApproved:
	default:
		return Result{}, fmt.Errorf("%s", i18n.T("This task has no run to send back to its agent"))
	}
	task, err := e.Store.GetTask(run.TaskID)
	if err != nil {
		return Result{}, err
	}
	if task.Agent == store.AgentMe || task.Agent == "" {
		return Result{}, fmt.Errorf("%s", i18n.T("This task has no agent: make the changes yourself with o"))
	}
	project, err := e.project(task.ProjectID)
	if err != nil {
		return Result{}, err
	}
	sess := e.sessionFor(run, project)
	if _, err := os.Stat(sess.Worktree); err != nil {
		return Result{}, fmt.Errorf("%s", i18n.T("The worktree is gone: discard the run and launch the task again"))
	}
	body := launch.FormatCommentFeedback(feedback, comments)
	if body == "" {
		return Result{}, fmt.Errorf("%s", i18n.T("Write what should change"))
	}
	if _, err := e.Store.ActiveRun(); err == nil {
		return Result{}, fmt.Errorf("%s", i18n.T("An agent is already running"))
	}
	if !launch.AgentInstalled(e.Cfg.Launch, task.Agent) {
		return Result{}, fmt.Errorf("%s", i18n.T("%s is not installed. Install it or pick another agent with A.", launch.AgentName(task.Agent)))
	}
	round, err := launch.RequestChanges(sess.Worktree, body)
	if err != nil {
		return Result{}, err
	}
	if err := e.stageTask(sess.Worktree, task, project); err != nil {
		return Result{}, err
	}
	note := i18n.T("Changes requested (round %d)", round)
	if err := e.beginRound(run, store.RoundChanges, strings.TrimSpace(feedback)); err != nil {
		return Result{}, err
	}
	if r, err := e.Store.OpenRound(run.ID); err == nil {
		_ = e.Store.ResolvePendingComments(run.ID, r.Number)
	}
	from := run.State
	if task.Mode == store.ModePair {
		run.Note = note
		if _, err := e.Store.UpdateRunIfState(run, from); err != nil {
			return Result{}, err
		}
		res := e.result(run, task, sess, note, false)
		res.Pair = true
		return res, nil
	}
	lc, err := e.launchCfg(project)
	if err != nil {
		return Result{}, err
	}
	cmd, err := launch.AgentCommandPrompt(lc, task.Agent, task.Model, sess.Worktree, launch.ChangesPrompt(sess.Worktree, round))
	if err != nil {
		return Result{}, err
	}
	if f, err := os.OpenFile(run.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintf(f, "\n\n── Troopa · %s ──\n%s\n\n", note, body)
		f.Close()
	}
	run.State = store.RunRunning
	run.Note = note
	if _, err := e.Store.UpdateRunIfState(run, from); err != nil {
		return Result{}, err
	}
	if err := e.moveTask(task.ID, store.StatusInProgress); err != nil {
		return Result{}, err
	}
	if err := launch.StartAgent(cmd, run.Worktree, run.LogPath); err != nil {
		return e.Finish(run.ID, err)
	}
	return e.result(run, task, sess, note+" · "+launch.AgentName(task.Agent), true), nil
}

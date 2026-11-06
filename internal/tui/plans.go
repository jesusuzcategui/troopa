package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// A plan is an ordered list of tasks of one project. Each step is a normal
// task with its own run and review; it starts from the branch of the step
// before it, and when a step is approved the next one starts by itself. The
// last branch holds the whole plan and is merged (or opened as a PR) once.

type planView int

const (
	planList planView = iota
	planDetail
	planForm
	planAddStep
)

type plansState struct {
	view    planView
	list    []store.Plan
	cursor  int
	plan    store.Plan
	steps   []store.Task
	stepCur int
	focus   int // plan form: 0 title, 1 goal, 2 agent, 3 model, 4 auto
	title   textinput.Model
	goal    textarea.Model
	model   textinput.Model
	step    textinput.Model
	draft   store.Plan
	confirm int64
}

const planFormFields = 5

func (m *Model) openPlans() {
	ps := &plansState{}
	ps.title = textinput.New()
	ps.title.Placeholder = i18n.T("Plan title")
	ps.title.CharLimit = 200
	ps.title.Width = 56
	ps.title.Prompt = ""
	ps.goal = textarea.New()
	ps.goal.Placeholder = i18n.T("Goal of the plan: what is done when every step is done")
	ps.goal.SetHeight(4)
	ps.goal.SetWidth(64)
	ps.goal.ShowLineNumbers = false
	ps.goal.Prompt = ""
	ps.model = textinput.New()
	ps.model.Placeholder = i18n.T("model (optional)")
	ps.model.CharLimit = 80
	ps.model.Width = 32
	ps.model.Prompt = ""
	ps.step = textinput.New()
	ps.step.Placeholder = i18n.T("Step title")
	ps.step.CharLimit = 200
	ps.step.Width = 56
	ps.step.Prompt = ""
	m.plans = ps
	m.reloadPlans()
	m.screen = screenPlans
	m.status = planListHelp()
}

func planListHelp() string {
	return i18n.T("n new plan · enter open · d delete · esc board")
}

func planDetailHelp() string {
	return i18n.T("a add step · enter edit step · K/J move · s start · p pause · t auto · e edit · g merge · esc plans")
}

func (m *Model) reloadPlans() {
	list, err := m.store.ListPlans(m.boardProject.ID)
	if err != nil {
		m.err = err.Error()
		return
	}
	m.plans.list = list
	if m.plans.cursor >= len(list) {
		m.plans.cursor = max(0, len(list)-1)
	}
}

func (m *Model) loadPlan(id int64) {
	ps := m.plans
	p, err := m.store.GetPlan(id)
	if err != nil {
		m.err = err.Error()
		return
	}
	ps.plan = p
	ps.steps, _ = m.store.PlanSteps(id)
	if ps.stepCur >= len(ps.steps) {
		ps.stepCur = max(0, len(ps.steps)-1)
	}
}

func (m *Model) editPlan(p store.Plan) tea.Cmd {
	ps := m.plans
	if p.ID == 0 {
		p.ProjectID = m.boardProject.ID
		p.Agent = m.cfg.DefaultAgent
		if p.Agent == "" || p.Agent == store.AgentMe {
			p.Agent = store.AgentClaude
		}
		p.Model = m.cfg.DefaultModel
		p.Auto = true
	}
	ps.draft = p
	ps.view = planForm
	ps.focus = 0
	ps.title.SetValue(p.Title)
	ps.goal.SetValue(p.Goal)
	ps.model.SetValue(p.Model)
	m.status = i18n.T("tab field · ←/→ agent and auto · ctrl+s save · esc cancel")
	return m.focusPlanField()
}

func (m *Model) focusPlanField() tea.Cmd {
	ps := m.plans
	ps.title.Blur()
	ps.goal.Blur()
	ps.model.Blur()
	switch ps.focus {
	case 0:
		return ps.title.Focus()
	case 1:
		return ps.goal.Focus()
	case 3:
		return ps.model.Focus()
	}
	return nil
}

func planAgents() []string {
	return []string{store.AgentClaude, store.AgentCodex, store.AgentOpenCode, store.AgentPi}
}

func (m Model) updatePlansKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ps := m.plans
	key := msg.String()
	switch ps.view {
	case planForm:
		return m.updatePlanFormKey(msg)
	case planAddStep:
		switch key {
		case "esc":
			ps.view = planDetail
			m.status = planDetailHelp()
			return m, nil
		case "enter":
			title := strings.TrimSpace(ps.step.Value())
			if title == "" {
				ps.view = planDetail
				return m, nil
			}
			t, err := m.store.AddPlanStep(ps.plan.ID, store.Task{Title: title, Priority: store.PriorityMedium, Commits: store.CommitsNone, Type: store.TypeFeature})
			if err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.fireTaskHook(launch.HookTaskCreated, t)
			ps.step.SetValue("")
			m.loadPlan(ps.plan.ID)
			ps.stepCur = len(ps.steps) - 1
			m.status = i18n.T("Step added. Type the next one or press esc.")
			return m, nil
		}
		var cmd tea.Cmd
		ps.step, cmd = ps.step.Update(msg)
		return m, cmd
	}
	if key != "d" {
		ps.confirm = 0
	}
	if ps.view == planList {
		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.plans = nil
			m.screen = screenBoard
			m.reloadTasks()
			m.status = ""
		case "j", "down":
			if ps.cursor < len(ps.list)-1 {
				ps.cursor++
			}
		case "k", "up":
			if ps.cursor > 0 {
				ps.cursor--
			}
		case "n", "a":
			return m, m.editPlan(store.Plan{})
		case "enter":
			if len(ps.list) == 0 {
				return m, m.editPlan(store.Plan{})
			}
			ps.view = planDetail
			ps.stepCur = 0
			m.loadPlan(ps.list[ps.cursor].ID)
			m.status = planDetailHelp()
		case "d":
			if len(ps.list) == 0 {
				return m, nil
			}
			p := ps.list[ps.cursor]
			if ps.confirm != p.ID {
				ps.confirm = p.ID
				m.status = i18n.T("Press d again to delete the plan %s (its tasks stay on the board)", p.Title)
				return m, nil
			}
			ps.confirm = 0
			if err := m.store.DeletePlan(p.ID); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.reloadPlans()
			m.status = i18n.T("Deleted: ") + p.Title
		}
		return m, nil
	}
	// planDetail
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		ps.view = planList
		m.reloadPlans()
		m.status = planListHelp()
	case "j", "down":
		if ps.stepCur < len(ps.steps)-1 {
			ps.stepCur++
		}
	case "k", "up":
		if ps.stepCur > 0 {
			ps.stepCur--
		}
	case "a", "n":
		ps.view = planAddStep
		ps.step.SetValue("")
		m.status = i18n.T("enter add · esc done")
		return m, ps.step.Focus()
	case "J", "K":
		if len(ps.steps) == 0 {
			return m, nil
		}
		dir := 1
		if key == "K" {
			dir = -1
		}
		if err := m.store.MovePlanStep(ps.steps[ps.stepCur].ID, dir); err != nil {
			m.err = err.Error()
			return m, nil
		}
		ps.stepCur = min(max(0, ps.stepCur+dir), len(ps.steps)-1)
		m.loadPlan(ps.plan.ID)
	case "enter":
		if len(ps.steps) == 0 {
			return m, nil
		}
		t := ps.steps[ps.stepCur]
		m.plans = nil
		if run, err := m.store.LatestRun(t.ID); err == nil && run.State != store.RunDiscarded {
			return m.openRunView(t)
		}
		m.openTaskForm(t)
		return m.withTaskFocus()
	case "e":
		return m, m.editPlan(ps.plan)
	case "t":
		ps.plan.Auto = !ps.plan.Auto
		_ = m.store.UpdatePlan(ps.plan)
		m.status = i18n.T("Auto-advance: ") + onOff(ps.plan.Auto)
	case "p":
		if ps.plan.State == store.PlanRunning {
			ps.plan.State = store.PlanPaused
			_ = m.store.UpdatePlan(ps.plan)
			m.status = i18n.T("Plan paused. The running step finishes; the next one waits for s.")
		}
	case "s":
		return m.startPlan(ps.plan.ID)
	case "g":
		return m.mergePlan(ps.plan.ID)
	}
	return m, nil
}

func onOff(b bool) string {
	if b {
		return i18n.T("on")
	}
	return i18n.T("off")
}

func (m Model) updatePlanFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ps := m.plans
	key := msg.String()
	switch key {
	case "esc":
		if ps.plan.ID != 0 {
			ps.view = planDetail
			m.status = planDetailHelp()
		} else {
			ps.view = planList
			m.status = planListHelp()
		}
		return m, nil
	case "tab", "down":
		if key == "down" && ps.focus == 1 {
			break
		}
		ps.focus = (ps.focus + 1) % planFormFields
		return m, m.focusPlanField()
	case "shift+tab", "up":
		if key == "up" && ps.focus == 1 {
			break
		}
		ps.focus = (ps.focus - 1 + planFormFields) % planFormFields
		return m, m.focusPlanField()
	case "ctrl+s":
		return m.savePlan()
	case "enter":
		if ps.focus != 1 {
			return m.savePlan()
		}
	case "left", "right", " ":
		dir := 1
		if key == "left" {
			dir = -1
		}
		switch ps.focus {
		case 2:
			ps.draft.Agent = cycleValue(ps.draft.Agent, planAgents(), dir)
			return m, nil
		case 4:
			ps.draft.Auto = !ps.draft.Auto
			return m, nil
		}
	}
	var cmd tea.Cmd
	switch ps.focus {
	case 0:
		ps.title, cmd = ps.title.Update(msg)
	case 1:
		ps.goal, cmd = ps.goal.Update(msg)
	case 3:
		ps.model, cmd = ps.model.Update(msg)
	}
	return m, cmd
}

func (m Model) savePlan() (tea.Model, tea.Cmd) {
	ps := m.plans
	p := ps.draft
	p.Title = ps.title.Value()
	p.Goal = ps.goal.Value()
	p.Model = strings.TrimSpace(ps.model.Value())
	var err error
	if p.ID == 0 {
		p, err = m.store.CreatePlan(p)
	} else {
		err = m.store.UpdatePlan(p)
	}
	if err != nil {
		m.err = i18n.T(err.Error())
		return m, nil
	}
	m.err = ""
	ps.view = planDetail
	m.loadPlan(p.ID)
	m.reloadPlans()
	m.status = planDetailHelp()
	if len(ps.steps) == 0 {
		ps.view = planAddStep
		ps.step.SetValue("")
		m.status = i18n.T("Add the steps in order. enter add · esc done")
		return m, ps.step.Focus()
	}
	return m, nil
}

// startPlan starts (or resumes) a plan: it launches the first step that is
// not done.
func (m Model) startPlan(id int64) (tea.Model, tea.Cmd) {
	plan, err := m.store.GetPlan(id)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	switch plan.State {
	case store.PlanDone, store.PlanMerged, store.PlanCancelled:
		m.status = i18n.T("This plan is finished")
		return m, nil
	}
	project := m.taskProjectByID(plan.ProjectID)
	if plan.Base == "" {
		repo, err := launch.RepoRoot(project.RepoPath)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		base, err := launch.HeadBranch(repo)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		plan.Base = base
	}
	wasDraft := plan.State == store.PlanDraft
	plan.State = store.PlanRunning
	if err := m.store.UpdatePlan(plan); err != nil {
		m.err = err.Error()
		return m, nil
	}
	if wasDraft {
		m.fireHook(launch.HookEvent{Event: launch.HookPlanStart, PlanID: plan.ID, Plan: plan.Title}, project)
	}
	return m.advancePlan(plan.ID)
}

// markPlanStarted turns a draft plan into a running one when one of its
// steps is launched from the board, so approving it moves the plan on.
func (m Model) markPlanStarted(t store.Task, base string, project store.Project) {
	if t.PlanID == 0 {
		return
	}
	plan, err := m.store.GetPlan(t.PlanID)
	if err != nil || plan.State != store.PlanDraft {
		return
	}
	if plan.Base == "" {
		plan.Base = base
	}
	plan.State = store.PlanRunning
	if m.store.UpdatePlan(plan) == nil {
		m.fireHook(launch.HookEvent{Event: launch.HookPlanStart, PlanID: plan.ID, Plan: plan.Title}, project)
	}
}

// advancePlan launches the next step of a running plan, or marks it done.
func (m Model) advancePlan(id int64) (tea.Model, tea.Cmd) {
	plan, err := m.store.GetPlan(id)
	if err != nil || plan.State != store.PlanRunning {
		return m, nil
	}
	next, ok, err := m.store.NextPlanStep(id)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	project := m.taskProjectByID(plan.ProjectID)
	if !ok {
		plan.State = store.PlanDone
		_ = m.store.UpdatePlan(plan)
		m.fireHook(launch.HookEvent{Event: launch.HookPlanDone, PlanID: plan.ID, Plan: plan.Title}, project)
		m.status = i18n.T("Plan %s is done. Open it with w and press g to merge it.", plan.Title)
		if m.cfg.Launch.Notify {
			launch.Notify("Troopa: "+i18n.T("plan done"), plan.Title)
		}
		if m.plans != nil {
			m.loadPlan(plan.ID)
		}
		return m, nil
	}
	switch next.Status {
	case store.StatusInProgress, store.StatusReview, store.StatusBlocked, store.StatusPairPaused:
		m.status = i18n.T("Step %d (%s) is %s: finish it to continue the plan", next.PlanStep, next.Title, next.Status)
		return m, nil
	}
	if next.Agent == "" || next.Agent == store.AgentMe {
		m.status = i18n.T("Step %d (%s) has no agent: give it one with A, or do it yourself and move it to done", next.PlanStep, next.Title)
		return m, nil
	}
	if next.Mode != store.ModeYolo {
		next.Mode = store.ModeYolo
		_ = m.store.UpdateTask(next)
	}
	if m.boardProject.ID != project.ID {
		m.openBoard(project)
	}
	m.plans = nil
	return m.launchAgent(next)
}

// planStepBase is the branch a plan step starts from: the approved branch of
// the step before it, or the branch the plan started from.
func (m Model) planStepBase(t store.Task) (string, error) {
	if t.PlanID == 0 {
		return "", nil
	}
	plan, err := m.store.GetPlan(t.PlanID)
	if err != nil {
		return "", nil
	}
	steps, err := m.store.PlanSteps(t.PlanID)
	if err != nil {
		return "", err
	}
	base := plan.Base
	for _, s := range steps {
		if s.ID == t.ID {
			break
		}
		run, err := m.store.LatestRun(s.ID)
		if err != nil || (run.State != store.RunApproved && run.State != store.RunMerged) {
			if s.Status == store.StatusDone {
				continue // done by hand, nothing to build on
			}
			return "", fmt.Errorf("%s", i18n.T("Step %d (%s) is not approved yet", s.PlanStep, s.Title))
		}
		if run.Branch != "" {
			base = run.Branch
		}
	}
	return base, nil
}

// planLastBranch is the branch that holds the whole plan, and its run.
func (m Model) planLastRun(planID int64) (store.Run, store.Task, bool) {
	steps, err := m.store.PlanSteps(planID)
	if err != nil {
		return store.Run{}, store.Task{}, false
	}
	for i := len(steps) - 1; i >= 0; i-- {
		run, err := m.store.LatestRun(steps[i].ID)
		if err == nil && run.State == store.RunApproved && run.Branch != "" {
			return run, steps[i], true
		}
	}
	return store.Run{}, store.Task{}, false
}

// mergePlan merges the last branch of a finished plan into the branch the plan
// started from, or pushes it and opens a PR when the repo has a remote.
func (m Model) mergePlan(id int64) (tea.Model, tea.Cmd) {
	plan, err := m.store.GetPlan(id)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	if plan.State != store.PlanDone {
		m.status = i18n.T("Finish every step before merging the plan")
		return m, nil
	}
	run, _, ok := m.planLastRun(id)
	if !ok {
		m.status = i18n.T("No approved step to merge")
		return m, nil
	}
	project := m.taskProjectByID(plan.ProjectID)
	repo, err := launch.RepoRoot(project.RepoPath)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	sess := launch.Session{Repo: repo, Worktree: run.Worktree, Branch: run.Branch, Base: plan.Base, LogPath: run.LogPath}
	var msg string
	if launch.HasRemote(repo) {
		if err := launch.PushBranch(run.Worktree, run.Branch); err != nil {
			m.err = err.Error()
			return m, nil
		}
		body := plan.Goal
		if steps, err := m.store.PlanSteps(id); err == nil {
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
	} else {
		kind, err := launch.NormalizeMergeStrategy(m.cfg.Launch.Merge)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		msg, err = launch.MergeLocal(sess, store.Task{Title: plan.Title, Commits: store.CommitsAuto}, kind)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.cleanPlanWorktrees(id, run.ID, repo)
	}
	plan.State = store.PlanMerged
	_ = m.store.UpdatePlan(plan)
	m.fireHook(launch.HookEvent{Event: launch.HookMerge, PlanID: plan.ID, Plan: plan.Title, Branch: run.Branch, Message: msg}, project)
	if m.plans != nil {
		m.loadPlan(id)
	}
	m.err = ""
	m.status = msg
	return m, nil
}

// cleanPlanWorktrees removes the worktrees and branches of the steps once the
// plan is merged; the last one was already removed by the merge.
func (m Model) cleanPlanWorktrees(planID, mergedRun int64, repo string) {
	steps, err := m.store.PlanSteps(planID)
	if err != nil {
		return
	}
	for _, s := range steps {
		run, err := m.store.LatestRun(s.ID)
		if err != nil {
			continue
		}
		if run.ID != mergedRun {
			_ = launch.Discard(launch.Session{Repo: repo, Worktree: run.Worktree, Branch: run.Branch})
		}
		run.State = store.RunMerged
		_ = m.store.UpdateRun(run)
	}
}

func (m Model) viewPlans() string {
	th := m.theme
	ps := m.plans
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Plans")) + th.Muted.Render("  ·  "+m.boardProject.Name) + "\n\n")
	switch ps.view {
	case planForm:
		title := i18n.T("New plan")
		if ps.draft.ID != 0 {
			title = i18n.T("Edit plan")
		}
		b.WriteString(th.Label.Render(title) + "\n\n")
		rows := []struct{ label, view string }{
			{i18n.T("Title"), ps.title.View()},
			{i18n.T("Goal"), "\n" + ps.goal.View()},
			{i18n.T("Agent"), "‹ " + launch.AgentName(ps.draft.Agent) + " ›"},
			{i18n.T("Model"), ps.model.View()},
			{i18n.T("Auto-advance"), "‹ " + onOff(ps.draft.Auto) + " ›  " + th.Muted.Render(i18n.T("launch the next step when one is approved"))},
		}
		for i, r := range rows {
			marker := "  "
			if i == ps.focus {
				marker = th.Border.Render("> ")
			}
			b.WriteString(marker + th.Label.Render(r.label) + " " + r.view + "\n")
		}
		b.WriteString("\n" + th.Muted.Render(i18n.T("tab field · ←/→ agent and auto · ctrl+s save · esc cancel")))
		return th.App.Render(b.String())
	case planList:
		if len(ps.list) == 0 {
			b.WriteString(th.Muted.Render(i18n.T("No plans yet. A plan runs its steps one after the other: press n.")) + "\n")
		}
		for i, p := range ps.list {
			done, total, _ := m.store.PlanProgress(p.ID)
			line := fmt.Sprintf("%-10s %3d/%-3d %s", planStateLabel(p.State), done, total, p.Title)
			line = truncate(line, max(10, m.width-4))
			if i == ps.cursor {
				b.WriteString(th.Selected.Render(line) + "\n")
			} else {
				b.WriteString(th.Normal.Render(line) + "\n")
			}
		}
		b.WriteString("\n" + th.Muted.Render(planListHelp()))
		return th.App.Render(b.String())
	}
	p := ps.plan
	done, total, _ := m.store.PlanProgress(p.ID)
	b.WriteString(th.Label.Render(p.Title) + th.Muted.Render(fmt.Sprintf("  ·  %s  ·  %d/%d  ·  %s %s  ·  auto %s",
		planStateLabel(p.State), done, total, launch.AgentName(p.Agent), p.Model, onOff(p.Auto))) + "\n")
	if p.Goal != "" {
		b.WriteString(th.Muted.Render(truncate(firstLine(p.Goal), max(10, m.width-4))) + "\n")
	}
	if p.Base != "" {
		b.WriteString(th.Muted.Render(i18n.T("Base branch: ")+p.Base) + "\n")
	}
	b.WriteString("\n")
	if len(ps.steps) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("No steps yet: press a.")) + "\n")
	}
	for i, s := range ps.steps {
		mark := "○"
		switch s.Status {
		case store.StatusDone:
			mark = "●"
		case store.StatusInProgress:
			mark = "◐"
		case store.StatusReview:
			mark = "◑"
		case store.StatusBlocked:
			mark = "✗"
		}
		line := fmt.Sprintf(" %s %2d. %-40s %-12s %s", mark, s.PlanStep, truncate(s.Title, 40), s.Status, launch.AgentName(s.Agent))
		line = truncate(line, max(10, m.width-4))
		if i == ps.stepCur {
			b.WriteString(th.Selected.Render(line) + "\n")
		} else {
			b.WriteString(th.Normal.Render(line) + "\n")
		}
	}
	if ps.view == planAddStep {
		b.WriteString("\n" + th.Border.Render("+ ") + ps.step.View() + "\n")
	}
	b.WriteString("\n" + th.Muted.Render(planDetailHelp()))
	return th.App.Render(b.String())
}

func planStateLabel(state string) string {
	switch state {
	case store.PlanDraft:
		return i18n.T("draft")
	case store.PlanRunning:
		return i18n.T("running")
	case store.PlanPaused:
		return i18n.T("paused")
	case store.PlanDone:
		return i18n.T("done")
	case store.PlanMerged:
		return i18n.T("merged")
	case store.PlanCancelled:
		return i18n.T("cancelled")
	}
	return state
}

// taskProject is the project of a task (usually the open board).
func (m Model) taskProject(t store.Task) store.Project {
	return m.taskProjectByID(t.ProjectID)
}

func (m Model) taskProjectByID(id int64) store.Project {
	if id == 0 || m.boardProject.ID == id {
		return m.boardProject
	}
	if p, err := m.store.Get(id); err == nil {
		return p
	}
	return m.boardProject
}

// Package app contains the application operations shared by Troopa's user
// interfaces. It deliberately knows nothing about terminal widgets or MCP.
package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/engine"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
	"github.com/jesusuzcategui/troopa/internal/theme"
)

// DiffLine and FileDiff are the serializable diff shape consumed by desktop UI.
type DiffLine struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Old  int    `json:"old"`
	New  int    `json:"new"`
}

type FileDiff struct {
	Path  string     `json:"path"`
	Lines []DiffLine `json:"lines"`
}

// ProjectSummary is the compact project status used by project lists.
type ProjectSummary struct {
	Project      store.Project  `json:"project"`
	Tasks        map[string]int `json:"tasks"`
	ActivePlans  int            `json:"active_plans"`
	LastActivity time.Time      `json:"last_activity"`
}

// Service coordinates store changes and their shared side effects.
type Service struct {
	Store  *store.Store
	Engine *engine.Engine
}

func New(s *store.Store, cfg config.Config, configPath string) *Service {
	return &Service{Store: s, Engine: engine.New(s, cfg, configPath)}
}

func (a *Service) rememberProject(p store.Project) {
	projects, err := a.Store.List(true)
	if err == nil {
		_ = brain.OnProject(brain.FromConfig(a.Engine.Cfg), projects, p)
	}
}

func (a *Service) rememberTask(t store.Task) {
	p, err := a.Store.Get(t.ProjectID)
	if err == nil {
		_ = brain.OnTask(brain.FromConfig(a.Engine.Cfg), p, t)
	}
}

func (a *Service) ListProjects(includeArchived bool) ([]store.Project, error) {
	return a.Store.List(includeArchived)
}

// ListProjectSummaries returns each project with the task, plan, and activity
// information needed to show a useful project list without UI-side aggregation.
func (a *Service) ListProjectSummaries(includeArchived bool) ([]ProjectSummary, error) {
	projects, err := a.Store.List(includeArchived)
	if err != nil {
		return nil, err
	}
	runs, err := a.Store.ListRuns("")
	if err != nil {
		return nil, err
	}
	lastRunActivity := make(map[int64]time.Time)
	for _, run := range runs {
		if run.UpdatedAt.After(lastRunActivity[run.ProjectID]) {
			lastRunActivity[run.ProjectID] = run.UpdatedAt
		}
	}
	out := make([]ProjectSummary, 0, len(projects))
	for _, project := range projects {
		summary := ProjectSummary{
			Project:      project,
			Tasks:        make(map[string]int),
			LastActivity: project.UpdatedAt,
		}
		tasks, err := a.Store.ListTasks(project.ID, store.TaskFilter{})
		if err != nil {
			return nil, err
		}
		for _, task := range tasks {
			summary.Tasks[task.Status]++
			if task.UpdatedAt.After(summary.LastActivity) {
				summary.LastActivity = task.UpdatedAt
			}
		}
		plans, err := a.Store.ListPlans(project.ID)
		if err != nil {
			return nil, err
		}
		for _, plan := range plans {
			if plan.State != store.PlanMerged && plan.State != store.PlanCancelled {
				summary.ActivePlans++
			}
			if plan.UpdatedAt.After(summary.LastActivity) {
				summary.LastActivity = plan.UpdatedAt
			}
		}
		if lastRunActivity[project.ID].After(summary.LastActivity) {
			summary.LastActivity = lastRunActivity[project.ID]
		}
		out = append(out, summary)
	}
	return out, nil
}

func (a *Service) GetProject(id int64) (store.Project, error) {
	return a.Store.Get(id)
}

func (a *Service) CreateProject(p store.Project) (store.Project, error) {
	p, err := a.Store.Create(p)
	if err == nil {
		a.rememberProject(p)
	}
	return p, err
}

func (a *Service) UpdateProject(p store.Project) error {
	if err := a.Store.Update(p); err != nil {
		return err
	}
	a.rememberProject(p)
	return nil
}

func (a *Service) ArchiveProject(id int64) error {
	if err := a.Store.Archive(id); err != nil {
		return err
	}
	if p, err := a.Store.Get(id); err == nil {
		a.rememberProject(p)
	}
	return nil
}

func (a *Service) PathExists(path string) bool {
	p := store.NormalizeRepoPath(path)
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

func (a *Service) InitRepo(path, name string) error {
	return gitinit.InitRepo(store.NormalizeRepoPath(path), name)
}

func (a *Service) ListAccounts() []string {
	return a.Engine.Cfg.Launch.AccountNames()
}

func (a *Service) themesDir() string {
	if a.Engine.ConfigPath == "" {
		return ""
	}
	return config.ThemesDir(a.Engine.ConfigPath)
}

func (a *Service) ListThemes() []theme.Palette {
	return theme.Load(a.themesDir())
}

func (a *Service) SetTheme(name string) error {
	if a.Engine.ConfigPath == "" {
		return fmt.Errorf("config path is empty")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "dark"
	}
	chosen := theme.ByName(a.ListThemes(), name)
	cfg := a.Engine.Cfg
	if _, err := os.Stat(a.Engine.ConfigPath); err == nil {
		loaded, err := config.Load(a.Engine.ConfigPath)
		if err != nil {
			return err
		}
		cfg = loaded
	}
	cfg.Theme = chosen.Name
	if err := config.Save(a.Engine.ConfigPath, cfg); err != nil {
		return err
	}
	a.Engine.Cfg = cfg
	return nil
}

func (a *Service) ListClients(includeArchived bool) ([]store.Client, error) {
	return a.Store.ListClients(includeArchived)
}

func (a *Service) ListProfiles() ([]store.Profile, error) {
	return a.Store.ListProfiles()
}

func (a *Service) ListTasks(projectID int64, query string) ([]store.Task, error) {
	return a.Store.ListTasks(projectID, store.TaskFilter{Query: query})
}

func (a *Service) GetTask(id int64) (store.Task, error) {
	return a.Store.GetTask(id)
}

func (a *Service) CreateTask(t store.Task) (store.Task, error) {
	t, err := a.Store.CreateTask(t)
	if err == nil {
		a.rememberTask(t)
	}
	return t, err
}

func (a *Service) UpdateTask(t store.Task) error {
	if err := a.Store.UpdateTask(t); err != nil {
		return err
	}
	a.rememberTask(t)
	return nil
}

func (a *Service) MoveTask(id int64, status string) error {
	if err := a.Store.MoveTask(id, status); err != nil {
		return err
	}
	if t, err := a.Store.GetTask(id); err == nil {
		a.rememberTask(t)
	}
	return nil
}

func (a *Service) DeleteTask(id int64) error { return a.Engine.DeleteTask(id) }

// LaunchTask starts the task's agent (moves it to in-progress). Call it when the
// user confirms a move to ready with an agent other than "me".
func (a *Service) LaunchTask(id int64) error {
	t, err := a.Store.GetTask(id)
	if err != nil {
		return err
	}
	_, err = a.Engine.Launch(t)
	return err
}

func (a *Service) GetRun(id int64) (store.Run, error) { return a.Store.GetRun(id) }

func (a *Service) LatestRun(taskID int64) (store.Run, error) { return a.Store.LatestRun(taskID) }

func (a *Service) ListRunRounds(runID int64) ([]store.Round, error) {
	run, err := a.Store.GetRun(runID)
	if err != nil {
		return nil, err
	}
	return a.Store.RoundsForRun(run)
}

func (a *Service) ListRunComments(runID int64) ([]store.ReviewComment, error) {
	return a.Store.ListComments(runID)
}

func (a *Service) RunDiff(runID int64) ([]FileDiff, error) {
	run, err := a.Store.GetRun(runID)
	if err != nil {
		return nil, err
	}
	if run.Worktree == "" {
		return []FileDiff{}, nil
	}
	if _, err := os.Stat(run.Worktree); os.IsNotExist(err) {
		return []FileDiff{}, nil
	} else if err != nil {
		return nil, err
	}
	items, err := launch.WorktreeDiff(run.Worktree, run.Base)
	if err != nil {
		return nil, err
	}
	out := make([]FileDiff, len(items))
	for i, item := range items {
		out[i].Path = item.Path
		out[i].Lines = make([]DiffLine, len(item.Lines))
		for j, line := range item.Lines {
			out[i].Lines[j] = DiffLine{Kind: string(line.Kind), Text: line.Text, Old: line.Old, New: line.New}
		}
	}
	return out, nil
}

// RunLog returns the current log contents. A cleaned worktree has no log.
func (a *Service) RunLog(runID int64) (string, error) {
	run, err := a.Store.GetRun(runID)
	if err != nil {
		return "", err
	}
	if run.LogPath == "" {
		return "", nil
	}
	raw, err := os.ReadFile(run.LogPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(raw), err
}

func (a *Service) ApproveRun(runID int64) error {
	_, err := a.Engine.Approve(runID)
	return err
}

func (a *Service) RequestRunChanges(runID int64, feedback string) error {
	comments, err := a.Store.ListPendingComments(runID)
	if err != nil {
		return err
	}
	_, err = a.Engine.RequestChanges(runID, feedback, comments)
	return err
}

func (a *Service) MergeRun(runID int64) error {
	_, err := a.Engine.MergeTask(runID)
	return err
}

func (a *Service) OpenRunPR(runID int64) error {
	_, err := a.Engine.OpenTaskPR(runID)
	return err
}

func (a *Service) CleanRun(runID int64) error {
	_, err := a.Engine.Clean(runID)
	return err
}

func (a *Service) DiscardRun(runID int64) error {
	_, err := a.Engine.Discard(runID)
	return err
}

func (a *Service) OpenRunEditor(runID int64) error {
	run, err := a.Store.GetRun(runID)
	if err != nil {
		return err
	}
	cmd, err := launch.EditorCommand(a.Engine.Cfg.Launch.Editor, run.Worktree)
	if err != nil {
		return err
	}
	return cmd.Start()
}

// ChangeStamp mirrors store.ChangeStamp so the desktop UI can poll the same
// signal the TUI uses (about every 2s) and reload without a full restart.
func (a *Service) ChangeStamp() string {
	return a.Store.ChangeStamp()
}

func (a *Service) ListAttachments(taskID int64) ([]store.Attachment, error) {
	return a.Store.ListAttachments(taskID)
}

// AttachTaskFile copies src into Troopa's attachments dir for the task.
func (a *Service) AttachTaskFile(taskID int64, src string) (store.Attachment, error) {
	dir, err := config.AttachmentsDir(taskID)
	if err != nil {
		return store.Attachment{}, err
	}
	return a.Store.AttachFile(taskID, src, dir, a.Engine.Cfg.MaxAttachmentBytes())
}

func (a *Service) CreatePlan(p store.Plan) (store.Plan, error) { return a.Store.CreatePlan(p) }
func (a *Service) UpdatePlan(p store.Plan) error               { return a.Store.UpdatePlan(p) }
func (a *Service) DeletePlan(id int64) error                   { return a.Store.DeletePlan(id) }
func (a *Service) ListPlans(projectID int64) ([]store.Plan, error) {
	return a.Store.ListPlans(projectID)
}
func (a *Service) PlanSteps(planID int64) ([]store.Task, error) { return a.Store.PlanSteps(planID) }
func (a *Service) MovePlanStep(id int64, direction int) error {
	return a.Store.MovePlanStep(id, direction)
}

func (a *Service) AddPlanStep(planID int64, t store.Task) (store.Task, error) {
	t, err := a.Store.AddPlanStep(planID, t)
	if err == nil {
		a.rememberTask(t)
	}
	return t, err
}

func (a *Service) StartPlan(id int64) error {
	_, err := a.Engine.StartPlan(id)
	return err
}

func (a *Service) AdvancePlan(id int64) error {
	_, err := a.Engine.AdvancePlan(id)
	return err
}

func (a *Service) PausePlan(id int64) error {
	_, err := a.Engine.PausePlan(id)
	return err
}

func (a *Service) MergePlan(id int64) error {
	_, err := a.Engine.MergePlan(id)
	return err
}

func (a *Service) OpenPlanPR(id int64) error {
	_, err := a.Engine.OpenPlanPR(id)
	return err
}

func (a *Service) CreateClient(c store.Client) (store.Client, error) { return a.Store.CreateClient(c) }
func (a *Service) UpdateClient(c store.Client) error                 { return a.Store.UpdateClient(c) }
func (a *Service) ArchiveClient(id int64) error                      { return a.Store.ArchiveClient(id) }
func (a *Service) CreateProfile(p store.Profile) (store.Profile, error) {
	return a.Store.CreateProfile(p)
}
func (a *Service) UpdateProfile(p store.Profile) error { return a.Store.UpdateProfile(p) }
func (a *Service) DeleteProfile(id int64) error        { return a.Store.DeleteProfile(id) }
func (a *Service) CreateComment(c store.ReviewComment) (store.ReviewComment, error) {
	return a.Store.CreateComment(c)
}
func (a *Service) UpdateComment(c store.ReviewComment) error { return a.Store.UpdateComment(c) }
func (a *Service) DeleteComment(id int64) error              { return a.Store.DeleteComment(id) }
func (a *Service) AttachFile(taskID int64, src, dir string, limit int64) (store.Attachment, error) {
	return a.Store.AttachFile(taskID, src, dir, limit)
}
func (a *Service) DeleteAttachment(id int64) error { return a.Store.DeleteAttachment(id) }

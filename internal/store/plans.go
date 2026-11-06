package store

import (
	"fmt"
	"strings"
	"time"
)

const (
	PlanDraft     = "draft"     // being written; nothing runs
	PlanRunning   = "running"   // steps run one after the other
	PlanPaused    = "paused"    // stopped by the person; resume continues
	PlanDone      = "done"      // every step approved; ready to merge
	PlanMerged    = "merged"    // merged into the base branch
	PlanCancelled = "cancelled" // stopped for good
)

var PlanStates = []string{PlanDraft, PlanRunning, PlanPaused, PlanDone, PlanMerged, PlanCancelled}

// Plan is an ordered list of tasks of one project. Each step starts from the
// branch of the step before it, so the last branch holds the whole plan.
type Plan struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Title     string    `json:"title"`
	Goal      string    `json:"goal"`
	Agent     string    `json:"agent"` // default agent for new steps
	Model     string    `json:"model"`
	State     string    `json:"state"`
	Auto      bool      `json:"auto"` // launch the next step when one is approved
	Base      string    `json:"base"` // branch the plan started from
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const planCols = `id, project_id, title, goal, agent, model, state, auto, base, created_at, updated_at`

func (s *Store) ListPlans(projectID int64) ([]Plan, error) {
	q := `SELECT ` + planCols + ` FROM plans`
	args := []any{}
	if projectID != 0 {
		q += ` WHERE project_id = ?`
		args = append(args, projectID)
	}
	q += ` ORDER BY CASE state WHEN 'running' THEN 0 WHEN 'paused' THEN 1 WHEN 'draft' THEN 2 WHEN 'done' THEN 3 ELSE 4 END, id DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Plan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPlan(id int64) (Plan, error) {
	return scanPlan(s.db.QueryRow(`SELECT `+planCols+` FROM plans WHERE id = ?`, id))
}

func normalizePlan(p Plan) (Plan, error) {
	p.Title = strings.TrimSpace(p.Title)
	p.Goal = strings.TrimSpace(p.Goal)
	p.Agent = strings.TrimSpace(p.Agent)
	p.Model = strings.TrimSpace(p.Model)
	if p.State == "" {
		p.State = PlanDraft
	}
	if p.Title == "" {
		return p, fmt.Errorf("title is required")
	}
	if p.ProjectID == 0 {
		return p, fmt.Errorf("project is required")
	}
	if !oneOf(p.State, PlanStates) {
		return p, fmt.Errorf("invalid plan state: %s", p.State)
	}
	return p, nil
}

func (s *Store) CreatePlan(p Plan) (Plan, error) {
	p, err := normalizePlan(p)
	if err != nil {
		return Plan{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`INSERT INTO plans (project_id, title, goal, agent, model, state, auto, base, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProjectID, p.Title, p.Goal, p.Agent, p.Model, p.State, boolInt(p.Auto), p.Base, now, now)
	if err != nil {
		return Plan{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Plan{}, err
	}
	return s.GetPlan(id)
}

func (s *Store) UpdatePlan(p Plan) error {
	p, err := normalizePlan(p)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.Exec(`UPDATE plans SET title = ?, goal = ?, agent = ?, model = ?, state = ?, auto = ?, base = ?, updated_at = ? WHERE id = ?`,
		p.Title, p.Goal, p.Agent, p.Model, p.State, boolInt(p.Auto), p.Base, now, p.ID)
	return err
}

// DeletePlan removes the plan; its steps stay as ordinary tasks.
func (s *Store) DeletePlan(id int64) error {
	if _, err := s.db.Exec(`UPDATE tasks SET plan_id = 0, plan_step = 0 WHERE plan_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM plans WHERE id = ?`, id)
	return err
}

// PlanSteps returns the tasks of a plan in order.
func (s *Store) PlanSteps(planID int64) ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, project_id, title, description, acceptance_criteria, status, priority,
		agent, model, mode, commits, type, position, plan_id, plan_step, created_at, updated_at
		FROM tasks WHERE plan_id = ? ORDER BY plan_step, id`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AddPlanStep adds a task at the end of the plan.
func (s *Store) AddPlanStep(planID int64, t Task) (Task, error) {
	p, err := s.GetPlan(planID)
	if err != nil {
		return Task{}, err
	}
	var last int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(plan_step), 0) FROM tasks WHERE plan_id = ?`, planID).Scan(&last); err != nil {
		return Task{}, err
	}
	t.ProjectID = p.ProjectID
	t.PlanID = planID
	t.PlanStep = last + 1
	if t.Agent == "" {
		t.Agent = p.Agent
	}
	if t.Model == "" {
		t.Model = p.Model
	}
	if t.Mode == "" {
		t.Mode = ModeYolo
	}
	if t.Status == "" {
		t.Status = StatusBacklog
	}
	return s.CreateTask(t)
}

// MovePlanStep swaps a step with its neighbour (dir -1 up, +1 down).
func (s *Store) MovePlanStep(taskID int64, dir int) error {
	t, err := s.GetTask(taskID)
	if err != nil || t.PlanID == 0 {
		return fmt.Errorf("task is not in a plan")
	}
	steps, err := s.PlanSteps(t.PlanID)
	if err != nil {
		return err
	}
	for i, st := range steps {
		if st.ID != taskID {
			continue
		}
		j := i + dir
		if j < 0 || j >= len(steps) {
			return nil
		}
		steps[i], steps[j] = steps[j], steps[i]
		break
	}
	for i, st := range steps {
		if _, err := s.db.Exec(`UPDATE tasks SET plan_step = ? WHERE id = ?`, i+1, st.ID); err != nil {
			return err
		}
	}
	return nil
}

// NextPlanStep is the first step that is not done, or false when all are.
func (s *Store) NextPlanStep(planID int64) (Task, bool, error) {
	steps, err := s.PlanSteps(planID)
	if err != nil {
		return Task{}, false, err
	}
	for _, t := range steps {
		if t.Status != StatusDone {
			return t, true, nil
		}
	}
	return Task{}, false, nil
}

// PlanProgress counts done steps.
func (s *Store) PlanProgress(planID int64) (done, total int, err error) {
	err = s.db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN status = 'done' THEN 1 ELSE 0 END), 0), COUNT(*) FROM tasks WHERE plan_id = ?`, planID).Scan(&done, &total)
	return
}

func scanPlan(row scanner) (Plan, error) {
	var p Plan
	var auto int
	var created, updated string
	if err := row.Scan(&p.ID, &p.ProjectID, &p.Title, &p.Goal, &p.Agent, &p.Model, &p.State, &auto, &p.Base, &created, &updated); err != nil {
		return Plan{}, err
	}
	p.Auto = auto != 0
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return p, nil
}

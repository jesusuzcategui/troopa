package store

import (
	"fmt"
	"strings"
	"time"
)

const (
	StatusInbox      = "inbox"
	StatusBacklog    = "backlog"
	StatusReady      = "ready"
	StatusInProgress = "in-progress"
	StatusPairPaused = "pair-paused"
	StatusReview     = "review"
	StatusBlocked    = "blocked"
	StatusDone       = "done"

	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"

	AgentClaude   = "claude"
	AgentCodex    = "codex"
	AgentOpenCode = "opencode"
	AgentPi       = "pi"
	AgentMe       = "me"

	ModeYolo = "yolo"
	ModePair = "pair"

	CommitsAuto = "auto"
	CommitsNone = "none"

	TypeFeature = "feature"
	TypeBugfix  = "bugfix"
	TypeChore   = "chore"
)

var Statuses = []string{
	StatusInbox, StatusBacklog, StatusReady, StatusInProgress, StatusPairPaused, StatusReview, StatusBlocked, StatusDone,
}

var Priorities = []string{PriorityLow, PriorityMedium, PriorityHigh}
var Agents = []string{AgentClaude, AgentCodex, AgentOpenCode, AgentPi, AgentMe}
var Modes = []string{ModeYolo, ModePair}
var CommitModes = []string{CommitsAuto, CommitsNone}
var Types = []string{TypeFeature, TypeBugfix, TypeChore}

type Task struct {
	ID                 int64     `json:"id"`
	ProjectID          int64     `json:"project_id"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	AcceptanceCriteria string    `json:"acceptance_criteria"`
	Status             string    `json:"status"`
	Priority           string    `json:"priority"`
	Agent              string    `json:"agent"`
	Model              string    `json:"model"`
	Mode               string    `json:"mode"`
	Commits            string    `json:"commits"`
	Type               string    `json:"type"`
	Position           int       `json:"position"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type TaskFilter struct {
	Query    string
	Status   string
	Priority string
	ClientID int64
}

func (s *Store) ListTasks(projectID int64, f TaskFilter) ([]Task, error) {
	q := `SELECT id, project_id, title, description, acceptance_criteria, status, priority,
		agent, model, mode, commits, type, position, created_at, updated_at
		FROM tasks WHERE 1=1`
	args := []any{}
	if projectID != 0 {
		q += ` AND project_id = ?`
		args = append(args, projectID)
	}
	if f.ClientID != 0 {
		q += ` AND project_id IN (SELECT id FROM projects WHERE client_id = ?)`
		args = append(args, f.ClientID)
	}
	if st := strings.TrimSpace(f.Status); st != "" {
		if !oneOf(st, Statuses) {
			return nil, fmt.Errorf("invalid status: %s", st)
		}
		q += ` AND status = ?`
		args = append(args, st)
	}
	if pr := strings.TrimSpace(f.Priority); pr != "" {
		if !oneOf(pr, Priorities) {
			return nil, fmt.Errorf("invalid priority: %s", pr)
		}
		q += ` AND priority = ?`
		args = append(args, pr)
	}
	if query := strings.TrimSpace(f.Query); query != "" {
		like := "%" + query + "%"
		q += ` AND (title LIKE ? OR description LIKE ?)`
		args = append(args, like, like)
	}
	q += ` ORDER BY CASE status
		WHEN 'inbox' THEN 0 WHEN 'backlog' THEN 1 WHEN 'ready' THEN 2
		WHEN 'in-progress' THEN 3 WHEN 'pair-paused' THEN 4 WHEN 'review' THEN 5
		WHEN 'blocked' THEN 6 WHEN 'done' THEN 7 ELSE 8 END, position ASC, id ASC`
	rows, err := s.db.Query(q, args...)
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

func (s *Store) GetTask(id int64) (Task, error) {
	row := s.db.QueryRow(`SELECT id, project_id, title, description, acceptance_criteria, status, priority,
		agent, model, mode, commits, type, position, created_at, updated_at
		FROM tasks WHERE id = ?`, id)
	return scanTask(row)
}

func (s *Store) CreateTask(t Task) (Task, error) {
	t = normalizeTask(t)
	if t.Title == "" {
		return Task{}, fmt.Errorf("title is required")
	}
	if t.ProjectID == 0 {
		return Task{}, fmt.Errorf("project is required")
	}
	if err := validateTaskEnums(t); err != nil {
		return Task{}, err
	}
	pos, err := s.nextPosition(t.ProjectID, t.Status)
	if err != nil {
		return Task{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`INSERT INTO tasks (
		project_id, title, description, acceptance_criteria, status, priority,
		agent, model, mode, commits, type, position, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ProjectID, t.Title, t.Description, t.AcceptanceCriteria, t.Status, t.Priority,
		t.Agent, t.Model, t.Mode, t.Commits, t.Type, pos, now, now)
	if err != nil {
		return Task{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Task{}, err
	}
	return s.GetTask(id)
}

func (s *Store) UpdateTask(t Task) error {
	t = normalizeTask(t)
	if t.Title == "" {
		return fmt.Errorf("title is required")
	}
	if err := validateTaskEnums(t); err != nil {
		return err
	}
	cur, err := s.GetTask(t.ID)
	if err != nil {
		return fmt.Errorf("task not found")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	pos := cur.Position
	if t.Status != cur.Status {
		pos, err = s.nextPosition(cur.ProjectID, t.Status)
		if err != nil {
			return err
		}
	}
	res, err := s.db.Exec(`UPDATE tasks SET title = ?, description = ?, acceptance_criteria = ?,
		status = ?, priority = ?, agent = ?, model = ?, mode = ?, commits = ?, type = ?,
		position = ?, updated_at = ? WHERE id = ?`,
		t.Title, t.Description, t.AcceptanceCriteria, t.Status, t.Priority, t.Agent, t.Model,
		t.Mode, t.Commits, t.Type, pos, now, t.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

func (s *Store) MoveTask(id int64, status string) error {
	if !oneOf(status, Statuses) {
		return fmt.Errorf("invalid status: %s", status)
	}
	cur, err := s.GetTask(id)
	if err != nil {
		return fmt.Errorf("task not found")
	}
	if cur.Status == status {
		return nil
	}
	pos, err := s.nextPosition(cur.ProjectID, status)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.Exec(`UPDATE tasks SET status = ?, position = ?, updated_at = ? WHERE id = ?`,
		status, pos, now, id)
	return err
}

func (s *Store) nextPosition(projectID int64, status string) (int, error) {
	var pos int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(position), -1) + 1 FROM tasks WHERE project_id = ? AND status = ?`,
		projectID, status).Scan(&pos)
	return pos, err
}

func normalizeTask(t Task) Task {
	t.Title = strings.TrimSpace(t.Title)
	t.Description = strings.TrimSpace(t.Description)
	t.AcceptanceCriteria = strings.TrimSpace(t.AcceptanceCriteria)
	t.Model = strings.TrimSpace(t.Model)
	if t.Status == "" {
		t.Status = StatusInbox
	}
	if t.Priority == "" {
		t.Priority = PriorityMedium
	}
	if t.Agent == "" {
		t.Agent = AgentMe
	}
	if t.Mode == "" {
		t.Mode = ModePair
	}
	if t.Commits == "" {
		t.Commits = CommitsNone
	}
	if t.Type == "" {
		t.Type = TypeFeature
	}
	return t
}

func validateTaskEnums(t Task) error {
	if !oneOf(t.Status, Statuses) {
		return fmt.Errorf("invalid status: %s", t.Status)
	}
	if !oneOf(t.Priority, Priorities) {
		return fmt.Errorf("invalid priority: %s", t.Priority)
	}
	if !oneOf(t.Agent, Agents) {
		return fmt.Errorf("invalid agent: %s", t.Agent)
	}
	if !oneOf(t.Mode, Modes) {
		return fmt.Errorf("invalid mode: %s", t.Mode)
	}
	if !oneOf(t.Commits, CommitModes) {
		return fmt.Errorf("invalid commits: %s", t.Commits)
	}
	if !oneOf(t.Type, Types) {
		return fmt.Errorf("invalid type: %s", t.Type)
	}
	return nil
}

func oneOf(v string, opts []string) bool {
	for _, o := range opts {
		if v == o {
			return true
		}
	}
	return false
}

func scanTask(row scanner) (Task, error) {
	var t Task
	var created, updated string
	if err := row.Scan(
		&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.AcceptanceCriteria,
		&t.Status, &t.Priority, &t.Agent, &t.Model, &t.Mode, &t.Commits, &t.Type,
		&t.Position, &created, &updated,
	); err != nil {
		return Task{}, err
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return t, nil
}

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const (
	RunRunning    = "running"
	RunPairPaused = "pair-paused"
	RunReview     = "review"
	RunBlocked    = "blocked"
	RunApproved   = "approved"
	RunMerged     = "merged"
	RunCleaned    = "cleaned"
	RunDiscarded  = "discarded"
)

const runCols = `id, task_id, project_id, worktree, branch, base, log_path, summary, checks, state, note, agent, duration_ms, tokens_in, tokens_out, tokens_cache, tokens_context, cost_usd, created_at, updated_at`

type Run struct {
	ID            int64           `json:"id"`
	TaskID        int64           `json:"task_id"`
	ProjectID     int64           `json:"project_id"`
	Worktree      string          `json:"worktree"`
	Branch        string          `json:"branch"`
	Base          string          `json:"base"`
	LogPath       string          `json:"log_path"`
	Summary       string          `json:"summary"`
	Checks        string          `json:"checks"`
	State         string          `json:"state"`
	Note          string          `json:"note"`
	Agent         string          `json:"agent"`
	DurationMS    sql.NullInt64   `json:"duration_ms"`
	TokensIn      sql.NullInt64   `json:"tokens_in"`
	TokensOut     sql.NullInt64   `json:"tokens_out"`
	TokensCache   sql.NullInt64   `json:"tokens_cache"`
	TokensContext sql.NullInt64   `json:"tokens_context"`
	CostUSD       sql.NullFloat64 `json:"cost_usd"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// MarshalJSON writes the nullable metrics as plain numbers or null, so the CLI
// contract never exposes database/sql types.
func (r Run) MarshalJSON() ([]byte, error) {
	type plain Run
	return json.Marshal(struct {
		plain
		DurationMS    *int64   `json:"duration_ms"`
		TokensIn      *int64   `json:"tokens_in"`
		TokensOut     *int64   `json:"tokens_out"`
		TokensCache   *int64   `json:"tokens_cache"`
		TokensContext *int64   `json:"tokens_context"`
		CostUSD       *float64 `json:"cost_usd"`
	}{
		plain:         plain(r),
		DurationMS:    jsonInt(r.DurationMS),
		TokensIn:      jsonInt(r.TokensIn),
		TokensOut:     jsonInt(r.TokensOut),
		TokensCache:   jsonInt(r.TokensCache),
		TokensContext: jsonInt(r.TokensContext),
		CostUSD:       jsonFloat(r.CostUSD),
	})
}

func jsonInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func jsonFloat(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

type MetricRow struct {
	Key           string
	Runs          int
	Approved      int
	AvgContext    sql.NullFloat64
	AvgDurationMS sql.NullFloat64
	SumIn         sql.NullInt64
	SumOut        sql.NullInt64
	SumCache      sql.NullInt64
	SumCost       sql.NullFloat64
}

func (s *Store) CreateRun(r Run) (Run, error) {
	if r.TaskID == 0 {
		return Run{}, fmt.Errorf("task is required")
	}
	if r.State == "" {
		r.State = RunRunning
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`INSERT INTO runs (
		task_id, project_id, worktree, branch, base, log_path, summary, checks, state, note, agent,
		duration_ms, tokens_in, tokens_out, tokens_cache, tokens_context, cost_usd, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.TaskID, r.ProjectID, r.Worktree, r.Branch, r.Base, r.LogPath, r.Summary, r.Checks, r.State, r.Note, r.Agent,
		jsonInt(r.DurationMS), jsonInt(r.TokensIn), jsonInt(r.TokensOut), jsonInt(r.TokensCache), jsonInt(r.TokensContext),
		nullFloat(r.CostUSD), now, now)
	if err != nil {
		return Run{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Run{}, err
	}
	return s.GetRun(id)
}

func (s *Store) ListRuns(state string) ([]Run, error) {
	q := `SELECT ` + runCols + ` FROM runs`
	args := []any{}
	if state != "" {
		q += ` WHERE state = ?`
		args = append(args, state)
	}
	q += ` ORDER BY id DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetRun(id int64) (Run, error) {
	row := s.db.QueryRow(`SELECT `+runCols+` FROM runs WHERE id = ?`, id)
	return scanRun(row)
}

func (s *Store) LatestRun(taskID int64) (Run, error) {
	row := s.db.QueryRow(`SELECT `+runCols+` FROM runs WHERE task_id = ? ORDER BY id DESC LIMIT 1`, taskID)
	return scanRun(row)
}

func (s *Store) ActiveRun() (Run, error) {
	row := s.db.QueryRow(`SELECT `+runCols+` FROM runs WHERE state = ? ORDER BY id DESC LIMIT 1`, RunRunning)
	return scanRun(row)
}

func (s *Store) UpdateRun(r Run) error {
	ok, err := s.UpdateRunIfState(r, "")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("run not found")
	}
	return nil
}

// UpdateRunIfState writes r only when the row is still in from (empty from
// matches any state). It reports whether this caller won the update.
func (s *Store) UpdateRunIfState(r Run, from string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	q := `UPDATE runs SET worktree = ?, branch = ?, base = ?, log_path = ?, summary = ?, checks = ?, state = ?, note = ?, agent = ?,
		duration_ms = ?, tokens_in = ?, tokens_out = ?, tokens_cache = ?, tokens_context = ?, cost_usd = ?, updated_at = ?
		WHERE id = ?`
	args := []any{
		r.Worktree, r.Branch, r.Base, r.LogPath, r.Summary, r.Checks, r.State, r.Note, r.Agent,
		jsonInt(r.DurationMS), jsonInt(r.TokensIn), jsonInt(r.TokensOut), jsonInt(r.TokensCache), jsonInt(r.TokensContext),
		nullFloat(r.CostUSD), now, r.ID,
	}
	if from != "" {
		q += ` AND state = ?`
		args = append(args, from)
	}
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) MetricsByAgent() ([]MetricRow, error) {
	rows, err := s.db.Query(`SELECT COALESCE(NULLIF(agent, ''), '(sin agente)'),
		COUNT(*),
		SUM(CASE WHEN state = ? THEN 1 ELSE 0 END),
		AVG(tokens_context), AVG(duration_ms),
		SUM(tokens_in), SUM(tokens_out), SUM(tokens_cache), SUM(cost_usd)
		FROM runs GROUP BY 1 ORDER BY 1 COLLATE NOCASE`, RunApproved)
	if err != nil {
		return nil, err
	}
	return scanMetricRows(rows)
}

func (s *Store) MetricsByProject() ([]MetricRow, error) {
	rows, err := s.db.Query(`SELECT COALESCE(NULLIF(p.name, ''), '(sin proyecto)'),
		COUNT(*),
		SUM(CASE WHEN r.state = ? THEN 1 ELSE 0 END),
		AVG(r.tokens_context), AVG(r.duration_ms),
		SUM(r.tokens_in), SUM(r.tokens_out), SUM(r.tokens_cache), SUM(r.cost_usd)
		FROM runs r
		LEFT JOIN projects p ON p.id = r.project_id
		GROUP BY r.project_id
		ORDER BY 1 COLLATE NOCASE`, RunApproved)
	if err != nil {
		return nil, err
	}
	return scanMetricRows(rows)
}

func scanMetricRows(rows *sql.Rows) ([]MetricRow, error) {
	defer rows.Close()
	var out []MetricRow
	for rows.Next() {
		var r MetricRow
		var runs, approved int64
		if err := rows.Scan(&r.Key, &runs, &approved, &r.AvgContext, &r.AvgDurationMS, &r.SumIn, &r.SumOut, &r.SumCache, &r.SumCost); err != nil {
			return nil, err
		}
		r.Runs = int(runs)
		r.Approved = int(approved)
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanRun(row scanner) (Run, error) {
	var r Run
	var created, updated string
	if err := row.Scan(
		&r.ID, &r.TaskID, &r.ProjectID, &r.Worktree, &r.Branch, &r.Base, &r.LogPath,
		&r.Summary, &r.Checks, &r.State, &r.Note, &r.Agent,
		&r.DurationMS, &r.TokensIn, &r.TokensOut, &r.TokensCache, &r.TokensContext, &r.CostUSD,
		&created, &updated,
	); err != nil {
		return Run{}, err
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return r, nil
}

func nullInt(n sql.NullInt64) any {
	if !n.Valid {
		return nil
	}
	return n.Int64
}

func nullFloat(n sql.NullFloat64) any {
	if !n.Valid {
		return nil
	}
	return n.Float64
}

// ChangeStamp changes whenever a task, run, plan or review comment is added
// or updated, by this process or another one (the daemon, the bot, the CLI).
// The TUI polls it to stay in sync without reloading on every tick.
func (s *Store) ChangeStamp() string {
	var stamp string
	_ = s.db.QueryRow(`SELECT
		(SELECT COUNT(*) || '|' || COALESCE(MAX(updated_at), '') FROM tasks) || '#' ||
		(SELECT COUNT(*) || '|' || COALESCE(MAX(updated_at), '') FROM runs) || '#' ||
		(SELECT COUNT(*) || '|' || COALESCE(MAX(updated_at), '') FROM plans) || '#' ||
		(SELECT COUNT(*) || '|' || COALESCE(SUM(resolved), 0) FROM review_comments)`).Scan(&stamp)
	return stamp
}

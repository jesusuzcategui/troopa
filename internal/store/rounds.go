package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const (
	RoundLaunch  = "launch"
	RoundChanges = "changes"

	RoundRunning = "running"
	RoundReview  = "review"
	RoundBlocked = "blocked"
)

const roundCols = `id, run_id, number, kind, feedback, summary, checks, state, duration_ms, tokens_in, tokens_out, tokens_cache, tokens_context, cost_usd, log_from, log_to, created_at, finished_at`

type Round struct {
	ID            int64           `json:"id"`
	RunID         int64           `json:"run_id"`
	Number        int             `json:"number"`
	Kind          string          `json:"kind"`
	Feedback      string          `json:"feedback"`
	Summary       string          `json:"summary"`
	Checks        string          `json:"checks"`
	State         string          `json:"state"`
	DurationMS    sql.NullInt64   `json:"duration_ms"`
	TokensIn      sql.NullInt64   `json:"tokens_in"`
	TokensOut     sql.NullInt64   `json:"tokens_out"`
	TokensCache   sql.NullInt64   `json:"tokens_cache"`
	TokensContext sql.NullInt64   `json:"tokens_context"`
	CostUSD       sql.NullFloat64 `json:"cost_usd"`
	LogFrom       int64           `json:"log_from"`
	LogTo         sql.NullInt64   `json:"log_to"`
	CreatedAt     time.Time       `json:"created_at"`
	FinishedAt    time.Time       `json:"finished_at"`
}

func (r Round) MarshalJSON() ([]byte, error) {
	type plain Round
	var finished *string
	if !r.FinishedAt.IsZero() {
		s := r.FinishedAt.UTC().Format(time.RFC3339Nano)
		finished = &s
	}
	created := r.CreatedAt.UTC().Format(time.RFC3339Nano)
	if r.CreatedAt.IsZero() {
		created = ""
	}
	return json.Marshal(struct {
		plain
		ID            *int64   `json:"id"`
		DurationMS    *int64   `json:"duration_ms"`
		TokensIn      *int64   `json:"tokens_in"`
		TokensOut     *int64   `json:"tokens_out"`
		TokensCache   *int64   `json:"tokens_cache"`
		TokensContext *int64   `json:"tokens_context"`
		CostUSD       *float64 `json:"cost_usd"`
		LogTo         *int64   `json:"log_to"`
		CreatedAt     string   `json:"created_at"`
		FinishedAt    *string  `json:"finished_at"`
	}{
		plain:         plain(r),
		ID:            jsonID(r.ID),
		DurationMS:    jsonInt(r.DurationMS),
		TokensIn:      jsonInt(r.TokensIn),
		TokensOut:     jsonInt(r.TokensOut),
		TokensCache:   jsonInt(r.TokensCache),
		TokensContext: jsonInt(r.TokensContext),
		CostUSD:       jsonFloat(r.CostUSD),
		LogTo:         jsonInt(r.LogTo),
		CreatedAt:     created,
		FinishedAt:    finished,
	})
}

func jsonID(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func (s *Store) CreateRound(r Round) (Round, error) {
	if r.RunID == 0 {
		return Round{}, fmt.Errorf("run is required")
	}
	if r.Number < 1 {
		n, err := s.NextRoundNumber(r.RunID)
		if err != nil {
			return Round{}, err
		}
		r.Number = n
	}
	if r.Kind == "" {
		r.Kind = RoundLaunch
	}
	if r.State == "" {
		r.State = RoundRunning
	}
	now := time.Now().UTC()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	finished := ""
	if !r.FinishedAt.IsZero() {
		finished = r.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	res, err := s.db.Exec(`INSERT INTO run_rounds (
		run_id, number, kind, feedback, summary, checks, state,
		duration_ms, tokens_in, tokens_out, tokens_cache, tokens_context, cost_usd,
		log_from, log_to, created_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RunID, r.Number, r.Kind, r.Feedback, r.Summary, r.Checks, r.State,
		jsonInt(r.DurationMS), jsonInt(r.TokensIn), jsonInt(r.TokensOut), jsonInt(r.TokensCache), jsonInt(r.TokensContext),
		nullFloat(r.CostUSD), r.LogFrom, jsonInt(r.LogTo),
		r.CreatedAt.UTC().Format(time.RFC3339Nano), finished)
	if err != nil {
		return Round{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Round{}, err
	}
	return s.GetRound(id)
}

func (s *Store) GetRound(id int64) (Round, error) {
	row := s.db.QueryRow(`SELECT `+roundCols+` FROM run_rounds WHERE id = ?`, id)
	return scanRound(row)
}

func (s *Store) ListRounds(runID int64) ([]Round, error) {
	rows, err := s.db.Query(`SELECT `+roundCols+` FROM run_rounds WHERE run_id = ? ORDER BY number ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Round
	for rows.Next() {
		r, err := scanRound(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []Round{}
	}
	return out, rows.Err()
}

func (s *Store) NextRoundNumber(runID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(number), 0) + 1 FROM run_rounds WHERE run_id = ?`, runID).Scan(&n)
	return n, err
}

func (s *Store) OpenRound(runID int64) (Round, error) {
	row := s.db.QueryRow(`SELECT `+roundCols+` FROM run_rounds WHERE run_id = ? AND state = ? ORDER BY number DESC LIMIT 1`, runID, RoundRunning)
	return scanRound(row)
}

func (s *Store) UpdateRound(r Round) error {
	finished := ""
	if !r.FinishedAt.IsZero() {
		finished = r.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	res, err := s.db.Exec(`UPDATE run_rounds SET kind = ?, feedback = ?, summary = ?, checks = ?, state = ?,
		duration_ms = ?, tokens_in = ?, tokens_out = ?, tokens_cache = ?, tokens_context = ?, cost_usd = ?,
		log_from = ?, log_to = ?, finished_at = ? WHERE id = ?`,
		r.Kind, r.Feedback, r.Summary, r.Checks, r.State,
		jsonInt(r.DurationMS), jsonInt(r.TokensIn), jsonInt(r.TokensOut), jsonInt(r.TokensCache), jsonInt(r.TokensContext),
		nullFloat(r.CostUSD), r.LogFrom, jsonInt(r.LogTo), finished, r.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("round not found")
	}
	return nil
}

// RoundsForRun returns stored rounds, or a single synthetic round for older
// runs that finished before run_rounds existed.
func (s *Store) RoundsForRun(run Run) ([]Round, error) {
	items, err := s.ListRounds(run.ID)
	if err != nil {
		return nil, err
	}
	if len(items) > 0 {
		return items, nil
	}
	return []Round{LegacyRound(run)}, nil
}

func LegacyRound(run Run) Round {
	st := RoundReview
	switch run.State {
	case RunRunning, RunPairPaused:
		st = RoundRunning
	case RunBlocked:
		st = RoundBlocked
	}
	r := Round{
		RunID:         run.ID,
		Number:        1,
		Kind:          RoundLaunch,
		Summary:       run.Summary,
		Checks:        run.Checks,
		State:         st,
		DurationMS:    run.DurationMS,
		TokensIn:      run.TokensIn,
		TokensOut:     run.TokensOut,
		TokensCache:   run.TokensCache,
		TokensContext: run.TokensContext,
		CostUSD:       run.CostUSD,
		CreatedAt:     run.CreatedAt,
	}
	if st != RoundRunning && !run.UpdatedAt.IsZero() {
		r.FinishedAt = run.UpdatedAt
	}
	return r
}

func scanRound(row scanner) (Round, error) {
	var r Round
	var created, finished string
	if err := row.Scan(
		&r.ID, &r.RunID, &r.Number, &r.Kind, &r.Feedback, &r.Summary, &r.Checks, &r.State,
		&r.DurationMS, &r.TokensIn, &r.TokensOut, &r.TokensCache, &r.TokensContext, &r.CostUSD,
		&r.LogFrom, &r.LogTo, &created, &finished,
	); err != nil {
		return Round{}, err
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	if finished != "" {
		r.FinishedAt, _ = time.Parse(time.RFC3339Nano, finished)
	}
	return r, nil
}

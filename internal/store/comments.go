package store

import (
	"database/sql"
	"fmt"
	"time"
)

const commentCols = `id, run_id, round, path, line, side, quote, body, resolved, created_at`

type ReviewComment struct {
	ID        int64     `json:"id"`
	RunID     int64     `json:"run_id"`
	Round     int       `json:"round"`
	Path      string    `json:"path"`
	Line      int       `json:"line"`
	Side      string    `json:"side"`
	Quote     string    `json:"quote"`
	Body      string    `json:"body"`
	Resolved  bool      `json:"resolved"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) CreateComment(c ReviewComment) (ReviewComment, error) {
	if c.RunID == 0 {
		return ReviewComment{}, fmt.Errorf("run is required")
	}
	if c.Side == "" {
		c.Side = " "
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	res, err := s.db.Exec(`INSERT INTO review_comments (run_id, round, path, line, side, quote, body, resolved, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.RunID, c.Round, c.Path, c.Line, c.Side, c.Quote, c.Body, boolInt(c.Resolved),
		c.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return ReviewComment{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ReviewComment{}, err
	}
	return s.GetComment(id)
}

func (s *Store) GetComment(id int64) (ReviewComment, error) {
	row := s.db.QueryRow(`SELECT `+commentCols+` FROM review_comments WHERE id = ?`, id)
	return scanComment(row)
}

func (s *Store) UpdateComment(c ReviewComment) error {
	res, err := s.db.Exec(`UPDATE review_comments SET round = ?, path = ?, line = ?, side = ?, quote = ?, body = ?, resolved = ? WHERE id = ?`,
		c.Round, c.Path, c.Line, c.Side, c.Quote, c.Body, boolInt(c.Resolved), c.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("comment not found")
	}
	return nil
}

func (s *Store) DeleteComment(id int64) error {
	res, err := s.db.Exec(`DELETE FROM review_comments WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("comment not found")
	}
	return nil
}

func (s *Store) ListComments(runID int64) ([]ReviewComment, error) {
	rows, err := s.db.Query(`SELECT `+commentCols+` FROM review_comments WHERE run_id = ? ORDER BY path ASC, line ASC, id ASC`, runID)
	if err != nil {
		return nil, err
	}
	return scanComments(rows)
}

func (s *Store) ListPendingComments(runID int64) ([]ReviewComment, error) {
	rows, err := s.db.Query(`SELECT `+commentCols+` FROM review_comments WHERE run_id = ? AND resolved = 0 ORDER BY path ASC, line ASC, id ASC`, runID)
	if err != nil {
		return nil, err
	}
	return scanComments(rows)
}

func (s *Store) ListCommentsByRound(runID int64, round int) ([]ReviewComment, error) {
	rows, err := s.db.Query(`SELECT `+commentCols+` FROM review_comments WHERE run_id = ? AND round = ? ORDER BY path ASC, line ASC, id ASC`, runID, round)
	if err != nil {
		return nil, err
	}
	return scanComments(rows)
}

func (s *Store) ResolvePendingComments(runID int64, round int) error {
	_, err := s.db.Exec(`UPDATE review_comments SET resolved = 1, round = ? WHERE run_id = ? AND resolved = 0`, round, runID)
	return err
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func scanComments(rows *sql.Rows) ([]ReviewComment, error) {
	defer rows.Close()
	var out []ReviewComment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []ReviewComment{}
	}
	return out, rows.Err()
}

func scanComment(row scanner) (ReviewComment, error) {
	var c ReviewComment
	var resolved int
	var created string
	if err := row.Scan(&c.ID, &c.RunID, &c.Round, &c.Path, &c.Line, &c.Side, &c.Quote, &c.Body, &resolved, &created); err != nil {
		return ReviewComment{}, err
	}
	c.Resolved = resolved != 0
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return c, nil
}

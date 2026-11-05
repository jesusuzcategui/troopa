package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Client struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Contact   string    `json:"contact"`
	Notes     string    `json:"notes"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) ListClients(includeArchived bool) ([]Client, error) {
	q := `SELECT id, name, contact, notes, archived, created_at, updated_at FROM clients`
	if !includeArchived {
		q += ` WHERE archived = 0`
	}
	q += ` ORDER BY archived ASC, name COLLATE NOCASE ASC`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetClient(id int64) (Client, error) {
	row := s.db.QueryRow(`SELECT id, name, contact, notes, archived, created_at, updated_at
		FROM clients WHERE id = ?`, id)
	return scanClient(row)
}

func (s *Store) CreateClient(c Client) (Client, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Contact = strings.TrimSpace(c.Contact)
	c.Notes = strings.TrimSpace(c.Notes)
	if c.Name == "" {
		return Client{}, fmt.Errorf("name is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`INSERT INTO clients (name, contact, notes, archived, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?)`, c.Name, c.Contact, c.Notes, now, now)
	if err != nil {
		return Client{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Client{}, err
	}
	return s.GetClient(id)
}

func (s *Store) UpdateClient(c Client) error {
	c.Name = strings.TrimSpace(c.Name)
	c.Contact = strings.TrimSpace(c.Contact)
	c.Notes = strings.TrimSpace(c.Notes)
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE clients SET name = ?, contact = ?, notes = ?, updated_at = ?
		WHERE id = ?`, c.Name, c.Contact, c.Notes, now, c.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("client not found")
	}
	return nil
}

func (s *Store) ArchiveClient(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE clients SET archived = 1, updated_at = ? WHERE id = ? AND archived = 0`, now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("client not found or already archived")
	}
	return nil
}

func scanClient(row scanner) (Client, error) {
	var c Client
	var archived int
	var created, updated string
	if err := row.Scan(&c.ID, &c.Name, &c.Contact, &c.Notes, &archived, &created, &updated); err != nil {
		return Client{}, err
	}
	c.Archived = archived != 0
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return c, nil
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func scanNullID(n sql.NullInt64) int64 {
	if n.Valid {
		return n.Int64
	}
	return 0
}

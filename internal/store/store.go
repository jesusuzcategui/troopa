package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Project struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	RepoPath    string    `json:"repo_path"`
	Stack       string    `json:"stack"`
	Description string    `json:"description"`
	ClientID    int64     `json:"client_id"`
	Account     string    `json:"account"`    // agent login (config [accounts.*]); "" = default
	ProfileID   int64     `json:"profile_id"` // who the agents work for; 0 = default profile
	Archived    bool      `json:"archived"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProjectFilter struct {
	IncludeArchived bool
	ClientID        int64
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}
	entries, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)
	for _, name := range entries {
		version := strings.TrimSuffix(filepath.Base(name), ".sql")
		var found string
		err := s.db.QueryRow(`SELECT version FROM schema_migrations WHERE version = ?`, version).Scan(&found)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s: %w", version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) List(includeArchived bool) ([]Project, error) {
	return s.ListFiltered(ProjectFilter{IncludeArchived: includeArchived})
}

func (s *Store) ListFiltered(f ProjectFilter) ([]Project, error) {
	q := `SELECT id, name, slug, repo_path, stack, description, client_id, account, profile_id, archived, created_at, updated_at
		FROM projects WHERE 1=1`
	args := []any{}
	if !f.IncludeArchived {
		q += ` AND archived = 0`
	}
	if f.ClientID != 0 {
		q += ` AND client_id = ?`
		args = append(args, f.ClientID)
	}
	q += ` ORDER BY archived ASC, name COLLATE NOCASE ASC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Get(id int64) (Project, error) {
	row := s.db.QueryRow(`SELECT id, name, slug, repo_path, stack, description, client_id, account, profile_id, archived, created_at, updated_at
		FROM projects WHERE id = ?`, id)
	return scanProject(row)
}

func (s *Store) Create(p Project) (Project, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Slug = strings.TrimSpace(p.Slug)
	p.RepoPath = NormalizeRepoPath(p.RepoPath)
	p.Stack = strings.TrimSpace(p.Stack)
	p.Description = strings.TrimSpace(p.Description)
	p.Account = strings.TrimSpace(p.Account)
	if p.Name == "" {
		return Project{}, fmt.Errorf("name is required")
	}
	if p.Slug == "" {
		p.Slug = Slugify(p.Name)
	}
	if p.Slug == "" {
		return Project{}, fmt.Errorf("slug is required")
	}
	if p.RepoPath == "" {
		return Project{}, fmt.Errorf("repo path is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`INSERT INTO projects (name, slug, repo_path, stack, description, client_id, account, profile_id, archived, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		p.Name, p.Slug, p.RepoPath, p.Stack, p.Description, nullID(p.ClientID), p.Account, p.ProfileID, now, now)
	if err != nil {
		return Project{}, wrapUnique(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Project{}, err
	}
	return s.Get(id)
}

func (s *Store) Update(p Project) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Slug = strings.TrimSpace(p.Slug)
	p.RepoPath = NormalizeRepoPath(p.RepoPath)
	p.Stack = strings.TrimSpace(p.Stack)
	p.Description = strings.TrimSpace(p.Description)
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	if p.Slug == "" {
		p.Slug = Slugify(p.Name)
	}
	if p.RepoPath == "" {
		return fmt.Errorf("repo path is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE projects SET name = ?, slug = ?, repo_path = ?, stack = ?, description = ?, client_id = ?, account = ?, profile_id = ?, updated_at = ?
		WHERE id = ?`,
		p.Name, p.Slug, p.RepoPath, p.Stack, p.Description, nullID(p.ClientID), strings.TrimSpace(p.Account), p.ProfileID, now, p.ID)
	if err != nil {
		return wrapUnique(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

func (s *Store) Archive(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE projects SET archived = 1, updated_at = ? WHERE id = ? AND archived = 0`, now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("project not found or already archived")
	}
	return nil
}

func wrapUnique(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "unique") {
		return fmt.Errorf("a project with that slug already exists")
	}
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(row scanner) (Project, error) {
	var p Project
	var archived int
	var clientID sql.NullInt64
	var created, updated string
	if err := row.Scan(&p.ID, &p.Name, &p.Slug, &p.RepoPath, &p.Stack, &p.Description, &clientID, &p.Account, &p.ProfileID, &archived, &created, &updated); err != nil {
		return Project{}, err
	}
	p.RepoPath = NormalizeRepoPath(p.RepoPath)
	p.ClientID = scanNullID(clientID)
	p.Archived = archived != 0
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return p, nil
}

// NormalizeRepoPath expands a leading ~ to the home directory and makes the
// path absolute, so "~/src/app" never becomes a folder literally named "~".
func NormalizeRepoPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~"+string(filepath.Separator)) {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				p = home
			} else {
				p = filepath.Join(home, p[2:])
			}
		}
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return p
}

func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

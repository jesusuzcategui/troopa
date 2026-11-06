package store

import (
	"fmt"
	"strings"
	"time"
)

// Profile is who the agents work for: one person can keep several, one per
// profession (developer, chef, accountant…). Its text goes into TASK.md.
type Profile struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Profession   string    `json:"profession"`
	About        string    `json:"about"`
	Instructions string    `json:"instructions"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Label is "Name · Profession", or whichever is set.
func (p Profile) Label() string {
	switch {
	case p.Name != "" && p.Profession != "":
		return p.Name + " · " + p.Profession
	case p.Profession != "":
		return p.Profession
	}
	return p.Name
}

const profileCols = `id, name, profession, about, instructions, created_at, updated_at`

func (s *Store) ListProfiles() ([]Profile, error) {
	rows, err := s.db.Query(`SELECT ` + profileCols + ` FROM profiles ORDER BY name COLLATE NOCASE, profession COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProfile(id int64) (Profile, error) {
	return scanProfile(s.db.QueryRow(`SELECT `+profileCols+` FROM profiles WHERE id = ?`, id))
}

func normalizeProfile(p Profile) (Profile, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Profession = strings.TrimSpace(p.Profession)
	p.About = strings.TrimSpace(p.About)
	p.Instructions = strings.TrimSpace(p.Instructions)
	if p.Name == "" && p.Profession == "" {
		return p, fmt.Errorf("a profile needs a name or a profession")
	}
	return p, nil
}

func (s *Store) CreateProfile(p Profile) (Profile, error) {
	p, err := normalizeProfile(p)
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`INSERT INTO profiles (name, profession, about, instructions, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, p.Name, p.Profession, p.About, p.Instructions, now, now)
	if err != nil {
		return Profile{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Profile{}, err
	}
	return s.GetProfile(id)
}

func (s *Store) UpdateProfile(p Profile) error {
	p, err := normalizeProfile(p)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.Exec(`UPDATE profiles SET name = ?, profession = ?, about = ?, instructions = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Profession, p.About, p.Instructions, now, p.ID)
	return err
}

// DeleteProfile removes a profile; projects that used it fall back to the
// default profile.
func (s *Store) DeleteProfile(id int64) error {
	if _, err := s.db.Exec(`UPDATE projects SET profile_id = 0 WHERE profile_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM profiles WHERE id = ?`, id)
	return err
}

func scanProfile(row scanner) (Profile, error) {
	var p Profile
	var created, updated string
	if err := row.Scan(&p.ID, &p.Name, &p.Profession, &p.About, &p.Instructions, &created, &updated); err != nil {
		return Profile{}, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return p, nil
}

package store

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const attachmentCols = `id, task_id, name, path, mime, size, created_at`

type Attachment struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	MIME      string    `json:"mime"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) GetAttachment(id int64) (Attachment, error) {
	row := s.db.QueryRow(`SELECT `+attachmentCols+` FROM attachments WHERE id = ?`, id)
	return scanAttachment(row)
}

func (s *Store) ListAttachments(taskID int64) ([]Attachment, error) {
	rows, err := s.db.Query(`SELECT `+attachmentCols+` FROM attachments WHERE task_id = ? ORDER BY id ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if out == nil {
		out = []Attachment{}
	}
	return out, rows.Err()
}

func (s *Store) DeleteAttachment(id int64) error {
	cur, err := s.GetAttachment(id)
	if err != nil {
		return fmt.Errorf("attachment not found")
	}
	res, err := s.db.Exec(`DELETE FROM attachments WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("attachment not found")
	}
	if cur.Path != "" {
		_ = os.Remove(cur.Path)
	}
	return nil
}

// AttachFile copies src into destDir and stores the row. destDir is typically
// $XDG_DATA_HOME/troopa/attachments/<task-id>.
func (s *Store) AttachFile(taskID int64, src, destDir string, maxBytes int64) (Attachment, error) {
	if taskID < 1 {
		return Attachment{}, fmt.Errorf("task is required")
	}
	if _, err := s.GetTask(taskID); err != nil {
		return Attachment{}, fmt.Errorf("task not found")
	}
	src = strings.TrimSpace(src)
	if src == "" {
		return Attachment{}, fmt.Errorf("file is required")
	}
	src, err := filepath.Abs(src)
	if err != nil {
		return Attachment{}, err
	}
	st, err := os.Stat(src)
	if err != nil {
		return Attachment{}, fmt.Errorf("file not found: %s", src)
	}
	if st.IsDir() {
		return Attachment{}, fmt.Errorf("not a file: %s", src)
	}
	if maxBytes > 0 && st.Size() > maxBytes {
		return Attachment{}, fmt.Errorf("file is too large: %s (limit %s)", FormatSize(st.Size()), FormatSize(maxBytes))
	}
	name := sanitizeName(st.Name())
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return Attachment{}, err
	}
	dest := uniquePath(destDir, name)
	if err := copyFile(src, dest); err != nil {
		return Attachment{}, err
	}
	mimeType := DetectMIME(dest, name)
	now := time.Now().UTC()
	res, err := s.db.Exec(`INSERT INTO attachments (task_id, name, path, mime, size, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		taskID, filepath.Base(dest), dest, mimeType, st.Size(), now.Format(time.RFC3339Nano))
	if err != nil {
		_ = os.Remove(dest)
		return Attachment{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Attachment{}, err
	}
	return s.GetAttachment(id)
}

func DetectMIME(path, name string) string {
	if ext := filepath.Ext(name); ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:n])
}

func FormatSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%d KB", n/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

func sanitizeName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, string(filepath.Separator), "_")
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}

func uniquePath(dir, name string) string {
	dest := filepath.Join(dir, name)
	if _, err := os.Stat(dest); err != nil {
		return dest
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 2; i < 1000; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(cand); err != nil {
			return cand
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, os.Getpid(), ext))
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dest)
		return err
	}
	return out.Close()
}

func scanAttachment(row scanner) (Attachment, error) {
	var a Attachment
	var created string
	if err := row.Scan(&a.ID, &a.TaskID, &a.Name, &a.Path, &a.MIME, &a.Size, &created); err != nil {
		return Attachment{}, err
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return a, nil
}

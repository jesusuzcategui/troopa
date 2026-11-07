package store

import "time"

// QueueItem is a task waiting for its turn: Troopa runs one agent at a time,
// so a launch asked for while another agent works waits here. Chat is the
// Telegram chat to tell when it starts.
type QueueItem struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	Chat      string    `json:"chat"`
	CreatedAt time.Time `json:"created_at"`
}

// Enqueue adds a task to the launch queue (once) and returns its position.
func (s *Store) Enqueue(taskID int64, chat string) (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM launch_queue WHERE task_id = ?`, taskID).Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		if _, err := s.db.Exec(`INSERT INTO launch_queue (task_id, chat, created_at) VALUES (?, ?, ?)`,
			taskID, chat, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return 0, err
		}
	}
	items, err := s.Queue()
	if err != nil {
		return 0, err
	}
	for i, it := range items {
		if it.TaskID == taskID {
			return i + 1, nil
		}
	}
	return len(items), nil
}

// Queue lists the waiting tasks, oldest first.
func (s *Store) Queue() ([]QueueItem, error) {
	rows, err := s.db.Query(`SELECT id, task_id, chat, created_at FROM launch_queue ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueueItem
	for rows.Next() {
		var it QueueItem
		var created string
		if err := rows.Scan(&it.ID, &it.TaskID, &it.Chat, &created); err != nil {
			return nil, err
		}
		it.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, it)
	}
	return out, rows.Err()
}

// Dequeue removes an item from the queue.
func (s *Store) Dequeue(id int64) error {
	_, err := s.db.Exec(`DELETE FROM launch_queue WHERE id = ?`, id)
	return err
}

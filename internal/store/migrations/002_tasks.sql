CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    acceptance_criteria TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'inbox',
    priority TEXT NOT NULL DEFAULT 'medium',
    agent TEXT NOT NULL DEFAULT 'me',
    model TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL DEFAULT 'pair',
    commits TEXT NOT NULL DEFAULT 'none',
    type TEXT NOT NULL DEFAULT 'feature',
    position INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_tasks_project_status ON tasks(project_id, status, position);

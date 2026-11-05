CREATE TABLE runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    worktree TEXT NOT NULL DEFAULT '',
    branch TEXT NOT NULL DEFAULT '',
    base TEXT NOT NULL DEFAULT '',
    log_path TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    checks TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'running',
    note TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_runs_task ON runs(task_id, id DESC);

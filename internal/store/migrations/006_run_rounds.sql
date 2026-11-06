CREATE TABLE run_rounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    number INTEGER NOT NULL,
    kind TEXT NOT NULL DEFAULT 'launch',
    feedback TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    checks TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'running',
    duration_ms INTEGER,
    tokens_in INTEGER,
    tokens_out INTEGER,
    tokens_cache INTEGER,
    tokens_context INTEGER,
    cost_usd REAL,
    log_from INTEGER NOT NULL DEFAULT 0,
    log_to INTEGER,
    created_at TEXT NOT NULL,
    finished_at TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX idx_run_rounds_run_number ON run_rounds(run_id, number);
CREATE INDEX idx_run_rounds_run ON run_rounds(run_id, number);

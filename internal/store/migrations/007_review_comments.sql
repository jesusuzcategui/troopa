CREATE TABLE review_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    round INTEGER NOT NULL DEFAULT 0,
    path TEXT NOT NULL DEFAULT '',
    line INTEGER NOT NULL DEFAULT 0,
    side TEXT NOT NULL DEFAULT ' ',
    quote TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    resolved INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_review_comments_run ON review_comments(run_id, resolved, path, line);
CREATE INDEX idx_review_comments_round ON review_comments(run_id, round);

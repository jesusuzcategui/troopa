/** Task board enums — keep in sync with internal/store/tasks.go */
export const STATUSES = [
  "inbox",
  "backlog",
  "ready",
  "in-progress",
  "pair-paused",
  "review",
  "blocked",
  "done",
] as const;

export const PRIORITIES = ["low", "medium", "high"] as const;

export const AGENTS = ["claude", "codex", "opencode", "cursor", "pi", "me"] as const;

export const MODES = ["yolo", "pair"] as const;

export const TYPES = ["feature", "bugfix", "chore"] as const;

export const SYNC_MS = 2000;

package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestRequestChangesRounds(t *testing.T) {
	repo, sess, _ := setupMergeRepo(t)
	_ = repo
	if _, err := RequestChanges(sess.Worktree, "  "); err == nil {
		t.Fatal("empty feedback should fail")
	}
	r1, err := RequestChanges(sess.Worktree, "Add requirements.txt")
	if err != nil || r1 != 1 {
		t.Fatalf("round 1: %d %v", r1, err)
	}
	r2, err := RequestChanges(sess.Worktree, "Explain the venv in the README")
	if err != nil || r2 != 2 || ChangeRounds(sess.Worktree) != 2 {
		t.Fatalf("round 2: %d %v", r2, err)
	}
	raw, _ := os.ReadFile(filepath.Join(sess.Worktree, "CHANGES.md"))
	if !strings.Contains(string(raw), "## Round 2") || !strings.Contains(string(raw), "requirements.txt") {
		t.Fatalf("CHANGES.md:\n%s", raw)
	}
	status, _ := Git(sess.Worktree, "status", "--porcelain")
	if strings.Contains(status, "CHANGES.md") {
		t.Fatalf("CHANGES.md must not show up in git status: %q", status)
	}
	p := ChangesPrompt(sess.Worktree, 2)
	if !strings.Contains(p, filepath.Join(sess.Worktree, "CHANGES.md")) || !strings.Contains(p, "round 2") {
		t.Fatalf("prompt: %s", p)
	}
}

func TestTaskHasHowToRunAndProjectInstructions(t *testing.T) {
	dir := t.TempDir()
	toml := "instructions = \"\"\"\nPython: use a venv in .venv and keep requirements.txt updated.\n\"\"\"\n"
	if err := os.WriteFile(filepath.Join(dir, "troopa.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteTASK(dir, store.Task{Title: "App"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "TASK.md"))
	body := string(raw)
	if !strings.Contains(body, "How to run") || !strings.Contains(body, "## Project instructions") || !strings.Contains(body, "use a venv in .venv") {
		t.Fatalf("TASK.md:\n%s", body)
	}
}

package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestStageTaskContextCopiesAndLists(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "lib.go"), []byte("package lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(repo, "add", "lib.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(repo, "commit", "-m", "lib"); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(src, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt")}
	task := store.Task{
		ID: 9, Title: "With files", Type: store.TypeFeature, Commits: store.CommitsNone,
		Description: "See @lib.go and @missing.md",
	}
	sess, err := Prepare(cfg, store.Project{RepoPath: repo, Slug: "demo"}, task)
	if err != nil {
		t.Fatal(err)
	}
	atts := []store.Attachment{{Name: "shot.png", Path: src, MIME: "image/png", Size: 3}}
	if err := StageTaskContext(sess.Worktree, task, atts, "", ""); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(sess.Worktree, ".troopa", "attachments", "shot.png")
	if _, err := os.Stat(copied); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(sess.Worktree, "TASK.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "## Attachments") || !strings.Contains(s, ".troopa/attachments/shot.png") {
		t.Fatalf("attachments section:\n%s", s)
	}
	if !strings.Contains(s, "## Referenced files") || !strings.Contains(s, "`lib.go`") {
		t.Fatalf("refs:\n%s", s)
	}
	if !strings.Contains(s, "`missing.md` (not found in the worktree)") {
		t.Fatalf("missing ref:\n%s", s)
	}
	if _, err := Git(sess.Worktree, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	staged, _ := Git(sess.Worktree, "diff", "--cached", "--name-only")
	if strings.Contains(staged, ".troopa") {
		t.Fatalf("attachments should be excluded from git: %s", staged)
	}
}

func TestFilterRepoFiles(t *testing.T) {
	files := []string{"internal/tui/taskform.go", "README.md", "cmd/troopa/main.go"}
	got := FilterRepoFiles(files, "task")
	if len(got) != 1 || got[0] != "internal/tui/taskform.go" {
		t.Fatalf("%v", got)
	}
}

func TestReferencedPaths(t *testing.T) {
	got := ReferencedPaths("See @foo.go and @bar/baz.md.", "Also @foo.go")
	if len(got) != 2 || got[0] != "foo.go" || got[1] != "bar/baz.md" {
		t.Fatalf("%v", got)
	}
}

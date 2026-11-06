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

func setupMergeRepo(t *testing.T) (repo string, sess Session, task store.Task) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo = filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt")}
	task = store.Task{ID: 3, Title: "Add button", Type: store.TypeFeature, Commits: store.CommitsAuto}
	var err error
	sess, err = Prepare(cfg, store.Project{RepoPath: repo, Slug: "demo"}, task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "btn.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "SUMMARY.md"), []byte("did the button\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitAll(sess.Worktree, "troopa: "+task.Title); err != nil {
		t.Fatal(err)
	}
	head, err := Git(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != sess.Base {
		t.Fatalf("repo HEAD %s want %s", head, sess.Base)
	}
	return repo, sess, task
}

func TestMergeLocalSquash(t *testing.T) {
	repo, sess, task := setupMergeRepo(t)
	before, err := Git(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := MergeLocal(sess, task, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sess.Worktree); !os.IsNotExist(err) {
		t.Fatal("worktree still exists")
	}
	if BranchExists(repo, sess.Branch) {
		t.Fatal("branch still exists")
	}
	subject, err := Git(repo, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if subject != task.Title {
		t.Fatalf("subject %q", subject)
	}
	body, err := Git(repo, "log", "-1", "--format=%b")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "did the button") {
		t.Fatalf("body %q", body)
	}
	if _, err := Git(repo, "rev-parse", "HEAD^2"); err == nil {
		t.Fatal("squash should not be a merge commit")
	}
	after, _ := Git(repo, "rev-parse", "HEAD")
	if after == before {
		t.Fatal("no new commit")
	}
	if !strings.Contains(msg, after[:7]) && !strings.Contains(msg, "squash") {
		t.Fatalf("msg %q sha %s", msg, after)
	}
	if _, err := os.Stat(filepath.Join(repo, "btn.txt")); err != nil {
		t.Fatal("file missing on base")
	}
}

func TestMergeLocalNoFF(t *testing.T) {
	repo, sess, task := setupMergeRepo(t)
	msg, err := MergeLocal(sess, task, "merge")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Git(repo, "rev-parse", "HEAD^2"); err != nil {
		t.Fatalf("expected merge commit: %v", err)
	}
	subject, err := Git(repo, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	want := "Merge " + sess.Branch + ": " + task.Title
	if subject != want {
		t.Fatalf("subject %q want %q", subject, want)
	}
	if BranchExists(repo, sess.Branch) {
		t.Fatal("branch still exists")
	}
	if !strings.Contains(msg, "merge") {
		t.Fatalf("msg %q", msg)
	}
}

func TestMergeLocalConflictLeavesRepoIntact(t *testing.T) {
	repo, sess, task := setupMergeRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "btn.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitAll(repo, "change on base"); err != nil {
		t.Fatal(err)
	}
	before, _ := Git(repo, "rev-parse", "HEAD")
	_, err := MergeLocal(sess, task, "squash")
	if err == nil {
		t.Fatal("expected conflict")
	}
	if !strings.Contains(err.Error(), "btn.txt") {
		t.Fatalf("conflict files: %v", err)
	}
	if mergeInProgress(repo) {
		t.Fatal("MERGE_HEAD left behind")
	}
	after, _ := Git(repo, "rev-parse", "HEAD")
	if after != before {
		t.Fatal("HEAD moved")
	}
	st, _ := Git(repo, "status", "--porcelain")
	if strings.TrimSpace(st) != "" {
		t.Fatalf("dirty after abort: %q", st)
	}
	if !BranchExists(repo, sess.Branch) {
		t.Fatal("branch removed")
	}
	if _, err := os.Stat(sess.Worktree); err != nil {
		t.Fatal("worktree removed")
	}
}

func TestMergeLocalDirtyRepo(t *testing.T) {
	repo, sess, task := setupMergeRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := Git(repo, "rev-parse", "HEAD")
	_, err := MergeLocal(sess, task, "squash")
	if err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("want dirty error, got %v", err)
	}
	after, _ := Git(repo, "rev-parse", "HEAD")
	if after != before {
		t.Fatal("HEAD moved")
	}
	if !BranchExists(repo, sess.Branch) {
		t.Fatal("branch removed")
	}
}

func TestMergeLocalWrongBranch(t *testing.T) {
	repo, sess, task := setupMergeRepo(t)
	if _, err := Git(repo, "checkout", "-b", "other"); err != nil {
		t.Fatal(err)
	}
	before, _ := Git(repo, "rev-parse", "HEAD")
	_, err := MergeLocal(sess, task, "merge")
	if err == nil || !strings.Contains(err.Error(), "other") {
		t.Fatalf("want wrong-branch error, got %v", err)
	}
	after, _ := Git(repo, "rev-parse", "HEAD")
	if after != before {
		t.Fatal("HEAD moved")
	}
	cur, _ := Git(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if cur != "other" {
		t.Fatalf("left on %s", cur)
	}
}

func TestCloseFlowSquashMergeConflictCleanDiscard(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt")}
	p := store.Project{RepoPath: repo, Slug: "demo"}

	t.Log("approve without remote + squash")
	t1 := store.Task{ID: 1, Title: "Squash feature", Type: store.TypeFeature, Commits: store.CommitsAuto}
	s1, err := Prepare(cfg, p, t1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s1.Worktree, "squash.txt"), []byte("from squash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg, err := Approve(s1, t1, "")
	if err != nil {
		t.Fatal(err)
	}
	if HasRemote(s1.Repo) {
		t.Fatal("unexpected remote")
	}
	if !strings.Contains(msg, "no remote") {
		t.Fatalf("approve: %s", msg)
	}
	if _, err := MergeLocal(s1, t1, "squash"); err != nil {
		t.Fatal(err)
	}

	t.Log("no-ff merge")
	t2 := store.Task{ID: 2, Title: "Merge feature", Type: store.TypeFeature, Commits: store.CommitsAuto}
	s2, err := Prepare(cfg, p, t2)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s2.Worktree, "merge.txt"), []byte("from merge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitAll(s2.Worktree, "troopa: merge"); err != nil {
		t.Fatal(err)
	}
	if _, err := MergeLocal(s2, t2, "merge"); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(repo, "rev-parse", "HEAD^2"); err != nil {
		t.Fatal("expected merge commit")
	}

	t.Log("conflict leaves repo intact")
	t3 := store.Task{ID: 3, Title: "Conflict", Type: store.TypeFeature, Commits: store.CommitsAuto}
	s3, err := Prepare(cfg, p, t3)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s3.Worktree, "squash.txt"), []byte("agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitAll(s3.Worktree, "troopa: conflict"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "squash.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitAll(repo, "base change"); err != nil {
		t.Fatal(err)
	}
	before, _ := Git(repo, "rev-parse", "HEAD")
	if _, err := MergeLocal(s3, t3, "squash"); err == nil {
		t.Fatal("expected conflict")
	}
	after, _ := Git(repo, "rev-parse", "HEAD")
	if after != before {
		t.Fatal("HEAD moved")
	}

	t.Log("clean keeps branch")
	t4 := store.Task{ID: 4, Title: "Clean me", Type: store.TypeFeature, Commits: store.CommitsAuto}
	s4, err := Prepare(cfg, p, t4)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s4.Worktree, "clean.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitAll(s4.Worktree, "troopa: clean"); err != nil {
		t.Fatal(err)
	}
	if err := CleanWorktree(s4); err != nil {
		t.Fatal(err)
	}
	if !BranchExists(repo, s4.Branch) {
		t.Fatal("clean deleted branch")
	}
	ahead, err := BranchAheadOfBase(repo, s4.Base, s4.Branch)
	if err != nil || !ahead {
		t.Fatal("discard must warn: branch is ahead")
	}
	if err := Discard(s4); err != nil {
		t.Fatal(err)
	}
	if BranchExists(repo, s4.Branch) {
		t.Fatal("discard left branch")
	}
}

// A worktree that cannot be removed (untracked build output) must not turn a
// finished merge into an error.
func TestMergeLocalKeepsMergeWhenCleanupFails(t *testing.T) {
	repo, sess, task := setupMergeRepo(t)
	if err := os.WriteFile(filepath.Join(sess.Worktree, "build.bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg, err := MergeLocal(sess, task, "squash")
	if err != nil {
		t.Fatalf("merge reported as failed: %v", err)
	}
	subject, _ := Git(repo, "log", "-1", "--format=%s")
	if subject != task.Title {
		t.Fatalf("merge commit missing, got %q", subject)
	}
	if !strings.Contains(msg, "Cleanup pending") {
		t.Fatalf("expected cleanup warning, got %q", msg)
	}
	if !BranchExists(repo, sess.Branch) {
		t.Fatal("branch should be kept while its worktree exists")
	}
}

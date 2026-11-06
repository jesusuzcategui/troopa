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

func TestBranchName(t *testing.T) {
	got := BranchName("", "feature", "Fix Login")
	if got != "feature/fix-login" {
		t.Fatalf("got %q", got)
	}
	got = BranchName("troppa/", "bugfix", "X")
	if got != "troppa/bugfix/x" {
		t.Fatalf("prefix got %q", got)
	}
}

func TestPrepareWriteTASKAndDiscard(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt")}
	task := store.Task{ID: 7, Title: "Añadir botón", Type: store.TypeFeature, Commits: store.CommitsNone,
		Description: "Haz el botón", AcceptanceCriteria: "Se ve el botón"}
	sess, err := Prepare(cfg, store.Project{RepoPath: repo, Slug: "demo"}, task)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Branch != "feature/anadir-boton" && sess.Branch != "feature/añadir-botón" {
		if !strings.HasPrefix(sess.Branch, "feature/") {
			t.Fatalf("branch %q", sess.Branch)
		}
	}
	body, err := os.ReadFile(filepath.Join(sess.Worktree, "TASK.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "# Añadir botón") || !strings.Contains(s, "Se ve el botón") {
		t.Fatalf("TASK.md: %s", s)
	}
	if !strings.Contains(s, "Do not commit, push or create branches") {
		t.Fatal("commits=none rule missing")
	}
	if !strings.Contains(s, "Do not change tests just to make them pass") {
		t.Fatal("test rule missing")
	}
	cur, err := Git(sess.Worktree, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if cur != sess.Branch {
		t.Fatalf("HEAD %s want %s", cur, sess.Branch)
	}
	if err := Discard(sess); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sess.Worktree); !os.IsNotExist(err) {
		t.Fatal("worktree still exists")
	}
	if BranchExists(repo, sess.Branch) {
		t.Fatal("branch still exists")
	}
	main, _ := Git(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if main != "main" {
		t.Fatalf("main was touched: %s", main)
	}
}

func TestCleanWorktreeKeepsBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt")}
	task := store.Task{ID: 4, Title: "Keep branch", Type: store.TypeFeature, Commits: store.CommitsAuto}
	sess, err := Prepare(cfg, store.Project{RepoPath: repo, Slug: "demo"}, task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "ok.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitAll(sess.Worktree, "troopa: keep"); err != nil {
		t.Fatal(err)
	}
	if err := CleanWorktree(sess); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sess.Worktree); !os.IsNotExist(err) {
		t.Fatal("worktree still exists")
	}
	if !BranchExists(repo, sess.Branch) {
		t.Fatal("branch was deleted")
	}
}

func TestCleanWorktreeDirtyRefuses(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt")}
	task := store.Task{ID: 5, Title: "Dirty", Type: store.TypeFeature, Commits: store.CommitsNone}
	sess, err := Prepare(cfg, store.Project{RepoPath: repo, Slug: "demo"}, task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "dirty.txt"), []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CleanWorktree(sess); err == nil {
		t.Fatal("expected error for uncommitted changes")
	}
	if _, err := os.Stat(sess.Worktree); err != nil {
		t.Fatal("worktree should still exist")
	}
	if !BranchExists(repo, sess.Branch) {
		t.Fatal("branch should still exist")
	}
}

func TestApproveNoneLeavesUncommitted(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt")}
	task := store.Task{ID: 1, Title: "Cambio", Type: store.TypeFeature, Commits: store.CommitsNone}
	sess, err := Prepare(cfg, store.Project{RepoPath: repo, Slug: "demo"}, task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "hola.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg, err := Approve(sess, task, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, sess.Worktree) {
		t.Fatalf("msg should show path: %s", msg)
	}
	st, err := Git(sess.Worktree, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(st, "hola.txt") {
		t.Fatalf("expected uncommitted hola.txt, status=%q", st)
	}
	log, err := Git(sess.Worktree, "log", "--oneline")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(log, "troopa:") {
		t.Fatalf("should not commit: %s", log)
	}
}

func TestApproveAutoCommitAndPR(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	root := t.TempDir()
	bare := filepath.Join(root, "remote.git")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(bare, "init", "--bare", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(repo, "remote", "add", "origin", bare); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(repo, "push", "-u", "origin", "main"); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	gh := filepath.Join(bin, "gh")
	if err := os.WriteFile(gh, []byte("#!/bin/sh\necho https://github.com/example/demo/pull/3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := config.Launch{Worktrees: filepath.Join(root, "wt")}
	task := store.Task{ID: 2, Title: "PR demo", Type: store.TypeFeature, Commits: store.CommitsAuto}
	sess, err := Prepare(cfg, store.Project{RepoPath: repo, Slug: "demo"}, task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "x.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	msg, err := Approve(sess, task, "hecho")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "pull/3") {
		t.Fatalf("want PR url, got %s", msg)
	}
	if IsProtected(sess.Branch) {
		t.Fatal("pushed protected")
	}
	_, err = Git(bare, "rev-parse", "refs/heads/"+sess.Branch)
	if err != nil {
		t.Fatalf("branch not on remote: %v", err)
	}
	_, err = Git(bare, "rev-parse", "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRefuseProtectedPush(t *testing.T) {
	if err := PushBranch("/tmp", "main"); err == nil {
		t.Fatal("expected error")
	}
	if !IsProtected("master") {
		t.Fatal("master protected")
	}
}

func TestWorktreeDiffColorsParse(t *testing.T) {
	raw := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n"
	files := ParseDiff(raw)
	if len(files) != 1 || files[0].Path != "a.go" {
		t.Fatalf("%+v", files)
	}
	kinds := ""
	for _, l := range files[0].Lines {
		kinds += string(l.Kind)
	}
	if !strings.Contains(kinds, "+") || !strings.Contains(kinds, "-") {
		t.Fatalf("kinds %s", kinds)
	}
}

func TestExpandArgsAndDefaultAgents(t *testing.T) {
	args := ExpandArgs([]string{"-p", "{{prompt}}", "--dangerously-skip-permissions"}, "hola", "")
	if args[1] != "hola" {
		t.Fatalf("%v", args)
	}
	l := config.DefaultLaunch(t.TempDir())
	for _, name := range []string{"claude", "codex", "opencode", "pi"} {
		if l.Agents[name].Command == "" {
			t.Fatalf("missing %s", name)
		}
		if !HasPlaceholder(l.Agents[name].Args) {
			t.Fatalf("%s args need {{prompt}}: %v", name, l.Agents[name].Args)
		}
	}
	if !strings.Contains(strings.Join(l.Agents["claude"].Args, " "), "--dangerously-skip-permissions") {
		t.Fatal("claude yolo flag")
	}
	if !strings.Contains(strings.Join(l.Agents["codex"].Args, " "), "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatal("codex yolo flag")
	}
	if !strings.Contains(strings.Join(l.Agents["opencode"].Args, " "), "--auto") {
		t.Fatal("opencode --auto")
	}
}

func TestAgentRunsInWorktree(t *testing.T) {
	t.Setenv("PWD", "/somewhere/else")
	wt := t.TempDir()
	env := AgentEnv(wt)
	pwd := ""
	for _, kv := range env {
		if strings.HasPrefix(kv, "PWD=") {
			if pwd != "" {
				t.Fatal("PWD set twice")
			}
			pwd = strings.TrimPrefix(kv, "PWD=")
		}
	}
	if pwd != wt {
		t.Fatalf("PWD = %q, want %q", pwd, wt)
	}
	if p := YoloPrompt(wt); !strings.Contains(p, filepath.Join(wt, "TASK.md")) || !strings.Contains(p, filepath.Join(wt, "SUMMARY.md")) {
		t.Fatalf("prompt does not name the worktree: %s", p)
	}
}

func TestPrepareTASKIncludesBrainContext(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	brainDir := t.TempDir()
	proj := store.Project{RepoPath: repo, Slug: "demo", Name: "Demo"}
	if err := os.MkdirAll(filepath.Join(brainDir, "projects", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: \"Demo\"\n---\n# Demo\n\n## Current state\n\nLive on staging.\n\n## Next\n\nShip it.\n\n## Decisions\n\nSQLite.\n"
	if err := os.WriteFile(filepath.Join(brainDir, "projects", "demo", "project.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt"), Brain: brainDir}
	sess, err := Prepare(cfg, proj, store.Task{ID: 1, Title: "Hello", Type: store.TypeFeature})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(sess.Worktree, "TASK.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "## Project context") || !strings.Contains(s, "Live on staging.") {
		t.Fatalf("TASK.md missing brain context:\n%s", s)
	}
	if !strings.Contains(s, "Do not modify the second brain unless this task asks you to") {
		t.Fatal("brain rule missing")
	}
}

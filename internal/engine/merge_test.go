package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// A repo with a bare remote: merge, PR and pull are separate ways out.
func TestMergePRAndPullAreSeparate(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	if _, err := launch.Git(root, "init", "--bare", "-b", "main", remote); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte("checks = [\"true\"]\n"), 0o644)
	_, _ = launch.Git(repo, "add", "troopa.toml")
	_, _ = launch.Git(repo, "commit", "-m", "checks")
	branch, _ := launch.Git(repo, "rev-parse", "--abbrev-ref", "HEAD")
	branch = strings.TrimSpace(branch)
	if _, err := launch.Git(repo, "remote", "add", "origin", remote); err != nil {
		t.Fatal(err)
	}
	if _, err := launch.Git(repo, "push", "-u", "origin", branch); err != nil {
		t.Fatal(err)
	}
	fakeClaude(t, "#!/bin/sh\necho hi > hello.txt\necho done > SUMMARY.md\n")
	e, s := testEngine(t)
	p, _ := s.Create(store.Project{Name: "App", Slug: "app", RepoPath: repo})
	task, _ := s.CreateTask(store.Task{ProjectID: p.ID, Title: "Hello", Agent: store.AgentClaude, Mode: store.ModeYolo,
		Status: store.StatusBacklog, Commits: store.CommitsNone})
	res, err := e.Launch(task)
	if err != nil {
		t.Fatal(err)
	}
	waitAgent(t, res.Run.Worktree)
	if _, err := e.Finish(res.Run.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Approve(res.Run.ID); err != nil {
		t.Fatal(err)
	}
	merge, pr := e.MergeOptions(res.Run.ID)
	if !merge || !pr {
		t.Fatalf("with a remote both merge and PR should be offered: merge=%v pr=%v", merge, pr)
	}
	// The PR pushes the branch (gh is missing in tests, so no PR URL).
	prRes, err := e.OpenTaskPR(res.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prRes.Message, res.Run.Branch) {
		t.Fatalf("PR message %q", prRes.Message)
	}
	if out, _ := launch.Git(remote, "branch", "--list", res.Run.Branch); !strings.Contains(out, res.Run.Branch) {
		t.Fatalf("branch should be on the remote: %q", out)
	}
	// A local merge is still possible.
	if _, err := e.MergeTask(res.Run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "hello.txt")); err != nil {
		t.Fatal("the merge should bring hello.txt to the base branch")
	}

	// Someone else pushes to the remote; Pull fast-forwards the base branch.
	other := filepath.Join(root, "other")
	if _, err := launch.Git(root, "clone", remote, other); err != nil {
		t.Fatal(err)
	}
	_, _ = launch.Git(repo, "push", "origin", branch)
	_, _ = launch.Git(other, "pull")
	_ = os.WriteFile(filepath.Join(other, "news.txt"), []byte("news"), 0o644)
	_, _ = launch.Git(other, "add", "news.txt")
	_, _ = launch.Git(other, "-c", "user.name=x", "-c", "user.email=x@x", "commit", "-m", "news")
	if _, err := launch.Git(other, "push", "origin", "HEAD:"+branch); err != nil {
		t.Fatal(err)
	}
	msg, err := e.Pull(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "news.txt")); err != nil {
		t.Fatalf("pull should bring news.txt: %s", msg)
	}
}

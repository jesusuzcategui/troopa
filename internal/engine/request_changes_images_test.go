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

func TestRequestChangesRoundImages(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte("checks = [\"true\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeClaude(t, "#!/bin/sh\necho done > SUMMARY.md\n")
	e, s := testEngine(t)
	p, err := s.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(store.Task{
		ProjectID: p.ID, Title: "Shot", Agent: store.AgentClaude, Mode: store.ModeYolo,
		Status: store.StatusBacklog, Commits: store.CommitsNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Launch(task)
	if err != nil {
		t.Fatal(err)
	}
	waitAgent(t, res.Run.Worktree)
	done, err := e.Finish(res.Run.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(t.TempDir(), "screen.png")
	if err := os.WriteFile(img, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := e.RequestChanges(done.Run.ID, "fix the layout", nil, []string{img})
	if err != nil {
		t.Fatal(err)
	}
	if !again.Watch {
		t.Fatalf("expected watch")
	}
	atts, err := s.ListAttachments(task.ID)
	if err != nil || len(atts) != 1 {
		t.Fatalf("attachments %+v err=%v", atts, err)
	}
	wt := again.Session.Worktree
	copied := filepath.Join(wt, ".troopa", "attachments", atts[0].Name)
	if _, err := os.Stat(copied); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(wt, "CHANGES.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, "fix the layout") || !strings.Contains(body, copied) {
		t.Fatalf("CHANGES.md:\n%s", body)
	}
	launch.StopAgent(wt)
}

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestReviewCommentCursorEditListAndRequestChanges(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho \"$*\" > last_prompt.txt\nprintf 'hello\\nworld\\n' > work.txt\necho done > SUMMARY.md\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	m := newTestModel(t)
	m.cfg.Launch.Worktrees = filepath.Join(t.TempDir(), "wt")
	m.cfg.Launch.Notify = false
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Hola", Agent: store.AgentClaude, Mode: store.ModeYolo, Status: store.StatusBacklog}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	m.width, m.height = 120, 40
	got := press(t, m, "enter")
	got.colIndex = indexOfStatus(store.StatusBacklog)
	got.cardIndex = 0
	got.clampBoardCursor()
	got = press(t, got, "]", "enter")
	cmd, err := launch.AgentCommand(got.cfg.Launch, store.AgentClaude, "", got.runSess.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := got.Update(runAgent(cmd, got.runSess.LogPath, got.run.ID)())
	got = next.(Model)
	if got.screen != screenReview {
		t.Fatalf("want review, screen %v err %s", got.screen, got.err)
	}
	wt := got.runSess.Worktree
	plus := -1
	for fi, file := range got.diffFiles {
		for i, line := range file.Lines {
			if line.Kind == '+' && strings.TrimSpace(line.Quote()) != "" {
				got.diffIndex = fi
				plus = i
				break
			}
		}
		if plus >= 0 {
			break
		}
	}
	if plus < 0 {
		t.Fatalf("no added line in diff: %+v", got.diffFiles)
	}
	got.diffCursor = plus
	f := got.diffFiles[got.diffIndex]
	line := f.Lines[plus]
	num, _ := line.CommentTarget()

	got = press(t, got, "enter")
	if got.screen != screenComment {
		t.Fatalf("enter should open comment, screen %v", got.screen)
	}
	got.commentInput.SetValue("say hello louder")
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	got = next.(Model)
	if got.screen != screenReview || got.pendingCommentCount() != 1 {
		t.Fatalf("saved comment: screen %v pending %d err %s", got.screen, got.pendingCommentCount(), got.err)
	}
	view := got.viewReview()
	if !strings.Contains(view, "say hello louder") || !strings.Contains(view, "pending comment") {
		t.Fatalf("review should show comment and count: %s", view)
	}

	got = press(t, got, "C")
	if got.screen != screenCommentList || !strings.Contains(got.viewCommentList(), "say hello louder") {
		t.Fatalf("C should list comments, screen %v", got.screen)
	}
	got = press(t, got, "enter")
	if got.screen != screenReview || got.diffCursor != plus {
		t.Fatalf("jump: screen %v cursor %d want %d", got.screen, got.diffCursor, plus)
	}

	got = press(t, got, "e")
	if got.screen != screenComment {
		t.Fatalf("e should edit, screen %v", got.screen)
	}
	got.commentInput.SetValue("say hello much louder")
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	got = next.(Model)

	got = press(t, got, "x")
	if got.screen != screenChanges {
		t.Fatalf("x should open changes, screen %v", got.screen)
	}
	if !strings.Contains(got.viewChanges(), "say hello much louder") {
		t.Fatalf("changes should list pending comments: %s", got.viewChanges())
	}
	got.changesInput.SetValue("")
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	got = next.(Model)
	if got.err != "" && got.screen == screenChanges {
		t.Fatalf("comments-only send should work: %s", got.err)
	}
	raw, _ := os.ReadFile(filepath.Join(wt, "CHANGES.md"))
	body := string(raw)
	if !strings.Contains(body, f.Path+":") || !strings.Contains(body, "say hello much louder") {
		t.Fatalf("CHANGES.md missing line comment:\n%s", body)
	}
	if !strings.Contains(body, ">") {
		t.Fatalf("CHANGES.md should quote the line:\n%s", body)
	}
	pending, err := got.store.ListPendingComments(got.run.ID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("comments should be resolved: %+v %v", pending, err)
	}
	all, _ := got.store.ListComments(got.run.ID)
	if len(all) != 1 || !all[0].Resolved || all[0].Line != num {
		t.Fatalf("resolved comment %+v want line %d", all, num)
	}
}

func TestReviewCommentDeletePopup(t *testing.T) {
	m := newTestModel(t)
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := m.store.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, State: store.RunReview})
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.store.CreateComment(store.ReviewComment{RunID: run.ID, Path: "main.go", Line: 1, Side: "+", Quote: "x", Body: "fix me"})
	if err != nil {
		t.Fatal(err)
	}
	m.bindRun(task, run)
	m.loadComments()
	m.diffFiles = launch.ParseDiff("diff --git a/main.go b/main.go\n@@ -0,0 +1,1 @@\n+x\n")
	m.diffCursor = 0
	for i, line := range m.diffFiles[0].Lines {
		if line.Kind == '+' {
			m.diffCursor = i
			break
		}
	}
	m.screen = screenReview
	m.width, m.height = 80, 24
	got := press(t, m, "D")
	if got.popup == nil || got.popup.kind != propConfirmDeleteComment {
		t.Fatalf("D should confirm delete, popup=%+v", got.popup)
	}
	got = press(t, got, "enter") // cursor defaults to No
	if _, err := got.store.GetComment(c.ID); err != nil {
		t.Fatal("esc/no should keep the comment")
	}
	got = press(t, got, "D")
	got.popup.cursor = 0
	got = press(t, got, "enter")
	if _, err := got.store.GetComment(c.ID); err == nil {
		t.Fatal("yes should delete the comment")
	}
}

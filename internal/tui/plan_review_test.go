package tui

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestPlanReviewFullFlow(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	xdgData := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	hello := filepath.Join(repo, "hello.go")
	if err := os.WriteFile(hello, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := launch.Git(repo, "add", "hello.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := launch.Git(repo, "commit", "-m", "hello"); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho working\nprintf 'alpha\\nbeta\\n' > extra.txt\nprintf '## How to run\\n\\necho hi\\n' > SUMMARY.md\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	m := newTestModel(t)
	m.cfg.Launch.Worktrees = filepath.Join(t.TempDir(), "wt")
	m.cfg.Launch.Notify = false
	m.cfg.BrainGit = true
	m.width, m.height = 140, 40
	if err := brain.Ensure(m.brainDir()); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = m.brainDir()
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	git("init", "-b", "main")
	git("add", "-A")
	git("commit", "-m", "brain start")

	p, err := m.store.Create(store.Project{Name: "App", Slug: "app", RepoPath: repo, Stack: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	m.rememberProject(p)
	hand := "Hand-written current state — do not clobber."
	proj := brain.ProjectFile(m.brainDir(), "app")
	raw, err := os.ReadFile(proj)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(raw), "## Current state\n", "## Current state\n\n"+hand+"\n", 1)
	if err := os.WriteFile(proj, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	task, err := m.store.CreateTask(store.Task{
		ProjectID: p.ID, Title: "Ship notes", Agent: store.AgentClaude, Mode: store.ModeYolo,
		Status: store.StatusBacklog, Commits: store.CommitsAuto,
		Description: "Use @hello.go as a reference.",
	})
	if err != nil {
		t.Fatal(err)
	}
	attDir, err := config.AttachmentsDir(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"shot.png": {0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82},
		"spec.pdf": []byte("%PDF-1.1\n%%EOF\n"),
		"util.go":  []byte("package util\n"),
	}
	for name, data := range files {
		src := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(src, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := m.store.AttachFile(task.ID, src, attDir, 50<<20); err != nil {
			t.Fatal(err)
		}
	}
	m.rememberTask(task)
	m.reload()
	m = press(t, m, "enter")
	m.colIndex = indexOfStatus(store.StatusBacklog)
	m.cardIndex = 0
	m.clampBoardCursor()
	m = press(t, m, "]", "enter")
	if m.screen != screenLog {
		t.Fatalf("launch: screen %v err %s", m.screen, m.err)
	}
	cmd, err := launch.AgentCommand(m.cfg.Launch, store.AgentClaude, "", m.runSess.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(runAgent(cmd, m.runSess.LogPath, m.run.ID)())
	m = next.(Model)
	if m.screen != screenReview {
		t.Fatalf("review: screen %v err %s", m.screen, m.err)
	}
	wt := m.runSess.Worktree
	taskMD, _ := os.ReadFile(filepath.Join(wt, "TASK.md"))
	if !strings.Contains(string(taskMD), ".troopa/attachments/shot.png") || !strings.Contains(string(taskMD), "@hello.go") {
		t.Fatalf("TASK.md attachments/@ missing:\n%s", taskMD)
	}
	if !strings.Contains(string(taskMD), "Hand-written current state") {
		t.Fatalf("TASK.md missing project context:\n%s", taskMD)
	}
	plus := plusLines(m)
	if len(plus) < 2 {
		t.Fatalf("need two added lines, got %+v files %+v", plus, m.diffFiles)
	}
	m.diffIndex, m.diffCursor = plus[0].file, plus[0].line
	m = press(t, m, "2", "enter")
	m.commentInput.SetValue("first line note")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	m.diffIndex, m.diffCursor = plus[1].file, plus[1].line
	m = press(t, m, "enter")
	m.commentInput.SetValue("second line note")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	if m.pendingCommentCount() != 2 {
		t.Fatalf("pending %d", m.pendingCommentCount())
	}
	m = press(t, m, "x")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	if m.screen != screenLog {
		t.Fatalf("request changes: screen %v err %s", m.screen, m.err)
	}
	changes, _ := os.ReadFile(filepath.Join(wt, "CHANGES.md"))
	if !strings.Contains(string(changes), "first line note") || !strings.Contains(string(changes), "second line note") {
		t.Fatalf("CHANGES.md:\n%s", changes)
	}
	cmd, err = launch.AgentCommandPrompt(m.cfg.Launch, store.AgentClaude, "", wt, launch.ChangesPrompt(wt, 1))
	if err != nil {
		t.Fatal(err)
	}
	next, _ = m.Update(runAgent(cmd, m.runSess.LogPath, m.run.ID)())
	m = next.(Model)
	if m.screen != screenReview {
		t.Fatalf("second round: screen %v err %s", m.screen, m.err)
	}
	rounds, err := m.store.ListRounds(m.run.ID)
	if err != nil || len(rounds) != 2 {
		t.Fatalf("rounds %+v err %v", rounds, err)
	}
	st, err := launch.Git(wt, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(st, "CHANGES.md") || strings.Contains(st, ".troopa") {
		t.Fatalf("work files in git status:\n%s", st)
	}

	m = press(t, m, "y")
	if m.err != "" {
		t.Fatal(m.err)
	}
	m.selectTaskID(task.ID)
	m = press(t, m, "g")
	if m.popup == nil {
		t.Fatalf("expected merge confirm, err=%s status=%s", m.err, m.status)
	}
	m = press(t, m, "1")
	if m.err != "" {
		t.Fatal(m.err)
	}
	run, err := m.store.GetRun(m.run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.State != store.RunMerged {
		t.Fatalf("state %s", run.State)
	}
	note, err := os.ReadFile(proj)
	if err != nil {
		t.Fatal(err)
	}
	s := string(note)
	if !strings.Contains(s, hand) {
		t.Fatalf("project.md lost handwriting:\n%s", s)
	}
	if strings.Count(s, "troopa:approve:") != 1 || strings.Count(s, "troopa:merge:") != 1 {
		t.Fatalf("history markers:\n%s", s)
	}
	guides, _ := filepath.Glob(filepath.Join(m.brainDir(), "projects", "*", "guides", "*.md"))
	if len(guides) == 0 {
		t.Fatal("expected a merge guide")
	}

	first, err := brain.Sync(m.brainSettings(), m.store)
	if err != nil {
		t.Fatal(err)
	}
	before := readTree(t, m.brainDir())
	second, err := brain.Sync(m.brainSettings(), m.store)
	if err != nil {
		t.Fatal(err)
	}
	after := readTree(t, m.brainDir())
	if !bytes.Equal(before, after) {
		t.Fatal("brain sync is not idempotent")
	}
	if second.Committed {
		t.Fatal("second sync should not commit")
	}
	_ = first
	if !strings.Contains(string(after), hand) {
		t.Fatal("sync dropped handwriting")
	}

	m = press(t, m, "esc", "B")
	if m.screen != screenBrain {
		t.Fatalf("brain screen %v", m.screen)
	}
	m = press(t, m, "i")
	for _, r := range "follow up from review" {
		m = press(t, m, string(r))
	}
	m = press(t, m, "enter")
	inbox, _ := brain.ReadNote(m.brainDir(), "inbox.md")
	if !strings.Contains(inbox, "follow up from review") {
		t.Fatalf("inbox %q", inbox)
	}
}

type plusHit struct{ file, line int }

func plusLines(m Model) []plusHit {
	var out []plusHit
	for fi, file := range m.diffFiles {
		for i, line := range file.Lines {
			if line.Kind == '+' && strings.TrimSpace(line.Quote()) != "" {
				out = append(out, plusHit{file: fi, line: i})
			}
		}
	}
	return out
}

func readTree(t *testing.T, dir string) []byte {
	t.Helper()
	var b bytes.Buffer
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.WriteString(rel)
		b.WriteByte('\n')
		b.Write(body)
		b.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

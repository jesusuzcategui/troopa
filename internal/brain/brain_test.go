package brain

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestCommitWhenGitEnabled(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	dir := t.TempDir()
	cmd := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	cmd("init", "-b", "main")
	s := Settings{Dir: dir, Git: true}
	p := store.Project{Name: "App", Slug: "app", RepoPath: "/a"}
	if err := OnProject(s, []store.Project{p}, p); err != nil {
		t.Fatal(err)
	}
	out, err := gitOut(dir, "log", "-1", "--pretty=%s")
	if err != nil {
		t.Fatal(err)
	}
	if out != "troopa: update app" {
		t.Fatalf("commit %q", out)
	}
}

func TestEnsureCreatesLayout(t *testing.T) {
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"README.md", "inbox.md", "today.md", "projects/INDEX.md"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Fatal(p, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "inbox.md"), []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "inbox.md"))
	if string(got) != "keep me\n" {
		t.Fatalf("Ensure overwrote inbox: %q", got)
	}
}

func TestUpsertProjectKeepsSections(t *testing.T) {
	dir := t.TempDir()
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	p := store.Project{Name: "App", Slug: "app", RepoPath: "/repo", Stack: "Go"}
	if err := UpsertProject(dir, p); err != nil {
		t.Fatal(err)
	}
	path := ProjectFile(dir, "app")
	custom := "---\nname: \"old\"\n---\n# App\n\n## Current state\n\nHand-written state.\n\n## Next\n\nDo the thing.\n\n## Decisions\n\nUse SQLite.\n\n## History\n\n"
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	p.Stack = "Go · Bubble Tea"
	if err := UpsertProject(dir, p); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, `stack: "Go · Bubble Tea"`) {
		t.Fatalf("frontmatter not updated:\n%s", s)
	}
	if !strings.Contains(s, "Hand-written state.") || !strings.Contains(s, "Do the thing.") || !strings.Contains(s, "Use SQLite.") {
		t.Fatalf("sections overwritten:\n%s", s)
	}
	if strings.Contains(s, `name: "old"`) {
		t.Fatal("old frontmatter kept")
	}
}

func TestAppendHistoryNoDuplicate(t *testing.T) {
	dir := t.TempDir()
	p := store.Project{Name: "App", Slug: "app"}
	if err := UpsertProject(dir, p); err != nil {
		t.Fatal(err)
	}
	if err := AppendHistory(dir, "app", "troopa:approve:1", "- 2026-11-06 approved «T»"); err != nil {
		t.Fatal(err)
	}
	if err := AppendHistory(dir, "app", "troopa:approve:1", "- 2026-11-06 approved «T»"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(ProjectFile(dir, "app"))
	if n := strings.Count(string(got), "troopa:approve:1"); n != 1 {
		t.Fatalf("marker count %d:\n%s", n, got)
	}
}

func TestWriteIndexAndTask(t *testing.T) {
	dir := t.TempDir()
	projects := []store.Project{
		{Name: "App", Slug: "app", Stack: "Go"},
		{Name: "Old", Slug: "old", Archived: true},
	}
	if err := WriteIndex(dir, projects); err != nil {
		t.Fatal(err)
	}
	idx, _ := os.ReadFile(IndexFile(dir))
	if !strings.Contains(string(idx), "| app | App | active | Go |") || !strings.Contains(string(idx), "| old | Old | archived |") {
		t.Fatalf("INDEX:\n%s", idx)
	}
	t1 := store.Task{ID: 7, Title: "Add button", Status: store.StatusReady, Description: "Do it"}
	if err := WriteTask(dir, projects[0], t1); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(TaskFile(dir, "app", t1))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Add button") || !strings.Contains(string(body), `status: "ready"`) {
		t.Fatalf("task:\n%s", body)
	}
}

func TestHowToRunAndGuide(t *testing.T) {
	sum := "## How to run\n\n```\ngo test\n```\n\n## Notes\n\nmore"
	if got := HowToRun(sum); got != "```\ngo test\n```" {
		t.Fatalf("how %q", got)
	}
	dir := t.TempDir()
	p := store.Project{Name: "App", Slug: "app"}
	task := store.Task{ID: 1, Title: "Hello"}
	run := store.Run{Summary: sum, Note: "ok", UpdatedAt: time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	if err := WriteGuide(dir, p, task, run, "2026-11-06"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(GuideFile(dir, "app", "2026-11-06", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "go test") || !strings.Contains(string(got), "## Summary") {
		t.Fatalf("guide:\n%s", got)
	}
}

func TestTASKContext(t *testing.T) {
	dir := t.TempDir()
	p := store.Project{Name: "App", Slug: "app"}
	if err := UpsertProject(dir, p); err != nil {
		t.Fatal(err)
	}
	path := ProjectFile(dir, "app")
	raw, _ := os.ReadFile(path)
	body := string(raw)
	body = strings.Replace(body, "## Current state\n", "## Current state\n\nShipped v1.\n", 1)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := TASKContext(dir, "app")
	if !strings.Contains(ctx, "## Project context") || !strings.Contains(ctx, "Shipped v1.") {
		t.Fatalf("ctx:\n%s", ctx)
	}
	abs, _ := filepath.Abs(path)
	if !strings.Contains(ctx, abs) {
		t.Fatalf("missing abs path:\n%s", ctx)
	}
}

func TestSyncIdempotent(t *testing.T) {
	db := filepath.Join(t.TempDir(), "db")
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.Create(store.Project{Name: "App", Slug: "app", RepoPath: "/a", Stack: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := st.CreateTask(store.Task{ProjectID: p.ID, Title: "Hello"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, Agent: "me", State: store.RunApproved, Branch: "feature/hello", Summary: "## How to run\n\necho hi\n"})
	if err != nil {
		t.Fatal(err)
	}
	_ = run
	dir := t.TempDir()
	s := Settings{Dir: dir}
	if _, err := Sync(s, st); err != nil {
		t.Fatal(err)
	}
	path := ProjectFile(dir, "app")
	custom := "---\nname: \"App\"\n---\n# App\n\n## Current state\n\nKeep me.\n\n## Next\n\n## Decisions\n\n## History\n\n<!-- troopa:approve:" + strconv.FormatInt(run.ID, 10) + " -->\n- already\n"
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(s, st); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Keep me.") {
		t.Fatalf("sync overwrote Current state:\n%s", got)
	}
	if strings.Count(string(got), "troopa:approve:") != 1 {
		t.Fatalf("duplicated history:\n%s", got)
	}
}

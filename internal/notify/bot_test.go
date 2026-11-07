//go:build unix

package notify

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/engine"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

type fakeTG struct {
	mu    sync.Mutex
	calls []map[string]string
	next  int64
}

func (f *fakeTG) handler(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	m := map[string]string{"method": r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]}
	for k := range r.PostForm {
		m[k] = r.PostForm.Get(k)
	}
	f.mu.Lock()
	f.calls = append(f.calls, m)
	f.next++
	id := f.next
	f.mu.Unlock()
	res, _ := json.Marshal(map[string]any{"message_id": id, "chat": map[string]any{"id": 42}})
	_, _ = io.WriteString(w, `{"ok":true,"result":`+string(res)+`}`)
}

func (f *fakeTG) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if c["method"] == "sendMessage" {
			out = append(out, c["text"])
		}
	}
	return out
}

func botFixture(t *testing.T) (*Bot, *fakeTG, *engine.Engine, store.Run) {
	t.Helper()
	fake := &fakeTG{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(srv.Close)
	old := TelegramAPI
	TelegramAPI = srv.URL
	t.Cleanup(func() { TelegramAPI = old })

	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte("checks = [\"true\"]\n"), 0o644)
	_, _ = launch.Git(repo, "add", "troopa.toml")
	_, _ = launch.Git(repo, "commit", "-m", "checks")
	bin := filepath.Join(t.TempDir(), "bin")
	_ = os.MkdirAll(bin, 0o755)
	_ = os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho step >> steps.txt\necho done > SUMMARY.md\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	t.Cleanup(func() {
		runs, _ := s.ListRuns("")
		var wts []string
		for _, r := range runs {
			wts = append(wts, r.Worktree)
		}
		launch.WaitAgents(wts, 10*time.Second)
	})
	cfg := config.Config{BrainRaw: filepath.Join(dir, "brain"), DefaultAgent: store.AgentClaude}
	cfg.Launch = config.DefaultLaunch(filepath.Join(dir, "data"))
	cfg.Launch.Brain = cfg.BrainRaw
	cfg.Launch.Notify = false
	e := engine.New(s, cfg, filepath.Join(dir, "config.toml"))
	p, _ := s.Create(store.Project{Name: "App", Slug: "app", RepoPath: repo})
	plan, _ := s.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Menu", Agent: store.AgentClaude, Auto: true})
	_, _ = s.AddPlanStep(plan.ID, store.Task{Title: "One", Commits: store.CommitsNone})
	_, _ = s.AddPlanStep(plan.ID, store.Task{Title: "Two", Commits: store.CommitsNone})
	res, err := e.StartPlan(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	launch.WaitAgents([]string{res.Run.Worktree}, 10*time.Second)
	fin, err := e.Finish(res.Run.ID, nil)
	if err != nil || fin.Run.State != store.RunReview {
		t.Fatalf("finish %+v %v", fin.Run.State, err)
	}
	return NewBot("TOKEN", "42", e, dir, log.New(io.Discard, "", 0)), fake, e, fin.Run
}

func cb(data string) tgUpdate {
	return tgUpdate{CallbackQuery: &tgCallback{ID: "c1", Data: data, Message: &tgMessage{MessageID: 7, Chat: tgChat{ID: 42}}}}
}

func msg(text string, replyTo int64) tgUpdate {
	m := &tgMessage{MessageID: 99, Chat: tgChat{ID: 42}, Text: text}
	if replyTo != 0 {
		m.ReplyTo = &tgMessage{MessageID: replyTo}
	}
	return tgUpdate{Message: m}
}

func TestBotApproveAdvancesPlan(t *testing.T) {
	b, fake, e, run := botFixture(t)
	b.Handle(context.Background(), cb("approve:"+itoa(run.ID)))
	got, _ := e.Store.GetRun(run.ID)
	if got.State != store.RunApproved {
		t.Fatalf("state %s", got.State)
	}
	if active, err := e.Store.ActiveRun(); err != nil || active.TaskID == run.TaskID {
		t.Fatalf("next step should be running: %+v %v", active, err)
	}
	if s := fake.sent(); len(s) == 0 || !strings.Contains(s[len(s)-1], "Approved") {
		t.Fatalf("sent %v", s)
	}
}

func TestBotRequestChangesByReply(t *testing.T) {
	b, fake, e, run := botFixture(t)
	b.Handle(context.Background(), cb("changes:"+itoa(run.ID)))
	fake.mu.Lock()
	promptID := fake.next
	force := fake.calls[len(fake.calls)-1]["reply_markup"]
	fake.mu.Unlock()
	if !strings.Contains(force, "force_reply") {
		t.Fatalf("prompt should force a reply: %q", force)
	}
	b.Handle(context.Background(), msg("Use metric units", promptID))
	got, _ := e.Store.GetRun(run.ID)
	if got.State != store.RunRunning {
		t.Fatalf("state %s", got.State)
	}
	changes, _ := os.ReadFile(filepath.Join(run.Worktree, "CHANGES.md"))
	if !strings.Contains(string(changes), "Use metric units") {
		t.Fatalf("CHANGES.md: %s", changes)
	}
}

func TestBotCommandsAndStrangers(t *testing.T) {
	b, fake, _, _ := botFixture(t)
	b.Handle(context.Background(), msg("/plans", 0))
	b.Handle(context.Background(), msg("/plan 1", 0))
	b.Handle(context.Background(), msg("/status", 0))
	b.Handle(context.Background(), msg("/task app Dark mode toggle", 0))
	sent := fake.sent()
	all := strings.Join(sent, "\n---\n")
	for _, want := range []string{"Menu", "1. One", "👀", "Dark mode toggle"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in\n%s", want, all)
		}
	}
	before := len(sent)
	stranger := tgUpdate{Message: &tgMessage{MessageID: 5, Chat: tgChat{ID: 666}, Text: "/plans"}}
	b.Handle(context.Background(), stranger)
	b.Handle(context.Background(), tgUpdate{CallbackQuery: &tgCallback{ID: "x", Data: "approve:1", Message: &tgMessage{Chat: tgChat{ID: 666}}}})
	if len(fake.sent()) != before {
		t.Fatal("messages from other chats must be ignored")
	}
}

func TestKeyboardButtons(t *testing.T) {
	kb := Keyboard(launch.HookEvent{Event: launch.HookReview, RunID: 3})
	raw, _ := json.Marshal(kb)
	for _, want := range []string{"approve:3", "changes:3", "diff:3", "log:3"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing %s in %s", want, raw)
		}
	}
	if kb := Keyboard(launch.HookEvent{Event: launch.HookBlocked, RunID: 3}); strings.Contains(mustJSON(kb), "approve") {
		t.Fatal("a blocked run cannot be approved from Telegram")
	}
}

func mustJSON(v any) string { raw, _ := json.Marshal(v); return string(raw) }

func (f *fakeTG) last(method string) map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.calls) - 1; i >= 0; i-- {
		if f.calls[i]["method"] == method {
			return f.calls[i]
		}
	}
	return nil
}

func TestBotButtonsNavigatePlans(t *testing.T) {
	b, fake, e, run := botFixture(t)
	ctx := context.Background()
	b.Handle(ctx, cb("plans"))
	edit := fake.last("editMessageText")
	if edit == nil || edit["message_id"] != "7" || !strings.Contains(edit["reply_markup"], `"plan:1"`) {
		t.Fatalf("plans should redraw the message with a button per plan: %v", edit)
	}
	b.Handle(ctx, cb("plan:1"))
	edit = fake.last("editMessageText")
	approve := "approve:" + itoa(run.ID) + "|plan:1"
	for _, want := range []string{approve, "changes:" + itoa(run.ID), "ppause:1", `"plans"`} {
		if !strings.Contains(edit["reply_markup"], want) {
			t.Fatalf("plan view lacks %s: %s", want, edit["reply_markup"])
		}
	}
	if !strings.Contains(edit["text"], "1. One") {
		t.Fatalf("plan text %q", edit["text"])
	}
	b.Handle(ctx, cb(approve))
	if got, _ := e.Store.GetRun(run.ID); got.State != store.RunApproved {
		t.Fatalf("state %s", got.State)
	}
	if edit = fake.last("editMessageText"); !strings.Contains(edit["text"], "✅ 1. One") {
		t.Fatalf("the plan should redraw after approving: %q", edit["text"])
	}
	b.Handle(ctx, cb("ppause:1"))
	if p, _ := e.Store.GetPlan(1); p.State != store.PlanPaused {
		t.Fatalf("plan state %s", p.State)
	}
	if edit = fake.last("editMessageText"); !strings.Contains(edit["reply_markup"], "pstart:1") {
		t.Fatalf("a paused plan offers start: %s", edit["reply_markup"])
	}
}

func TestBotKeyboardAndTaskFlow(t *testing.T) {
	b, fake, e, _ := botFixture(t)
	ctx := context.Background()
	b.Handle(ctx, msg("/start", 0))
	if m := fake.last("sendMessage"); !strings.Contains(m["reply_markup"], `"is_persistent":true`) {
		t.Fatalf("/start should send the persistent keyboard: %v", m)
	}
	b.Handle(ctx, msg("📊 Estado", 0))
	if m := fake.last("sendMessage"); !strings.Contains(m["reply_markup"], "approve:") {
		t.Fatalf("status from the keyboard should carry review buttons: %v", m)
	}
	b.Handle(ctx, msg("➕ New task", 0))
	if m := fake.last("sendMessage"); !strings.Contains(m["reply_markup"], "newtask:1") {
		t.Fatalf("new task should list projects: %v", m)
	}
	b.Handle(ctx, cb("newtask:1"))
	fake.mu.Lock()
	promptID := fake.next
	fake.mu.Unlock()
	if m := fake.last("sendMessage"); !strings.Contains(m["reply_markup"], "force_reply") {
		t.Fatalf("title prompt should force a reply: %v", m)
	}
	b.Handle(ctx, msg("Dark mode toggle", promptID))
	tasks, _ := e.Store.ListTasks(1, store.TaskFilter{})
	found := false
	for _, tk := range tasks {
		found = found || tk.Title == "Dark mode toggle"
	}
	if !found {
		t.Fatal("the reply should create the task")
	}
	if err := b.api.setCommands(ctx, b.commands()); err != nil {
		t.Fatal(err)
	}
	if c := fake.last("setMyCommands"); !strings.Contains(c["commands"], `"plans"`) {
		t.Fatalf("menu commands %v", c)
	}
}

func TestBotPickAndLaunchTask(t *testing.T) {
	b, fake, e, run := botFixture(t)
	ctx := context.Background()
	// The fixture's plan step is in review, so no agent is running.
	if _, err := e.Store.ActiveRun(); err == nil {
		t.Fatal("fixture should have no running agent")
	}
	b.Handle(ctx, msg("/task app Dark mode toggle", 0))
	tasks, _ := e.Store.ListTasks(1, store.TaskFilter{})
	var task store.Task
	for _, tk := range tasks {
		if tk.Title == "Dark mode toggle" {
			task = tk
		}
	}
	launchData := "launch:" + itoa(task.ID) + ":claude"
	if m := fake.last("sendMessage"); !strings.Contains(m["reply_markup"], launchData) {
		t.Fatalf("a new task should offer launch buttons: %v", m)
	}
	b.Handle(ctx, msg("/tareas", 0))
	if m := fake.last("sendMessage"); !strings.Contains(m["reply_markup"], "task:"+itoa(task.ID)) {
		t.Fatalf("/tareas should list the task: %v", m)
	}
	b.Handle(ctx, msg("📋 Tareas", 0))
	if m := fake.last("sendMessage"); !strings.Contains(m["text"], "Dark mode toggle") {
		t.Fatalf("the keyboard button should list tasks: %v", m)
	}
	b.Handle(ctx, cb(launchData))
	got, _ := e.Store.GetTask(task.ID)
	if got.Status != store.StatusInProgress {
		t.Fatalf("task should be running, got %s", got.Status)
	}
	if edit := fake.last("editMessageText"); !strings.Contains(edit["text"], "Dark mode toggle") || !strings.Contains(edit["reply_markup"], "log:") {
		t.Fatalf("the launch should redraw the message: %v", edit)
	}
	_ = run
}

func TestBotCommentApproveAndMerge(t *testing.T) {
	b, fake, e, _ := botFixture(t)
	ctx := context.Background()
	task, err := e.Store.CreateTask(store.Task{ProjectID: 1, Title: "Footer", Status: store.StatusBacklog,
		Priority: store.PriorityMedium, Agent: store.AgentClaude, Mode: store.ModeYolo, Commits: store.CommitsAuto, Type: store.TypeFeature})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Launch(task)
	if err != nil {
		t.Fatal(err)
	}
	launch.WaitAgents([]string{res.Run.Worktree}, 10*time.Second)
	if _, err := e.Finish(res.Run.ID, nil); err != nil {
		t.Fatal(err)
	}
	rid := itoa(res.Run.ID)
	tid := itoa(task.ID)

	b.Handle(ctx, cb("tasks:review"))
	if edit := fake.last("editMessageText"); !strings.Contains(edit["reply_markup"], "task:"+tid) {
		t.Fatalf("the review tab should list the task: %v", edit)
	}
	b.Handle(ctx, cb("task:"+tid))
	edit := fake.last("editMessageText")
	for _, want := range []string{"approve:" + rid + "|task:" + tid, "comment:" + rid, "diff:" + rid} {
		if !strings.Contains(edit["reply_markup"], want) {
			t.Fatalf("review task lacks %s: %s", want, edit["reply_markup"])
		}
	}
	b.Handle(ctx, cb("comment:"+rid))
	fake.mu.Lock()
	promptID := fake.next
	fake.mu.Unlock()
	b.Handle(ctx, msg("Use the brand colors", promptID))
	if c, _ := e.Store.ListPendingComments(res.Run.ID); len(c) != 1 || c[0].Body != "Use the brand colors" {
		t.Fatalf("comment not saved: %+v", c)
	}
	if m := fake.last("sendMessage"); !strings.Contains(m["reply_markup"], "sendc:"+rid) {
		t.Fatalf("a saved comment should offer to send it: %v", m)
	}

	b.Handle(ctx, cb("approve:"+rid+"|task:"+tid))
	edit = fake.last("editMessageText")
	if !strings.Contains(edit["reply_markup"], "merge:"+rid) {
		t.Fatalf("an approved task should offer merge: %s", edit["reply_markup"])
	}
	b.Handle(ctx, cb("merge:"+rid+"|task:"+tid))
	if edit = fake.last("editMessageText"); !strings.Contains(edit["reply_markup"], "domerge:"+rid) {
		t.Fatalf("merge should ask first: %v", edit)
	}
	b.Handle(ctx, cb("domerge:"+rid+"|task:"+tid))
	if got, _ := e.Store.GetRun(res.Run.ID); got.State != store.RunMerged {
		t.Fatalf("run state %s", got.State)
	}
}

func TestBotMergesTaskApprovedWithoutCommits(t *testing.T) {
	b, fake, e, _ := botFixture(t)
	ctx := context.Background()
	task, _ := e.Store.CreateTask(store.Task{ProjectID: 1, Title: "Readme", Status: store.StatusBacklog,
		Priority: store.PriorityMedium, Agent: store.AgentClaude, Mode: store.ModeYolo, Commits: store.CommitsNone, Type: store.TypeFeature})
	res, err := e.Launch(task)
	if err != nil {
		t.Fatal(err)
	}
	launch.WaitAgents([]string{res.Run.Worktree}, 10*time.Second)
	if _, err := e.Finish(res.Run.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Approve(res.Run.ID); err != nil {
		t.Fatal(err)
	}
	rid, tid := itoa(res.Run.ID), itoa(task.ID)
	b.Handle(ctx, cb("task:"+tid))
	if edit := fake.last("editMessageText"); !strings.Contains(edit["reply_markup"], "merge:"+rid) {
		t.Fatalf("a task approved without commits should still offer merge: %s", edit["reply_markup"])
	}
	b.Handle(ctx, cb("merge:"+rid+"|task:"+tid))
	if edit := fake.last("editMessageText"); !strings.Contains(edit["text"], "without committing") {
		t.Fatalf("the confirmation should say it commits first: %q", edit["text"])
	}
	b.Handle(ctx, cb("domerge:"+rid+"|task:"+tid))
	if got, _ := e.Store.GetRun(res.Run.ID); got.State != store.RunMerged {
		t.Fatalf("run state %s, note %s", got.State, got.Note)
	}
	p, _ := e.Store.Get(1)
	if out, _ := launch.Git(p.RepoPath, "log", "--oneline", "-1"); !strings.Contains(out, "Readme") {
		t.Fatalf("main should have the merge: %s", out)
	}
}

func TestBotConversation(t *testing.T) {
	b, fake, e, _ := botFixture(t)
	ctx := context.Background()
	// The fake agent answers by writing to the reply file named in its prompt.
	bin := filepath.Join(t.TempDir(), "chatbin")
	_ = os.MkdirAll(bin, 0o755)
	script := "#!/bin/sh\nfor a in \"$@\"; do f=$(printf '%s' \"$a\" | grep -o '/[^ ]*reply-[0-9]*\\.md'); if [ -n \"$f\" ]; then printf 'Hola, **todo bien**' > \"$f\"; fi; done\n"
	_ = os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, _ = e.Store.Create(store.Project{Name: "Conversar", Slug: "conversar", RepoPath: t.TempDir()})

	b.Handle(ctx, msg("💬 Conversar", 0))
	if m := fake.last("sendMessage"); !strings.Contains(m["reply_markup"], `"chat:claude"`) {
		t.Fatalf("chat should list agents: %v", m)
	}
	b.Handle(ctx, cb("chat:claude"))
	edit := fake.last("editMessageText")
	if !strings.Contains(edit["reply_markup"], "chatp:claude:2") || strings.Index(edit["reply_markup"], "chatp:claude:2") > strings.Index(edit["reply_markup"], "chatp:claude:1") {
		t.Fatalf("Conversar should come first: %s", edit["reply_markup"])
	}
	b.Handle(ctx, cb("chatp:claude:2"))
	if !b.chatting() {
		t.Fatal("conversation should be open")
	}
	b.Handle(ctx, msg("¿Qué tal?", 0))
	b.wg.Wait()
	m := fake.last("sendMessage")
	if !strings.Contains(m["text"], "<b>todo bien</b>") || !strings.Contains(m["reply_markup"], "chatend") {
		t.Fatalf("answer not relayed: %v", m)
	}
	raw, _ := os.ReadFile(filepath.Join(b.conv.Dir, "transcript.md"))
	if !strings.Contains(string(raw), "¿Qué tal?") || !strings.Contains(string(raw), "todo bien") {
		t.Fatalf("transcript: %s", raw)
	}
	// A restart keeps the conversation open.
	b2 := NewBot("t", "42", e, b.dataDir, nil)
	if !b2.chatting() {
		t.Fatal("the conversation should survive a restart")
	}
	b.Handle(ctx, cb("chatend"))
	if b.chatting() {
		t.Fatal("conversation should be closed")
	}
	if m := fake.last("sendMessage"); !strings.Contains(m["text"], "1") {
		t.Fatalf("close message: %v", m)
	}
}

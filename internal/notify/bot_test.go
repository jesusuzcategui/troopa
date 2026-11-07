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

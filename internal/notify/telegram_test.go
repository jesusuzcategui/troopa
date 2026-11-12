package notify

import (
	"bytes"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func fixture(t *testing.T) (*store.Store, store.Run) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	p, _ := s.Create(store.Project{Name: "Solar system", RepoPath: t.TempDir()})
	plan, _ := s.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Moons", Agent: store.AgentCursor, Auto: true})
	task, _ := s.AddPlanStep(plan.ID, store.Task{Title: "Add <moons> & rings"})
	_, _ = s.AddPlanStep(plan.ID, store.Task{Title: "Labels"})
	run, err := s.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, State: store.RunReview, Agent: store.AgentCursor,
		Branch: "feature/moons", Worktree: t.TempDir(), LogPath: "x",
		Summary: "# How to run\n\n```sh\nnpm start\n```\n\n## What changed\n\n- Added **moons** in `app.js`\n- Rings use <canvas>\n",
		Checks:  `[{"cmd":"go vet ./...","ok":true,"output":""},{"cmd":"go test ./...","ok":false,"output":"line1\nFAIL: TestMoons\n"}]`})
	if err != nil {
		t.Fatal(err)
	}
	run.DurationMS = sql.NullInt64{Int64: 754000, Valid: true}
	run.TokensIn = sql.NullInt64{Int64: 120500, Valid: true}
	run.TokensOut = sql.NullInt64{Int64: 8200, Valid: true}
	run.CostUSD = sql.NullFloat64{Float64: 0.42, Valid: true}
	if err := s.UpdateRun(run); err != nil {
		t.Fatal(err)
	}
	return s, run
}

func TestBuildMessageReview(t *testing.T) {
	s, run := fixture(t)
	msg := BuildMessage(s, launch.HookEvent{Event: launch.HookReview, RunID: run.ID})
	for _, want := range []string{
		"Ready for review", "📁 Solar system", "🗺 Moons · step 1 of 2", "Add &lt;moons&gt; &amp; rings",
		"🤖 Cursor", "⏱ 12m 34s", "120.5k in", "$0.42", "<code>feature/moons</code>",
		"✅ <code>go vet ./...</code>", "❌ <code>go test ./...</code>", "FAIL: TestMoons",
		"<blockquote expandable>", "<b>What changed</b>", "• Added <b>moons</b> in <code>app.js</code>", "Rings use &lt;canvas&gt;",
		"<pre>npm start</pre>", "y approves",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in\n%s", want, msg)
		}
	}
}

func TestBuildMessageApproveNamesNextStep(t *testing.T) {
	s, run := fixture(t)
	_ = s.MoveTask(run.TaskID, store.StatusDone)
	msg := BuildMessage(s, launch.HookEvent{Event: launch.HookApprove, RunID: run.ID})
	if !strings.Contains(msg, "Next step: Labels") {
		t.Fatal(msg)
	}
}

func TestTelegramSends(t *testing.T) {
	s, run := fixture(t)
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/sendMessage" {
			t.Errorf("path %s", r.URL.Path)
		}
		_ = r.ParseForm()
		got = r.PostForm
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	old := TelegramAPI
	TelegramAPI = srv.URL
	defer func() { TelegramAPI = old }()
	in := strings.NewReader(`{"event":"review","run_id":` + itoa(run.ID) + `}`)
	var out, errb bytes.Buffer
	code := Telegram([]string{"--token", "TOKEN", "--chat", "42"}, in, &out, &errb, func() (*store.Store, error) { return s, nil }, Defaults{})
	if code != 0 {
		t.Fatalf("code %d: %s", code, errb.String())
	}
	if got.Get("chat_id") != "42" || got.Get("parse_mode") != "HTML" || !strings.Contains(got.Get("text"), "Ready for review") {
		t.Fatalf("form %v", got)
	}
}

func TestDryRunWithoutStore(t *testing.T) {
	var out, errb bytes.Buffer
	in := strings.NewReader(`{"event":"blocked","task":"Fix login","project":"App","message":"checks failed"}`)
	if code := Telegram([]string{"--dry-run"}, in, &out, &errb, nil, Defaults{}); code != 0 {
		t.Fatal(errb.String())
	}
	if !strings.Contains(out.String(), "Blocked") || !strings.Contains(out.String(), "Fix login") || !strings.Contains(out.String(), "checks failed") {
		t.Fatal(out.String())
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestBuildMessageReviewWithoutChecksOnAutoApprove(t *testing.T) {
	s, run := fixture(t)
	task, _ := s.GetTask(run.TaskID)
	plan, _ := s.GetPlan(task.PlanID)
	plan.AutoApprove = true
	if err := s.UpdatePlan(plan); err != nil {
		t.Fatal(err)
	}
	run.Checks = "null"
	if err := s.UpdateRun(run); err != nil {
		t.Fatal(err)
	}
	msg := BuildMessage(s, launch.HookEvent{Event: launch.HookReview, RunID: run.ID})
	if !strings.Contains(msg, "Not auto-approved: no checks ran") {
		t.Fatal(msg)
	}
}

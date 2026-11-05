package tui

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestMetricsKeyShowsAgentAndProjectTables(t *testing.T) {
	m := newTestModel(t)
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: "/a"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateRun(store.Run{
		TaskID: task.ID, ProjectID: p.ID, Agent: "pi", State: store.RunApproved,
		DurationMS:    sql.NullInt64{Int64: 1500, Valid: true},
		TokensIn:      sql.NullInt64{Int64: 100, Valid: true},
		TokensOut:     sql.NullInt64{Int64: 10, Valid: true},
		TokensContext: sql.NullInt64{Int64: 90, Valid: true},
		CostUSD:       sql.NullFloat64{Float64: 0.002, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.CreateRun(store.Run{
		TaskID: task.ID, ProjectID: p.ID, Agent: "claude", State: store.RunDiscarded,
	}); err != nil {
		t.Fatal(err)
	}
	m.width, m.height = 120, 40
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'M'}})
	got := next.(Model)
	if got.screen != screenMetrics {
		t.Fatalf("screen=%v", got.screen)
	}
	view := got.viewMetrics()
	for _, want := range []string{"pi", "claude", "App", "n/d", "ctx ini", "█"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in\n%s", want, view)
		}
	}
}

func TestApplyRunMetricsFromJSONLog(t *testing.T) {
	log := filepath.Join(t.TempDir(), "agent.log")
	body := `{"type":"assistant","message":{"usage":{"input_tokens":12,"output_tokens":3,"cache_read_input_tokens":4}}}`
	if err := os.WriteFile(log, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	run := store.Run{CreatedAt: time.Now().Add(-2 * time.Second)}
	applyRunMetrics(&run, log, "pi", config.DefaultPrice())
	if run.Agent != "pi" || !run.DurationMS.Valid || run.DurationMS.Int64 <= 0 {
		t.Fatalf("duration/agent %+v", run)
	}
	if !run.TokensContext.Valid || run.TokensContext.Int64 != 12 {
		t.Fatalf("context %+v", run.TokensContext)
	}
	if !run.TokensIn.Valid || run.TokensIn.Int64 != 12 || !run.TokensOut.Valid || run.TokensOut.Int64 != 3 {
		t.Fatalf("tokens in=%v out=%v", run.TokensIn, run.TokensOut)
	}
	if !run.CostUSD.Valid {
		t.Fatal("expected cost")
	}
}

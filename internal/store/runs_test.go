package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCreateAndUpdateRun(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	task, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.CreateRun(Run{TaskID: task.ID, ProjectID: p.ID, Worktree: "/wt", Branch: "feature/go", State: RunRunning})
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == 0 || r.State != RunRunning {
		t.Fatalf("%+v", r)
	}
	r.State = RunReview
	r.Summary = "listo"
	if err := s.UpdateRun(r); err != nil {
		t.Fatal(err)
	}
	got, err := s.LatestRun(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != RunReview || got.Summary != "listo" {
		t.Fatalf("%+v", got)
	}
}

func TestRunMetricsAndAggregation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p1 := testProject(t, s)
	p2, err := s.Create(Project{Name: "Otro", RepoPath: "/o"})
	if err != nil {
		t.Fatal(err)
	}
	t1, err := s.CreateTask(Task{ProjectID: p1.ID, Title: "A"})
	if err != nil {
		t.Fatal(err)
	}
	t2, err := s.CreateTask(Task{ProjectID: p2.ID, Title: "B"})
	if err != nil {
		t.Fatal(err)
	}
	r1, err := s.CreateRun(Run{
		TaskID: t1.ID, ProjectID: p1.ID, Agent: "pi", State: RunApproved,
		DurationMS:    sql.NullInt64{Int64: 2000, Valid: true},
		TokensIn:      sql.NullInt64{Int64: 100, Valid: true},
		TokensOut:     sql.NullInt64{Int64: 20, Valid: true},
		TokensCache:   sql.NullInt64{Int64: 5, Valid: true},
		TokensContext: sql.NullInt64{Int64: 80, Valid: true},
		CostUSD:       sql.NullFloat64{Float64: 0.01, Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r1.Agent != "pi" || !r1.TokensContext.Valid || r1.TokensContext.Int64 != 80 {
		t.Fatalf("%+v", r1)
	}
	if _, err := s.CreateRun(Run{TaskID: t2.ID, ProjectID: p2.ID, Agent: "claude", State: RunDiscarded}); err != nil {
		t.Fatal(err)
	}
	byAgent, err := s.MetricsByAgent()
	if err != nil {
		t.Fatal(err)
	}
	if len(byAgent) != 2 {
		t.Fatalf("agents=%d", len(byAgent))
	}
	var pi, claude MetricRow
	for _, row := range byAgent {
		switch row.Key {
		case "pi":
			pi = row
		case "claude":
			claude = row
		}
	}
	if pi.Runs != 1 || pi.Approved != 1 || !pi.AvgContext.Valid || pi.AvgContext.Float64 != 80 {
		t.Fatalf("pi %+v", pi)
	}
	if claude.Runs != 1 || claude.Approved != 0 || claude.AvgContext.Valid {
		t.Fatalf("claude %+v", claude)
	}
	byProj, err := s.MetricsByProject()
	if err != nil {
		t.Fatal(err)
	}
	if len(byProj) != 2 {
		t.Fatalf("projects=%d %+v", len(byProj), byProj)
	}
}

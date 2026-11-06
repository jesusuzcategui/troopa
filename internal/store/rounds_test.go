package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateAndListRounds(t *testing.T) {
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
	run, err := s.CreateRun(Run{TaskID: task.ID, ProjectID: p.ID, State: RunRunning})
	if err != nil {
		t.Fatal(err)
	}
	r1, err := s.CreateRound(Round{RunID: run.ID, Kind: RoundLaunch, LogFrom: 0})
	if err != nil {
		t.Fatal(err)
	}
	if r1.Number != 1 || r1.State != RoundRunning || r1.Kind != RoundLaunch {
		t.Fatalf("%+v", r1)
	}
	open, err := s.OpenRound(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if open.ID != r1.ID {
		t.Fatalf("open=%d want %d", open.ID, r1.ID)
	}
	r1.State = RoundReview
	r1.Summary = "listo"
	r1.Checks = `[{"ok":true}]`
	r1.DurationMS = sql.NullInt64{Int64: 1200, Valid: true}
	r1.LogTo = sql.NullInt64{Int64: 80, Valid: true}
	r1.FinishedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := s.UpdateRound(r1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenRound(run.ID); err != sql.ErrNoRows {
		t.Fatalf("expected no open round, err=%v", err)
	}
	r2, err := s.CreateRound(Round{RunID: run.ID, Kind: RoundChanges, Feedback: "add tests", LogFrom: 80})
	if err != nil {
		t.Fatal(err)
	}
	if r2.Number != 2 {
		t.Fatalf("number=%d", r2.Number)
	}
	all, err := s.ListRounds(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Summary != "listo" || all[1].Feedback != "add tests" {
		t.Fatalf("%+v", all)
	}
}

func TestLegacyRoundForOldRuns(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	task, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Old"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.CreateRun(Run{
		TaskID: task.ID, ProjectID: p.ID, State: RunReview, Summary: "antes",
		DurationMS: sql.NullInt64{Int64: 5000, Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.RoundsForRun(run)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 0 || got[0].Number != 1 || got[0].Kind != RoundLaunch {
		t.Fatalf("%+v", got)
	}
	if got[0].Summary != "antes" || !got[0].DurationMS.Valid {
		t.Fatalf("%+v", got[0])
	}
}

func TestRoundJSONHasPlainMetrics(t *testing.T) {
	r := Round{ID: 1, Number: 2, DurationMS: sql.NullInt64{Int64: 42, Valid: true}, CreatedAt: time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"duration_ms":42`) || !strings.Contains(s, `"tokens_in":null`) || strings.Contains(s, "Valid") {
		t.Fatalf("unexpected JSON: %s", s)
	}
	if !strings.Contains(s, `"finished_at":null`) || strings.Contains(s, `"sql.Null`) {
		t.Fatalf("unexpected JSON: %s", s)
	}
	legacy := Round{Number: 1, Kind: RoundLaunch}
	b, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"id":null`) {
		t.Fatalf("legacy id: %s", b)
	}
}

package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestJSONGolden(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cases := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"empty_project_list", []string{"project", "list", "--json"}, 0},
		{"not_found", []string{"task", "show", "--json", "--id", "99"}, 3},
		{"run_not_found", []string{"run", "merge", "--json", "--id", "99"}, 3},
		{"run_rounds_not_found", []string{"run", "rounds", "--json", "--id", "99"}, 3},
		{"invalid_resource", []string{"unknown", "list", "--json"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if got := run(tc.args, &out, &errOut); got != tc.wantCode {
				t.Fatalf("exit code = %d, want %d", got, tc.wantCode)
			}
			want, err := os.ReadFile(filepath.Join("testdata", tc.name+".golden.json"))
			if err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != string(want) {
				t.Fatalf("JSON mismatch\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestJSONRunInvalidState(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, _, dbPath, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(store.Task{ProjectID: p.ID, Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, Agent: "claude", State: store.RunDiscarded})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	id := strconv.FormatInt(saved.ID, 10)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"run_merge_invalid_state", []string{"run", "merge", "--json", "--id", id}},
		{"run_clean_invalid_state", []string{"run", "clean", "--json", "--id", id}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if got := run(tc.args, &out, &errOut); got != 1 {
				t.Fatalf("exit code = %d, want 1, out=%s", got, out.String())
			}
			want, err := os.ReadFile(filepath.Join("testdata", tc.name+".golden.json"))
			if err != nil {
				t.Fatal(err)
			}
			if got := out.String(); got != string(want) {
				t.Fatalf("JSON mismatch\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestJSONRunRounds(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, _, dbPath, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(store.Task{ProjectID: p.ID, Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, Agent: "claude", State: store.RunReview})
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := s.CreateRound(store.Round{
		RunID: saved.ID, Kind: store.RoundLaunch, State: store.RoundReview, Summary: "ok",
		DurationMS: sql.NullInt64{Int64: 1500, Valid: true},
		LogFrom:    0,
		LogTo:      sql.NullInt64{Int64: 10, Valid: true},
		CreatedAt:  created,
		FinishedAt: created.Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	var out, errOut bytes.Buffer
	args := []string{"run", "rounds", "--json", "--id", strconv.FormatInt(saved.ID, 10)}
	if got := run(args, &out, &errOut); got != 0 {
		t.Fatalf("exit code = %d, out=%s", got, out.String())
	}
	want, err := os.ReadFile(filepath.Join("testdata", "run_rounds.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != string(want) {
		t.Fatalf("JSON mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestJSONTaskAttachments(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, _, dbPath, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(store.Task{ProjectID: p.ID, Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	tid := strconv.FormatInt(task.ID, 10)

	var out, errOut bytes.Buffer
	if got := run([]string{"task", "attach", "--json", "--id", tid}, &out, &errOut); got != 2 {
		t.Fatalf("missing file: exit %d out=%s", got, out.String())
	}
	want, err := os.ReadFile(filepath.Join("testdata", "task_attach_missing_file.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("missing file JSON\n got: %s\nwant: %s", out.String(), want)
	}

	out.Reset()
	if got := run([]string{"task", "attachments", "--json", "--id", tid}, &out, &errOut); got != 0 {
		t.Fatalf("empty list: exit %d out=%s", got, out.String())
	}
	want, err = os.ReadFile(filepath.Join("testdata", "task_attachments_empty.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("empty list JSON\n got: %s\nwant: %s", out.String(), want)
	}

	src := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(src, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if got := run([]string{"task", "attach", "--json", "--id", tid, "--file", src}, &out, &errOut); got != 0 {
		t.Fatalf("attach: exit %d out=%s", got, out.String())
	}
	if !strings.Contains(out.String(), `"name":"note.txt"`) || !strings.Contains(out.String(), `"size":2`) {
		t.Fatalf("attach JSON: %s", out.String())
	}

	out.Reset()
	if got := run([]string{"task", "attachments", "--json", "--id", tid}, &out, &errOut); got != 0 {
		t.Fatalf("list: exit %d out=%s", got, out.String())
	}
	if !strings.Contains(out.String(), `"name":"note.txt"`) {
		t.Fatalf("list JSON: %s", out.String())
	}

	out.Reset()
	if got := run([]string{"task", "detach", "--json", "--id", tid, "--attachment", "99"}, &out, &errOut); got != 3 {
		t.Fatalf("detach missing: exit %d out=%s", got, out.String())
	}
	want, err = os.ReadFile(filepath.Join("testdata", "task_detach_not_found.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("detach JSON\n got: %s\nwant: %s", out.String(), want)
	}

	out.Reset()
	if got := run([]string{"task", "detach", "--json", "--id", tid, "--attachment", "1"}, &out, &errOut); got != 0 {
		t.Fatalf("detach: exit %d out=%s", got, out.String())
	}
	want, err = os.ReadFile(filepath.Join("testdata", "task_detach.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("detach ok JSON\n got: %s\nwant: %s", out.String(), want)
	}
}

func TestJSONBrainSync(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, _, dbPath, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(store.Project{Name: "App", RepoPath: t.TempDir(), Slug: "app"}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	var out, errOut bytes.Buffer
	if got := run([]string{"brain", "sync", "--json"}, &out, &errOut); got != 0 {
		t.Fatalf("exit %d out=%s", got, out.String())
	}
	if !strings.Contains(out.String(), `"projects":1`) {
		t.Fatalf("sync JSON: %s", out.String())
	}
	data, _, _, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "brain", "projects", "INDEX.md")); err != nil {
		t.Fatal(err)
	}
}

func TestHelpAndVersion(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		var out, errOut bytes.Buffer
		if got := run([]string{arg}, &out, &errOut); got != 0 {
			t.Fatalf("%s: exit code = %d, want 0", arg, got)
		}
		if !strings.Contains(out.String(), "Usage:") || !strings.Contains(out.String(), "daemon") {
			t.Fatalf("%s: missing usage, got %q", arg, out.String())
		}
	}
	for _, arg := range []string{"version", "-v", "--version"} {
		var out, errOut bytes.Buffer
		if got := run([]string{arg}, &out, &errOut); got != 0 {
			t.Fatalf("%s: exit code = %d, want 0", arg, got)
		}
		if !strings.HasPrefix(out.String(), "troopa ") {
			t.Fatalf("%s: got %q", arg, out.String())
		}
	}
}

func TestDaemonStatusAndSecondInstance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	var out, errOut bytes.Buffer
	if got := run([]string{"daemon", "status"}, &out, &errOut); got != 0 {
		t.Fatalf("status: %d %s %s", got, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "not running") {
		t.Fatalf("status: %q", out.String())
	}
	data, _, _, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "daemon.pid"), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if got := run([]string{"daemon"}, &out, &errOut); got != 1 {
		t.Fatalf("second daemon exit %d out=%q err=%q", got, out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), "already running") {
		t.Fatalf("second daemon message: %q", errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if got := run([]string{"daemon", "nope"}, &out, &errOut); got != 2 {
		t.Fatalf("unknown: %d %q", got, errOut.String())
	}
}

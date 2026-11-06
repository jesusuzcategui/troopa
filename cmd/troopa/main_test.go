package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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

func TestHelpAndVersion(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		var out, errOut bytes.Buffer
		if got := run([]string{arg}, &out, &errOut); got != 0 {
			t.Fatalf("%s: exit code = %d, want 0", arg, got)
		}
		if !strings.Contains(out.String(), "Usage:") {
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

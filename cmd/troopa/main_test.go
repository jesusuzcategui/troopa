package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
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

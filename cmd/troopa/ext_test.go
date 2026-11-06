//go:build unix

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunsExtensionsFromPath(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")
	script := "#!/bin/sh\necho \"$1 $TROOPA_DB\" > " + out + "\nexit 4\n"
	if err := os.WriteFile(filepath.Join(dir, "troopa-deploy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run([]string{"deploy", "staging"}, &stdout, &stderr); code != 4 {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	raw, _ := os.ReadFile(out)
	if !strings.HasPrefix(string(raw), "staging ") || !strings.Contains(string(raw), "troopa.db") {
		t.Fatalf("extension got %q", raw)
	}
	stdout.Reset()
	if code := run([]string{"extensions"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "deploy") {
		t.Fatalf("extensions: %d %q", code, stdout.String())
	}
}

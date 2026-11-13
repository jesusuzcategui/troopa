//go:build unix

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGUINotOnPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("TROOPA_LANG", "en")

	var stdout, stderr bytes.Buffer
	code := run([]string{"gui"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	errText := stderr.String()
	if !strings.Contains(errText, "troopa-desktop") || !strings.Contains(errText, "wails3 build") {
		t.Fatalf("expected install instructions in stderr, got %q", errText)
	}
}

func TestGUILaunchesFromPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran.txt")
	script := "#!/bin/sh\necho ok > " + marker + "\n"
	if err := os.WriteFile(filepath.Join(dir, "troopa-desktop"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	var stdout, stderr bytes.Buffer
	code := run([]string{"gui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	raw, err := os.ReadFile(marker)
	if err != nil || strings.TrimSpace(string(raw)) != "ok" {
		t.Fatalf("expected fake troopa-desktop to run, err=%v raw=%q", err, raw)
	}
}

func TestGUIHelpAndBadArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run([]string{"gui", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("--help exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "troopa gui") {
		t.Fatalf("unexpected --help output: %q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"gui", "extra"}, &stdout, &stderr); code != 2 {
		t.Fatalf("extra arg exit code = %d, want 2", code)
	}
}

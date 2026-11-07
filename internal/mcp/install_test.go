package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubPath(t *testing.T, found bool) {
	t.Helper()
	oldLook, oldRun := lookPath, runCmd
	t.Cleanup(func() {
		lookPath, runCmd = oldLook, oldRun
	})
	if found {
		lookPath = func(name string) (string, error) { return "/bin/" + name, nil }
		return
	}
	lookPath = func(string) (string, error) { return "", errors.New("missing") }
}

func TestInstallUnknownClient(t *testing.T) {
	var buf bytes.Buffer
	err := Install(&buf, "nope")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Fatalf("err: %v", err)
	}
}

func TestInstallClaudeMissingPrintsSnippet(t *testing.T) {
	stubPath(t, false)
	var ran int
	runCmd = func(string, ...string) error {
		ran++
		return nil
	}
	var buf bytes.Buffer
	if err := Install(&buf, "claude"); err != nil {
		t.Fatal(err)
	}
	if ran != 0 {
		t.Fatalf("ran %d", ran)
	}
	out := buf.String()
	if !strings.Contains(out, "not installed") || !strings.Contains(out, "claude mcp add troopa -- troopa mcp") {
		t.Fatalf("out: %q", out)
	}
}

func TestInstallClaudeRunsAdd(t *testing.T) {
	stubPath(t, true)
	var got string
	runCmd = func(name string, args ...string) error {
		got = strings.Join(append([]string{name}, args...), " ")
		return nil
	}
	var buf bytes.Buffer
	if err := Install(&buf, "claude"); err != nil {
		t.Fatal(err)
	}
	if got != "claude mcp add troopa -- troopa mcp" {
		t.Fatalf("cmd: %q", got)
	}
	if !strings.Contains(buf.String(), "Claude Code") {
		t.Fatalf("out: %q", buf.String())
	}
}

func TestInstallCodexWritesTOML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubPath(t, true)
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("model = \"gpt-5\"\n\n[mcp_servers.other]\ncommand = \"other\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Install(&buf, "codex"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "model = \"gpt-5\"") || !strings.Contains(text, "[mcp_servers.other]") {
		t.Fatalf("kept: %s", text)
	}
	if !strings.Contains(text, "[mcp_servers.troopa]") || !strings.Contains(text, `command = "troopa"`) || !strings.Contains(text, `args = ["mcp"]`) {
		t.Fatalf("troopa: %s", text)
	}
	if !strings.Contains(buf.String(), path) {
		t.Fatalf("print: %s", buf.String())
	}
	buf.Reset()
	if err := Install(&buf, "codex"); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), "[mcp_servers.troopa]"); n != 1 {
		t.Fatalf("tables: %d\n%s", n, raw)
	}
}

func TestInstallOpenCodeAndCursorJSON(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, "config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	stubPath(t, true)

	var buf bytes.Buffer
	if err := Install(&buf, "opencode"); err != nil {
		t.Fatal(err)
	}
	oc := filepath.Join(cfg, "opencode", "opencode.json")
	assertJSON(t, oc, []string{"mcp", "troopa", "command"})

	buf.Reset()
	if err := Install(&buf, "cursor"); err != nil {
		t.Fatal(err)
	}
	cu := filepath.Join(home, ".cursor", "mcp.json")
	raw, err := os.ReadFile(cu)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	servers, _ := root["mcpServers"].(map[string]any)
	troopa, _ := servers["troopa"].(map[string]any)
	if troopa["command"] != "troopa" {
		t.Fatalf("cursor: %s", raw)
	}
}

func TestInstallMissingPrintsFileSnippets(t *testing.T) {
	stubPath(t, false)
	for _, client := range []string{"codex", "opencode", "cursor"} {
		var buf bytes.Buffer
		if err := Install(&buf, client); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "not installed") {
			t.Fatalf("%s: %q", client, buf.String())
		}
	}
}

func assertJSON(t *testing.T, path string, keys []string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	cur := v
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("%s: %s", path, raw)
		}
		cur = m[k]
	}
	arr, ok := cur.([]any)
	if !ok || len(arr) != 2 || arr[0] != "troopa" || arr[1] != "mcp" {
		t.Fatalf("command: %v in %s", cur, raw)
	}
}

package launch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/config"
)

func TestDetectAgents(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fakeagent")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'fakeagent 1.2.3'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	cfg := config.Launch{Agents: map[string]config.AgentSpec{
		"claude": {Command: "fakeagent"},
		"codex":  {Command: "not-installed-xyz"},
	}}
	got := DetectAgents(cfg)
	if len(got) != 2 || got[0].ID != "claude" || !got[0].Installed || got[0].Version != "fakeagent 1.2.3" || got[0].Name != "Claude Code" {
		t.Fatalf("detect: %+v", got)
	}
	if got[1].Installed {
		t.Fatal("codex should not be installed")
	}
	if !AgentInstalled(cfg, "me") || AgentInstalled(cfg, "codex") || !AgentInstalled(cfg, "claude") {
		t.Fatal("AgentInstalled")
	}
	if tools := DetectTools("nope-1", "fakeagent"); len(tools) != 1 || tools[0] != "fakeagent" {
		t.Fatalf("tools %v", tools)
	}
}

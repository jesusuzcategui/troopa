//go:build unix

package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitState(t *testing.T, wt string, want AgentState) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st, code := AgentStatus(wt); st == want {
			return code
		}
		time.Sleep(20 * time.Millisecond)
	}
	st, _ := AgentStatus(wt)
	t.Fatalf("state %v, want %v", st, want)
	return 0
}

func TestStartAgentRecordsExitCode(t *testing.T) {
	wt := t.TempDir()
	logPath := filepath.Join(wt, "agent.log")
	cmd := exec.Command("/bin/sh", "-c", "echo working; sleep 0.3; exit 3")
	cmd.Dir = wt
	if err := StartAgent(cmd, wt, logPath); err != nil {
		t.Fatal(err)
	}
	if st, _ := AgentStatus(wt); st != AgentRunning {
		t.Fatalf("just started, state %v", st)
	}
	if code := waitState(t, wt, AgentExited); code != 3 {
		t.Fatalf("exit code %d", code)
	}
	raw, _ := os.ReadFile(logPath)
	if !strings.Contains(string(raw), "working") {
		t.Fatalf("log: %q", raw)
	}
}

func TestStoppedAgentIsLost(t *testing.T) {
	wt := t.TempDir()
	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	cmd.Dir = wt
	if err := StartAgent(cmd, wt, filepath.Join(wt, "agent.log")); err != nil {
		t.Fatal(err)
	}
	StopAgent(wt)
	// Killing the session leader's group stops the wrapper before it writes
	// the exit file, like a machine restart would.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st, _ := AgentStatus(wt); st != AgentRunning {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("agent still running after StopAgent")
}

func TestNoPidMeansLost(t *testing.T) {
	if st, _ := AgentStatus(t.TempDir()); st != AgentLost {
		t.Fatalf("state %v", st)
	}
}

package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Agents run detached from Troopa: closing the TUI (or the terminal it runs
// in) does not stop them. Their PID and, when they finish, their exit code
// live in .troopa/ inside the worktree, so Troopa can pick the run up again
// the next time it opens.

// AgentState is what Troopa knows about a detached agent.
type AgentState int

const (
	AgentRunning AgentState = iota // the process is still alive
	AgentExited                    // it finished; the exit code is known
	AgentLost                      // it is gone without an exit code (killed, or the machine restarted)
)

func agentPidFile(worktree string) string  { return filepath.Join(worktree, ".troopa", "agent.pid") }
func agentExitFile(worktree string) string { return filepath.Join(worktree, ".troopa", "agent.exit") }

// StartAgent starts cmd in the background with its output appended to
// logPath. It returns once the process is running.
func StartAgent(cmd *exec.Cmd, worktree, logPath string) error {
	if err := os.MkdirAll(filepath.Join(worktree, ".troopa"), 0o755); err != nil {
		return err
	}
	_ = os.Remove(agentExitFile(worktree))
	_ = os.Remove(agentPidFile(worktree))
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	proc, err := startDetached(cmd, agentExitFile(worktree), log)
	if err != nil {
		return err
	}
	return os.WriteFile(agentPidFile(worktree), []byte(strconv.Itoa(proc)+"\n"), 0o644)
}

// AgentStatus reports whether the agent of a worktree is running, finished
// (with its exit code) or lost.
func AgentStatus(worktree string) (AgentState, int) {
	if raw, err := os.ReadFile(agentExitFile(worktree)); err == nil {
		code, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err != nil {
			code = 1
		}
		return AgentExited, code
	}
	raw, err := os.ReadFile(agentPidFile(worktree))
	if err != nil {
		return AgentLost, -1
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || !processAlive(pid) {
		return AgentLost, -1
	}
	return AgentRunning, 0
}

// StopAgent asks a running agent to stop.
func StopAgent(worktree string) {
	raw, err := os.ReadFile(agentPidFile(worktree))
	if err != nil {
		return
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
		stopAgentPID(pid)
	}
}

// WaitAgents waits until no agent of these worktrees is running, up to
// timeout. Tests use it so their temporary folders are not removed while an
// agent still writes in them.
func WaitAgents(worktrees []string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for _, wt := range worktrees {
		for time.Now().Before(deadline) {
			if st, _ := AgentStatus(wt); st != AgentRunning {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

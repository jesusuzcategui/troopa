//go:build unix

package launch

import (
	"os"
	"os/exec"
	"syscall"
)

// wrapper runs the agent and writes its exit code atomically when it ends.
const wrapper = `"$@"; code=$?; printf '%s\n' "$code" > "$TROOPA_EXIT_FILE.tmp" && mv "$TROOPA_EXIT_FILE.tmp" "$TROOPA_EXIT_FILE"`

func startDetached(cmd *exec.Cmd, exitFile string, log *os.File) (int, error) {
	args := append([]string{"-c", wrapper, "troopa-agent", cmd.Path}, cmd.Args[1:]...)
	w := exec.Command("/bin/sh", args...)
	w.Dir = cmd.Dir
	w.Env = append(append([]string{}, cmd.Env...), "TROOPA_EXIT_FILE="+exitFile)
	if cmd.Env == nil {
		w.Env = append(os.Environ(), "TROOPA_EXIT_FILE="+exitFile)
	}
	w.Stdout = log
	w.Stderr = log
	w.Stdin = nil
	// A new session: no controlling terminal, so closing the terminal or
	// Ctrl+C in Troopa does not reach the agent.
	w.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := w.Start(); err != nil {
		return 0, err
	}
	pid := w.Process.Pid
	go func() { _ = w.Wait() }() // reap it while Troopa is open
	return pid, nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func stopAgentPID(pid int) {
	if pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGTERM) // the whole session group
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

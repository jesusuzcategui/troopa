//go:build windows

package launch

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// On Windows the agent runs in its own process group and Troopa writes the
// exit code while it is open. If Troopa closes first, the run is reported as
// lost on the next start and can be continued with "request changes".
func startDetached(cmd *exec.Cmd, exitFile string, log *os.File) (int, error) {
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	go func() {
		code := 0
		if err := cmd.Wait(); err != nil {
			code = 1
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
		}
		_ = os.WriteFile(exitFile, []byte(fmt.Sprintf("%d\n", code)), 0o644)
	}()
	return cmd.Process.Pid, nil
}

func processAlive(pid int) bool { return false }

func stopAgentPID(pid int) {
	if p, err := os.FindProcess(pid); err == nil {
		_ = p.Kill()
	}
}

//go:build windows

package launch

import (
	"os"
	"os/exec"
)

func setDetached(cmd *exec.Cmd) {}

func stopProcess(p *os.Process) {
	if p != nil {
		_ = p.Kill()
	}
}

func pidAlive(pid int) bool {
	_, err := os.FindProcess(pid)
	return err == nil
}

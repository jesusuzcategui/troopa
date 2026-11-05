package launch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PairProc struct {
	Where string
	Wait  func() error
	Stop  func()
}

var lookPath = exec.LookPath

func ResolveTerminal(configured string) (string, error) {
	if strings.TrimSpace(configured) != "" {
		return strings.TrimSpace(configured), nil
	}
	candidates := []string{"kitty -e", "alacritty -e", "wezterm start --", "ghostty -e", "xterm -e"}
	for _, c := range candidates {
		bin := strings.Fields(c)[0]
		if _, err := lookPath(bin); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("configura terminal en config.toml (p. ej. kitty -e)")
}

func InTmux() bool {
	return strings.TrimSpace(os.Getenv("TMUX")) != ""
}

func PairOpenArgs(inTmux bool, terminal, worktree, script string) []string {
	if inTmux {
		return []string{"tmux", "split-window", "-h", "-P", "-F", "#{pane_id}", "-c", worktree, "sh", script}
	}
	parts := strings.Fields(strings.TrimSpace(terminal))
	return append(append([]string{}, parts...), "sh", script)
}

func OpenPairSession(worktree, terminal string) (PairProc, error) {
	script := PairScriptPath(worktree)
	if _, err := os.Stat(script); err != nil {
		return PairProc{}, fmt.Errorf("falta pair.sh en el worktree")
	}
	if InTmux() {
		if _, err := lookPath("tmux"); err != nil {
			return PairProc{}, fmt.Errorf("tmux no está instalado")
		}
		args := PairOpenArgs(true, "", worktree, script)
		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return PairProc{}, fmt.Errorf("tmux split: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		pane := strings.TrimSpace(string(out))
		return PairProc{
			Where: "tmux:" + pane,
			Wait:  func() error { return waitTmuxPane(pane) },
			Stop:  func() { _ = exec.Command("tmux", "kill-pane", "-t", pane).Run() },
		}, nil
	}
	term, err := ResolveTerminal(terminal)
	if err != nil {
		return PairProc{}, err
	}
	args := PairOpenArgs(false, term, worktree, script)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = worktree
	setDetached(cmd)
	if err := cmd.Start(); err != nil {
		return PairProc{}, err
	}
	proc := cmd.Process
	return PairProc{
		Where: strconv.Itoa(proc.Pid),
		Wait:  cmd.Wait,
		Stop:  func() { stopProcess(proc) },
	}, nil
}

func PairStillOpen(where, worktree string) bool {
	if strings.HasPrefix(where, "tmux:") {
		pane := strings.TrimPrefix(where, "tmux:")
		out, err := exec.Command("tmux", "list-panes", "-F", "#{pane_id}").Output()
		if err != nil {
			return false
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == pane {
				return true
			}
		}
		return false
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(where)); err == nil && pid > 0 {
		return pidAlive(pid)
	}
	raw, err := os.ReadFile(filepath.Join(worktree, "pair.pid"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return false
	}
	return pidAlive(pid)
}

func waitTmuxPane(id string) error {
	for {
		cmd := exec.Command("tmux", "list-panes", "-F", "#{pane_id}")
		out, err := cmd.Output()
		if err != nil {
			return nil
		}
		found := false
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == id {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
}

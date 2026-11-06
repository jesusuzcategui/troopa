package launch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/config"
)

func ExpandArgs(args []string, prompt, model string) []string {
	var out []string
	for _, a := range args {
		switch a {
		case "{{prompt}}":
			out = append(out, prompt)
		case "{{model}}":
			if model != "" {
				out = append(out, model)
			}
		default:
			out = append(out, a)
		}
	}
	if model != "" && !containsModelFlag(out) {
		out = insertModel(out, model)
	}
	return out
}

func containsModelFlag(args []string) bool {
	for _, a := range args {
		if a == "-m" || a == "--model" {
			return true
		}
	}
	return false
}

func insertModel(args []string, model string) []string {
	flag := "--model"
	for _, a := range args {
		if a == "exec" || a == "run" {
			flag = "-m"
			break
		}
	}
	if len(args) == 0 {
		return []string{flag, model}
	}
	out := make([]string, 0, len(args)+2)
	out = append(out, args[0], flag, model)
	out = append(out, args[1:]...)
	return out
}

func AgentCommand(cfg config.Launch, agent, model, worktree string) (*exec.Cmd, error) {
	spec, ok := cfg.Agents[agent]
	if !ok || spec.Command == "" {
		return nil, fmt.Errorf("no command for agent %s in config.toml", agent)
	}
	bin, err := exec.LookPath(spec.Command)
	if err != nil {
		return nil, fmt.Errorf("agent %s: %s is not installed", agent, spec.Command)
	}
	prompt := YoloPrompt(worktree)
	args := ExpandArgs(spec.Args, prompt, model)
	cmd := exec.Command(bin, args...)
	cmd.Dir = worktree
	cmd.Env = AgentEnv(worktree)
	for k, v := range spec.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return cmd, nil
}

// AgentEnv is the parent environment with PWD pointing at the worktree. Some
// agent CLIs (opencode) take their project folder from PWD instead of the
// process working directory, so an inherited PWD sends them to the folder
// Troopa was started from.
func AgentEnv(worktree string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PWD=") {
			env = append(env, kv)
		}
	}
	return append(env, "PWD="+worktree)
}

// YoloPrompt names the worktree explicitly so the agent never works elsewhere.
func YoloPrompt(worktree string) string {
	return fmt.Sprintf("Work only inside %s. Read %s and complete the task. When you finish, write %s.",
		worktree, filepath.Join(worktree, "TASK.md"), filepath.Join(worktree, "SUMMARY.md"))
}

func DefaultPrompt() string {
	return "Read TASK.md and complete the task. When you finish, write SUMMARY.md."
}

func PairPrompt() string {
	return "Read TASK.md and PAIR.md and work together with the person."
}

func SetupPairAgent(cfg config.Launch, agent, model, worktree string) error {
	spec, ok := cfg.Agents[agent]
	if !ok || spec.Command == "" {
		return fmt.Errorf("no command for agent %s in config.toml", agent)
	}
	bin, err := exec.LookPath(spec.Command)
	if err != nil {
		return fmt.Errorf("agent %s: %s is not installed", agent, spec.Command)
	}
	args, err := InteractiveArgs(agent, model, PairPrompt())
	if err != nil {
		return err
	}
	return WritePairScript(worktree, spec, bin, args)
}

func InteractiveArgs(agent, model, prompt string) ([]string, error) {
	switch agent {
	case "claude":
		args := []string{"--permission-mode", "bypassPermissions"}
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(args, prompt), nil
	case "codex":
		var args []string
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(args, prompt), nil
	case "opencode":
		args := []string{"mini"}
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(args, "--prompt", prompt), nil
	case "pi":
		var args []string
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(args, prompt), nil
	default:
		return nil, fmt.Errorf("pair mode does not support agent %s", agent)
	}
}

func HasPlaceholder(args []string) bool {
	for _, a := range args {
		if strings.Contains(a, "{{prompt}}") {
			return true
		}
	}
	return false
}

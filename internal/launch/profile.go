package launch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// The profile lives in .troopa/profile.md inside the worktree, so every
// rewrite of TASK.md (attachments, rounds) keeps it.
func profileFile(dir string) string { return filepath.Join(dir, ".troopa", "profile.md") }

// ProfileSection is the TASK.md text that tells the agent who it works for.
func ProfileSection(p store.Profile) string {
	var b strings.Builder
	b.WriteString("## Who you work for\n\n")
	switch {
	case p.Name != "" && p.Profession != "":
		fmt.Fprintf(&b, "You are working for %s (%s). Use the words, standards and tools of that profession.\n", p.Name, p.Profession)
	case p.Profession != "":
		fmt.Fprintf(&b, "You are working for a %s. Use the words, standards and tools of that profession.\n", p.Profession)
	default:
		fmt.Fprintf(&b, "You are working for %s.\n", p.Name)
	}
	if p.About != "" {
		b.WriteString("\n" + p.About + "\n")
	}
	if p.Instructions != "" {
		b.WriteString("\nTheir standing instructions:\n\n" + p.Instructions + "\n")
	}
	return b.String()
}

// WriteProfile stores the profile of a run; nil removes it.
func WriteProfile(dir string, p *store.Profile) error {
	if p == nil {
		err := os.Remove(profileFile(dir))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(profileFile(dir)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(profileFile(dir), []byte(ProfileSection(*p)), 0o644)
}

func readProfile(dir string) string {
	raw, err := os.ReadFile(profileFile(dir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw)) + "\n"
}

// LoginCommand opens an agent CLI with the logins of an account, so the
// person can sign in to it once.
func LoginCommand(cfg config.Launch, configPath, account, agent string) (*exec.Cmd, error) {
	spec, ok := cfg.Agents[agent]
	if !ok || spec.Command == "" {
		return nil, fmt.Errorf("no command for agent %s in config.toml", agent)
	}
	bin, err := exec.LookPath(spec.Command)
	if err != nil {
		return nil, fmt.Errorf("agent %s: %s is not installed", agent, spec.Command)
	}
	env, err := cfg.AccountEnv(configPath, account, agent)
	if err != nil {
		return nil, err
	}
	var args []string
	switch agent {
	case "codex":
		args = []string{"login"}
	case "opencode":
		args = []string{"auth", "login"}
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return cmd, nil
}

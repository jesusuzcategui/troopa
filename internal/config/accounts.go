package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// accountHomes is the variable each agent CLI reads to find its login and
// settings. An account gives every agent its own folder, so logging in with
// one account never touches the other.
var accountHomes = map[string]string{
	"claude":   "CLAUDE_CONFIG_DIR",
	"codex":    "CODEX_HOME",
	"opencode": "XDG_DATA_HOME",
	"pi":       "PI_CODING_AGENT_DIR",
}

// AccountHomeVar is the variable that holds the login of an agent, or "".
func AccountHomeVar(agent string) string { return accountHomes[agent] }

// AccountNames lists the configured accounts, sorted.
func (l Launch) AccountNames() []string {
	names := make([]string, 0, len(l.Accounts))
	for n := range l.Accounts {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// EnsureAccount adds an empty account section when it does not exist yet.
// It is used before an interface stores a key or opens that account's login.
func EnsureAccount(path, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("invalid account name %q", name)
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return fmt.Errorf("invalid account name %q", name)
		}
	}
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	if _, ok := cfg.Launch.Accounts[name]; ok {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	text := string(raw)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if strings.TrimSpace(text) != "" {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text+"[accounts."+name+"]\n"), 0o644)
}

// AccountDir is the folder that holds the logins of an account.
func AccountDir(configPath, name string, acc Account) string {
	if d := strings.TrimSpace(acc.Dir); d != "" {
		return expandHome(d)
	}
	return filepath.Join(filepath.Dir(configPath), "accounts", name)
}

// AccountEnv is the environment an agent gets when it runs with an account.
func (l Launch) AccountEnv(configPath, name, agent string) (map[string]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	acc, ok := l.Accounts[name]
	if !ok {
		return nil, fmt.Errorf("account %q is not in config.toml", name)
	}
	env := map[string]string{}
	if v := accountHomes[agent]; v != "" {
		dir := filepath.Join(AccountDir(configPath, name, acc), agent)
		_ = os.MkdirAll(dir, 0o700)
		env[v] = dir
	}
	for k, v := range acc.Env {
		resolved, err := resolveEnvValue(name, k, v)
		if err != nil {
			return nil, err
		}
		env[k] = resolved
	}
	return env, nil
}

// ForAccount returns a copy of l whose agents run with the account's logins.
func (l Launch) ForAccount(configPath, name string) (Launch, error) {
	if strings.TrimSpace(name) == "" {
		return l, nil
	}
	out := l
	out.Agents = make(map[string]AgentSpec, len(l.Agents))
	for agent, spec := range l.Agents {
		env, err := l.AccountEnv(configPath, name, agent)
		if err != nil {
			return l, err
		}
		merged := map[string]string{}
		for k, v := range spec.Env {
			merged[k] = v
		}
		for k, v := range env {
			merged[k] = v
		}
		spec.Env = merged
		out.Agents[agent] = spec
	}
	return out, nil
}

// HookCommands reads [hooks]: each event takes one command or a list.
func HookCommands(raw map[string]any) map[string][]string {
	if len(raw) == 0 {
		return nil
	}
	out := map[string][]string{}
	for event, v := range raw {
		switch c := v.(type) {
		case string:
			if strings.TrimSpace(c) != "" {
				out[event] = append(out[event], c)
			}
		case []any:
			for _, item := range c {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					out[event] = append(out[event], s)
				}
			}
		}
	}
	return out
}

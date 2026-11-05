package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type AgentSpec struct {
	Command string            `toml:"command"`
	Args    []string          `toml:"args"`
	Env     map[string]string `toml:"env"`
}

// TokenPrice is USD per million tokens.
type TokenPrice struct {
	Input  float64 `toml:"input"`
	Output float64 `toml:"output"`
	Cache  float64 `toml:"cache"`
}

type Launch struct {
	BranchPrefix string                `toml:"branch_prefix"`
	Worktrees    string                `toml:"worktrees"`
	Notify       bool                  `toml:"notify"`
	Terminal     string                `toml:"terminal"`
	Agents       map[string]AgentSpec  `toml:"agents"`
	Prices       map[string]TokenPrice `toml:"prices"`
}

type Config struct {
	Theme        string `json:"theme"`
	WelcomeShown bool   `json:"welcome_shown"`
	Launch       Launch `json:"-"`
}

type tomlConfig struct {
	BranchPrefix string                `toml:"branch_prefix"`
	Worktrees    string                `toml:"worktrees"`
	Notify       *bool                 `toml:"notify"`
	Terminal     string                `toml:"terminal"`
	Agents       map[string]AgentSpec  `toml:"agents"`
	Prices       map[string]TokenPrice `toml:"prices"`
}

func (c Config) NormalizedTheme() string {
	if strings.TrimSpace(c.Theme) == "" {
		return "dark"
	}
	return strings.TrimSpace(c.Theme)
}

func DefaultPrice() TokenPrice {
	return TokenPrice{Input: 3, Output: 15, Cache: 0.3}
}

func DefaultPrices() map[string]TokenPrice {
	return map[string]TokenPrice{
		"default": DefaultPrice(),
		"codex":   {Input: 1.25, Output: 10, Cache: 0.125},
	}
}

func (l Launch) PriceFor(agent string) TokenPrice {
	if l.Prices != nil {
		if p, ok := l.Prices[agent]; ok {
			return p
		}
		if p, ok := l.Prices["default"]; ok {
			return p
		}
	}
	return DefaultPrice()
}

func DefaultLaunch(dataDir string) Launch {
	return Launch{
		BranchPrefix: "",
		Worktrees:    filepath.Join(dataDir, "worktrees"),
		Notify:       true,
		Prices:       DefaultPrices(),
		Agents: map[string]AgentSpec{
			"claude": {
				Command: "claude",
				Args:    []string{"-p", "{{prompt}}", "--dangerously-skip-permissions", "--output-format", "stream-json"},
			},
			"codex": {
				Command: "codex",
				Args:    []string{"exec", "--dangerously-bypass-approvals-and-sandbox", "{{prompt}}"},
			},
			"opencode": {
				Command: "opencode",
				Args:    []string{"run", "--auto", "{{prompt}}"},
				Env:     map[string]string{"OPENCODE_PERMISSION": `{"edit":"allow","bash":"allow","webfetch":"allow"}`},
			},
			"pi": {
				Command: "pi",
				Args:    []string{"-p", "{{prompt}}"},
			},
		},
	}
}

func Load(path string) (Config, error) {
	_, _, dbPath, err := Paths()
	dataDir := filepath.Dir(dbPath)
	if err != nil {
		dataDir = filepath.Join(os.TempDir(), "troopa")
	}
	c := Config{Theme: "dark", Launch: DefaultLaunch(dataDir)}
	if b, err := os.ReadFile(path); err != nil {
		if !os.IsNotExist(err) {
			return Config{}, err
		}
	} else {
		if err := json.Unmarshal(b, &c); err != nil {
			return Config{}, fmt.Errorf("config: %w", err)
		}
		c.Theme = c.NormalizedTheme()
		c.Launch = DefaultLaunch(dataDir)
	}
	tomlPath := TOMLPath(path)
	raw, err := os.ReadFile(tomlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return Config{}, err
	}
	var t tomlConfig
	if err := toml.Unmarshal(raw, &t); err != nil {
		return Config{}, fmt.Errorf("config.toml: %w", err)
	}
	if t.BranchPrefix != "" {
		c.Launch.BranchPrefix = t.BranchPrefix
	}
	if strings.TrimSpace(t.Worktrees) != "" {
		c.Launch.Worktrees = expandHome(t.Worktrees)
	}
	if t.Notify != nil {
		c.Launch.Notify = *t.Notify
	}
	if strings.TrimSpace(t.Terminal) != "" {
		c.Launch.Terminal = strings.TrimSpace(t.Terminal)
	}
	for name, spec := range t.Agents {
		if spec.Command == "" {
			continue
		}
		if c.Launch.Agents == nil {
			c.Launch.Agents = map[string]AgentSpec{}
		}
		c.Launch.Agents[name] = spec
	}
	for name, price := range t.Prices {
		if c.Launch.Prices == nil {
			c.Launch.Prices = map[string]TokenPrice{}
		}
		c.Launch.Prices[name] = price
	}
	return c, nil
}

func Save(path string, c Config) error {
	c.Theme = c.NormalizedTheme()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(struct {
		Theme        string `json:"theme"`
		WelcomeShown bool   `json:"welcome_shown"`
	}{Theme: c.Theme, WelcomeShown: c.WelcomeShown}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func ThemesDir(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "themes")
}

func TOMLPath(jsonPath string) string {
	return filepath.Join(filepath.Dir(jsonPath), "config.toml")
}

func Paths() (dataDir, configPath, dbPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	dataDir = filepath.Join(data, "troopa")
	configPath = filepath.Join(cfg, "troopa", "config.json")
	dbPath = filepath.Join(dataDir, "troopa.db")
	return dataDir, configPath, dbPath, nil
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		if p == "~" {
			return home
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

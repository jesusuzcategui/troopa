package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	Editor       string                `toml:"editor"`
	Merge        string                `toml:"merge"`
	Agents       map[string]AgentSpec  `toml:"agents"`
	Prices       map[string]TokenPrice `toml:"prices"`
}

// Config is everything Troopa reads from config.toml. Theme and WelcomeShown
// are written back by the TUI; the rest is edited by hand.
type Config struct {
	Theme        string
	WelcomeShown bool
	BoardView    string   // "board" or "list"
	Collapsed    []string // statuses folded on the board and in the list
	Launch       Launch
}

type tomlConfig struct {
	Theme        string                `toml:"theme"`
	WelcomeShown bool                  `toml:"welcome_shown"`
	BoardView    string                `toml:"board_view"`
	Collapsed    []string              `toml:"collapsed"`
	BranchPrefix string                `toml:"branch_prefix"`
	Worktrees    string                `toml:"worktrees"`
	Notify       *bool                 `toml:"notify"`
	Terminal     string                `toml:"terminal"`
	Editor       string                `toml:"editor"`
	Merge        string                `toml:"merge"`
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

// Load reads config.toml at path. A missing file means defaults.
func Load(path string) (Config, error) {
	_, _, dbPath, err := Paths()
	dataDir := filepath.Dir(dbPath)
	if err != nil {
		dataDir = filepath.Join(os.TempDir(), "troopa")
	}
	c := Config{Theme: "dark", Launch: DefaultLaunch(dataDir)}
	loadLegacyJSON(path, &c)
	raw, err := os.ReadFile(path)
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
	if strings.TrimSpace(t.Theme) != "" {
		c.Theme = strings.TrimSpace(t.Theme)
	}
	if t.WelcomeShown {
		c.WelcomeShown = true
	}
	if v := strings.TrimSpace(t.BoardView); v != "" {
		c.BoardView = v
	}
	if t.Collapsed != nil {
		c.Collapsed = t.Collapsed
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
	if strings.TrimSpace(t.Editor) != "" {
		c.Launch.Editor = strings.TrimSpace(t.Editor)
	}
	if strings.TrimSpace(t.Merge) != "" {
		c.Launch.Merge = strings.TrimSpace(t.Merge)
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

// loadLegacyJSON reads theme and welcome flag from the config.json used by
// early builds, so they survive the move to config.toml.
func loadLegacyJSON(tomlPath string, c *Config) {
	b, err := os.ReadFile(filepath.Join(filepath.Dir(tomlPath), "config.json"))
	if err != nil {
		return
	}
	var legacy struct {
		Theme        string `json:"theme"`
		WelcomeShown bool   `json:"welcome_shown"`
	}
	if json.Unmarshal(b, &legacy) != nil {
		return
	}
	if strings.TrimSpace(legacy.Theme) != "" {
		c.Theme = strings.TrimSpace(legacy.Theme)
	}
	c.WelcomeShown = legacy.WelcomeShown
}

// Save writes theme and welcome_shown into config.toml, keeping every other
// line (agents, prices, comments) as the user wrote it.
func Save(path string, c Config) error {
	c.Theme = c.NormalizedTheme()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := setRootKey(string(raw), "theme", strconv.Quote(c.Theme))
	text = setRootKey(text, "welcome_shown", strconv.FormatBool(c.WelcomeShown))
	if c.BoardView != "" {
		text = setRootKey(text, "board_view", strconv.Quote(c.BoardView))
	}
	if c.Collapsed != nil {
		quoted := make([]string, len(c.Collapsed))
		for i, s := range c.Collapsed {
			quoted[i] = strconv.Quote(s)
		}
		text = setRootKey(text, "collapsed", "["+strings.Join(quoted, ", ")+"]")
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// setRootKey replaces key = ... in the root table (before the first [table])
// or inserts it at the top of the file.
func setRootKey(text, key, value string) string {
	line := key + " = " + value
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") {
			break
		}
		if k, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(k) == key {
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}
	if strings.TrimSpace(text) == "" {
		return line + "\n"
	}
	return line + "\n" + text
}

func ThemesDir(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "themes")
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
	configPath = filepath.Join(cfg, "troopa", "config.toml")
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

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

// Account is a separate login for the agent CLIs, for example a company
// subscription next to a personal one. Each agent keeps its login in its
// own folder under Dir; Env adds or overrides variables.
type Account struct {
	Dir string            `toml:"dir"`
	Env map[string]string `toml:"env"`
}

type Launch struct {
	Accounts     map[string]Account    `toml:"-"`
	Hooks        map[string][]string   `toml:"-"`
	BranchPrefix string                `toml:"branch_prefix"`
	Worktrees    string                `toml:"worktrees"`
	Notify       bool                  `toml:"notify"`
	Terminal     string                `toml:"terminal"`
	Editor       string                `toml:"editor"`
	Merge        string                `toml:"merge"`
	Brain        string                `toml:"-"`
	Agents       map[string]AgentSpec  `toml:"agents"`
	Prices       map[string]TokenPrice `toml:"prices"`
}

// Config is everything Troopa reads from config.toml. Theme and WelcomeShown
// are written back by the TUI; the rest is edited by hand.
type Config struct {
	Theme        string
	WelcomeShown bool
	SetupDone    bool   // the first-run setup finished
	Language     string // "en", "es" or "auto" (system); empty means English
	DefaultAgent string // agent for new tasks; empty means "me"
	DefaultModel string
	// DefaultAccount is the account for projects that do not pick one; empty
	// means the agents' normal login.
	DefaultAccount string
	// DefaultProfile is the id of the profile used when a project has none.
	DefaultProfile int64
	HideLogo       bool // skip the logo screen at start
	// WorktreesRaw is worktrees exactly as written in config.toml ("" = default).
	WorktreesRaw    string
	BrainRaw        string // brain folder as written in config.toml ("" = default)
	BrainGit        bool   // local commit after grouped writes; never push
	BoardView       string // "board" or "list"
	Collapsed       []string
	MaxAttachmentMB int
	Launch          Launch
}

type tomlConfig struct {
	Theme           string                `toml:"theme"`
	WelcomeShown    bool                  `toml:"welcome_shown"`
	SetupDone       bool                  `toml:"setup_done"`
	Language        string                `toml:"language"`
	DefaultAgent    string                `toml:"default_agent"`
	DefaultModel    string                `toml:"default_model"`
	HideLogo        bool                  `toml:"hide_logo"`
	BoardView       string                `toml:"board_view"`
	Collapsed       []string              `toml:"collapsed"`
	BranchPrefix    string                `toml:"branch_prefix"`
	Worktrees       string                `toml:"worktrees"`
	Notify          *bool                 `toml:"notify"`
	Terminal        string                `toml:"terminal"`
	Editor          string                `toml:"editor"`
	Merge           string                `toml:"merge"`
	MaxAttachmentMB int                   `toml:"max_attachment_mb"`
	Brain           string                `toml:"brain"`
	BrainGit        *bool                 `toml:"brain_git"`
	Agents          map[string]AgentSpec  `toml:"agents"`
	Prices          map[string]TokenPrice `toml:"prices"`
	DefaultAccount  string                `toml:"default_account"`
	DefaultProfile  int64                 `toml:"default_profile"`
	Accounts        map[string]Account    `toml:"accounts"`
	Hooks           map[string]any        `toml:"hooks"`
}

const DefaultMaxAttachmentMB = 50

func (c Config) MaxAttachmentBytes() int64 {
	mb := c.MaxAttachmentMB
	if mb <= 0 {
		mb = DefaultMaxAttachmentMB
	}
	return int64(mb) << 20
}

func AttachmentsDir(taskID int64) (string, error) {
	dataDir, _, _, err := Paths()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "attachments", strconv.FormatInt(taskID, 10)), nil
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
			c.Launch.Brain = ResolveBrain("")
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
	c.SetupDone = t.SetupDone
	c.Language = strings.TrimSpace(t.Language)
	c.DefaultAgent = strings.TrimSpace(t.DefaultAgent)
	c.DefaultModel = strings.TrimSpace(t.DefaultModel)
	c.HideLogo = t.HideLogo
	c.WorktreesRaw = strings.TrimSpace(t.Worktrees)
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
	if t.MaxAttachmentMB > 0 {
		c.MaxAttachmentMB = t.MaxAttachmentMB
	}
	c.BrainRaw = strings.TrimSpace(t.Brain)
	c.Launch.Brain = ResolveBrain(c.BrainRaw)
	if t.BrainGit != nil {
		c.BrainGit = *t.BrainGit
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
	c.DefaultAccount = strings.TrimSpace(t.DefaultAccount)
	c.DefaultProfile = t.DefaultProfile
	for name, acc := range t.Accounts {
		if c.Launch.Accounts == nil {
			c.Launch.Accounts = map[string]Account{}
		}
		c.Launch.Accounts[name] = acc
	}
	c.Launch.Hooks = HookCommands(t.Hooks)
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
	text = setBool(text, "setup_done", c.SetupDone)
	text = setBool(text, "hide_logo", c.HideLogo)
	text = setString(text, "language", c.Language)
	text = setString(text, "default_agent", c.DefaultAgent)
	text = setString(text, "default_model", c.DefaultModel)
	text = setString(text, "default_account", c.DefaultAccount)
	if c.DefaultProfile > 0 {
		text = setRootKey(text, "default_profile", strconv.FormatInt(c.DefaultProfile, 10))
	}
	text = setString(text, "editor", c.Launch.Editor)
	text = setString(text, "terminal", c.Launch.Terminal)
	text = setString(text, "merge", c.Launch.Merge)
	text = setString(text, "branch_prefix", c.Launch.BranchPrefix)
	text = setString(text, "worktrees", c.WorktreesRaw)
	text = setString(text, "brain", c.BrainRaw)
	text = setBool(text, "brain_git", c.BrainGit)
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

// setString writes key = "value". Empty values are skipped, so a partial
// Config never wipes what the user wrote; use Unset to remove a key.
func setString(text, key, value string) string {
	if value == "" {
		return text
	}
	return setRootKey(text, key, strconv.Quote(value))
}

// Unset removes a root key from config.toml, so its default applies again.
func Unset(path, key string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines))
	inRoot := true
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") {
			inRoot = false
		}
		if inRoot {
			if k, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(k) == key {
				continue
			}
		}
		out = append(out, l)
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

// setBool writes key = true, or false only when the key is already there.
func setBool(text, key string, value bool) string {
	if !value && !hasRootKey(text, key) {
		return text
	}
	return setRootKey(text, key, strconv.FormatBool(value))
}

func hasRootKey(text, key string) bool {
	for _, l := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") {
			return false
		}
		if k, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(k) == key {
			return true
		}
	}
	return false
}

// ResolveWorktrees turns a worktrees setting into the folder Troopa uses:
// empty means the default under the data folder.
func ResolveWorktrees(raw string) string {
	if strings.TrimSpace(raw) != "" {
		return expandHome(strings.TrimSpace(raw))
	}
	dataDir, _, _, err := Paths()
	if err != nil {
		dataDir = filepath.Join(os.TempDir(), "troopa")
	}
	return filepath.Join(dataDir, "worktrees")
}

// ResolveBrain turns a brain setting into the folder Troopa uses:
// empty means $XDG_DATA_HOME/troopa/brain.
func ResolveBrain(raw string) string {
	if strings.TrimSpace(raw) != "" {
		return expandHome(strings.TrimSpace(raw))
	}
	dataDir, _, _, err := Paths()
	if err != nil {
		dataDir = filepath.Join(os.TempDir(), "troopa")
	}
	return filepath.Join(dataDir, "brain")
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

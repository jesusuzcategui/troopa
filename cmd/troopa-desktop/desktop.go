package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
)

// Settings exposes the desktop UI defaults derived from Troopa's configuration.
type Settings struct {
	Language       string  `json:"language"`
	Theme          string  `json:"theme"`
	ThemeMode      string  `json:"theme_mode"`
	DefaultAccount string  `json:"default_account"`
	DefaultProfile int64   `json:"default_profile"`
	DefaultAgent   string  `json:"default_agent"`
	Worktrees      string  `json:"worktrees"`
	Terminal       string  `json:"terminal"`
	SetupDone      bool    `json:"setup_done"`
	Agents         []Agent `json:"agents"`
}

// SettingsInput is the small, safe-to-edit part of config.toml shown in the
// desktop first-run flow and Settings page.
type SettingsInput struct {
	Language     string `json:"language"`
	DefaultAgent string `json:"default_agent"`
	Worktrees    string `json:"worktrees"`
	Terminal     string `json:"terminal"`
	SetupDone    bool   `json:"setup_done"`
}

// Agent is an installed-agent result for the desktop UI.
type Agent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Command   string `json:"command"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
}

// DesktopService provides desktop-specific helpers (native dialogs and initial UI settings).
type DesktopService struct {
	wailsApp   *application.App
	cfg        config.Config
	configPath string
}

func NewDesktopService(wailsApp *application.App, cfg config.Config, configPath ...string) *DesktopService {
	d := &DesktopService{wailsApp: wailsApp, cfg: cfg}
	if len(configPath) > 0 {
		d.configPath = configPath[0]
	}
	return d
}

func (d *DesktopService) GetSettings() Settings {
	d.reloadConfig()
	// Prefer config.toml (and TROOPA_LANG via Apply) so the desktop UI matches the TUI.
	if d.configPath != "" || strings.TrimSpace(d.cfg.Language) != "" {
		i18n.Apply(d.cfg.Language)
	}
	agents := launch.DetectAgents(d.cfg.Launch)
	items := make([]Agent, len(agents))
	for i, agent := range agents {
		items[i] = Agent{ID: agent.ID, Name: agent.Name, Command: agent.Command, Version: agent.Version, Installed: agent.Installed}
	}
	themeName := d.cfg.NormalizedTheme()
	return Settings{
		Language:       i18n.Lang(),
		Theme:          themeName,
		ThemeMode:      themeModeFromConfig(themeName),
		DefaultAccount: d.cfg.DefaultAccount,
		DefaultProfile: d.cfg.DefaultProfile,
		DefaultAgent:   d.cfg.DefaultAgent,
		Worktrees:      d.cfg.WorktreesRaw,
		Terminal:       d.cfg.Launch.Terminal,
		SetupDone:      d.cfg.SetupDone,
		Agents:         items,
	}
}

// SaveSettings persists the values changed from the desktop UI immediately.
func (d *DesktopService) SaveSettings(in SettingsInput) error {
	d.reloadConfig()
	d.cfg.Language = strings.TrimSpace(in.Language)
	d.cfg.DefaultAgent = strings.TrimSpace(in.DefaultAgent)
	d.cfg.WorktreesRaw = strings.TrimSpace(in.Worktrees)
	d.cfg.Launch.Worktrees = config.ResolveWorktrees(d.cfg.WorktreesRaw)
	d.cfg.Launch.Terminal = strings.TrimSpace(in.Terminal)
	d.cfg.SetupDone = in.SetupDone
	d.cfg.WelcomeShown = in.SetupDone
	if err := config.Save(d.configFile(), d.cfg); err != nil {
		return err
	}
	i18n.Apply(d.cfg.Language)
	return nil
}

// CreateAccount makes an account available for either a key or an interactive login.
func (d *DesktopService) CreateAccount(name string) error {
	if err := config.EnsureAccount(d.configFile(), name); err != nil {
		return err
	}
	d.reloadConfig()
	return nil
}

// SaveAccountKey writes the secret to the OS keyring; config.toml only keeps
// the keyring marker used when an agent is launched.
func (d *DesktopService) SaveAccountKey(account, variable, secret string) error {
	if err := d.CreateAccount(account); err != nil {
		return err
	}
	return config.SetAccountKey(d.configFile(), account, variable, secret)
}

// StartLogin opens the configured terminal with `troopa login`, which reads
// the account from config and never puts account keys into a temporary file.
func (d *DesktopService) StartLogin(account, agent string) error {
	d.reloadConfig()
	if _, ok := d.cfg.Launch.Accounts[strings.TrimSpace(account)]; !ok {
		return fmt.Errorf("account %q is not configured", account)
	}
	if _, err := exec.LookPath("troopa"); err != nil {
		return fmt.Errorf("troopa is not on PATH: %w", err)
	}
	terminal, err := launch.ResolveTerminal(d.cfg.Launch.Terminal)
	if err != nil {
		return err
	}
	parts := strings.Fields(terminal)
	if len(parts) == 0 {
		return fmt.Errorf("terminal is empty")
	}
	parts = append(parts, "troopa", "login", account, agent)
	if err := exec.Command(parts[0], parts[1:]...).Start(); err != nil {
		return err
	}
	return nil
}

func (d *DesktopService) configFile() string {
	if d.configPath != "" {
		return d.configPath
	}
	_, path, _, err := config.Paths()
	if err == nil {
		return path
	}
	return "config.toml"
}

func (d *DesktopService) reloadConfig() {
	if d.configPath == "" {
		return
	}
	if cfg, err := config.Load(d.configFile()); err == nil {
		d.cfg = cfg
	}
}

func (d *DesktopService) PickDirectory(title string) (string, error) {
	if d.wailsApp == nil {
		return "", nil
	}
	dlg := d.wailsApp.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false)
	if strings.TrimSpace(title) != "" {
		dlg.SetTitle(title)
	}
	return dlg.PromptForSingleSelection()
}

func (d *DesktopService) PickFile(title string) (string, error) {
	if d.wailsApp == nil {
		return "", nil
	}
	dlg := d.wailsApp.Dialog.OpenFile().
		CanChooseDirectories(false).
		CanChooseFiles(true)
	if strings.TrimSpace(title) != "" {
		dlg.SetTitle(title)
	}
	return dlg.PromptForSingleSelection()
}

func themeModeFromConfig(theme string) string {
	t := strings.ToLower(strings.TrimSpace(theme))
	if strings.Contains(t, "light") || strings.Contains(t, "latte") {
		return "light"
	}
	return "dark"
}

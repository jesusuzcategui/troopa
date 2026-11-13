package main

import (
	"path/filepath"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/app"
	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestDesktopSettingsThemeMode(t *testing.T) {
	cases := []struct {
		theme string
		want  string
	}{
		{"", "dark"},
		{"dark", "dark"},
		{"catppuccin-mocha", "dark"},
		{"light", "light"},
		{"catppuccin-latte", "light"},
		{"solarized-light", "light"},
		{"gruvbox-light", "light"},
	}
	for _, tc := range cases {
		if got := themeModeFromConfig(tc.theme); got != tc.want {
			t.Errorf("themeModeFromConfig(%q) = %q, want %q", tc.theme, got, tc.want)
		}
	}
}

func TestDesktopSaveSettingsPersistsLanguage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("TROOPA_LANG", "")

	_, configPath, _, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Language = "en"
	cfg.SetupDone = true
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	i18n.Apply("en")
	t.Cleanup(func() { i18n.Apply("") })

	ds := NewDesktopService(nil, cfg, configPath)
	if err := ds.SaveSettings(SettingsInput{
		Language:     "es",
		DefaultAgent: "me",
		SetupDone:    true,
	}); err != nil {
		t.Fatal(err)
	}
	got := ds.GetSettings()
	if got.Language != "es" {
		t.Fatalf("GetSettings language = %q, want es", got.Language)
	}
	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Language != "es" {
		t.Fatalf("config.toml language = %q, want es", reloaded.Language)
	}
}

func TestDesktopServiceSharesStoreWithTUI(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, configPath, dbPath, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Theme = "catppuccin-latte"
	cfg.DefaultAccount = "work"
	cfg.DefaultProfile = 2
	i18n.Apply("es")
	t.Cleanup(func() { i18n.Apply("") })

	ds := NewDesktopService(nil, cfg)
	got := ds.GetSettings()
	if got.Theme != "catppuccin-latte" || got.ThemeMode != "light" || got.Language != "es" || got.DefaultAccount != "work" || got.DefaultProfile != 2 {
		t.Fatalf("unexpected settings: %+v", got)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	svc := app.New(s, cfg, configPath)
	repo := filepath.Join(t.TempDir(), "repo")
	created, err := svc.CreateProject(store.Project{Name: "Shared", RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	// Re-open the same DB path (as TUI/CLI does) and verify the project is there.
	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	list, err := s2.List(false)
	if err != nil || len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("reopened store list = %+v, err = %v", list, err)
	}
}

func TestDesktopSaveSettingsPreservesThemeSetViaAppService(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	_, configPath, dbPath, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	svc := app.New(s, cfg, configPath)
	ds := NewDesktopService(nil, cfg, configPath)

	if err := svc.SetTheme("nord"); err != nil {
		t.Fatal(err)
	}
	if err := ds.SaveSettings(SettingsInput{
		Language:     "en",
		DefaultAgent: "me",
		SetupDone:    true,
	}); err != nil {
		t.Fatal(err)
	}
	got := ds.GetSettings()
	if got.Theme != "nord" {
		t.Fatalf("GetSettings().Theme = %q, want nord", got.Theme)
	}
	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Theme != "nord" {
		t.Fatalf("config.toml theme = %q, want nord", reloaded.Theme)
	}
}

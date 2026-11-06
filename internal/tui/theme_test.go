package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestBundledThemes(t *testing.T) {
	ts := loadThemes("")
	got := map[string]bool{}
	for _, th := range ts {
		got[th.Name] = true
	}
	for _, name := range []string{"dark", "light", "tropic", "nord", "dracula", "gruvbox", "gruvbox-light",
		"catppuccin-mocha", "catppuccin-latte", "tokyo-night", "rose-pine", "solarized-light",
		"cafe", "midnight", "high-contrast"} {
		if !got[name] {
			t.Fatalf("missing bundled theme %s", name)
		}
	}
}

func TestUserThemeOverridesAndCreate(t *testing.T) {
	m := newTestModel(t)
	dir := filepath.Dir(m.configPath)
	m.themeDir = filepath.Join(dir, "themes")
	if err := os.MkdirAll(m.themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := []byte(`
name = "sunset"
[colors]
fg = "#FFE8D6"
bg = "#2B1B17"
accent = "#FF7A59"
muted = "#A8897A"
selected = "#3D2A24"
status = "#35241F"
error = "#FF5D5D"
diff_add = "#8FBF6A"
diff_del = "#FF5D5D"
diff_hunk = "#7EB8FF"
`)
	if err := os.WriteFile(filepath.Join(m.themeDir, "sunset.toml"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	m.themes = loadThemes(m.themeDir)
	m.applyTheme("sunset")
	if m.theme.Name != "sunset" {
		t.Fatalf("theme %q", m.theme.Name)
	}
	m.createUserTheme()
	if m.theme.Name != "custom" {
		t.Fatalf("created %q", m.theme.Name)
	}
	if _, err := os.Stat(filepath.Join(m.themeDir, "custom.toml")); err != nil {
		t.Fatal(err)
	}
}

func TestCycleTheme(t *testing.T) {
	m := newTestModel(t)
	start := m.theme.Name
	m.cycleTheme()
	if m.theme.Name == start && len(m.themes) > 1 {
		t.Fatal("theme did not cycle")
	}
}

func TestFirstRunSetup(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	path := filepath.Join(dir, "config.toml")
	m := New(s, config.Config{Theme: "dark"}, path)
	if m.screen != screenSetup {
		t.Fatalf("first run should open the setup, screen %v", m.screen)
	}
	if !strings.Contains(m.View(), "▀█▀ █▀█") {
		t.Fatal("setup should show the logo")
	}
	var next tea.Model = m
	// language: move to Español, then accept every other step as proposed
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	for i := 0; i < int(stepCount); i++ {
		next, _ = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	got := next.(Model)
	defer i18n.SetLang(i18n.EN)
	if got.screen != screenList || !got.cfg.SetupDone || got.cfg.Language != "es" {
		t.Fatalf("setup not finished: screen %v cfg %+v", got.screen, got.cfg)
	}
	saved, err := config.Load(path)
	if err != nil || !saved.SetupDone || saved.Language != "es" {
		t.Fatalf("setup not saved: %+v %v", saved, err)
	}
}

func TestSettingsChangeAndAbout(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	got := next.(Model)
	if got.screen != screenSettings {
		t.Fatalf("S should open settings, screen %v", got.screen)
	}
	mergeRow := 0
	for i, r := range settingRows {
		if r.key == "merge" {
			mergeRow = i
		}
	}
	for i := 0; i < mergeRow; i++ {
		next, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
		got = next.(Model)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRight})
	got = next.(Model)
	if got.cfg.Launch.Merge != "merge" {
		t.Fatalf("merge = %q", got.cfg.Launch.Merge)
	}
	saved, _ := config.Load(got.configPath)
	if saved.Launch.Merge != "merge" {
		t.Fatal("merge not saved")
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	got = next.(Model)
	if !got.about || !strings.Contains(got.View(), "Jesús Uzcátegui") {
		t.Fatal("about window missing")
	}
}

func TestCtrlKPaletteRunsHelp(t *testing.T) {
	m := newTestModel(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	got := next.(Model)
	if got.screen != screenPalette {
		t.Fatalf("screen %v want palette", got.screen)
	}
	got.paletteInput.SetValue("help")
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if got.screen != screenHelp {
		t.Fatalf("screen %v want help", got.screen)
	}
}

func TestThemeIsLight(t *testing.T) {
	for _, th := range loadThemes("") {
		want := th.Name == "light" || strings.HasSuffix(th.Name, "-light") || th.Name == "catppuccin-latte"
		if th.IsLight() != want {
			t.Errorf("%s: IsLight = %v, want %v", th.Name, th.IsLight(), want)
		}
	}
}

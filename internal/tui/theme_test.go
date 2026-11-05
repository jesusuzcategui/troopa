package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestBundledThemes(t *testing.T) {
	ts := loadThemes("")
	got := map[string]bool{}
	for _, th := range ts {
		got[th.Name] = true
	}
	for _, name := range []string{"dark", "light", "tropic"} {
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

func TestWelcomeFirstRun(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	m := New(s, config.Config{Theme: "dark"}, filepath.Join(dir, "config.toml"))
	if m.screen != screenWelcome {
		t.Fatalf("screen %v", m.screen)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.screen != screenList {
		t.Fatalf("after enter screen %v", got.screen)
	}
	if !got.cfg.WelcomeShown {
		t.Fatal("welcome_shown not persisted in memory")
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

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTOMLAgents(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte(`
branch_prefix = "acme/"
notify = false
worktrees = "/tmp/wt"
terminal = "kitty -e"
editor = "code --new-window"

[agents.claude]
command = "claude"
args = ["-p", "{{prompt}}"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	if c.Launch.BranchPrefix != "acme/" {
		t.Fatalf("prefix %q", c.Launch.BranchPrefix)
	}
	if c.Launch.Notify {
		t.Fatal("notify should be false")
	}
	if c.Launch.Worktrees != "/tmp/wt" {
		t.Fatalf("wt %q", c.Launch.Worktrees)
	}
	if c.Launch.Editor != "code --new-window" {
		t.Fatalf("editor %q", c.Launch.Editor)
	}
	if c.Launch.Terminal != "kitty -e" {
		t.Fatalf("terminal %q", c.Launch.Terminal)
	}
	if c.Launch.Agents["claude"].Command != "claude" {
		t.Fatal("claude missing")
	}
}

func TestSaveThemeAndWelcome(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte("# my agents\nnotify = false\n\n[agents.pi]\ncommand = \"pi\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Config{Theme: "tropic", WelcomeShown: true}
	if err := Save(tomlPath, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "tropic" {
		t.Fatalf("theme %q", got.Theme)
	}
	if !got.WelcomeShown {
		t.Fatal("welcome_shown")
	}
	if got.Launch.Notify || got.Launch.Agents["pi"].Command != "pi" {
		t.Fatalf("hand-written settings lost: %+v", got.Launch)
	}
	raw, _ := os.ReadFile(tomlPath)
	if !strings.Contains(string(raw), "# my agents") {
		t.Fatalf("comment lost:\n%s", raw)
	}
	c.Theme = "light"
	if err := Save(tomlPath, c); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(tomlPath)
	if strings.Count(string(raw), "theme =") != 1 {
		t.Fatalf("theme duplicated:\n%s", raw)
	}
	if ThemesDir(tomlPath) != filepath.Join(dir, "themes") {
		t.Fatalf("themes dir %s", ThemesDir(tomlPath))
	}
}

func TestLoadTOMLPrices(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte(`
[prices.default]
input = 1.0
output = 2.0
cache = 0.1

[prices.pi]
input = 9.0
output = 18.0
cache = 0.5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	if c.Launch.PriceFor("pi").Input != 9 {
		t.Fatalf("pi price %+v", c.Launch.PriceFor("pi"))
	}
	if c.Launch.PriceFor("claude").Input != 1 {
		t.Fatalf("default fallback %+v", c.Launch.PriceFor("claude"))
	}
}

func TestLoadLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"theme":"tropic","welcome_shown":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme != "tropic" || !c.WelcomeShown {
		t.Fatalf("legacy config not read: %+v", c)
	}
}

func TestSaveBoardLayout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	c := Config{Theme: "dark", BoardView: "list", Collapsed: []string{"done", "blocked"}}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.BoardView != "list" || len(got.Collapsed) != 2 || got.Collapsed[1] != "blocked" {
		t.Fatalf("layout not saved: %+v", got)
	}
	c.Collapsed = []string{}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, _ = Load(path)
	if len(got.Collapsed) != 0 {
		t.Fatalf("collapsed not cleared: %+v", got.Collapsed)
	}
}

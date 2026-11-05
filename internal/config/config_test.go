package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTOMLAgents(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "config.json")
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte(`
branch_prefix = "acme/"
notify = false
worktrees = "/tmp/wt"
terminal = "kitty -e"

[agents.claude]
command = "claude"
args = ["-p", "{{prompt}}"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(jsonPath)
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
	if c.Launch.Terminal != "kitty -e" {
		t.Fatalf("terminal %q", c.Launch.Terminal)
	}
	if c.Launch.Agents["claude"].Command != "claude" {
		t.Fatal("claude missing")
	}
}

func TestSaveThemeAndWelcome(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "config.json")
	c := Config{Theme: "tropic", WelcomeShown: true}
	if err := Save(jsonPath, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "tropic" {
		t.Fatalf("theme %q", got.Theme)
	}
	if !got.WelcomeShown {
		t.Fatal("welcome_shown")
	}
	if ThemesDir(jsonPath) != filepath.Join(dir, "themes") {
		t.Fatalf("themes dir %s", ThemesDir(jsonPath))
	}
}

func TestLoadTOMLPrices(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "config.json")
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
	c, err := Load(jsonPath)
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

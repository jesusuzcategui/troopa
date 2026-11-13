package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAccountsAndHooksLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	text := `default_account = "work"
default_profile = 3

[accounts.work]
env = { ANTHROPIC_BASE_URL = "https://example.test" }

[accounts.home]
dir = "` + filepath.ToSlash(filepath.Join(dir, "homeacc")) + `"

[hooks]
review = "echo review"
approve = ["echo one", "echo two"]
`
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultAccount != "work" || c.DefaultProfile != 3 {
		t.Fatalf("defaults %q %d", c.DefaultAccount, c.DefaultProfile)
	}
	if got := c.Launch.AccountNames(); len(got) != 2 || got[0] != "home" {
		t.Fatalf("names %v", got)
	}
	if len(c.Launch.Hooks["approve"]) != 2 || c.Launch.Hooks["review"][0] != "echo review" {
		t.Fatalf("hooks %v", c.Launch.Hooks)
	}
	l, err := c.Launch.ForAccount(path, "work")
	if err != nil {
		t.Fatal(err)
	}
	env := l.Agents["claude"].Env
	if env["CLAUDE_CONFIG_DIR"] != filepath.Join(dir, "accounts", "work", "claude") || env["ANTHROPIC_BASE_URL"] == "" {
		t.Fatalf("env %v", env)
	}
	if c.Launch.Agents["claude"].Env["CLAUDE_CONFIG_DIR"] != "" {
		t.Fatal("ForAccount must not change the original config")
	}
	if _, err := c.Launch.ForAccount(path, "nope"); err == nil {
		t.Fatal("unknown account should fail")
	}
	home, _ := c.Launch.AccountEnv(path, "home", "codex")
	if home["CODEX_HOME"] != filepath.Join(dir, "homeacc", "codex") {
		t.Fatalf("home %v", home)
	}
	c.DefaultProfile = 5
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	again, _ := Load(path)
	if again.DefaultProfile != 5 || again.DefaultAccount != "work" || len(again.Launch.Accounts) != 2 {
		t.Fatalf("after save %+v", again)
	}
}

func TestEnsureAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := EnsureAccount(path, "personal"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAccount(path, "personal"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Launch.AccountNames(); len(got) != 1 || got[0] != "personal" {
		t.Fatalf("accounts %v", got)
	}
	if err := EnsureAccount(path, "bad.name"); err == nil {
		t.Fatal("invalid name should fail")
	}
}

func TestTelegramConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_ = os.WriteFile(path, []byte("[telegram]\ntoken = \"123:abc\"\nchat_id = -1001\nactions = true\n"), 0o644)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Telegram.Token != "123:abc" || c.Telegram.ChatID != "-1001" || !c.Telegram.Actions {
		t.Fatalf("%+v", c.Telegram)
	}
}

func TestTelegramUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_ = os.WriteFile(path, []byte("[telegram]\ntoken = \"x\"\nchat_id = 1\n\n[[telegram.users]]\nname = \"Ana\"\nchat_id = 77\nprojects = [\"crochet\"]\nagent = \"codex\"\n"), 0o644)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	u, ok := c.Telegram.UserFor("77")
	if !ok || u.Name != "Ana" || u.Agent != "codex" || !u.Allows("crochet", "Crochet") || u.Allows("app", "App") {
		t.Fatalf("%+v", c.Telegram.Users)
	}
}

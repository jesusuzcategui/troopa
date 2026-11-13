package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestAccountKeyringEnv(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	text := `[accounts.work]
env = { ANTHROPIC_API_KEY = "keyring:", ANTHROPIC_BASE_URL = "https://example.test" }
`
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := keyring.Set(keyringService, keyringUser("work", "ANTHROPIC_API_KEY"), "secret-from-keyring"); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	env, err := c.Launch.AccountEnv(path, "work", "claude")
	if err != nil {
		t.Fatal(err)
	}
	if env["ANTHROPIC_API_KEY"] != "secret-from-keyring" {
		t.Fatalf("key %q", env["ANTHROPIC_API_KEY"])
	}
	if env["ANTHROPIC_BASE_URL"] != "https://example.test" {
		t.Fatalf("url %q", env["ANTHROPIC_BASE_URL"])
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "secret-from-keyring") {
		t.Fatal("secret must not appear in config.toml")
	}
	if !strings.Contains(string(raw), `ANTHROPIC_API_KEY = "keyring:"`) {
		t.Fatalf("marker missing: %s", raw)
	}
}

func TestSetAndRemoveAccountKey(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[accounts.acme]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetAccountKey(path, "acme", "OPENAI_API_KEY", "sk-test"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "sk-test") {
		t.Fatal("secret leaked into config.toml")
	}
	if !strings.Contains(string(raw), KeyringMarker) {
		t.Fatalf("marker missing: %s", raw)
	}
	got, err := keyring.Get(keyringService, keyringUser("acme", "OPENAI_API_KEY"))
	if err != nil || got != "sk-test" {
		t.Fatalf("keyring get %q %v", got, err)
	}
	c, _ := Load(path)
	env, err := c.Launch.AccountEnv(path, "acme", "claude")
	if err != nil {
		t.Fatal(err)
	}
	if env["OPENAI_API_KEY"] != "sk-test" {
		t.Fatalf("env %v", env)
	}
	if err := RemoveAccountKey(path, "acme", "OPENAI_API_KEY"); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if strings.Contains(string(raw), "OPENAI_API_KEY") {
		t.Fatalf("env key still present: %s", raw)
	}
	if _, err := keyring.Get(keyringService, keyringUser("acme", "OPENAI_API_KEY")); err == nil {
		t.Fatal("secret should be gone from keyring")
	}
}

func TestAccountKeyringMissingSecret(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(path, []byte(`[accounts.work]
env = { GEMINI_API_KEY = "keyring:" }
`), 0o644)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Launch.AccountEnv(path, "work", "claude")
	if err == nil {
		t.Fatal("expected error when keyring secret is missing")
	}
}

func TestApplyAccountEnvPatchKeepsOtherKeys(t *testing.T) {
	in := `[accounts.work]
dir = "~/acc"
env = { FOO = "bar", BAZ = "qux" }
`
	out, err := applyAccountEnvPatch(in, "work", "FOO", KeyringMarker, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `FOO = "keyring:"`) || !strings.Contains(out, `BAZ = "qux"`) {
		t.Fatalf("%s", out)
	}
	out, err = applyAccountEnvPatch(out, "work", "FOO", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "FOO") || !strings.Contains(out, `BAZ = "qux"`) {
		t.Fatalf("%s", out)
	}
}

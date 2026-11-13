package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledThemes(t *testing.T) {
	ts := Load("")
	got := map[string]bool{}
	for _, th := range ts {
		got[th.Name] = true
		if th.Light != th.IsLight() {
			t.Errorf("%s: Light=%v, IsLight()=%v", th.Name, th.Light, th.IsLight())
		}
	}
	for _, name := range []string{"dark", "light", "tropic", "nord", "dracula", "gruvbox", "gruvbox-light",
		"catppuccin-mocha", "catppuccin-latte", "tokyo-night", "rose-pine", "solarized-light",
		"cafe", "midnight", "high-contrast"} {
		if !got[name] {
			t.Fatalf("missing bundled theme %s", name)
		}
	}
}

func TestUserThemesOverrideAndOrder(t *testing.T) {
	dir := t.TempDir()
	custom := `
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
`
	alpha := strings.ReplaceAll(custom, `"sunset"`, `"amber"`)
	overrideDark := strings.ReplaceAll(custom, `"sunset"`, `"dark"`)
	overrideDark = strings.ReplaceAll(overrideDark, `#FF7A59`, `#123456`)

	if err := os.WriteFile(filepath.Join(dir, "sunset.toml"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "amber.toml"), []byte(alpha), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dark.toml"), []byte(overrideDark), 0o644); err != nil {
		t.Fatal(err)
	}
	// Invalid TOML should be ignored without breaking loading.
	if err := os.WriteFile(filepath.Join(dir, "broken.toml"), []byte("name = "), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := Load(dir)
	dark := ByName(ts, "dark")
	if dark.Colors.Accent != "#123456" {
		t.Fatalf("dark override accent = %q, want #123456", dark.Colors.Accent)
	}
	// Extra user themes appear sorted at the end.
	n := len(ts)
	if n < 2 || ts[n-2].Name != "amber" || ts[n-1].Name != "sunset" {
		t.Fatalf("tail themes = %q, %q; want amber, sunset", ts[n-2].Name, ts[n-1].Name)
	}
}

func TestParseValidationAndEncode(t *testing.T) {
	if _, err := Parse([]byte(`[colors]`)); err == nil {
		t.Fatal("expected error for missing name")
	}
	if _, err := Parse([]byte("name = \"x\"\n[colors]\nfg = \"#fff\"\n")); err == nil {
		t.Fatal("expected error for incomplete colors")
	}
	p := DefaultDark()
	b, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "light =") {
		t.Fatalf("encoded TOML should not include light field:\n%s", string(b))
	}
	round, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if round != p {
		t.Fatalf("round-trip = %+v, want %+v", round, p)
	}
}

func TestByNameFallback(t *testing.T) {
	ts := Load("")
	if got := ByName(ts, "  nord  "); got.Name != "nord" {
		t.Fatalf("got %q, want nord", got.Name)
	}
	if got := ByName(ts, "missing"); got.Name != ts[0].Name {
		t.Fatalf("fallback = %q, want %q", got.Name, ts[0].Name)
	}
	if got := ByName(nil, "anything"); got.Name != "dark" {
		t.Fatalf("empty list fallback = %q, want dark", got.Name)
	}
}

package termimg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportsKitty(t *testing.T) {
	env := map[string]string{"KITTY_WINDOW_ID": "1", "TERM": "xterm-kitty"}
	if !SupportsEnv(func(k string) string { return env[k] }) {
		t.Fatal("expected kitty support")
	}
}

func TestSupportsGhostty(t *testing.T) {
	env := map[string]string{"TERM_PROGRAM": "ghostty", "TERM": "xterm-ghostty"}
	if !SupportsEnv(func(k string) string { return env[k] }) {
		t.Fatal("expected ghostty support")
	}
}

func TestSupportsWezTerm(t *testing.T) {
	env := map[string]string{"TERM_PROGRAM": "WezTerm", "TERM": "xterm-256color"}
	if !SupportsEnv(func(k string) string { return env[k] }) {
		t.Fatal("expected wezterm support")
	}
}

func TestSupportsPlainFalse(t *testing.T) {
	env := map[string]string{"TERM": "xterm-256color"}
	if SupportsEnv(func(k string) string { return env[k] }) {
		t.Fatal("plain xterm should not support graphics")
	}
}

func TestSupportsTmuxWithoutPassthrough(t *testing.T) {
	env := map[string]string{"TMUX": "/tmp/tmux-1", "TERM": "tmux-256color", "TERM_PROGRAM": "ghostty"}
	if SupportsEnv(func(k string) string { return env[k] }) {
		t.Fatal("tmux without kitty passthrough should be false")
	}
}

func TestSupportsTmuxWithKitty(t *testing.T) {
	env := map[string]string{"TMUX": "/tmp/tmux-1", "KITTY_WINDOW_ID": "9", "TERM": "xterm-kitty"}
	if !SupportsEnv(func(k string) string { return env[k] }) {
		t.Fatal("tmux under kitty should allow graphics")
	}
}

func TestPlaceAndClear(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.png")
	// Minimal 1x1 PNG
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xfe, 0xd4, 0xef, 0x00, 0x00,
		0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		t.Fatal(err)
	}
	seq, err := PlaceSequence(path, 7, 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seq, "\033_G") || !strings.Contains(seq, "i=7") {
		t.Fatalf("place seq: %q", seq)
	}
	clr := ClearSequence(7)
	if !strings.Contains(clr, "i=7") || !strings.Contains(clr, "a=d") {
		t.Fatalf("clear: %q", clr)
	}
}

func TestImageRef(t *testing.T) {
	p, ok := ImageRef("[image: /tmp/a.png]")
	if !ok || p != "/tmp/a.png" {
		t.Fatalf("got %q %v", p, ok)
	}
	p, ok = ImageRef("[imagen: /tmp/b.jpg]")
	if !ok || p != "/tmp/b.jpg" {
		t.Fatalf("es got %q %v", p, ok)
	}
	if _, ok := ImageRef("not an image"); ok {
		t.Fatal("false positive")
	}
}

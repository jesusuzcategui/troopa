package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallUninstallLinux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var buf bytes.Buffer
	if err := install("linux", &buf, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".config", "systemd", "user", "troopa.service")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "ExecStart=") || !strings.Contains(text, " daemon") {
		t.Fatalf("unit: %s", text)
	}
	if !strings.Contains(buf.String(), path) || !strings.Contains(buf.String(), "systemctl --user status troopa") {
		t.Fatalf("print: %s", buf.String())
	}
	buf.Reset()
	if err := uninstall("linux", &buf, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unit still there: %v", err)
	}
	if !strings.Contains(buf.String(), path) {
		t.Fatalf("removed print: %s", buf.String())
	}
}

func TestInstallUninstallDarwin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var buf bytes.Buffer
	if err := install("darwin", &buf, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "Library", "LaunchAgents", darwinPlist)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "<string>daemon</string>") || !strings.Contains(string(raw), darwinLabel) {
		t.Fatalf("plist: %s", raw)
	}
	if !strings.Contains(buf.String(), path) || !strings.Contains(buf.String(), "launchctl list") {
		t.Fatalf("print: %s", buf.String())
	}
	buf.Reset()
	if err := uninstall("darwin", &buf, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("plist still there: %v", err)
	}
}

func TestInstallWindows(t *testing.T) {
	var buf bytes.Buffer
	err := install("windows", &buf, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(buf.String(), "not supported") {
		t.Fatalf("%s", buf.String())
	}
}

package gitinit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no está en PATH")
	}
	dir := filepath.Join(t.TempDir(), "nuevo")
	if err := InitRepo(dir, "Demo"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Fatalf("readme: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("git dir: %v", err)
	}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	if string(out) != "main\n" {
		t.Fatalf("branch = %q", out)
	}
}

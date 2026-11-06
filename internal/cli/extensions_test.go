//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtensionsOnPath(t *testing.T) {
	dir := t.TempDir()
	for name, mode := range map[string]os.FileMode{"troopa-deploy": 0o755, "troopa-notes.txt": 0o644, "troopa-task": 0o755} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	if got := ListExtensions(); len(got) != 1 || got[0] != "deploy" {
		t.Fatalf("extensions %v", got)
	}
	if _, ok := FindExtension("deploy"); !ok {
		t.Fatal("deploy not found")
	}
	if _, ok := FindExtension("task"); ok {
		t.Fatal("a builtin must not be replaced")
	}
}

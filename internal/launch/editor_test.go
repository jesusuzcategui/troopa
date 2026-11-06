package launch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveEditorOrder(t *testing.T) {
	t.Setenv("VISUAL", "code")
	t.Setenv("EDITOR", "nano")
	if e, _ := ResolveEditor("nvim"); e != "nvim" {
		t.Fatalf("config should win, got %s", e)
	}
	if e, _ := ResolveEditor(""); e != "code" {
		t.Fatalf("VISUAL should win over EDITOR, got %s", e)
	}
	t.Setenv("VISUAL", "")
	if e, _ := ResolveEditor(""); e != "nano" {
		t.Fatalf("EDITOR fallback, got %s", e)
	}
}

func TestEditorCommandAppendsFolder(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "myeditor")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	wt := t.TempDir()
	cmd, err := EditorCommand("myeditor --new-window", wt)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Args) != 3 || cmd.Args[1] != "--new-window" || cmd.Args[2] != wt || cmd.Dir != wt {
		t.Fatalf("args %v dir %s", cmd.Args, cmd.Dir)
	}
	if _, err := EditorCommand("myeditor", filepath.Join(wt, "missing")); err == nil {
		t.Fatal("missing folder should fail")
	}
}

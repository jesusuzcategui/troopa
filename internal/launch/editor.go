package launch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ResolveEditor picks the editor command: config.toml "editor", then $VISUAL,
// then $EDITOR, then the first of nvim, vim, vi found in PATH.
func ResolveEditor(configured string) (string, error) {
	for _, e := range []string{configured, os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if e = strings.TrimSpace(e); e != "" {
			return e, nil
		}
	}
	for _, e := range []string{"nvim", "vim", "vi"} {
		if _, err := exec.LookPath(e); err == nil {
			return e, nil
		}
	}
	return "", fmt.Errorf("no editor found: set editor in config.toml (for example editor = \"nvim\")")
}

// EditorCommand opens dir in the editor. The editor string may carry
// arguments ("code --new-window"); the folder is appended last.
func EditorCommand(configured, dir string) (*exec.Cmd, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("folder not found: %s", dir)
	}
	editor, err := ResolveEditor(configured)
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(editor)
	bin, err := exec.LookPath(parts[0])
	if err != nil {
		return nil, fmt.Errorf("editor %s is not installed", parts[0])
	}
	cmd := exec.Command(bin, append(parts[1:], dir)...)
	cmd.Dir = dir
	cmd.Env = AgentEnv(dir)
	return cmd, nil
}

package launch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func IsTextMIME(mime, name string) bool {
	mime = strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	if strings.HasPrefix(mime, "text/") || strings.HasSuffix(mime, "+json") || strings.HasSuffix(mime, "+xml") {
		return true
	}
	switch mime {
	case "application/json", "application/xml", "application/javascript", "application/x-yaml", "application/toml", "inode/x-empty":
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".txt", ".go", ".py", ".js", ".ts", ".tsx", ".json", ".toml", ".yaml", ".yml", ".xml", ".html", ".css", ".sh", ".sql", ".rs", ".c", ".h", ".cpp":
		return true
	}
	return false
}

func OpenAttachment(configured, path, mime string) (*exec.Cmd, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	if st.IsDir() {
		return EditorCommand(configured, path)
	}
	if IsTextMIME(mime, filepath.Base(path)) {
		editor, err := ResolveEditor(configured)
		if err != nil {
			return OpenExternal(path)
		}
		parts := strings.Fields(editor)
		bin, err := exec.LookPath(parts[0])
		if err != nil {
			return OpenExternal(path)
		}
		cmd := exec.Command(bin, append(parts[1:], path)...)
		cmd.Dir = filepath.Dir(path)
		return cmd, nil
	}
	return OpenExternal(path)
}

func OpenExternal(path string) (*exec.Cmd, error) {
	var bin string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		bin = "open"
		args = []string{path}
	case "windows":
		bin = "cmd"
		args = []string{"/c", "start", "", path}
	default:
		bin = "xdg-open"
		args = []string{path}
	}
	found, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %s is not installed", path, bin)
	}
	cmd := exec.Command(found, args...)
	cmd.Dir = filepath.Dir(path)
	return cmd, nil
}

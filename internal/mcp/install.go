package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/i18n"
)

const (
	claudeSnippet = "claude mcp add troopa -- troopa mcp"

	codexSnippet = `[mcp_servers.troopa]
command = "troopa"
args = ["mcp"]
`

	opencodeSnippet = `{
  "mcp": {
    "troopa": {
      "type": "local",
      "command": ["troopa", "mcp"]
    }
  }
}
`

	cursorSnippet = `{
  "mcpServers": {
    "troopa": {
      "command": "troopa",
      "args": ["mcp"]
    }
  }
}
`
)

var lookPath = exec.LookPath

var runCmd = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%s: %s", err, msg)
	}
	return nil
}

// Install registers the Troopa MCP server with a client, or prints the
// snippet when that client is not on PATH.
func Install(w io.Writer, client string) error {
	switch strings.ToLower(strings.TrimSpace(client)) {
	case "claude":
		return installClaude(w)
	case "codex":
		return installCodex(w)
	case "opencode":
		return installOpenCode(w)
	case "cursor":
		return installCursor(w)
	default:
		return fmt.Errorf("%s", i18n.T("Unknown MCP client %s. Use claude, codex, opencode or cursor.", client))
	}
}

func installClaude(w io.Writer) error {
	if _, err := lookPath("claude"); err != nil {
		printMissing(w, "Claude Code", claudeSnippet)
		return nil
	}
	if err := runCmd("claude", "mcp", "add", "troopa", "--", "troopa", "mcp"); err != nil {
		fmt.Fprintln(w, i18n.T("Could not register with Claude Code: %s", err.Error()))
		printMissing(w, "Claude Code", claudeSnippet)
		return err
	}
	fmt.Fprintln(w, i18n.T("Registered Troopa MCP with Claude Code."))
	return nil
}

func installCodex(w io.Writer) error {
	if _, err := lookPath("codex"); err != nil {
		printMissing(w, "Codex", strings.TrimSpace(codexSnippet)+"\n# ~/.codex/config.toml")
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".codex", "config.toml")
	raw, err := readOptional(path)
	if err != nil {
		return err
	}
	text := upsertCodexTable(string(raw))
	if err := writeFile(path, []byte(text)); err != nil {
		return err
	}
	fmt.Fprintln(w, i18n.T("Wrote %s", path))
	return nil
}

func installOpenCode(w io.Writer) error {
	if _, err := lookPath("opencode"); err != nil {
		printMissing(w, "OpenCode", strings.TrimSpace(opencodeSnippet)+"\n# ~/.config/opencode/opencode.json")
		return nil
	}
	path, err := opencodeConfigPath()
	if err != nil {
		return err
	}
	raw, err := readOptional(path)
	if err != nil {
		return err
	}
	out, err := mergeJSON(raw, []string{"mcp", "troopa"}, map[string]any{
		"type":    "local",
		"command": []any{"troopa", "mcp"},
	})
	if err != nil {
		return err
	}
	if err := writeFile(path, out); err != nil {
		return err
	}
	fmt.Fprintln(w, i18n.T("Wrote %s", path))
	return nil
}

func installCursor(w io.Writer) error {
	if _, err := lookPath("cursor"); err != nil {
		printMissing(w, "Cursor", strings.TrimSpace(cursorSnippet)+"\n# ~/.cursor/mcp.json")
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".cursor", "mcp.json")
	raw, err := readOptional(path)
	if err != nil {
		return err
	}
	out, err := mergeJSON(raw, []string{"mcpServers", "troopa"}, map[string]any{
		"command": "troopa",
		"args":    []any{"mcp"},
	})
	if err != nil {
		return err
	}
	if err := writeFile(path, out); err != nil {
		return err
	}
	fmt.Fprintln(w, i18n.T("Wrote %s", path))
	return nil
}

func printMissing(w io.Writer, name, snippet string) {
	fmt.Fprintln(w, i18n.T("%s is not installed. Add this to the client:", name))
	fmt.Fprintln(w, snippet)
}

func opencodeConfigPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); v != "" {
		return filepath.Join(v, "opencode", "opencode.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
}

func readOptional(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return raw, nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func upsertCodexTable(text string) string {
	text = removeTOMLTables(text, "mcp_servers.troopa")
	text = strings.TrimRight(text, "\n")
	if text != "" {
		text += "\n\n"
	}
	return text + strings.TrimSpace(codexSnippet) + "\n"
}

func removeTOMLTables(text, name string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	skip := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			table := strings.TrimSpace(trim[1 : len(trim)-1])
			skip = table == name || strings.HasPrefix(table, name+".")
		}
		if skip {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func mergeJSON(raw []byte, path []string, value any) ([]byte, error) {
	root := map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, err
		}
	}
	cur := root
	for _, key := range path[:len(path)-1] {
		next, ok := cur[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[key] = next
		}
		cur = next
	}
	cur[path[len(path)-1]] = value
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

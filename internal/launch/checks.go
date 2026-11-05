package launch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type CheckResult struct {
	Cmd    string `json:"cmd"`
	OK     bool   `json:"ok"`
	Output string `json:"output"`
}

type projectTOML struct {
	Checks []string `toml:"checks"`
}

func LoadProjectChecks(repo string) []string {
	raw, err := os.ReadFile(filepath.Join(repo, "troopa.toml"))
	if err != nil {
		return nil
	}
	var t projectTOML
	if err := toml.Unmarshal(raw, &t); err != nil {
		return nil
	}
	return t.Checks
}

func RunChecks(worktree string, cmds []string) []CheckResult {
	var out []CheckResult
	for _, c := range cmds {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		cmd := exec.Command("sh", "-c", c)
		cmd.Dir = worktree
		raw, err := cmd.CombinedOutput()
		out = append(out, CheckResult{
			Cmd:    c,
			OK:     err == nil,
			Output: tail(string(raw), 30),
		})
	}
	return out
}

func ChecksOK(rs []CheckResult) bool {
	for _, r := range rs {
		if !r.OK {
			return false
		}
	}
	return true
}

func FormatChecks(rs []CheckResult) string {
	if len(rs) == 0 {
		return "Sin checks de proyecto (añade checks = [\"…\"] en troopa.toml)."
	}
	var b strings.Builder
	for _, r := range rs {
		mark := "✓"
		if !r.OK {
			mark = "✗"
		}
		fmt.Fprintf(&b, "%s %s\n", mark, r.Cmd)
		if !r.OK && r.Output != "" {
			b.WriteString(r.Output)
			if !strings.HasSuffix(r.Output, "\n") {
				b.WriteByte('\n')
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func Notify(title, body string) {
	if bin, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command(bin, title, body).Start()
		return
	}
	if bin, err := exec.LookPath("osascript"); err == nil {
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		_ = exec.Command(bin, "-e", script).Start()
	}
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

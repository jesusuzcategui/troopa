package launch

import (
	"context"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jesusuzcategui/troopa/internal/config"
)

// AgentInfo describes one agent CLI Troopa can launch and whether it is on PATH.
type AgentInfo struct {
	ID        string // the value stored on tasks: claude, codex, opencode, pi
	Name      string // what people call it: Claude Code, Codex…
	Command   string
	Path      string
	Version   string
	Installed bool
}

var agentNames = map[string]string{
	"claude":   "Claude Code",
	"codex":    "Codex",
	"opencode": "OpenCode",
	"cursor":   "Cursor",
	"pi":       "Pi",
	"me":       "me",
}

// AgentName is the display name of an agent id.
func AgentName(id string) string {
	if n, ok := agentNames[id]; ok {
		return n
	}
	return id
}

// versionTimeout keeps a slow or hanging CLI from blocking the setup screen.
var versionTimeout = 2 * time.Second

// DetectAgents checks every configured agent command on PATH and asks it for
// its version. Agents come back sorted with the installed ones first.
func DetectAgents(cfg config.Launch) []AgentInfo {
	ids := make([]string, 0, len(cfg.Agents))
	for id := range cfg.Agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]AgentInfo, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		spec := cfg.Agents[id]
		out[i] = AgentInfo{ID: id, Name: AgentName(id), Command: spec.Command}
		if p, err := exec.LookPath(spec.Command); err == nil {
			out[i].Path = p
			out[i].Installed = true
			wg.Add(1)
			go func(i int, p string) {
				defer wg.Done()
				out[i].Version = agentVersion(p)
			}(i, p)
		}
	}
	wg.Wait()
	sort.SliceStable(out, func(i, j int) bool { return out[i].Installed && !out[j].Installed })
	return out
}

func agentVersion(bin string) string {
	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()
	b, err := exec.CommandContext(ctx, bin, "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
	if len(line) > 40 {
		line = line[:40]
	}
	return line
}

// AgentInstalled reports whether the agent's command is on PATH. "me" always is.
func AgentInstalled(cfg config.Launch, id string) bool {
	if id == "" || id == "me" {
		return true
	}
	spec, ok := cfg.Agents[id]
	if !ok || spec.Command == "" {
		return false
	}
	_, err := exec.LookPath(spec.Command)
	return err == nil
}

// DetectTools returns the candidates found on PATH, in order.
func DetectTools(candidates ...string) []string {
	var found []string
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			found = append(found, c)
		}
	}
	return found
}

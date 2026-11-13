package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Extensions are programs named troopa-<name> on the PATH, like git
// subcommands: "troopa deploy" runs troopa-deploy with the remaining
// arguments. They can read Troopa's data through "troopa ... --json" and the
// TROOPA_* variables.

// Builtin lists the first words Troopa handles itself; an extension cannot
// replace them.
var Builtin = []string{"setup", "help", "version", "login", "notify", "extensions", "daemon", "mcp", "gui", "project", "client", "task", "run", "brain", "plan", "profile"}

func IsBuiltin(name string) bool {
	for _, b := range Builtin {
		if b == name {
			return true
		}
	}
	return strings.HasPrefix(name, "-")
}

// FindExtension returns the path of troopa-<name>, if it is on the PATH.
func FindExtension(name string) (string, bool) {
	if name == "" || IsBuiltin(name) || strings.ContainsAny(name, `/\`) {
		return "", false
	}
	p, err := exec.LookPath("troopa-" + name)
	return p, err == nil
}

// ListExtensions names every troopa-* program on the PATH.
func ListExtensions() []string {
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "troopa-") || e.IsDir() {
				continue
			}
			if runtime.GOOS == "windows" {
				name = strings.TrimSuffix(strings.TrimSuffix(name, ".exe"), ".bat")
			} else if info, err := e.Info(); err != nil || info.Mode()&0o111 == 0 {
				continue
			}
			ext := strings.TrimPrefix(name, "troopa-")
			if ext != "" && !IsBuiltin(ext) {
				seen[ext] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

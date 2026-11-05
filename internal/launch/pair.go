package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/store"
)

const pairMD = `# Pair session (TDD)

You are programming **with** the person in an interactive terminal, not in yolo mode.

1. Write a failing test (red) and **stop**.
2. Write the minimum code to make it pass (green) and **stop**.
3. Refactor **only if the person asks**.
4. If the task has commits=none: do not commit, push or create branches.

Read TASK.md for scope, criteria and rules. When you are done, the person presses **Finish pair** in Troopa (checks and review).
If this terminal is closed before finishing, the session moves to pair-paused and can be resumed.
`

func WritePAIR(dir string, t store.Task) error {
	body := pairMD
	if t.Commits == store.CommitsNone {
		body += "\nThis task asks for commits=none: leave the changes uncommitted.\n"
	}
	return os.WriteFile(filepath.Join(dir, "PAIR.md"), []byte(body), 0o644)
}

func PreparePair(cfg config.Launch, project store.Project, task store.Task, prev Session) (Session, error) {
	if sess, ok := reusePair(prev, cfg, project, task); ok {
		if err := WriteTASK(sess.Worktree, task); err != nil {
			return Session{}, err
		}
		if err := WritePAIR(sess.Worktree, task); err != nil {
			return Session{}, err
		}
		return sess, nil
	}
	sess, err := Prepare(cfg, project, task)
	if err != nil {
		return Session{}, err
	}
	if err := WritePAIR(sess.Worktree, task); err != nil {
		_ = Discard(sess)
		return Session{}, err
	}
	return sess, nil
}

func reusePair(prev Session, cfg config.Launch, project store.Project, task store.Task) (Session, bool) {
	dest := prev.Worktree
	if dest == "" {
		dest = WorktreePath(cfg, project, task)
	}
	st, err := os.Stat(dest)
	if err != nil || !st.IsDir() {
		return Session{}, false
	}
	if _, err := Git(dest, "rev-parse", "--is-inside-work-tree"); err != nil {
		return Session{}, false
	}
	sess := prev
	sess.Worktree = dest
	if sess.Branch == "" {
		if b, err := HeadBranch(dest); err == nil {
			sess.Branch = b
		}
	}
	if sess.Repo == "" {
		if r, err := RepoFromWorktree(dest); err == nil {
			sess.Repo = r
		}
	}
	if sess.Base == "" {
		sess.Base = "main"
	}
	if sess.LogPath == "" {
		sess.LogPath = filepath.Join(dest, "agent.log")
	}
	return sess, true
}

func RepoFromWorktree(worktree string) (string, error) {
	common, err := Git(worktree, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(worktree, common)
	}
	return filepath.Clean(filepath.Join(common, "..")), nil
}

func WritePairScript(dir string, spec config.AgentSpec, bin string, args []string) error {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	fmt.Fprintf(&b, "cd %s || exit 1\n", shQuote(dir))
	b.WriteString("echo $$ > pair.pid\n")
	for k, v := range spec.Env {
		fmt.Fprintf(&b, "export %s=%s\n", k, shQuote(v))
	}
	b.WriteString("printf '\\n▶ Troopa · pair\\n  Read TASK.md and PAIR.md\\n  Finish the session in Troopa (f) when you are done.\\n\\n'\n")
	b.WriteString("exec " + shQuote(bin))
	for _, a := range args {
		b.WriteString(" " + shQuote(a))
	}
	b.WriteString("\n")
	return os.WriteFile(filepath.Join(dir, "pair.sh"), []byte(b.String()), 0o700)
}

func PairScriptPath(dir string) string {
	return filepath.Join(dir, "pair.sh")
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func FinishPair(sess Session, repo string) (summary string, checksJSON string, ok bool) {
	summary = ReadSummary(sess.Worktree)
	if repo == "" {
		repo = sess.Repo
	}
	checks := RunChecks(sess.Worktree, LoadProjectChecks(repo))
	ok = ChecksOK(checks)
	raw, _ := json.Marshal(checks)
	return summary, string(raw), ok
}

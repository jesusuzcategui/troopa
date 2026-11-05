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

const pairMD = `# Sesión en pareja (TDD)

Estás programando **con** la persona en una terminal interactiva, no en modo yolo.

1. Escribe un test que falle (rojo) y **para**.
2. Escribe el código mínimo para que pase (verde) y **para**.
3. Refactoriza **solo si la persona lo pide**.
4. Si la tarea tiene commits=none: no hagas commits, push ni crees ramas.

Lee TASK.md para el alcance, criterios y reglas. Al terminar, la persona pulsa **Finish pair** en Troopa (checks y revisión).
Si cierras esta terminal sin terminar, la sesión queda en pair-paused y se puede retomar.
`

func WritePAIR(dir string, t store.Task) error {
	body := pairMD
	if t.Commits == store.CommitsNone {
		body += "\nEsta tarea pide commits=none: deja los cambios sin commitear.\n"
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
	b.WriteString("printf '\\n▶ Troopa · pareja\\n  Lee TASK.md y PAIR.md\\n  Termina la sesión en Troopa (f) cuando acabéis.\\n\\n'\n")
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

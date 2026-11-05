package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/store"
)

type Session struct {
	Repo     string
	Worktree string
	Branch   string
	Base     string
	LogPath  string
}

func Prepare(cfg config.Launch, project store.Project, task store.Task) (Session, error) {
	repo, err := RepoRoot(project.RepoPath)
	if err != nil {
		return Session{}, err
	}
	base, err := HeadBranch(repo)
	if err != nil {
		return Session{}, err
	}
	branch := UniqueBranch(repo, BranchName(cfg.BranchPrefix, task.Type, task.Title))
	if IsProtected(branch) {
		return Session{}, fmt.Errorf("la rama %s está protegida", branch)
	}
	dest := WorktreePath(cfg, project, task)
	if dest == "" {
		return Session{}, fmt.Errorf("worktrees sin configurar")
	}
	if err := AddWorktree(repo, dest, branch, base); err != nil {
		return Session{}, err
	}
	if err := ExcludeWorkFiles(dest); err != nil {
		_ = RemoveWorktree(repo, dest, branch)
		return Session{}, err
	}
	if err := WriteTASK(dest, task); err != nil {
		_ = RemoveWorktree(repo, dest, branch)
		return Session{}, err
	}
	logPath := filepath.Join(dest, "agent.log")
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		_ = RemoveWorktree(repo, dest, branch)
		return Session{}, err
	}
	return Session{Repo: repo, Worktree: dest, Branch: branch, Base: base, LogPath: logPath}, nil
}

func WorktreePath(cfg config.Launch, project store.Project, task store.Task) string {
	if strings.TrimSpace(cfg.Worktrees) == "" {
		return ""
	}
	return filepath.Join(cfg.Worktrees, project.Slug, fmt.Sprintf("%d-%s", task.ID, store.Slugify(task.Title)))
}

func Approve(sess Session, task store.Task, summary string) (string, error) {
	if task.Commits == store.CommitsNone {
		return "Aprobada sin commitear. Worktree: " + sess.Worktree, nil
	}
	if err := CommitAll(sess.Worktree, "troopa: "+task.Title); err != nil {
		return "", err
	}
	if !HasRemote(sess.Worktree) {
		return fmt.Sprintf("Aprobada. Rama local %s (sin remoto). Worktree quitado al cerrar no se hace: se conserva hasta descartar.", sess.Branch), nil
	}
	if err := PushBranch(sess.Worktree, sess.Branch); err != nil {
		return "", err
	}
	body := summary
	if body == "" {
		body = "Hecho con Troopa."
	}
	url, err := CreatePR(sess.Worktree, sess.Branch, sess.Base, task.Title, body)
	if err != nil {
		return "Rama " + sess.Branch + " subida. " + err.Error(), nil
	}
	return "PR abierto: " + url, nil
}

func Discard(sess Session) error {
	return RemoveWorktree(sess.Repo, sess.Worktree, sess.Branch)
}

package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
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
		return Session{}, fmt.Errorf("branch %s is protected", branch)
	}
	dest := WorktreePath(cfg, project, task)
	if dest == "" {
		return Session{}, fmt.Errorf("worktrees are not configured")
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
		return i18n.T("Approved without committing. Worktree: ") + sess.Worktree, nil
	}
	if err := CommitAll(sess.Worktree, "troopa: "+task.Title); err != nil {
		return "", err
	}
	if !HasRemote(sess.Worktree) {
		return i18n.T("Approved. Local branch %s (no remote). The worktree is kept until you discard it.", sess.Branch), nil
	}
	if err := PushBranch(sess.Worktree, sess.Branch); err != nil {
		return "", err
	}
	body := summary
	if body == "" {
		body = "Made with Troopa."
	}
	url, err := CreatePR(sess.Worktree, sess.Branch, sess.Base, task.Title, body)
	if err != nil {
		return i18n.T("Branch %s pushed. ", sess.Branch) + err.Error(), nil
	}
	return i18n.T("PR opened: ") + url, nil
}

func Discard(sess Session) error {
	return RemoveWorktree(sess.Repo, sess.Worktree, sess.Branch)
}

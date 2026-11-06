package brain

import (
	"os"
	"os/exec"
	"strings"
)

func Commit(s Settings, message string) error {
	_, err := commitResult(s, message)
	return err
}

func commitResult(s Settings, message string) (bool, error) {
	if !s.Git || strings.TrimSpace(s.Dir) == "" {
		return false, nil
	}
	if !isRepo(s.Dir) {
		return false, nil
	}
	if err := runGit(s.Dir, "add", "-A"); err != nil {
		return false, err
	}
	st, err := gitOut(s.Dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(st) == "" {
		return false, nil
	}
	if message == "" {
		message = "troopa: update brain"
	}
	if err := runGit(s.Dir, "commit", "-m", message); err != nil {
		return false, err
	}
	return true, nil
}

func IsRepo(dir string) bool {
	return isRepo(dir)
}

func Dirty(s Settings) bool {
	if !s.Git || strings.TrimSpace(s.Dir) == "" || !isRepo(s.Dir) {
		return false
	}
	st, err := gitOut(s.Dir, "status", "--porcelain")
	return err == nil && strings.TrimSpace(st) != ""
}

func isRepo(dir string) bool {
	_, err := gitOut(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

func runGit(dir string, args ...string) error {
	_, err := gitOut(dir, args...)
	return err
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Troopa",
		"GIT_AUTHOR_EMAIL=troopa@local",
		"GIT_COMMITTER_NAME=Troopa",
		"GIT_COMMITTER_EMAIL=troopa@local",
	)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

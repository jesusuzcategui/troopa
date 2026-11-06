package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func NormalizeMergeStrategy(strategy string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(strategy))
	switch s {
	case "", "squash":
		return "squash", nil
	case "merge":
		return "merge", nil
	default:
		return "", fmt.Errorf("unknown merge strategy %q (use squash or merge)", strategy)
	}
}

func MergeLocal(sess Session, task store.Task, strategy string) (string, error) {
	kind, err := NormalizeMergeStrategy(strategy)
	if err != nil {
		return "", err
	}
	if err := validateMerge(sess); err != nil {
		return "", err
	}
	before, err := Git(sess.Repo, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	var mergeErr error
	if kind == "merge" {
		msg := fmt.Sprintf("Merge %s: %s", sess.Branch, task.Title)
		_, mergeErr = Git(sess.Repo, "merge", "--no-ff", "-m", msg, sess.Branch)
	} else {
		_, mergeErr = Git(sess.Repo, "merge", "--squash", sess.Branch)
		if mergeErr == nil {
			args := []string{"commit", "-m", task.Title}
			if body := ReadSummary(sess.Worktree); body != "" {
				args = append(args, "-m", body)
			}
			_, mergeErr = Git(sess.Repo, args...)
		}
	}
	if mergeErr != nil {
		files := conflictedFiles(sess.Repo)
		abortMerge(sess.Repo)
		if len(files) > 0 {
			return "", fmt.Errorf("merge conflict in %s", strings.Join(files, ", "))
		}
		return "", mergeErr
	}
	if err := confirmMerged(sess, before); err != nil {
		abortMerge(sess.Repo)
		return "", err
	}
	// The merge is already on the base branch: from here on, cleanup problems
	// are reported but never turn the merge into an error, so the run is still
	// marked as merged and nobody merges the same branch twice.
	var pending []string
	if err := removeWorktreeKeepBranch(sess.Repo, sess.Worktree); err != nil {
		pending = append(pending, err.Error())
	} else if sess.Branch != "" && !IsProtected(sess.Branch) {
		if _, err := Git(sess.Repo, "branch", "-D", sess.Branch); err != nil {
			pending = append(pending, err.Error())
		}
	}
	sha, _ := Git(sess.Repo, "rev-parse", "--short", "HEAD")
	msg := i18n.T("Merged %s into %s (%s) as %s", sess.Branch, sess.Base, kind, sha)
	if len(pending) > 0 {
		msg += " · " + i18n.T("Cleanup pending: %s", strings.Join(pending, "; "))
	}
	return msg, nil
}

func BranchAheadOfBase(repo, base, branch string) (bool, error) {
	out, err := Git(repo, "rev-list", "--count", base+".."+branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "0", nil
}

func validateMerge(sess Session) error {
	if sess.Repo == "" {
		return fmt.Errorf("project repo is missing")
	}
	if sess.Branch == "" {
		return fmt.Errorf("task branch is missing")
	}
	if IsProtected(sess.Branch) {
		return fmt.Errorf("refusing to merge protected branch %s", sess.Branch)
	}
	cur, err := HeadBranch(sess.Repo)
	if err != nil {
		return err
	}
	if cur != sess.Base {
		return fmt.Errorf("project repo is on %s, not the base branch %s", cur, sess.Base)
	}
	if dirty := uncommittedExceptTroopa(sess.Repo); dirty != "" {
		return fmt.Errorf("project repo has uncommitted changes: %s", dirty)
	}
	if mergeInProgress(sess.Repo) {
		return fmt.Errorf("project repo already has a merge in progress")
	}
	if !BranchExists(sess.Repo, sess.Branch) {
		return fmt.Errorf("branch %s does not exist", sess.Branch)
	}
	ahead, err := BranchAheadOfBase(sess.Repo, sess.Base, sess.Branch)
	if err != nil {
		return err
	}
	if !ahead {
		return fmt.Errorf("branch %s has no commits that are not in %s", sess.Branch, sess.Base)
	}
	return nil
}

func confirmMerged(sess Session, before string) error {
	cur, err := HeadBranch(sess.Repo)
	if err != nil {
		return err
	}
	if cur != sess.Base {
		return fmt.Errorf("merge did not stay on base branch %s", sess.Base)
	}
	if mergeInProgress(sess.Repo) {
		return fmt.Errorf("merge did not complete")
	}
	after, err := Git(sess.Repo, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if after == before {
		return fmt.Errorf("merge produced no commit on %s", sess.Base)
	}
	return nil
}

func abortMerge(repo string) {
	_, _ = Git(repo, "merge", "--abort")
	_, _ = Git(repo, "reset", "--merge")
}

func mergeInProgress(repo string) bool {
	gitDir, err := Git(repo, "rev-parse", "--git-dir")
	if err != nil {
		return false
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repo, gitDir)
	}
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD"} {
		if _, err := os.Stat(filepath.Join(gitDir, name)); err == nil {
			return true
		}
	}
	return false
}

func conflictedFiles(repo string) []string {
	out, err := Git(repo, "diff", "--name-only", "--diff-filter=U")
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func uncommittedExceptTroopa(repo string) string {
	out, err := Git(repo, "status", "--porcelain")
	if err != nil {
		return err.Error()
	}
	var dirty []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		path := porcelainPath(line)
		if isTroopaExcluded(path) {
			continue
		}
		dirty = append(dirty, path)
	}
	return strings.Join(dirty, ", ")
}

func porcelainPath(line string) string {
	path := line
	if len(line) >= 3 {
		path = strings.TrimSpace(line[3:])
	}
	if i := strings.Index(path, " -> "); i >= 0 {
		path = path[i+4:]
	}
	return strings.Trim(path, `"`)
}

func isTroopaExcluded(p string) bool {
	p = strings.TrimPrefix(p, "./")
	switch filepath.Base(p) {
	case "TASK.md", "SUMMARY.md", "CHANGES.md", "PAIR.md", "pair.sh", "pair.pid", "agent.log":
		return true
	}
	return p == ".troopa" || strings.HasPrefix(p, ".troopa/") ||
		p == ".venv" || strings.HasPrefix(p, ".venv/") || strings.Contains(p, "/.venv/") ||
		p == "node_modules" || strings.HasPrefix(p, "node_modules/") || strings.Contains(p, "/node_modules/")
}

func removeWorktreeKeepBranch(repo, dest string) error {
	if dest == "" {
		return nil
	}
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		_, _ = Git(repo, "worktree", "prune")
		return nil
	}
	if _, err := Git(repo, "worktree", "remove", dest); err != nil {
		return fmt.Errorf("worktree %s kept: %w", dest, err)
	}
	_ = os.RemoveAll(dest)
	_, _ = Git(repo, "worktree", "prune")
	return nil
}

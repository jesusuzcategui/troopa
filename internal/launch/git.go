package launch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Troopa",
		"GIT_AUTHOR_EMAIL=troopa@local",
		"GIT_COMMITTER_NAME=Troopa",
		"GIT_COMMITTER_EMAIL=troopa@local",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func RepoRoot(path string) (string, error) {
	root, err := Git(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", path)
	}
	return root, nil
}

func HeadBranch(repo string) (string, error) {
	b, err := Git(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if b == "HEAD" {
		if _, err := Git(repo, "show-ref", "--verify", "--quiet", "refs/heads/main"); err == nil {
			return "main", nil
		}
		return "main", fmt.Errorf("HEAD detached")
	}
	return b, nil
}

func BranchExists(repo, branch string) bool {
	_, err := Git(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func UniqueBranch(repo, want string) string {
	if !BranchExists(repo, want) {
		return want
	}
	for i := 2; i < 100; i++ {
		cand := fmt.Sprintf("%s-%d", want, i)
		if !BranchExists(repo, cand) {
			return cand
		}
	}
	return fmt.Sprintf("%s-%d", want, os.Getpid())
}

func IsProtected(branch string) bool {
	b := strings.TrimPrefix(strings.TrimSpace(branch), "refs/heads/")
	return b == "main" || b == "master"
}

func AddWorktree(repo, dest, branch, startPoint string) error {
	if IsProtected(branch) {
		return fmt.Errorf("refusing to create a protected branch: %s", branch)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if startPoint == "" {
		startPoint = "HEAD"
	}
	_, err := Git(repo, "worktree", "add", "-b", branch, dest, startPoint)
	return err
}

func RemoveWorktree(repo, dest, branch string) error {
	if dest != "" {
		_, _ = Git(repo, "worktree", "remove", "--force", dest)
		_ = os.RemoveAll(dest)
		_, _ = Git(repo, "worktree", "prune")
	}
	if branch != "" && !IsProtected(branch) {
		_, _ = Git(repo, "branch", "-D", branch)
	}
	return nil
}

func ExcludeWorkFiles(worktree string) error {
	common, err := Git(worktree, "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(worktree, common)
	}
	exclude := filepath.Join(common, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return err
	}
	cur, _ := os.ReadFile(exclude)
	need := []string{"TASK.md", "SUMMARY.md", "CHANGES.md", "PAIR.md", "pair.sh", "pair.pid", "agent.log", ".troopa/", ".venv/", "node_modules/"}
	var add []string
	body := "\n" + string(cur) + "\n"
	for _, p := range need {
		if !strings.Contains(body, "\n"+p+"\n") {
			add = append(add, p)
		}
	}
	if len(add) == 0 {
		return nil
	}
	f, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n# Troopa\n%s\n", strings.Join(add, "\n"))
	return err
}

func HasRemote(repo string) bool {
	_, err := Git(repo, "remote", "get-url", "origin")
	return err == nil
}

func CommitAll(worktree, message string) error {
	if _, err := Git(worktree, "add", "-A"); err != nil {
		return err
	}
	if _, err := Git(worktree, "diff", "--cached", "--quiet"); err == nil {
		return nil
	}
	_, err := Git(worktree, "commit", "-m", message)
	return err
}

func PushBranch(worktree, branch string) error {
	if IsProtected(branch) {
		return fmt.Errorf("refusing to push to %s", branch)
	}
	_, err := Git(worktree, "push", "-u", "origin", "HEAD:refs/heads/"+branch)
	return err
}

func CreatePR(worktree, branch, base, title, body string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh is not installed")
	}
	if IsProtected(branch) {
		return "", fmt.Errorf("refusing to open a PR from %s", branch)
	}
	cmd := exec.Command("gh", "pr", "create", "--head", branch, "--base", base, "--title", title, "--body", body)
	cmd.Dir = worktree
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("gh pr create: %w (%s)", err, s)
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return s, nil
	}
	return fields[len(fields)-1], nil
}

// ChangedImages lists the image files a run added or changed in its worktree
// (committed on the branch or not), up to max, as absolute paths.
func ChangedImages(worktree, base string, max int) []string {
	if worktree == "" {
		return nil
	}
	var names []string
	if base != "" {
		if out, err := Git(worktree, "diff", "--name-only", "--diff-filter=AM", base); err == nil {
			names = append(names, strings.Split(out, "\n")...)
		}
	}
	if out, err := Git(worktree, "ls-files", "--others", "--exclude-standard"); err == nil {
		names = append(names, strings.Split(out, "\n")...)
	}
	seen := map[string]bool{}
	var res []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] || strings.HasPrefix(n, ".troopa/") {
			continue
		}
		seen[n] = true
		switch strings.ToLower(filepath.Ext(n)) {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		default:
			continue
		}
		p := filepath.Join(worktree, n)
		if _, err := os.Stat(p); err == nil {
			res = append(res, p)
			if len(res) == max {
				break
			}
		}
	}
	return res
}

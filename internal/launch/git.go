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
		return "", fmt.Errorf("no es un repositorio git: %s", path)
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
		return fmt.Errorf("no se crea una rama protegida: %s", branch)
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
	need := []string{"TASK.md", "RESUMEN.md", "PAIR.md", "pair.sh", "pair.pid", "agent.log", ".venv/", "node_modules/"}
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
		return fmt.Errorf("nunca se hace push a %s", branch)
	}
	_, err := Git(worktree, "push", "-u", "origin", "HEAD:refs/heads/"+branch)
	return err
}

func CreatePR(worktree, branch, base, title, body string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh no está instalado")
	}
	if IsProtected(branch) {
		return "", fmt.Errorf("nunca se abre un PR desde %s", branch)
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

package gitinit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func InitRepo(path, name string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("ruta vacía")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("crear carpeta: %w", err)
	}
	readme := filepath.Join(path, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		title := strings.TrimSpace(name)
		if title == "" {
			title = filepath.Base(path)
		}
		body := "# " + title + "\n"
		if err := os.WriteFile(readme, []byte(body), 0o644); err != nil {
			return fmt.Errorf("escribir README: %w", err)
		}
	}
	if err := runGit(path, "init", "-b", "main"); err != nil {
		return err
	}
	if err := runGit(path, "add", "README.md"); err != nil {
		return err
	}
	return runGit(path, "commit", "-m", "Initial commit")
}

func runGit(dir string, args ...string) error {
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
		return fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

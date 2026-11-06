package launch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var atRef = regexp.MustCompile(`@([^\s]+)`)

func ListRepoFiles(repo string) []string {
	if strings.TrimSpace(repo) == "" {
		return nil
	}
	out, err := Git(repo, "ls-files")
	if err != nil {
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

func FilterRepoFiles(files []string, query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []string
	for _, f := range files {
		if q == "" || strings.Contains(strings.ToLower(f), q) {
			out = append(out, f)
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func ReferencedPaths(texts ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, text := range texts {
		for _, m := range atRef.FindAllStringSubmatch(text, -1) {
			p := strings.TrimRight(m[1], ".,;:)")
			p = strings.TrimPrefix(p, "/")
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func WorktreeHas(worktree, rel string) bool {
	if rel == "" || strings.Contains(rel, "..") {
		return false
	}
	_, err := os.Stat(filepath.Join(worktree, filepath.FromSlash(rel)))
	return err == nil
}

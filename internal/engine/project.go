package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// IsGitURL reports whether s looks like a repository to clone.
func IsGitURL(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "git@") || strings.HasPrefix(s, "ssh://") || strings.HasSuffix(s, ".git")
}

// RepoName is the folder name a clone of url would get.
func RepoName(url string) string {
	url = strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(url), "/"), ".git")
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	return url
}

// CreateProject makes a project in parent/<slug>: it clones source when it is a
// git URL, or starts a new repository named after it. A folder that already
// holds a git repository is registered as it is.
func (e *Engine) CreateProject(source, parent string) (store.Project, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return store.Project{}, fmt.Errorf("%s", i18n.T("The project needs a name"))
	}
	name := source
	clone := IsGitURL(source)
	if clone {
		name = RepoName(source)
	}
	slug := store.Slugify(name)
	if slug == "" {
		return store.Project{}, fmt.Errorf("%s", i18n.T("The project needs a name"))
	}
	path := filepath.Join(parent, slug)
	isRepo := false
	if entries, err := os.ReadDir(path); err == nil {
		if _, err := launch.RepoRoot(path); err == nil {
			isRepo = true
		} else if len(entries) > 0 {
			return store.Project{}, fmt.Errorf("%s", i18n.T("%s already exists and is not a git repository", path))
		}
	}
	switch {
	case isRepo:
	case clone:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		out, err := exec.CommandContext(ctx, "git", "clone", source, path).CombinedOutput()
		if err != nil {
			return store.Project{}, fmt.Errorf("git clone: %s", strings.TrimSpace(string(out)))
		}
	default:
		if err := gitinit.InitRepo(path, name); err != nil {
			return store.Project{}, err
		}
	}
	p, err := e.Store.Create(store.Project{Name: name, Slug: slug, RepoPath: path})
	if err != nil {
		return store.Project{}, err
	}
	if all, err := e.Store.List(true); err == nil {
		_ = brain.OnProject(brain.FromConfig(e.Cfg), all, p)
	}
	return p, nil
}

// ProjectParents are the folders the existing projects live in, most used
// first: the natural places for a new one.
func (e *Engine) ProjectParents(max int) []string {
	count := map[string]int{}
	var order []string
	if list, err := e.Store.List(true); err == nil {
		for _, p := range list {
			if p.RepoPath == "" {
				continue
			}
			dir := filepath.Dir(p.RepoPath)
			if count[dir] == 0 {
				order = append(order, dir)
			}
			count[dir]++
		}
	}
	// Stable sort by use, keeping first-seen order on ties.
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && count[order[j]] > count[order[j-1]]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	if len(order) == 0 {
		if home, err := os.UserHomeDir(); err == nil {
			order = append(order, home)
		}
	}
	if len(order) > max {
		order = order[:max]
	}
	return order
}

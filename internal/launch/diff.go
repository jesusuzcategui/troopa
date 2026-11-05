package launch

import (
	"os"
	"path/filepath"
	"strings"
)

type DiffLine struct {
	Kind byte // ' ', '+', '-', '@', 'h'
	Text string
}

type FileDiff struct {
	Path  string
	Lines []DiffLine
}

func WorktreeDiff(worktree, base string) ([]FileDiff, error) {
	if base == "" {
		base = "HEAD"
	}
	raw, err := Git(worktree, "diff", "--no-color", "--no-ext-diff", base)
	if err != nil {
		raw, err = Git(worktree, "diff", "--no-color", "--no-ext-diff")
		if err != nil {
			return nil, err
		}
	}
	files := ParseDiff(raw)
	untracked, _ := Git(worktree, "ls-files", "--others", "--exclude-standard")
	for _, p := range strings.Split(untracked, "\n") {
		p = strings.TrimSpace(p)
		if p == "" || p == "TASK.md" || p == "SUMMARY.md" || p == "agent.log" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(worktree, p))
		if err != nil {
			continue
		}
		fd := FileDiff{Path: p}
		for _, line := range strings.Split(string(body), "\n") {
			fd.Lines = append(fd.Lines, DiffLine{Kind: '+', Text: "+" + line})
		}
		files = append(files, fd)
	}
	return files, nil
}

func ParseDiff(raw string) []FileDiff {
	var files []FileDiff
	var cur *FileDiff
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			if cur != nil {
				files = append(files, *cur)
			}
			path := strings.TrimPrefix(line, "diff --git ")
			parts := strings.SplitN(path, " ", 2)
			name := path
			if len(parts) == 2 {
				name = strings.TrimPrefix(parts[1], "b/")
			}
			cur = &FileDiff{Path: name, Lines: []DiffLine{{Kind: 'h', Text: line}}}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "@@"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: '@', Text: line})
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: '+', Text: line})
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: '-', Text: line})
		default:
			cur.Lines = append(cur.Lines, DiffLine{Kind: ' ', Text: line})
		}
	}
	if cur != nil {
		files = append(files, *cur)
	}
	return files
}

package launch

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type DiffLine struct {
	Kind byte // ' ', '+', '-', '@', 'h'
	Text string
	Old  int // old-file line; 0 if this row has none
	New  int // new-file line; 0 if this row has none
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
		if p == "" || p == "TASK.md" || p == "SUMMARY.md" || p == "CHANGES.md" || p == "agent.log" ||
			p == ".troopa" || strings.HasPrefix(p, ".troopa/") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(worktree, p))
		if err != nil {
			continue
		}
		fd := FileDiff{Path: p}
		n := 1
		for _, line := range strings.Split(string(body), "\n") {
			fd.Lines = append(fd.Lines, DiffLine{Kind: '+', Text: "+" + line, New: n})
			n++
		}
		files = append(files, fd)
	}
	return files, nil
}

func ParseDiff(raw string) []FileDiff {
	var files []FileDiff
	var cur *FileDiff
	oldN, newN := 0, 0
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
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
			oldN, newN = 0, 0
		case cur == nil:
			continue
		case strings.HasPrefix(line, "@@"):
			oldN, newN, _ = hunkStarts(line)
			cur.Lines = append(cur.Lines, DiffLine{Kind: '@', Text: line})
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: '+', Text: line, New: newN})
			newN++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: '-', Text: line, Old: oldN})
			oldN++
		default:
			dl := DiffLine{Kind: ' ', Text: line}
			if strings.HasPrefix(line, " ") {
				dl.Old, dl.New = oldN, newN
				oldN++
				newN++
			}
			cur.Lines = append(cur.Lines, dl)
		}
	}
	if cur != nil {
		files = append(files, *cur)
	}
	return files
}

// CommentTarget is the new-file line (0 on a deleted line or a header) and the
// side of the diff the row belongs to.
func (d DiffLine) CommentTarget() (line int, side string) {
	switch d.Kind {
	case '+':
		return d.New, "+"
	case '-':
		return d.Old, "-"
	case ' ':
		if strings.HasPrefix(d.Text, " ") {
			return d.New, " "
		}
		return 0, " "
	default:
		return 0, " "
	}
}

func (d DiffLine) Quote() string {
	if d.Text == "" {
		return ""
	}
	switch d.Kind {
	case '+', '-', ' ':
		if d.Text[0] == '+' || d.Text[0] == '-' || d.Text[0] == ' ' {
			return d.Text[1:]
		}
	}
	return d.Text
}

func hunkStarts(header string) (oldN, newN int, ok bool) {
	minus := strings.Index(header, "-")
	plus := strings.Index(header, "+")
	if minus < 0 || plus < 0 || plus < minus {
		return 0, 0, false
	}
	oldN, _ = strconv.Atoi(leadingDigits(header[minus+1:]))
	newN, _ = strconv.Atoi(leadingDigits(header[plus+1:]))
	return oldN, newN, true
}

func leadingDigits(s string) string {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return s[:n]
}

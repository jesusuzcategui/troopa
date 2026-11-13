package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jesusuzcategui/troopa/internal/store"
)

// CHANGES.md keeps every round of feedback the person gave after a review.
// It lives next to TASK.md and is never committed.
const changesFile = "CHANGES.md"

// ChangeRounds counts the rounds of feedback already in CHANGES.md.
func ChangeRounds(worktree string) int {
	raw, err := os.ReadFile(filepath.Join(worktree, changesFile))
	if err != nil {
		return 0
	}
	return strings.Count(string(raw), "\n## Round ")
}

// RequestChanges appends a round of feedback to CHANGES.md and returns its
// number. The worktree keeps everything the agent already did.
func RequestChanges(worktree, feedback string) (int, error) {
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		return 0, fmt.Errorf("write what should change")
	}
	if _, err := os.Stat(worktree); err != nil {
		return 0, fmt.Errorf("worktree is missing: %s", worktree)
	}
	if err := ExcludeWorkFiles(worktree); err != nil {
		return 0, err
	}
	path := filepath.Join(worktree, changesFile)
	round := ChangeRounds(worktree) + 1
	var b strings.Builder
	if round == 1 {
		b.WriteString("# Changes requested\n\nThe person reviewed your work and asked for these changes. The newest round is last.\n")
	}
	fmt.Fprintf(&b, "\n## Round %d · %s\n\n%s\n", round, time.Now().Format("2006-01-02 15:04"), feedback)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := f.WriteString(b.String()); err != nil {
		return 0, err
	}
	return round, nil
}

// FormatCommentFeedback builds the CHANGES.md body: optional general notes,
// then each comment as path:line, the quoted line, and the comment.
func FormatCommentFeedback(general string, comments []store.ReviewComment) string {
	var b strings.Builder
	general = strings.TrimSpace(general)
	if general != "" {
		b.WriteString(general)
		b.WriteString("\n")
	}
	for _, c := range comments {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		if c.Path != "" {
			fmt.Fprintf(&b, "%s:%d\n", c.Path, c.Line)
		}
		if q := strings.TrimSpace(c.Quote); q != "" {
			fmt.Fprintf(&b, "> %s\n", q)
		}
		b.WriteString(strings.TrimSpace(c.Body))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// AppendFeedbackImages adds worktree image paths to the CHANGES.md body so
// the agent knows where to read them.
func AppendFeedbackImages(body string, worktreePaths []string) string {
	body = strings.TrimSpace(body)
	if len(worktreePaths) == 0 {
		return body
	}
	var b strings.Builder
	if body != "" {
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	b.WriteString("Attached images (in .troopa/attachments):\n")
	for _, p := range worktreePaths {
		if p = strings.TrimSpace(p); p != "" {
			fmt.Fprintf(&b, "- %s\n", p)
		}
	}
	return strings.TrimSpace(b.String())
}

// ChangesPrompt asks the agent to apply the newest round on top of its work.
func ChangesPrompt(worktree string, round int) string {
	return fmt.Sprintf("Work only inside %s. Read %s for the task, then %s: the person reviewed your work and asked for changes (round %d is the newest). "+
		"Keep what is already done, apply the changes, and rewrite %s, including the \"How to run\" section.",
		worktree, filepath.Join(worktree, "TASK.md"), filepath.Join(worktree, changesFile), round, filepath.Join(worktree, "SUMMARY.md"))
}

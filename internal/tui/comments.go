package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func (m *Model) loadComments() {
	if m.run.ID == 0 || m.store == nil {
		m.comments = nil
		return
	}
	items, err := m.store.ListComments(m.run.ID)
	if err != nil {
		m.comments = nil
		return
	}
	m.comments = items
}

func (m Model) pendingComments() []store.ReviewComment {
	var out []store.ReviewComment
	for _, c := range m.comments {
		if !c.Resolved {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) pendingCommentCount() int {
	return len(m.pendingComments())
}

func (m Model) currentReviewRound() int {
	if n := len(m.rounds); n > 0 {
		return m.rounds[n-1].Number
	}
	return 0
}

func (m Model) commentsOnLine(path string, line launch.DiffLine) []store.ReviewComment {
	num, side := line.CommentTarget()
	var out []store.ReviewComment
	for _, c := range m.comments {
		if c.Path == path && c.Line == num && c.Side == side && !c.Resolved {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) commentsForRound(round int) []store.ReviewComment {
	var out []store.ReviewComment
	for _, c := range m.comments {
		if c.Resolved && c.Round == round {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) reviewFile() (launch.FileDiff, bool) {
	if m.diffIndex < 0 || m.diffIndex >= len(m.diffFiles) {
		return launch.FileDiff{}, false
	}
	return m.diffFiles[m.diffIndex], true
}

func (m *Model) moveDiffCursor(delta int) {
	f, ok := m.reviewFile()
	if !ok || len(f.Lines) == 0 {
		return
	}
	m.diffCursor += delta
	if m.diffCursor < 0 {
		m.diffCursor = 0
	}
	if m.diffCursor >= len(f.Lines) {
		m.diffCursor = len(f.Lines) - 1
	}
	if m.diffCursor < m.diffOffset {
		m.diffOffset = m.diffCursor
	}
	m.ensureDiffCursorVisible(max(4, m.height-12))
}

func (m Model) diffLineHeight(path string, line launch.DiffLine) int {
	return 1 + len(m.commentsOnLine(path, line))
}

func (m *Model) ensureDiffCursorVisible(viewH int) {
	f, ok := m.reviewFile()
	if !ok || viewH <= 0 {
		return
	}
	if m.diffCursor < m.diffOffset {
		m.diffOffset = m.diffCursor
		return
	}
	for {
		h := 0
		for i := m.diffOffset; i <= m.diffCursor && i < len(f.Lines); i++ {
			h += m.diffLineHeight(f.Path, f.Lines[i])
		}
		if h <= viewH || m.diffOffset >= m.diffCursor {
			return
		}
		m.diffOffset++
	}
}

func (m Model) openCommentOnCursor() (tea.Model, tea.Cmd) {
	f, ok := m.reviewFile()
	if !ok || m.diffCursor < 0 || m.diffCursor >= len(f.Lines) {
		m.status = i18n.T("No diff line to comment")
		return m, nil
	}
	return m.openCommentEditor(0, "")
}

func (m Model) openCommentEditor(id int64, body string) (tea.Model, tea.Cmd) {
	in := textarea.New()
	in.Placeholder = i18n.T("Comment on this line")
	in.SetWidth(max(30, min(80, m.width-6)))
	in.SetHeight(6)
	in.ShowLineNumbers = false
	in.Prompt = ""
	in.SetValue(body)
	in.Focus()
	m.commentInput = in
	m.commentEditID = id
	m.screen = screenComment
	m.err = ""
	m.status = i18n.T("ctrl+s save · esc cancel")
	return m, textarea.Blink
}

func (m Model) editCommentOnCursor() (tea.Model, tea.Cmd) {
	c, ok := m.commentUnderCursor()
	if !ok {
		m.status = i18n.T("No comment on this line")
		return m, nil
	}
	return m.openCommentEditor(c.ID, c.Body)
}

func (m Model) confirmDeleteCommentOnCursor() (tea.Model, tea.Cmd) {
	c, ok := m.commentUnderCursor()
	if !ok {
		m.status = i18n.T("No comment on this line")
		return m, nil
	}
	m.commentDeleteID = c.ID
	m.popup = &popup{
		kind:    propConfirmDeleteComment,
		title:   i18n.T("Delete this comment?"),
		options: []string{"yes", "no"},
		labels:  []string{i18n.T("Yes, delete"), i18n.T("No")},
		cursor:  1,
	}
	return m, nil
}

func (m Model) commentUnderCursor() (store.ReviewComment, bool) {
	f, ok := m.reviewFile()
	if !ok || m.diffCursor < 0 || m.diffCursor >= len(f.Lines) {
		return store.ReviewComment{}, false
	}
	items := m.commentsOnLine(f.Path, f.Lines[m.diffCursor])
	if len(items) == 0 {
		return store.ReviewComment{}, false
	}
	return items[len(items)-1], true
}

func (m Model) updateCommentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.screen = screenReview
		m.status = m.reviewBar()
		return m, nil
	case "ctrl+s":
		return m.saveComment(m.commentInput.Value())
	}
	var cmd tea.Cmd
	m.commentInput, cmd = m.commentInput.Update(msg)
	return m, cmd
}

func (m Model) saveComment(body string) (tea.Model, tea.Cmd) {
	body = strings.TrimSpace(body)
	if body == "" {
		m.err = i18n.T("Write a comment")
		return m, nil
	}
	if m.commentEditID != 0 {
		c, err := m.store.GetComment(m.commentEditID)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		c.Body = body
		if err := m.store.UpdateComment(c); err != nil {
			m.err = err.Error()
			return m, nil
		}
	} else {
		f, ok := m.reviewFile()
		if !ok || m.diffCursor < 0 || m.diffCursor >= len(f.Lines) {
			m.err = i18n.T("No diff line to comment")
			return m, nil
		}
		line := f.Lines[m.diffCursor]
		num, side := line.CommentTarget()
		if _, err := m.store.CreateComment(store.ReviewComment{
			RunID: m.run.ID,
			Round: m.currentReviewRound(),
			Path:  f.Path,
			Line:  num,
			Side:  side,
			Quote: line.Quote(),
			Body:  body,
		}); err != nil {
			m.err = err.Error()
			return m, nil
		}
	}
	m.loadComments()
	m.screen = screenReview
	m.err = ""
	m.status = m.reviewBar()
	return m, nil
}

func (m Model) deleteComment(id int64) (tea.Model, tea.Cmd) {
	m.commentDeleteID = 0
	if id == 0 {
		return m, nil
	}
	if err := m.store.DeleteComment(id); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.loadComments()
	m.err = ""
	m.status = i18n.T("Comment deleted")
	return m, nil
}

func (m Model) viewComment() string {
	th := m.theme
	var b strings.Builder
	title := i18n.T("Comment")
	if m.commentEditID != 0 {
		title = i18n.T("Edit comment")
	}
	b.WriteString(th.Title.Render(title))
	if f, ok := m.reviewFile(); ok && m.diffCursor >= 0 && m.diffCursor < len(f.Lines) {
		line := f.Lines[m.diffCursor]
		num, _ := line.CommentTarget()
		b.WriteString(th.Muted.Render(fmt.Sprintf("  ·  %s:%d", f.Path, num)))
		b.WriteString("\n\n")
		b.WriteString(th.Muted.Render(truncate(line.Text, max(8, m.width-2))) + "\n\n")
	} else {
		b.WriteString("\n\n")
	}
	b.WriteString(m.commentInput.View())
	b.WriteString("\n\n")
	b.WriteString(th.Muted.Render(i18n.T("ctrl+s save · esc cancel")))
	return th.App.Render(b.String())
}

func (m Model) openCommentList() (tea.Model, tea.Cmd) {
	pending := m.pendingComments()
	if len(pending) == 0 {
		m.status = i18n.T("No pending comments")
		return m, nil
	}
	m.commentListCur = min(m.commentListCur, len(pending)-1)
	m.screen = screenCommentList
	m.status = i18n.T("enter jump to line · esc review")
	return m, nil
}

func (m Model) updateCommentListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.pendingComments()
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = screenReview
		m.status = m.reviewBar()
		return m, nil
	case "j", "down":
		if m.commentListCur < len(items)-1 {
			m.commentListCur++
		}
	case "k", "up":
		if m.commentListCur > 0 {
			m.commentListCur--
		}
	case "enter":
		if m.commentListCur >= 0 && m.commentListCur < len(items) {
			return m.jumpToComment(items[m.commentListCur])
		}
	}
	return m, nil
}

func (m Model) jumpToComment(c store.ReviewComment) (tea.Model, tea.Cmd) {
	for i, f := range m.diffFiles {
		if f.Path != c.Path {
			continue
		}
		m.diffIndex = i
		m.diffCursor = 0
		for j, line := range f.Lines {
			num, side := line.CommentTarget()
			if num == c.Line && side == c.Side {
				m.diffCursor = j
				break
			}
		}
		m.diffOffset = m.diffCursor
		break
	}
	m.screen = screenReview
	m.status = m.reviewBar()
	return m, nil
}

func (m Model) viewCommentList() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Pending comments")))
	b.WriteString(th.Muted.Render(fmt.Sprintf("  ·  %d", m.pendingCommentCount())))
	b.WriteString("\n\n")
	items := m.pendingComments()
	prev := ""
	for i, c := range items {
		if c.Path != prev {
			b.WriteString(th.Title.Render(c.Path) + "\n")
			prev = c.Path
		}
		line := fmt.Sprintf("  %d  %s", c.Line, firstLine(c.Body))
		if i == m.commentListCur {
			b.WriteString(th.Selected.UnsetPadding().Width(max(20, m.width-2)).Render(line))
		} else {
			b.WriteString(th.Normal.UnsetPadding().Render(line))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("enter jump to line · esc review")))
	return th.App.Render(b.String())
}

func (m Model) updateCommentListMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	items := m.pendingComments()
	y := msg.Y - 3
	row := 0
	prev := ""
	for i, c := range items {
		if c.Path != prev {
			if y == row {
				return m, nil
			}
			row++
			prev = c.Path
		}
		if y == row {
			m.commentListCur = i
			return m.jumpToComment(c)
		}
		row++
	}
	return m, nil
}

func (m Model) reviewBar() string {
	n := m.pendingCommentCount()
	base := reviewHelp(m.run.State)
	if n == 1 {
		return i18n.T("1 pending comment") + "  ·  " + base
	}
	if n > 1 {
		return i18n.T("%d pending comments", n) + "  ·  " + base
	}
	return base
}

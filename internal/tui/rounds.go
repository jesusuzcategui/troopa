package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

type roundHit struct {
	Y0, Y1 int
	Index  int
}

func (m *Model) loadRounds() {
	if m.run.ID == 0 || m.store == nil {
		m.rounds = nil
		return
	}
	prevLast := 0
	if n := len(m.rounds); n > 0 {
		prevLast = m.rounds[n-1].Number
	}
	items, err := m.store.RoundsForRun(m.run)
	if err != nil {
		m.rounds = nil
		return
	}
	m.rounds = items
	if m.roundOpen == nil {
		m.roundOpen = map[int]bool{}
	}
	if n := len(items); n > 0 {
		last := items[n-1].Number
		if last != prevLast {
			m.roundOpen[last] = true
			m.roundIndex = n - 1
		}
		if m.roundIndex < 0 || m.roundIndex >= n {
			m.roundIndex = n - 1
		}
	}
}

func (m *Model) beginRound(kind, feedback string) error {
	r, err := m.store.CreateRound(store.Round{
		RunID:    m.run.ID,
		Kind:     kind,
		Feedback: feedback,
		State:    store.RoundRunning,
		LogFrom:  logSize(m.run.LogPath),
	})
	if err != nil {
		return err
	}
	m.rounds = append(m.rounds, r)
	m.roundIndex = len(m.rounds) - 1
	if m.roundOpen == nil {
		m.roundOpen = map[int]bool{}
	}
	m.roundOpen[r.Number] = true
	return nil
}

func (m *Model) ensureRound(kind, feedback string) error {
	if _, err := m.store.OpenRound(m.run.ID); err == nil {
		return nil
	}
	return m.beginRound(kind, feedback)
}

func (m *Model) closeOpenRound(state, summary, checksJSON, logPath string, price config.TokenPrice) {
	r, err := m.store.OpenRound(m.run.ID)
	if err != nil {
		return
	}
	r.State = state
	r.Summary = summary
	r.Checks = checksJSON
	r.LogTo = sql.NullInt64{Int64: logSize(logPath), Valid: true}
	r.FinishedAt = time.Now().UTC()
	applyRoundMetrics(&r, logPath, price)
	_ = m.store.UpdateRound(r)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

func roundKindLabel(kind string) string {
	switch kind {
	case store.RoundChanges:
		return i18n.T("Changes")
	default:
		return i18n.T("Launch")
	}
}

func roundHeader(r store.Round, open bool, inner int, extra string) string {
	mark := "▸"
	if open {
		mark = "▾"
	}
	dur := ""
	if r.DurationMS.Valid {
		dur = " · " + fmtDuration(r.DurationMS.Int64)
	}
	line := fmt.Sprintf("%s %d %s · %s%s", mark, r.Number, roundKindLabel(r.Kind), r.State, dur)
	if !open && extra != "" {
		line += " · " + extra
	}
	return truncate(line, max(8, inner))
}

func (m Model) roundFoldExtra(r store.Round) string {
	if fb := firstLine(r.Feedback); fb != "" {
		return fb
	}
	if notes := m.commentsForRound(r.Number); len(notes) > 0 {
		return fmt.Sprintf("%s:%d %s", notes[0].Path, notes[0].Line, firstLine(notes[0].Body))
	}
	return ""
}

func (m Model) selectedRound() (store.Round, bool) {
	if m.roundIndex < 0 || m.roundIndex >= len(m.rounds) {
		return store.Round{}, false
	}
	return m.rounds[m.roundIndex], true
}

func (m *Model) moveRound(delta int) {
	if len(m.rounds) == 0 {
		return
	}
	n := len(m.rounds)
	m.roundIndex = (m.roundIndex + delta + n) % n
}

func (m *Model) toggleSelectedRound() {
	r, ok := m.selectedRound()
	if !ok {
		return
	}
	if m.roundOpen == nil {
		m.roundOpen = map[int]bool{}
	}
	m.roundOpen[r.Number] = !m.roundOpen[r.Number]
}

func (m Model) roundIsOpen(r store.Round) bool {
	return m.roundOpen[r.Number]
}

func roundLogSlice(raw []byte, r store.Round) []byte {
	from := r.LogFrom
	to := int64(len(raw))
	if r.LogTo.Valid {
		to = r.LogTo.Int64
	}
	if from < 0 {
		from = 0
	}
	if from > int64(len(raw)) {
		from = int64(len(raw))
	}
	if to > int64(len(raw)) {
		to = int64(len(raw))
	}
	if to < from {
		to = from
	}
	return raw[from:to]
}

func lineCount(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func lipglossHeight(s string) int {
	n := lineCount(s)
	if n == 0 {
		return 1
	}
	return n
}

func (m Model) renderRounds(inner int, forceLastOpen bool) (string, []roundHit) {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Muted.Render(i18n.T("Rounds")) + "\n")
	hits := make([]roundHit, 0, len(m.rounds))
	last := len(m.rounds) - 1
	for i, r := range m.rounds {
		open := m.roundIsOpen(r)
		if forceLastOpen {
			open = i == last
		}
		y0 := lineCount(b.String())
		extra := ""
		if !open {
			extra = m.roundFoldExtra(r)
		}
		head := roundHeader(r, open, inner, extra)
		if i == m.roundIndex && !forceLastOpen {
			b.WriteString(th.Selected.UnsetPadding().Width(inner).Render(head))
		} else {
			b.WriteString(th.Muted.Render(head))
		}
		b.WriteString("\n")
		if open {
			if fb := strings.TrimSpace(r.Feedback); fb != "" {
				b.WriteString(th.Muted.Render(i18n.T("Feedback")) + "\n")
				b.WriteString(th.Normal.Width(inner).Render(fb) + "\n")
			}
			for _, c := range m.commentsForRound(r.Number) {
				loc := fmt.Sprintf("%s:%d", c.Path, c.Line)
				b.WriteString(th.Muted.Render(loc) + "\n")
				if q := strings.TrimSpace(c.Quote); q != "" {
					b.WriteString(th.Muted.Render("> "+q) + "\n")
				}
				b.WriteString(th.Normal.Width(inner).Render(c.Body) + "\n")
			}
			if strings.TrimSpace(r.Summary) != "" {
				b.WriteString(th.Muted.Render(i18n.T("Summary")) + "\n")
				b.WriteString(m.renderMarkdownW(r.Summary, inner) + "\n")
			}
			b.WriteString(th.Muted.Render("Checks") + "\n")
			b.WriteString(launch.FormatChecks(decodeChecks(r.Checks)) + "\n")
		}
		hits = append(hits, roundHit{Y0: y0, Y1: lineCount(b.String()), Index: i})
	}
	return strings.TrimRight(b.String(), "\n"), hits
}

func (m Model) updateReviewMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.popup != nil {
		return m.updatePopupMouse(msg)
	}
	wheel := 0
	if msg.Button == tea.MouseButtonWheelDown {
		wheel = 1
	} else if msg.Button == tea.MouseButtonWheelUp {
		wheel = -1
	}
	if wheel != 0 {
		switch m.runTab {
		case tabSummary:
			m.scrollSummary(wheel * 3)
		case tabLog:
			m.scrollLog(wheel * 3)
		default:
			m.moveDiffCursor(wheel)
		}
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if msg.Y == 1 {
		// Tab bar: " 1 Summary   2 Changes   3 Log ".
		x := 0
		for t := 0; t < runTabCount; t++ {
			w := len([]rune(fmt.Sprintf(" %d %s ", t+1, runTabLabel(t)))) + 1
			if t == tabDiff && len(m.diffFiles) > 0 {
				w += len(fmt.Sprintf(" (%d)", len(m.diffFiles)))
			}
			if msg.X < x+w {
				m.setRunTab(t)
				return m, nil
			}
			x += w
		}
		return m, nil
	}
	const top = 3 // title, tab bar, blank line
	switch m.runTab {
	case tabSummary:
		inner := max(20, m.width-2)
		_, hits := m.renderRounds(inner, false)
		y := msg.Y - top + m.sumOffset
		for _, h := range hits {
			if y == h.Y0 {
				m.roundIndex = h.Index
				m.toggleSelectedRound()
				return m, nil
			}
		}
		return m, nil
	case tabLog:
		return m, nil
	}
	if len(m.diffFiles) == 0 {
		return m, nil
	}
	f := m.diffFiles[m.diffIndex]
	y := msg.Y
	lineY := top + 1 // skip the path header
	start := m.diffOffset
	if start > len(f.Lines) {
		start = max(0, len(f.Lines)-1)
	}
	h := max(4, m.height-(top+1)-3)
	shown := 0
	for i := start; i < len(f.Lines); i++ {
		need := m.diffLineHeight(f.Path, f.Lines[i])
		if shown > 0 && shown+need > h {
			break
		}
		y0 := lineY
		y1 := lineY + need
		if y >= y0 && y < y1 {
			m.diffCursor = i
			return m.openCommentOnCursor()
		}
		lineY = y1
		shown += need
	}
	return m, nil
}

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

const brainSplitMin = 100

func (m Model) brainDir() string {
	return m.brainSettings().Dir
}

func (m Model) brainWide() bool { return m.width >= brainSplitMin }

func (m *Model) refreshBrainTree() {
	dir := m.brainDir()
	_ = brain.Ensure(dir)
	nodes, err := brain.Tree(dir, m.brainFold)
	if err != nil {
		m.err = err.Error()
		return
	}
	m.brainNodes = nodes
	if m.brainCursor >= len(m.brainNodes) {
		m.brainCursor = max(0, len(m.brainNodes)-1)
	}
	m.brainDirty = brain.Dirty(m.brainSettings())
}

func (m *Model) loadBrainNote(rel string, push bool) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		rel = "inbox.md"
	}
	body, err := brain.ReadNote(m.brainDir(), rel)
	if err != nil {
		m.err = err.Error()
		return
	}
	if push && m.brainRel != "" && m.brainRel != rel {
		m.brainHist = append(m.brainHist, m.brainRel)
	}
	m.brainRel = rel
	m.brainBody = body
	m.brainNoteOff = 0
	m.brainHitLine = ""
	out := brain.NoteLinks(m.brainDir(), rel, body)
	m.brainOutN = len(out)
	m.brainLinks = append(out, brain.Backlinks(m.brainDir(), rel)...)
	m.brainLinkIdx = -1
	m.err = ""
	m.selectBrainRel(rel)
}

func (m *Model) selectBrainRel(rel string) {
	for i, n := range m.brainNodes {
		if n.Rel == rel {
			m.brainCursor = i
			m.clampBrainOffset()
			return
		}
	}
}

func (m *Model) clampBrainOffset() {
	h := max(3, m.height-6)
	if m.brainCursor < m.brainOffset {
		m.brainOffset = m.brainCursor
	}
	if m.brainCursor >= m.brainOffset+h {
		m.brainOffset = m.brainCursor - h + 1
	}
	if m.brainOffset < 0 {
		m.brainOffset = 0
	}
}

func (m Model) openBrain(from screen, rel string) (tea.Model, tea.Cmd) {
	if m.brainFold == nil {
		m.brainFold = map[string]bool{}
	}
	m.brainFrom = from
	if m.brainFrom == screenBrain || m.brainFrom == screenPalette {
		m.brainFrom = screenList
	}
	m.screen = screenBrain
	m.brainFind = false
	m.brainInbox = false
	m.brainNotePane = false
	m.brainHist = nil
	m.refreshBrainTree()
	if rel == "" {
		rel = "inbox.md"
	}
	m.loadBrainNote(rel, false)
	m.status = m.brainStatus()
	return m, nil
}

func (m Model) openBrainProject() (tea.Model, tea.Cmd) {
	from := m.screen
	if from == screenPalette {
		from = m.paletteFrom
	}
	p := m.boardProject
	if p.ID == 0 {
		if sel, ok := m.selected(); ok {
			p = sel
		}
	}
	if p.ID == 0 {
		m.status = i18n.T("No project selected")
		return m, nil
	}
	_ = brain.UpsertProject(m.brainDir(), p)
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		slug = store.Slugify(p.Name)
	}
	if slug == "" {
		slug = "project"
	}
	return m.openBrain(from, "projects/"+slug+"/project.md")
}

func (m Model) closeBrain() Model {
	from := m.brainFrom
	if from == screenBrain {
		from = screenList
	}
	m.screen = from
	m.brainFind = false
	m.brainInbox = false
	if from == screenList {
		m.status = listStatus()
		m.reload()
	} else if from == screenBoard {
		m.status = m.boardHints()
	}
	return m
}

func (m Model) brainStatus() string {
	s := i18n.T("j/k tree  enter open  tab link  / search  o edit  i inbox  esc back")
	if m.cfg.BrainGit {
		if m.brainDirty {
			s += i18n.T("  · uncommitted  ctrl+s commit")
		} else {
			s += i18n.T("  · brain git clean")
		}
	}
	return s
}

func (m Model) updateBrainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.brainInbox {
		return m.updateBrainInboxKey(msg)
	}
	if m.brainFind {
		return m.updateBrainFindKey(msg)
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "ctrl+s":
		return m.commitBrain()
	case "esc":
		if !m.brainWide() && m.brainNotePane {
			if m.brainBack() {
				return m, nil
			}
			m.brainNotePane = false
			m.status = m.brainStatus()
			return m, nil
		}
		if m.brainBack() {
			return m, nil
		}
		return m.closeBrain(), nil
	case "backspace":
		if m.brainBack() {
			return m, nil
		}
		return m, nil
	case "?":
		m.helpFrom = screenBrain
		m.screen = screenHelp
		return m, nil
	case "/":
		m.brainFind = true
		m.searchInput.SetValue("")
		m.searchInput.Placeholder = i18n.T("search notes")
		m.searchInput.Focus()
		m.brainHits = nil
		m.brainHitCur = 0
		m.status = i18n.T("type to search  enter open  esc cancel")
		return m, textinput.Blink
	case "i":
		m.brainInbox = true
		m.inboxInput.SetValue("")
		m.inboxInput.Placeholder = i18n.T("line for inbox.md")
		m.inboxInput.Focus()
		m.status = i18n.T("enter add to inbox  esc cancel")
		return m, textinput.Blink
	case "o":
		return m.openBrainEditor()
	case "tab":
		if len(m.brainLinks) == 0 {
			m.status = i18n.T("No links in this note")
			return m, nil
		}
		m.brainLinkIdx = (m.brainLinkIdx + 1) % len(m.brainLinks)
		m.status = i18n.T("Link: ") + m.brainLinks[m.brainLinkIdx].Label
		return m, nil
	case "shift+tab":
		if len(m.brainLinks) == 0 {
			return m, nil
		}
		if m.brainLinkIdx < 0 {
			m.brainLinkIdx = 0
		} else {
			m.brainLinkIdx = (m.brainLinkIdx - 1 + len(m.brainLinks)) % len(m.brainLinks)
		}
		m.status = i18n.T("Link: ") + m.brainLinks[m.brainLinkIdx].Label
		return m, nil
	case "enter":
		return m.activateBrainEnter()
	case "h", "left":
		return m.foldBrain(true)
	case "l", "right":
		return m.foldBrain(false)
	case "j", "down":
		if !m.brainWide() && m.brainNotePane {
			m.brainNoteOff++
			return m, nil
		}
		if m.brainCursor < len(m.brainNodes)-1 {
			m.brainCursor++
			m.brainLinkIdx = -1
			m.clampBrainOffset()
		}
		return m, nil
	case "k", "up":
		if !m.brainWide() && m.brainNotePane {
			if m.brainNoteOff > 0 {
				m.brainNoteOff--
			}
			return m, nil
		}
		if m.brainCursor > 0 {
			m.brainCursor--
			m.brainLinkIdx = -1
			m.clampBrainOffset()
		}
		return m, nil
	case "pgdown":
		m.brainNoteOff += max(1, m.height-8)
	case "pgup":
		m.brainNoteOff = max(0, m.brainNoteOff-max(1, m.height-8))
	case "t":
		m.cycleTheme()
	}
	return m, nil
}

func (m Model) activateBrainEnter() (tea.Model, tea.Cmd) {
	if m.brainLinkIdx >= 0 && m.brainLinkIdx < len(m.brainLinks) && (m.brainWide() || m.brainNotePane) {
		l := m.brainLinks[m.brainLinkIdx]
		if l.Rel == "" {
			m.status = i18n.T("Link not found: ") + l.Target
			return m, nil
		}
		m.loadBrainNote(l.Rel, true)
		if !m.brainWide() {
			m.brainNotePane = true
		}
		m.status = m.brainStatus()
		return m, nil
	}
	if m.brainCursor < 0 || m.brainCursor >= len(m.brainNodes) {
		return m, nil
	}
	n := m.brainNodes[m.brainCursor]
	if n.Dir {
		m.brainFold[n.Rel] = !m.brainFold[n.Rel]
		m.refreshBrainTree()
		m.selectBrainRel(n.Rel)
		return m, nil
	}
	m.loadBrainNote(n.Rel, m.brainRel != "" && m.brainRel != n.Rel)
	if !m.brainWide() {
		m.brainNotePane = true
	}
	m.status = m.brainStatus()
	return m, nil
}

func (m Model) foldBrain(collapse bool) (tea.Model, tea.Cmd) {
	if m.brainCursor < 0 || m.brainCursor >= len(m.brainNodes) {
		return m, nil
	}
	n := m.brainNodes[m.brainCursor]
	rel := n.Rel
	if !n.Dir {
		if i := strings.LastIndex(rel, "/"); i >= 0 {
			rel = rel[:i]
		} else {
			return m, nil
		}
	}
	m.brainFold[rel] = collapse
	m.refreshBrainTree()
	m.selectBrainRel(rel)
	return m, nil
}

func (m *Model) brainBack() bool {
	if len(m.brainHist) == 0 {
		return false
	}
	rel := m.brainHist[len(m.brainHist)-1]
	m.brainHist = m.brainHist[:len(m.brainHist)-1]
	m.loadBrainNote(rel, false)
	m.status = m.brainStatus()
	return true
}

func (m Model) updateBrainFindKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.brainFind = false
		m.searchInput.Blur()
		m.status = m.brainStatus()
		return m, nil
	case "up", "k", "shift+tab":
		if m.brainHitCur > 0 {
			m.brainHitCur--
		}
		return m, nil
	case "down", "j", "tab":
		if m.brainHitCur < len(m.brainHits)-1 {
			m.brainHitCur++
		}
		return m, nil
	case "enter":
		if m.brainHitCur >= 0 && m.brainHitCur < len(m.brainHits) {
			h := m.brainHits[m.brainHitCur]
			m.brainFind = false
			m.searchInput.Blur()
			m.loadBrainNote(h.Rel, true)
			m.brainHitLine = fmt.Sprintf("%s:%d  %s", h.Rel, h.Line, h.Text)
			if !m.brainWide() {
				m.brainNotePane = true
			}
			m.status = m.brainHitLine
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.brainHits = brain.Search(m.brainDir(), m.searchInput.Value())
	m.brainHitCur = 0
	return m, cmd
}

func (m Model) updateBrainInboxKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.brainInbox = false
		m.inboxInput.Blur()
		m.status = m.brainStatus()
		return m, nil
	case "enter":
		line := strings.TrimSpace(m.inboxInput.Value())
		m.brainInbox = false
		m.inboxInput.Blur()
		if line == "" {
			m.status = m.brainStatus()
			return m, nil
		}
		if err := brain.AppendInbox(m.brainDir(), line); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.refreshBrainTree()
		if m.brainRel == "inbox.md" {
			m.loadBrainNote("inbox.md", false)
		}
		m.status = i18n.T("Added to inbox")
		return m, nil
	}
	var cmd tea.Cmd
	m.inboxInput, cmd = m.inboxInput.Update(msg)
	return m, cmd
}

func (m Model) commitBrain() (tea.Model, tea.Cmd) {
	if !m.cfg.BrainGit {
		m.status = i18n.T("Local brain git is off")
		return m, nil
	}
	if !brain.IsRepo(m.brainDir()) {
		m.status = i18n.T("Brain folder is not a git repo")
		return m, nil
	}
	if !brain.Dirty(m.brainSettings()) {
		m.status = i18n.T("Nothing to commit")
		return m, nil
	}
	if err := brain.Commit(m.brainSettings(), "troopa: notes"); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.brainDirty = brain.Dirty(m.brainSettings())
	m.status = i18n.T("Brain committed locally")
	return m, nil
}

func (m Model) openBrainEditor() (tea.Model, tea.Cmd) {
	path, ok := brain.Abs(m.brainDir(), m.brainRel)
	if !ok {
		m.err = i18n.T("No note selected")
		return m, nil
	}
	cmd, err := launch.OpenAttachment(m.cfg.Launch.Editor, path, "text/markdown")
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.status = i18n.T("Opening in the editor: ") + path
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorClosedMsg{err: err} })
}

func (m Model) updateBrainMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	wheelUp := msg.Button == tea.MouseButtonWheelUp
	wheelDown := msg.Button == tea.MouseButtonWheelDown
	if wheelUp || wheelDown {
		delta := 1
		if wheelUp {
			delta = -1
		}
		treeW := m.brainTreeWidth()
		if m.brainWide() && msg.X >= treeW {
			m.brainNoteOff = max(0, m.brainNoteOff+delta)
			return m, nil
		}
		if !m.brainWide() && m.brainNotePane {
			m.brainNoteOff = max(0, m.brainNoteOff+delta)
			return m, nil
		}
		m.brainCursor = min(len(m.brainNodes)-1, max(0, m.brainCursor+delta))
		m.clampBrainOffset()
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if m.brainFind || m.brainInbox {
		return m, nil
	}
	if !m.brainWide() && m.brainNotePane {
		return m, nil
	}
	row := msg.Y - 2
	idx := m.brainOffset + row
	if idx >= 0 && idx < len(m.brainNodes) && (msg.X < m.brainTreeWidth() || !m.brainWide()) {
		m.brainCursor = idx
		return m.activateBrainEnter()
	}
	return m, nil
}

func (m Model) brainTreeWidth() int {
	if !m.brainWide() {
		return m.width
	}
	return min(42, max(28, m.width*32/100))
}

func (m Model) viewBrain() string {
	th := m.theme
	if m.brainFind {
		return m.viewBrainSearch()
	}
	if m.brainInbox {
		return m.viewBrainInbox()
	}
	title := th.Title.Render(i18n.T("Second brain"))
	if m.brainRel != "" {
		title += th.Muted.Render("  ·  " + m.brainRel)
	}
	tree := m.viewBrainTree(m.brainTreeWidth())
	if !m.brainWide() {
		if m.brainNotePane {
			return th.App.Render(title + "\n\n" + m.viewBrainNote(m.width, max(4, m.height-4)))
		}
		return th.App.Render(title + "\n\n" + tree)
	}
	note := m.viewBrainNote(max(20, m.width-m.brainTreeWidth()), max(4, m.height-4))
	return th.App.Render(title + "\n\n" + lipgloss.JoinHorizontal(lipgloss.Top, tree, note))
}

func (m Model) viewBrainTree(w int) string {
	th := m.theme
	h := max(3, m.height-6)
	var b strings.Builder
	if len(m.brainNodes) == 0 {
		return th.Muted.Width(w).Render(i18n.T("  (empty)"))
	}
	end := min(len(m.brainNodes), m.brainOffset+h)
	for i := m.brainOffset; i < end; i++ {
		n := m.brainNodes[i]
		pad := strings.Repeat("  ", n.Depth)
		mark := "  "
		if n.Dir {
			if m.brainFold[n.Rel] {
				mark = "▸ "
			} else {
				mark = "▾ "
			}
		}
		line := pad + mark + n.Name
		st := th.Normal
		if i == m.brainCursor {
			st = th.Selected
		} else if n.Rel == m.brainRel {
			st = th.Title
		}
		b.WriteString(st.Width(max(0, w)).Render(truncate(line, max(1, w-1))))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) viewBrainNote(w, h int) string {
	th := m.theme
	inner := max(16, w-2)
	rendered := m.renderMarkdownW(brain.WikiToMarkdown(m.brainBody), inner)
	var extra strings.Builder
	if m.brainHitLine != "" {
		extra.WriteString(th.Selected.UnsetPadding().Width(inner).Render(m.brainHitLine) + "\n\n")
	}
	if len(m.brainLinks) > 0 {
		extra.WriteString("\n" + th.Muted.Render(i18n.T("Links")) + "\n")
		for i, l := range m.brainLinks {
			if i == m.brainOutN {
				extra.WriteString(th.Muted.Render(i18n.T("Backlinks")) + "\n")
			}
			line := "  " + l.Label + "  " + l.Rel
			st := th.Normal
			if i == m.brainLinkIdx {
				st = th.Selected
				line = "› " + l.Label + "  " + l.Rel
			}
			extra.WriteString(st.UnsetPadding().Width(inner).Render(truncate(line, inner)) + "\n")
		}
	}
	text := rendered + extra.String()
	lines := strings.Split(text, "\n")
	off := m.brainNoteOff
	if off > max(0, len(lines)-1) {
		off = max(0, len(lines)-1)
	}
	if off > 0 {
		lines = lines[off:]
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	border := th.Muted.GetForeground()
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(th.App.GetBackground()).
		Background(th.App.GetBackground()).
		Padding(0, 1).
		Width(max(0, w-2)).
		Height(max(1, h-2)).
		MaxHeight(h).
		Render(strings.Join(lines, "\n"))
}

func (m Model) viewBrainSearch() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Search notes")) + "\n")
	b.WriteString(th.Normal.Render("> "+m.searchInput.View()) + "\n\n")
	if len(m.brainHits) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("  No matches")))
		return th.App.Render(b.String())
	}
	h := max(5, m.height-8)
	start := 0
	if m.brainHitCur >= h {
		start = m.brainHitCur - h + 1
	}
	end := min(len(m.brainHits), start+h)
	for i := start; i < end; i++ {
		hit := m.brainHits[i]
		line := fmt.Sprintf("%s:%d  %s", hit.Rel, hit.Line, hit.Text)
		st := th.Normal
		if i == m.brainHitCur {
			st = th.Selected
		}
		b.WriteString(st.Width(max(0, m.width)).Render(truncate(line, max(1, m.width-2))))
		b.WriteString("\n")
	}
	return th.App.Render(b.String())
}

func (m Model) viewBrainInbox() string {
	th := m.theme
	return th.App.Render(th.Title.Render(i18n.T("Inbox")) + "\n\n> " + m.inboxInput.View() + "\n\n" +
		th.Muted.Render(i18n.T("enter add to inbox  esc cancel")))
}

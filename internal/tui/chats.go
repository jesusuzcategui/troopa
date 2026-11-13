package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/termimg"
)

type chatEntry struct {
	Name string
	Dir  string
}

func chatsDir() string {
	data, _, _, err := config.Paths()
	if err != nil {
		return ""
	}
	return filepath.Join(data, "chats")
}

func listChatEntries() []chatEntry {
	dir := chatsDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []chatEntry
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		td := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(td, "transcript.md")); err != nil {
			continue
		}
		out = append(out, chatEntry{Name: e.Name(), Dir: td})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out
}

func (m Model) openChats(from screen) (tea.Model, tea.Cmd) {
	m.chatsFrom = from
	m.chats = listChatEntries()
	m.chatCursor = 0
	m.chatOffset = 0
	m.transcript = nil
	m.transcriptPath = ""
	m.screen = screenChats
	m.status = i18n.T("enter open  esc back")
	return m, nil
}

func (m Model) openTranscript(dir string) (tea.Model, tea.Cmd) {
	path := filepath.Join(dir, "transcript.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.transcriptPath = path
	m.transcript = strings.Split(string(raw), "\n")
	m.chatOffset = 0
	m.screen = screenTranscript
	m.status = i18n.T("j/k scroll  enter/o open image  esc chats")
	m.err = ""
	return m, nil
}

func (m Model) updateChatsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.imageOverlay != nil {
		return m.updateImageOverlayKey(msg)
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = m.chatsFrom
		m.status = ""
		return m, nil
	case "j", "down":
		if m.chatCursor < len(m.chats)-1 {
			m.chatCursor++
		}
	case "k", "up":
		if m.chatCursor > 0 {
			m.chatCursor--
		}
	case "enter", "o":
		if m.chatCursor >= 0 && m.chatCursor < len(m.chats) {
			return m.openTranscript(m.chats[m.chatCursor].Dir)
		}
	}
	return m, nil
}

func (m Model) updateTranscriptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.imageOverlay != nil {
		return m.updateImageOverlayKey(msg)
	}
	h := max(4, m.height-4)
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = screenChats
		m.status = i18n.T("enter open  esc back")
		return m, nil
	case "j", "down":
		if m.chatOffset < len(m.transcript)-1 {
			m.chatOffset++
		}
	case "k", "up":
		if m.chatOffset > 0 {
			m.chatOffset--
		}
	case "pgdown", "ctrl+d":
		m.chatOffset = min(m.chatOffset+h, max(0, len(m.transcript)-1))
	case "pgup", "ctrl+u":
		m.chatOffset = max(0, m.chatOffset-h)
	case "enter", "o":
		if m.chatOffset >= 0 && m.chatOffset < len(m.transcript) {
			line := m.transcript[m.chatOffset]
			if p, ok := termimg.ImageRef(line); ok {
				return m.openImageOrFile(p, "")
			}
			m.status = i18n.T("No image on this line")
		}
	}
	return m, nil
}

func (m Model) viewChats() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Conversations")) + th.Muted.Render("  ·  "+i18n.T("Telegram transcripts")) + "\n\n")
	if len(m.chats) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("No conversations yet.")))
	} else {
		h := max(4, m.height-5)
		start := 0
		if m.chatCursor >= h {
			start = m.chatCursor - h + 1
		}
		end := min(len(m.chats), start+h)
		for i := start; i < end; i++ {
			mark := "  "
			if i == m.chatCursor {
				mark = th.Border.Render("> ")
			}
			b.WriteString(mark + m.chats[i].Name + "\n")
		}
	}
	b.WriteString("\n" + th.Muted.Render(i18n.T("enter open  esc back")))
	if m.imageOverlay != nil {
		return th.App.Render(b.String() + "\n\n" + m.viewImageOverlay())
	}
	return th.App.Render(b.String())
}

func (m Model) viewTranscript() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Transcript")) + th.Muted.Render("  ·  "+filepath.Base(filepath.Dir(m.transcriptPath))) + "\n\n")
	h := max(4, m.height-5)
	lines := m.transcript
	if lines == nil {
		lines = []string{}
	}
	off := m.chatOffset
	if off > len(lines) {
		off = max(0, len(lines)-1)
	}
	end := min(len(lines), off+h)
	for i := off; i < end; i++ {
		line := lines[i]
		text := truncate(line, max(8, m.width-4))
		if i == m.chatOffset {
			if _, ok := termimg.ImageRef(line); ok {
				b.WriteString(th.Selected.UnsetPadding().Width(max(8, m.width-2)).Render(text) + "\n")
			} else {
				b.WriteString(th.Selected.UnsetPadding().Width(max(8, m.width-2)).Render(text) + "\n")
			}
		} else if _, ok := termimg.ImageRef(line); ok {
			b.WriteString(th.Border.Render(text) + "\n")
		} else {
			b.WriteString(th.Normal.UnsetPadding().Render(text) + "\n")
		}
	}
	b.WriteString("\n" + th.Muted.Render(i18n.T("j/k scroll  enter/o open image  esc chats")))
	if m.imageOverlay != nil {
		return th.App.Render(b.String() + "\n\n" + m.viewImageOverlay())
	}
	return th.App.Render(b.String())
}

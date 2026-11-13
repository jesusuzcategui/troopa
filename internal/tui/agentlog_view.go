package tui

import (
	"os"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/agentlog"
	"github.com/jesusuzcategui/troopa/internal/termimg"
)

func (m Model) logRawBytes() []byte {
	raw, _ := os.ReadFile(m.run.LogPath)
	if r, ok := m.selectedRound(); ok {
		raw = roundLogSlice(raw, r)
	}
	return raw
}

func (m Model) logDisplayLines() []string {
	raw := string(m.logRawBytes())
	if m.logRaw {
		return strings.Split(raw, "\n")
	}
	lines := agentlog.Format(raw, agentlog.Options{
		ExpandThinking: m.logExpandThinking,
		MaxToolOut:     3,
	})
	out := make([]string, len(lines))
	th := m.theme
	for i, l := range lines {
		t := l.Text
		if l.Dim {
			t = th.Muted.Render(t)
		} else {
			t = th.Normal.Render(t)
		}
		if p, ok := termimg.ImageRef(l.Text); ok && termimg.IsImagePath(p) {
			t = th.Border.Render("▸ ") + th.Normal.Render(l.Text)
		}
		out[i] = t
	}
	return out
}

func (m *Model) toggleLogFormatted() {
	m.logRaw = !m.logRaw
}

func (m *Model) toggleLogThinking() {
	m.logExpandThinking = !m.logExpandThinking
}

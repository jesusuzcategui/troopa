package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/agentlog"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/termimg"
)

type imageOverlay struct {
	path string
	id   int
	cols int
	rows int
}

type imageShownMsg struct {
	path string
	err  string
}

type imageClosedMsg struct{}

func (m Model) openImageOrFile(path, mime string) (Model, tea.Cmd) {
	if path == "" {
		m.status = i18n.T("No attachment selected")
		return m, nil
	}
	if termimg.IsImagePath(path) || termimg.IsImageMIME(mime) {
		if termimg.Supports() {
			return m.openImageOverlay(path)
		}
		cmd, err := launch.OpenAttachment(m.cfg.Launch.Editor, path, mime)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.status = i18n.T("Terminal cannot show images here; opening with the system viewer")
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorClosedMsg{err: err} })
	}
	cmd, err := launch.OpenAttachment(m.cfg.Launch.Editor, path, mime)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.status = i18n.T("Opening: ") + filepath.Base(path)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorClosedMsg{err: err} })
}

func (m Model) openImageOverlay(path string) (Model, tea.Cmd) {
	if _, err := os.Stat(path); err != nil {
		m.err = err.Error()
		return m, nil
	}
	cols := max(20, m.width-12)
	rows := max(8, m.height-8)
	m.imageOverlay = &imageOverlay{path: path, id: 1, cols: cols, rows: rows}
	return m, showImageCmd(path, cols, rows)
}

func showImageCmd(path string, cols, rows int) tea.Cmd {
	return func() tea.Msg {
		seq, err := termimg.PlaceSequence(path, 1, cols, rows)
		if err != nil {
			return imageShownMsg{err: err.Error()}
		}
		fmt.Print(seq)
		return imageShownMsg{path: path}
	}
}

func hideImageCmd(id int) tea.Cmd {
	return func() tea.Msg {
		fmt.Print(termimg.ClearSequence(id))
		return imageClosedMsg{}
	}
}

func (m Model) updateImageOverlayKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "o", "q":
		ov := m.imageOverlay
		m.imageOverlay = nil
		m.status = ""
		if ov != nil {
			return m, hideImageCmd(ov.id)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) viewImageOverlay() string {
	if m.imageOverlay == nil {
		return ""
	}
	th := m.theme
	title := th.Title.Render(i18n.T("Image")) + th.Muted.Render("  ·  "+filepath.Base(m.imageOverlay.path))
	help := th.Muted.Render(i18n.T("esc close"))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Title.GetForeground()).
		Padding(0, 1).
		Width(max(20, m.width-4)).
		Render(title + "\n\n" + help + "\n")
	return box
}

func (m Model) openLogImageLine() (Model, tea.Cmd) {
	raw := string(m.logRawBytes())
	rawLines := strings.Split(raw, "\n")
	off := m.logOffset
	if off < len(rawLines) {
		if p, ok := termimg.ImageRef(rawLines[off]); ok {
			return m.openImageOrFile(p, "")
		}
	}
	if !m.logRaw {
		formatted := agentlog.Format(raw, agentlog.Options{ExpandThinking: m.logExpandThinking, MaxToolOut: 3})
		if off < len(formatted) {
			if p, ok := termimg.ImageRef(formatted[off].Text); ok {
				return m.openImageOrFile(p, "")
			}
		}
	}
	return m, nil
}

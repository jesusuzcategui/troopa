package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/i18n"
)

const (
	creatorName    = "Jesús Uzcátegui"
	creatorWebsite = "jesusuzcategui.com"
	projectRepo    = "github.com/jesusuzcategui/troopa"
)

func (m Model) openAbout() (tea.Model, tea.Cmd) {
	m.about = true
	return m, nil
}

func (m Model) aboutBox() (box string, x, y int) {
	th := m.theme
	inner := min(64, max(30, m.width-8))
	label := lipgloss.NewStyle().Foreground(th.Muted.GetForeground())
	var b strings.Builder
	b.WriteString(m.renderLogo(inner))
	b.WriteString("\n\n")
	b.WriteString(label.Render(i18n.T("Version")) + "  " + th.Normal.UnsetPadding().Render(m.version))
	b.WriteString("\n\n")
	b.WriteString(th.Title.Render(i18n.T("Created by")+" "+creatorName) + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(th.Normal.GetForeground()).Width(inner).Render(i18n.T(i18n.AboutCreator)))
	b.WriteString("\n\n")
	b.WriteString(label.Render(i18n.T("Website")) + "  " + creatorWebsite + "\n")
	b.WriteString(label.Render(i18n.T("Code")) + "     " + projectRepo + "\n")
	b.WriteString(label.Render(i18n.T("License")) + "  MIT")
	b.WriteString("\n\n")
	b.WriteString(label.Render(i18n.T("esc close")))
	box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Title.GetForeground()).
		BorderBackground(th.App.GetBackground()).
		Background(th.App.GetBackground()).
		Padding(1, 2).
		Render(b.String())
	x = max(0, (m.width-lipgloss.Width(box))/2)
	y = max(0, (m.height-1-lipgloss.Height(box))/2)
	return box, x, y
}

func (m Model) updateAboutKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "enter", "q", "i", " ":
		m.about = false
	}
	return m, nil
}

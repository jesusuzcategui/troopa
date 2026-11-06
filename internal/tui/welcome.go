package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
)

// The welcome screen is the logo shown at start. Any key, a click or a short
// timer moves on to the project list.
func (m Model) updateWelcomeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	return m.dismissWelcome()
}

func (m Model) dismissWelcome() (tea.Model, tea.Cmd) {
	if !m.cfg.WelcomeShown {
		m.cfg.WelcomeShown = true
		if err := config.Save(m.configPath, m.cfg); err != nil {
			m.err = i18n.T("Could not save the config: ") + err.Error()
		}
	}
	m.screen = screenList
	m.status = listStatus()
	return m, nil
}

func (m Model) viewWelcome() string {
	th := m.theme
	hint := th.Muted.Render(i18n.T("press any key") + "  ·  " + m.version)
	block := lipgloss.JoinVertical(lipgloss.Left, m.renderLogo(m.width), "", hint)
	return lipgloss.Place(m.width, max(1, m.height-1), lipgloss.Center, lipgloss.Center, block,
		lipgloss.WithWhitespaceBackground(th.App.GetBackground()))
}

package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
)

func (m Model) updateWelcomeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter", "esc", " ":
		return m.dismissWelcome()
	}
	return m, nil
}

func (m Model) dismissWelcome() (tea.Model, tea.Cmd) {
	m.cfg.WelcomeShown = true
	if err := config.Save(m.configPath, m.cfg); err != nil {
		m.err = i18n.T("Could not save the config: ") + err.Error()
	}
	m.screen = screenList
	m.status = listStatus()
	return m, nil
}

func (m Model) viewWelcome() string {
	th := m.theme
	text := i18n.T(i18n.WelcomeText)
	return th.Help.Render(strings.TrimSpace(text) + "\n")
}

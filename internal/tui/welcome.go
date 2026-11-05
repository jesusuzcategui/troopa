package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
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
		m.err = "No se pudo guardar la config: " + err.Error()
	}
	m.screen = screenList
	m.status = listStatus
	return m, nil
}

func (m Model) viewWelcome() string {
	th := m.theme
	text := `Troopa
tu tropa de agentes en la terminal

  • Proyectos, tablero, clientes y métricas
  • Temas en TOML (t cicla · Ctrl+K → Elegir tema)
  • Paleta de comandos: Ctrl+K
  • Agentes en ~/.config/troopa/config.toml

Pulsa enter para empezar.`
	return th.Help.Render(strings.TrimSpace(text) + "\n")
}

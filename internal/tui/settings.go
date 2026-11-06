package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
)

type settingKind int

const (
	settingChoice settingKind = iota
	settingText
	settingToggle
	settingAction
)

type settingRow struct {
	key  string
	kind settingKind
}

var settingRows = []settingRow{
	{"language", settingChoice},
	{"theme", settingChoice},
	{"default_agent", settingChoice},
	{"default_model", settingText},
	{"editor", settingChoice},
	{"terminal", settingChoice},
	{"merge", settingChoice},
	{"branch_prefix", settingText},
	{"worktrees", settingText},
	{"show_logo", settingToggle},
	{"rerun_setup", settingAction},
	{"about", settingAction},
}

type settingsState struct {
	cursor  int
	editing bool
	input   textinput.Model
	agents  []launch.AgentInfo
	from    screen
}

func (m *Model) openSettings(from screen) {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 200
	in.Width = 40
	m.settings = &settingsState{input: in, agents: launch.DetectAgents(m.cfg.Launch), from: from}
	m.screen = screenSettings
	m.status = i18n.T("↑/↓ setting · ←/→ change · enter edit · r rescan agents · esc back")
}

func (m Model) settingLabel(key string) string {
	switch key {
	case "language":
		return i18n.T("Language")
	case "theme":
		return i18n.T("Theme")
	case "default_agent":
		return i18n.T("Default agent")
	case "default_model":
		return i18n.T("Default model")
	case "editor":
		return i18n.T("Editor")
	case "terminal":
		return i18n.T("Terminal")
	case "merge":
		return i18n.T("Merge strategy")
	case "branch_prefix":
		return i18n.T("Branch prefix")
	case "worktrees":
		return i18n.T("Worktrees folder")
	case "show_logo":
		return i18n.T("Logo at start")
	case "rerun_setup":
		return i18n.T("Run the setup again")
	case "about":
		return i18n.T("About Troopa")
	}
	return key
}

func (m Model) settingChoices(key string) []choice {
	switch key {
	case "language":
		return m.languageChoices()
	case "theme":
		return m.themeChoices()
	case "default_agent":
		return m.agentChoices(m.settings.agents)
	case "editor":
		return m.editorChoices()
	case "terminal":
		return m.terminalChoices()
	case "merge":
		return mergeChoices()
	}
	return nil
}

func (m Model) settingValue(key string) string {
	switch key {
	case "language":
		return orDefault(m.cfg.Language, "en")
	case "theme":
		return m.theme.Name
	case "default_agent":
		return orDefault(m.cfg.DefaultAgent, "me")
	case "default_model":
		return m.cfg.DefaultModel
	case "editor":
		return m.cfg.Launch.Editor
	case "terminal":
		return m.cfg.Launch.Terminal
	case "merge":
		return orDefault(m.cfg.Launch.Merge, "squash")
	case "branch_prefix":
		return m.cfg.Launch.BranchPrefix
	case "worktrees":
		return m.cfg.WorktreesRaw
	}
	return ""
}

func (m Model) settingDisplay(row settingRow) string {
	switch row.kind {
	case settingToggle:
		if m.cfg.HideLogo {
			return i18n.T("off")
		}
		return i18n.T("on")
	case settingAction:
		return "→"
	case settingText:
		v := m.settingValue(row.key)
		if v == "" {
			if row.key == "worktrees" {
				return config.ResolveWorktrees("") + "  " + i18n.T("(default)")
			}
			return "—"
		}
		return v
	}
	v := m.settingValue(row.key)
	for _, c := range m.settingChoices(row.key) {
		if c.value == v {
			if c.detail != "" {
				return c.label + "  ·  " + c.detail
			}
			return c.label
		}
	}
	return orAuto(v)
}

// setSetting stores a value, applies it right away and saves config.toml.
func (m *Model) setSetting(key, value string) {
	switch key {
	case "language":
		m.cfg.Language = value
		i18n.Apply(value)
		m.relocalize()
	case "theme":
		m.theme = themeByName(m.themes, value)
		m.cfg.Theme = m.theme.Name
	case "default_agent":
		m.cfg.DefaultAgent = value
	case "default_model":
		m.cfg.DefaultModel = value
	case "editor":
		m.cfg.Launch.Editor = value
	case "terminal":
		m.cfg.Launch.Terminal = value
	case "merge":
		m.cfg.Launch.Merge = value
	case "branch_prefix":
		m.cfg.Launch.BranchPrefix = value
	case "worktrees":
		m.cfg.WorktreesRaw = value
		m.cfg.Launch.Worktrees = config.ResolveWorktrees(value)
	}
	if err := m.saveConfig(); err != nil {
		m.err = i18n.T("Could not save the config: ") + err.Error()
		return
	}
	m.err = ""
	m.status = fmt.Sprintf("%s: %s", m.settingLabel(key), orDash(value))
}

func (m Model) updateSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.settings
	key := msg.String()
	if s.editing {
		switch key {
		case "enter":
			s.editing = false
			s.input.Blur()
			m.setSetting(settingRows[s.cursor].key, strings.TrimSpace(s.input.Value()))
			return m, nil
		case "esc":
			s.editing = false
			s.input.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return m, cmd
	}
	row := settingRows[s.cursor]
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.screen = s.from
		if m.screen == screenSettings {
			m.screen = screenList
		}
		m.settings = nil
		m.status = ""
		if m.screen == screenList {
			m.status = listStatus()
		}
	case "j", "down", "tab":
		s.cursor = (s.cursor + 1) % len(settingRows)
	case "k", "up", "shift+tab":
		s.cursor = (s.cursor - 1 + len(settingRows)) % len(settingRows)
	case "r":
		s.agents = launch.DetectAgents(m.cfg.Launch)
		m.status = i18n.T("Agents scanned again")
	case "left", "h", "right", "l":
		dir := 1
		if key == "left" || key == "h" {
			dir = -1
		}
		switch row.kind {
		case settingChoice:
			ch := m.settingChoices(row.key)
			vals := make([]string, len(ch))
			for i, c := range ch {
				vals[i] = c.value
			}
			if len(vals) > 0 {
				m.setSetting(row.key, cycleValue(m.settingValue(row.key), vals, dir))
			}
		case settingToggle:
			m.toggleLogo()
		}
	case "enter", " ":
		switch row.kind {
		case settingText:
			s.editing = true
			s.input.SetValue(m.settingValue(row.key))
			s.input.CursorEnd()
			s.input.Focus()
			return m, textinput.Blink
		case settingToggle:
			m.toggleLogo()
		case settingChoice:
			ch := m.settingChoices(row.key)
			vals := make([]string, len(ch))
			for i, c := range ch {
				vals[i] = c.value
			}
			if len(vals) > 0 {
				m.setSetting(row.key, cycleValue(m.settingValue(row.key), vals, 1))
			}
		case settingAction:
			if row.key == "about" {
				return m.openAbout()
			}
			m.settings = nil
			m.startSetup(screenSettings)
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) toggleLogo() {
	m.cfg.HideLogo = !m.cfg.HideLogo
	if err := m.saveConfig(); err != nil {
		m.err = i18n.T("Could not save the config: ") + err.Error()
		return
	}
	m.status = m.settingLabel("show_logo") + ": " + m.settingDisplay(settingRow{key: "show_logo", kind: settingToggle})
}

const settingsTop = 2 // title, blank line, then the rows

func (m Model) updateSettingsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft || m.settings.editing {
		return m, nil
	}
	idx := msg.Y - settingsTop
	if idx >= 0 && idx < len(settingRows) {
		if m.settings.cursor == idx {
			return m.updateSettingsKey(tea.KeyMsg{Type: tea.KeyEnter})
		}
		m.settings.cursor = idx
	}
	return m, nil
}

func (m Model) viewSettings() string {
	th := m.theme
	s := m.settings
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Settings")) + th.Muted.Render("  ·  "+m.configPath) + "\n\n")
	labelW := 20
	for i, row := range settingRows {
		label := fmt.Sprintf("%-*s", labelW, m.settingLabel(row.key))
		val := m.settingDisplay(row)
		if s.editing && i == s.cursor {
			val = s.input.View()
		} else if row.kind == settingChoice && i == s.cursor {
			val = "‹ " + val + " ›"
		}
		line := truncate(label+val, max(10, m.width-4))
		if s.editing && i == s.cursor {
			line = label + val
		}
		if i == s.cursor {
			b.WriteString(th.Selected.Render(line) + "\n")
		} else {
			b.WriteString(th.Normal.Render(line) + "\n")
		}
	}
	b.WriteString("\n" + th.Title.Render(i18n.T("Agents on this computer")) + "\n")
	for _, a := range s.agents {
		mark, detail := "✗", i18n.T("not installed")
		style := th.Muted
		if a.Installed {
			mark, detail, style = "✓", orDefault(a.Version, i18n.T("installed")), th.Normal.UnsetPadding()
		}
		b.WriteString(style.Render(truncate(fmt.Sprintf("  %s %-12s %-10s %s", mark, a.Name, a.Command, detail), max(10, m.width-4))) + "\n")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(th.Muted.GetForeground()).Render(
		"\n" + i18n.T("Agent commands and prices live in config.toml under [agents.*] and [prices.*].")))
	return th.App.Render(b.String())
}

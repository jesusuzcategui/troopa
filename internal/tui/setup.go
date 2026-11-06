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
	"github.com/jesusuzcategui/troopa/internal/store"
)

// A choice is one option in the setup wizard or in a settings row.
type choice struct {
	value  string
	label  string
	detail string
}

type setupStep int

const (
	stepLanguage setupStep = iota
	stepTheme
	stepAgent
	stepModel
	stepEditor
	stepTerminal
	stepMerge
	stepBrain
	stepBrainPath
	stepSummary
	stepCount
)

type setupState struct {
	step   setupStep
	cursor int
	agents []launch.AgentInfo
	model  textinput.Model
	from   screen // where to go back when the setup was opened from settings
}

var editorCandidates = []string{"nvim", "vim", "hx", "code", "zed", "cursor", "subl", "emacs", "nano"}
var terminalCandidates = []string{"kitty -e", "alacritty -e", "wezterm start --", "ghostty -e", "foot", "gnome-terminal --", "konsole -e", "xterm -e"}

func (m *Model) startSetup(from screen) {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 80
	in.Width = 40
	in.Placeholder = i18n.T("leave empty for the agent's default")
	in.SetValue(m.cfg.DefaultModel)
	m.setup = &setupState{agents: launch.DetectAgents(m.cfg.Launch), model: in, from: from}
	m.screen = screenSetup
	m.status = ""
	m.err = ""
	m.setupCursorToCurrent()
}

// languageChoices, agentChoices… are shared by the wizard and the settings screen.
func (m Model) languageChoices() []choice {
	sys := "English"
	if i18n.Detect() == i18n.ES {
		sys = "Español"
	}
	return []choice{
		{value: "en", label: "English"},
		{value: "es", label: "Español"},
		{value: "auto", label: i18n.T("System language"), detail: sys},
	}
}

func (m Model) themeChoices() []choice {
	out := make([]choice, 0, len(m.themes))
	for _, t := range m.themes {
		d := i18n.T("dark")
		if t.IsLight() {
			d = i18n.T("light")
		}
		out = append(out, choice{value: t.Name, label: t.Name, detail: d})
	}
	return out
}

func (m Model) agentChoices(agents []launch.AgentInfo) []choice {
	out := make([]choice, 0, len(agents)+1)
	for _, a := range agents {
		detail := i18n.T("not installed")
		if a.Installed {
			detail = "✓ " + a.Version
			if a.Version == "" {
				detail = "✓ " + i18n.T("installed")
			}
		}
		out = append(out, choice{value: a.ID, label: a.Name, detail: detail})
	}
	return append(out, choice{value: store.AgentMe, label: i18n.T("Me"), detail: i18n.T("no agent, I do the work")})
}

func (m Model) editorChoices() []choice {
	out := []choice{{value: "", label: i18n.T("Automatic"), detail: "$VISUAL, $EDITOR, nvim"}}
	seen := map[string]bool{"": true}
	for _, e := range launch.DetectTools(editorCandidates...) {
		out = append(out, choice{value: e, label: e, detail: i18n.T("found")})
		seen[e] = true
	}
	if cur := m.cfg.Launch.Editor; !seen[cur] {
		out = append(out, choice{value: cur, label: cur, detail: i18n.T("from config.toml")})
	}
	return out
}

func (m Model) terminalChoices() []choice {
	out := []choice{{value: "", label: i18n.T("Automatic"), detail: i18n.T("tmux split, or the first terminal found")}}
	seen := map[string]bool{"": true}
	for _, c := range terminalCandidates {
		if len(launch.DetectTools(strings.Fields(c)[0])) > 0 {
			out = append(out, choice{value: c, label: c, detail: i18n.T("found")})
			seen[c] = true
		}
	}
	if cur := m.cfg.Launch.Terminal; !seen[cur] {
		out = append(out, choice{value: cur, label: cur, detail: i18n.T("from config.toml")})
	}
	return out
}

func mergeChoices() []choice {
	return []choice{
		{value: "squash", label: "squash", detail: i18n.T("one commit on the base branch")},
		{value: "merge", label: "merge", detail: i18n.T("a merge commit that keeps the branch history")},
	}
}

func (m Model) brainChoices() []choice {
	def := config.ResolveBrain("")
	return []choice{
		{value: "troopa", label: i18n.T("Troopa folder"), detail: def},
		{value: "existing", label: i18n.T("Existing folder"), detail: i18n.T("a vault or notes folder you already have")},
	}
}

func (m Model) setupChoices() []choice {
	switch m.setup.step {
	case stepLanguage:
		return m.languageChoices()
	case stepTheme:
		return m.themeChoices()
	case stepAgent:
		return m.agentChoices(m.setup.agents)
	case stepEditor:
		return m.editorChoices()
	case stepTerminal:
		return m.terminalChoices()
	case stepMerge:
		return mergeChoices()
	case stepBrain:
		return m.brainChoices()
	}
	return nil
}

func (m Model) setupCurrentValue() string {
	switch m.setup.step {
	case stepLanguage:
		if m.cfg.Language == "" {
			return "en"
		}
		return m.cfg.Language
	case stepTheme:
		return m.theme.Name
	case stepAgent:
		if m.cfg.DefaultAgent == "" {
			for _, a := range m.setup.agents {
				if a.Installed {
					return a.ID
				}
			}
			return store.AgentMe
		}
		return m.cfg.DefaultAgent
	case stepEditor:
		return m.cfg.Launch.Editor
	case stepTerminal:
		return m.cfg.Launch.Terminal
	case stepMerge:
		if m.cfg.Launch.Merge == "" {
			return "squash"
		}
		return m.cfg.Launch.Merge
	case stepBrain:
		if strings.TrimSpace(m.cfg.BrainRaw) != "" {
			return "existing"
		}
		return "troopa"
	}
	return ""
}

func (m *Model) setupCursorToCurrent() {
	m.setup.cursor = 0
	cur := m.setupCurrentValue()
	for i, c := range m.setupChoices() {
		if c.value == cur {
			m.setup.cursor = i
		}
	}
	if m.setup.step == stepModel || m.setup.step == stepBrainPath {
		if m.setup.step == stepBrainPath {
			m.setup.model.Placeholder = i18n.T("folder path (~/Notes, or leave empty for the default)")
			m.setup.model.SetValue(m.cfg.BrainRaw)
		} else {
			m.setup.model.Placeholder = i18n.T("leave empty for the agent's default")
			m.setup.model.SetValue(m.cfg.DefaultModel)
		}
		m.setup.model.Focus()
	} else {
		m.setup.model.Blur()
	}
}

// previewChoice applies language and theme while the cursor moves, so the
// person sees the result before choosing.
func (m *Model) previewChoice() {
	ch := m.setupChoices()
	if m.setup.cursor >= len(ch) {
		return
	}
	switch m.setup.step {
	case stepLanguage:
		i18n.Apply(ch[m.setup.cursor].value)
		m.relocalize()
	case stepTheme:
		m.theme = themeByName(m.themes, ch[m.setup.cursor].value)
	}
}

func (m *Model) commitSetupStep() {
	ch := m.setupChoices()
	val := ""
	if m.setup.cursor < len(ch) {
		val = ch[m.setup.cursor].value
	}
	switch m.setup.step {
	case stepLanguage:
		m.cfg.Language = val
	case stepTheme:
		m.cfg.Theme = val
	case stepAgent:
		m.cfg.DefaultAgent = val
	case stepModel:
		m.cfg.DefaultModel = strings.TrimSpace(m.setup.model.Value())
	case stepEditor:
		m.cfg.Launch.Editor = val
	case stepTerminal:
		m.cfg.Launch.Terminal = val
	case stepMerge:
		m.cfg.Launch.Merge = val
	case stepBrain:
		if val == "existing" {
			if strings.TrimSpace(m.cfg.BrainRaw) == "" {
				m.cfg.BrainRaw = config.ResolveBrain("")
			}
		} else {
			m.cfg.BrainRaw = ""
		}
		m.cfg.Launch.Brain = config.ResolveBrain(m.cfg.BrainRaw)
	case stepBrainPath:
		m.cfg.BrainRaw = strings.TrimSpace(m.setup.model.Value())
		m.cfg.Launch.Brain = config.ResolveBrain(m.cfg.BrainRaw)
	}
}

func (m Model) finishSetup() (tea.Model, tea.Cmd) {
	m.cfg.SetupDone = true
	m.cfg.WelcomeShown = true
	m.cfg.Launch.Brain = config.ResolveBrain(m.cfg.BrainRaw)
	m.ensureBrain()
	if err := m.saveConfig(); err != nil {
		m.err = i18n.T("Could not save the config: ") + err.Error()
		return m, nil
	}
	from := m.setup.from
	m.setup = nil
	m.err = ""
	if from == screenSettings {
		m.openSettings(screenList)
		m.status = i18n.T("Setup saved")
		return m, nil
	}
	m.screen = screenList
	m.status = i18n.T("Setup saved. Press n to create your first project.")
	return m, nil
}

// saveConfig writes the config and removes keys whose value went back to the
// default, so config.toml only holds what the person chose.
func (m *Model) saveConfig() error {
	if err := config.Save(m.configPath, m.cfg); err != nil {
		return err
	}
	empty := map[string]string{
		"editor":        m.cfg.Launch.Editor,
		"terminal":      m.cfg.Launch.Terminal,
		"default_model": m.cfg.DefaultModel,
		"branch_prefix": m.cfg.Launch.BranchPrefix,
		"worktrees":     m.cfg.WorktreesRaw,
		"brain":         m.cfg.BrainRaw,
	}
	for key, v := range empty {
		if v == "" {
			if err := config.Unset(m.configPath, key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m Model) updateSetupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.setup
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if s.step == stepModel || s.step == stepBrainPath {
		switch key {
		case "enter":
			m.commitSetupStep()
			s.step++
			m.setupCursorToCurrent()
			return m, nil
		case "esc":
			s.step--
			m.setupCursorToCurrent()
			return m, nil
		}
		var cmd tea.Cmd
		s.model, cmd = s.model.Update(msg)
		return m, cmd
	}
	n := len(m.setupChoices())
	switch key {
	case "j", "down", "tab":
		if n > 0 {
			s.cursor = (s.cursor + 1) % n
			m.previewChoice()
		}
	case "k", "up", "shift+tab":
		if n > 0 {
			s.cursor = (s.cursor - 1 + n) % n
			m.previewChoice()
		}
	case "enter", "right", "l":
		if s.step == stepSummary {
			return m.finishSetup()
		}
		m.commitSetupStep()
		if s.step == stepBrain && strings.TrimSpace(m.cfg.BrainRaw) == "" {
			s.step = stepSummary
		} else {
			s.step++
		}
		m.setupCursorToCurrent()
		if s.step == stepModel || s.step == stepBrainPath {
			return m, textinput.Blink
		}
	case "esc", "left", "h":
		if s.step == stepLanguage {
			if s.from == screenSettings {
				i18n.Apply(m.cfg.Language)
				m.relocalize()
				m.theme = themeByName(m.themes, m.cfg.NormalizedTheme())
				m.setup = nil
				m.openSettings(screenList)
			}
			return m, nil
		}
		if s.step == stepSummary && strings.TrimSpace(m.cfg.BrainRaw) == "" {
			s.step = stepBrain
		} else {
			s.step--
		}
		m.setupCursorToCurrent()
	}
	return m, nil
}

func (m Model) updateSetupMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft || m.setup.step == stepModel || m.setup.step == stepBrainPath || m.setup.step == stepSummary {
		return m, nil
	}
	idx := msg.Y - m.setupOptionsTop()
	if idx >= 0 && idx < len(m.setupChoices()) {
		if m.setup.cursor == idx {
			m.commitSetupStep()
			if m.setup.step == stepBrain && strings.TrimSpace(m.cfg.BrainRaw) == "" {
				m.setup.step = stepSummary
			} else {
				m.setup.step++
			}
			m.setupCursorToCurrent()
			return m, nil
		}
		m.setup.cursor = idx
		m.previewChoice()
	}
	return m, nil
}

func (m Model) setupTitle(step setupStep) (string, string) {
	switch step {
	case stepLanguage:
		return i18n.T("Language"), i18n.T("Pick the language of the interface. You can change it later in Settings.")
	case stepTheme:
		return i18n.T("Theme"), i18n.T("The colors change as you move. Your own TOML themes show up here too.")
	case stepAgent:
		return i18n.T("Default agent"), i18n.T("New tasks use this agent. These are the agent CLIs found on this computer.")
	case stepModel:
		return i18n.T("Default model"), i18n.T("Optional. Passed to the agent with --model, for example sonnet or gpt-5.")
	case stepEditor:
		return i18n.T("Editor"), i18n.T("Used by o to open a worktree or a project.")
	case stepTerminal:
		return i18n.T("Terminal"), i18n.T("Where pair sessions open when you are not inside tmux.")
	case stepMerge:
		return i18n.T("Merge strategy"), i18n.T("How g brings an approved branch into the base branch.")
	case stepBrain:
		return i18n.T("Second brain"), i18n.T("A markdown folder for project notes. Troopa writes INDEX, tasks and history; you keep Current state, Next and Decisions.")
	case stepBrainPath:
		return i18n.T("Brain folder"), i18n.T("Absolute path or ~/folder. Created with inbox.md, today.md, projects/INDEX.md and a short README if it does not exist.")
	}
	return i18n.T("Summary"), i18n.T("Press enter to save these settings to config.toml.")
}

// setupHeader is the logo plus the step line; setupOptionsTop counts its rows.
func (m Model) setupHeader() string {
	th := m.theme
	title, help := m.setupTitle(m.setup.step)
	progress := th.Muted.Render(fmt.Sprintf(i18n.T("Step %d of %d"), int(m.setup.step)+1, int(stepCount)))
	return m.renderLogo(m.width) + "\n\n" + progress + "\n" + th.Title.Render(title) + "\n" +
		lipgloss.NewStyle().Foreground(th.Muted.GetForeground()).Width(max(20, min(72, m.width-4))).Render(help) + "\n\n"
}

func (m Model) setupOptionsTop() int {
	return lipgloss.Height(m.setupHeader()) - 1
}

func (m Model) viewSetup() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(m.setupHeader())
	switch m.setup.step {
	case stepModel:
		b.WriteString(th.Muted.Render(i18n.T("Model")+"  ") + m.setup.model.View())
	case stepBrainPath:
		b.WriteString(th.Muted.Render(i18n.T("Path")+"  ") + m.setup.model.View())
	case stepSummary:
		rows := [][2]string{
			{i18n.T("Language"), m.languageLabel()},
			{i18n.T("Theme"), m.cfg.NormalizedTheme()},
			{i18n.T("Default agent"), agentLabel(m.cfg.DefaultAgent)},
			{i18n.T("Default model"), orDash(m.cfg.DefaultModel)},
			{i18n.T("Editor"), orAuto(m.cfg.Launch.Editor)},
			{i18n.T("Terminal"), orAuto(m.cfg.Launch.Terminal)},
			{i18n.T("Merge strategy"), orDefault(m.cfg.Launch.Merge, "squash")},
			{i18n.T("Second brain"), orDefault(m.cfg.BrainRaw, config.ResolveBrain(""))},
		}
		for _, r := range rows {
			b.WriteString(th.Muted.Render(fmt.Sprintf("  %-16s", r[0])) + th.Normal.UnsetPadding().Render(r[1]) + "\n")
		}
	default:
		for i, c := range m.setupChoices() {
			line := fmt.Sprintf("%-18s %s", c.label, c.detail)
			if i == m.setup.cursor {
				b.WriteString(th.Selected.Render("▸ "+line) + "\n")
			} else {
				b.WriteString(th.Normal.Render("  "+line) + "\n")
			}
		}
	}
	b.WriteString("\n")
	hint := i18n.T("↑/↓ choose · enter next · esc back · ctrl+c quit")
	if m.setup.step == stepSummary {
		hint = i18n.T("enter save · esc back")
	}
	b.WriteString(th.Muted.Render(hint))
	return th.App.Render(b.String())
}

func (m Model) languageLabel() string {
	for _, c := range m.languageChoices() {
		if c.value == m.cfg.Language || (m.cfg.Language == "" && c.value == "en") {
			if c.detail != "" {
				return c.label + " (" + c.detail + ")"
			}
			return c.label
		}
	}
	return m.cfg.Language
}

func agentLabel(id string) string {
	if id == "" || id == store.AgentMe {
		return i18n.T("Me")
	}
	return launch.AgentName(id)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func orAuto(s string) string {
	if s == "" {
		return i18n.T("Automatic")
	}
	return s
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// relocalize rebuilds the inputs whose placeholders and labels were set in
// the previous language.
func (m *Model) relocalize() {
	m.quickInput.Placeholder = i18n.T("Task title")
	m.searchInput.Placeholder = i18n.T("search title or description")
	m.paletteInput.Placeholder = i18n.T("search command")
	m.inputs = newInputs()
	m.taskTitle, m.taskModelIn, m.taskDesc, m.taskCrit = newTaskInputs()
	m.clientInputs = newClientInputs()
	if m.setup != nil {
		m.setup.model.Placeholder = i18n.T("leave empty for the agent's default")
	}
}

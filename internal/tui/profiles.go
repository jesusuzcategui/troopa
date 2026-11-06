package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// Profiles say who the agents work for (a developer, a chef, an
// accountant…). Accounts say which login the agent CLIs use. Both are picked
// per project, with a default in config.toml.

type profilesState struct {
	list    []store.Profile
	cursor  int
	editing bool
	draft   store.Profile
	focus   int // 0 name, 1 profession, 2 about, 3 instructions
	name    textinput.Model
	prof    textinput.Model
	about   textarea.Model
	instr   textarea.Model
	confirm int64 // profile id waiting for a second d
	from    screen
}

const profileFields = 4

func (m *Model) openProfiles(from screen) {
	ps := &profilesState{from: from}
	ps.name = textinput.New()
	ps.name.Placeholder = i18n.T("Name (as the agent should call you)")
	ps.name.CharLimit = 120
	ps.name.Width = 48
	ps.name.Prompt = ""
	ps.prof = textinput.New()
	ps.prof.Placeholder = i18n.T("Profession: developer, chef, accountant…")
	ps.prof.CharLimit = 120
	ps.prof.Width = 48
	ps.prof.Prompt = ""
	ps.about = textarea.New()
	ps.about.Placeholder = i18n.T("About you: experience, tools, who you work for")
	ps.about.SetHeight(4)
	ps.about.SetWidth(64)
	ps.about.ShowLineNumbers = false
	ps.about.Prompt = ""
	ps.instr = textarea.New()
	ps.instr.Placeholder = i18n.T("Standing instructions for the agents, one per line")
	ps.instr.SetHeight(4)
	ps.instr.SetWidth(64)
	ps.instr.ShowLineNumbers = false
	ps.instr.Prompt = ""
	m.profiles = ps
	m.reloadProfiles()
	m.screen = screenProfiles
	m.status = i18n.T("n new · enter edit · * default · d delete · esc back")
}

func (m *Model) reloadProfiles() {
	list, err := m.store.ListProfiles()
	if err != nil {
		m.err = err.Error()
		return
	}
	m.profiles.list = list
	if m.profiles.cursor >= len(list) {
		m.profiles.cursor = max(0, len(list)-1)
	}
}

func (m *Model) editProfile(p store.Profile) tea.Cmd {
	ps := m.profiles
	ps.editing = true
	ps.draft = p
	ps.focus = 0
	ps.name.SetValue(p.Name)
	ps.prof.SetValue(p.Profession)
	ps.about.SetValue(p.About)
	ps.instr.SetValue(p.Instructions)
	m.status = i18n.T("tab field · ctrl+s save · esc cancel")
	return m.focusProfileField()
}

func (m *Model) focusProfileField() tea.Cmd {
	ps := m.profiles
	ps.name.Blur()
	ps.prof.Blur()
	ps.about.Blur()
	ps.instr.Blur()
	switch ps.focus {
	case 0:
		return ps.name.Focus()
	case 1:
		return ps.prof.Focus()
	case 2:
		return ps.about.Focus()
	default:
		return ps.instr.Focus()
	}
}

func (m Model) updateProfilesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ps := m.profiles
	key := msg.String()
	if ps.editing {
		switch key {
		case "esc":
			ps.editing = false
			m.status = i18n.T("n new · enter edit · * default · d delete · esc back")
			return m, nil
		case "tab":
			ps.focus = (ps.focus + 1) % profileFields
			return m, m.focusProfileField()
		case "shift+tab":
			ps.focus = (ps.focus - 1 + profileFields) % profileFields
			return m, m.focusProfileField()
		case "ctrl+s":
			return m.saveProfile()
		case "enter":
			if ps.focus < 2 {
				return m.saveProfile()
			}
		}
		var cmd tea.Cmd
		switch ps.focus {
		case 0:
			ps.name, cmd = ps.name.Update(msg)
		case 1:
			ps.prof, cmd = ps.prof.Update(msg)
		case 2:
			ps.about, cmd = ps.about.Update(msg)
		default:
			ps.instr, cmd = ps.instr.Update(msg)
		}
		return m, cmd
	}
	if key != "d" {
		ps.confirm = 0
	}
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		from := ps.from
		m.profiles = nil
		if from == screenSettings {
			m.openSettings(screenList)
			return m, nil
		}
		m.screen = screenList
		m.status = listStatus()
	case "j", "down":
		if ps.cursor < len(ps.list)-1 {
			ps.cursor++
		}
	case "k", "up":
		if ps.cursor > 0 {
			ps.cursor--
		}
	case "n", "a":
		return m, m.editProfile(store.Profile{})
	case "enter", "e":
		if len(ps.list) > 0 {
			return m, m.editProfile(ps.list[ps.cursor])
		}
		return m, m.editProfile(store.Profile{})
	case "*", " ":
		if len(ps.list) == 0 {
			return m, nil
		}
		id := ps.list[ps.cursor].ID
		if m.cfg.DefaultProfile == id {
			id = 0
		}
		m.cfg.DefaultProfile = id
		if id == 0 {
			_ = config.Unset(m.configPath, "default_profile")
		}
		if err := m.saveConfig(); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.status = i18n.T("Default profile: ") + m.profileLabel(0)
	case "d":
		if len(ps.list) == 0 {
			return m, nil
		}
		p := ps.list[ps.cursor]
		if ps.confirm != p.ID {
			ps.confirm = p.ID
			m.status = i18n.T("Press d again to delete ") + p.Label()
			return m, nil
		}
		ps.confirm = 0
		if err := m.store.DeleteProfile(p.ID); err != nil {
			m.err = err.Error()
			return m, nil
		}
		if m.cfg.DefaultProfile == p.ID {
			m.cfg.DefaultProfile = 0
			_ = config.Unset(m.configPath, "default_profile")
		}
		m.reloadProfiles()
		m.status = i18n.T("Deleted: ") + p.Label()
	}
	return m, nil
}

func (m Model) saveProfile() (tea.Model, tea.Cmd) {
	ps := m.profiles
	p := ps.draft
	p.Name = ps.name.Value()
	p.Profession = ps.prof.Value()
	p.About = ps.about.Value()
	p.Instructions = ps.instr.Value()
	var err error
	if p.ID == 0 {
		p, err = m.store.CreateProfile(p)
		if err == nil && m.cfg.DefaultProfile == 0 {
			// The first profile becomes the default.
			m.cfg.DefaultProfile = p.ID
			_ = m.saveConfig()
		}
	} else {
		err = m.store.UpdateProfile(p)
	}
	if err != nil {
		m.err = i18n.T(err.Error())
		return m, nil
	}
	m.err = ""
	ps.editing = false
	m.reloadProfiles()
	for i, q := range ps.list {
		if q.ID == p.ID {
			ps.cursor = i
		}
	}
	m.status = i18n.T("Saved: ") + p.Label()
	return m, nil
}

func (m Model) viewProfiles() string {
	th := m.theme
	ps := m.profiles
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Profiles")) + th.Muted.Render("  ·  "+i18n.T("who the agents work for")) + "\n\n")
	if ps.editing {
		title := i18n.T("New profile")
		if ps.draft.ID != 0 {
			title = i18n.T("Edit profile")
		}
		b.WriteString(th.Label.Render(title) + "\n\n")
		rows := []struct {
			label string
			view  string
		}{
			{i18n.T("Name"), ps.name.View()},
			{i18n.T("Profession"), ps.prof.View()},
			{i18n.T("About"), "\n" + ps.about.View()},
			{i18n.T("Instructions"), "\n" + ps.instr.View()},
		}
		for i, r := range rows {
			marker := "  "
			if i == ps.focus {
				marker = th.Border.Render("> ")
			}
			b.WriteString(marker + th.Label.Render(r.label) + " " + r.view + "\n")
		}
		b.WriteString("\n" + th.Muted.Render(i18n.T("tab field · ctrl+s save · esc cancel")))
		return th.App.Render(b.String())
	}
	if len(ps.list) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("No profiles yet. Press n to say who you are: the agents get it in every task.")) + "\n")
	}
	for i, p := range ps.list {
		mark := "  "
		if p.ID == m.cfg.DefaultProfile {
			mark = "★ "
		}
		line := mark + p.Label()
		if p.About != "" {
			line += th.Muted.Render("  ·  " + firstLine(p.About))
		}
		line = truncate(line, max(10, m.width-4))
		if i == ps.cursor {
			b.WriteString(th.Selected.Render(line) + "\n")
		} else {
			b.WriteString(th.Normal.Render(line) + "\n")
		}
	}
	b.WriteString("\n" + th.Muted.Render(i18n.T("n new · enter edit · * default · d delete · esc back")))
	return th.App.Render(b.String())
}

// profileLabel names a profile id; 0 means the default profile.
func (m Model) profileLabel(id int64) string {
	if id == 0 {
		if m.cfg.DefaultProfile == 0 {
			return i18n.T("(none)")
		}
		if p, err := m.store.GetProfile(m.cfg.DefaultProfile); err == nil {
			return i18n.T("default") + " · " + p.Label()
		}
		return i18n.T("(none)")
	}
	if p, err := m.store.GetProfile(id); err == nil {
		return p.Label()
	}
	return fmt.Sprintf("#%d", id)
}

func (m Model) accountLabel(name string) string {
	if name == "" {
		if m.cfg.DefaultAccount != "" {
			return i18n.T("default") + " · " + m.cfg.DefaultAccount
		}
		return i18n.T("default login")
	}
	if _, ok := m.cfg.Launch.Accounts[name]; !ok {
		return name + " " + i18n.T("(not in config.toml)")
	}
	return name
}

// cycleFormChoice changes the client, account or profile of the project form.
func (m *Model) cycleFormChoice(dir int) {
	switch m.focus {
	case fieldClient:
		m.cycleFormClient(dir)
	case fieldAccount:
		opts := append([]string{""}, m.cfg.Launch.AccountNames()...)
		m.pending.Account = cycleValue(m.pending.Account, opts, dir)
	case fieldProfile:
		ids := []int64{0}
		if list, err := m.store.ListProfiles(); err == nil {
			for _, p := range list {
				ids = append(ids, p.ID)
			}
		}
		i := 0
		for k, id := range ids {
			if id == m.pending.ProfileID {
				i = k
			}
		}
		n := len(ids)
		m.pending.ProfileID = ids[((i+dir)%n+n)%n]
	}
}

// projectProfile is the profile the agents of a project work for, or nil.
func (m Model) projectProfile(p store.Project) *store.Profile {
	id := p.ProfileID
	if id == 0 {
		id = m.cfg.DefaultProfile
	}
	if id == 0 {
		return nil
	}
	prof, err := m.store.GetProfile(id)
	if err != nil {
		return nil
	}
	return &prof
}

// projectAccount is the account a project runs its agents with.
func (m Model) projectAccount(p store.Project) string {
	if p.Account != "" {
		return p.Account
	}
	return m.cfg.DefaultAccount
}

// launchCfg is the launch config of a project: its agents use the project's
// account.
func (m Model) launchCfg(p store.Project) (config.Launch, error) {
	return m.cfg.Launch.ForAccount(m.configPath, m.projectAccount(p))
}

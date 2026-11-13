package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/theme"
)

type Theme struct {
	Name string

	App       lipgloss.Style
	Title     lipgloss.Style
	Muted     lipgloss.Style
	Selected  lipgloss.Style
	Normal    lipgloss.Style
	Archived  lipgloss.Style
	StatusBar lipgloss.Style
	Help      lipgloss.Style
	Error     lipgloss.Style
	Border    lipgloss.Style
	Label     lipgloss.Style
	DiffAdd   lipgloss.Style
	DiffDel   lipgloss.Style
	DiffHunk  lipgloss.Style

	palette theme.Palette
}

func loadThemes(userDir string) []Theme {
	palettes := theme.Load(userDir)
	out := make([]Theme, len(palettes))
	for i, p := range palettes {
		out[i] = themeFromPalette(p)
	}
	return out
}

func parseTheme(b []byte) (Theme, error) {
	p, err := theme.Parse(b)
	if err != nil {
		return Theme{}, err
	}
	return themeFromPalette(p), nil
}

func themeFromPalette(p theme.Palette) Theme {
	fg := lipgloss.Color(p.Colors.FG)
	bg := lipgloss.Color(p.Colors.BG)
	accent := lipgloss.Color(p.Colors.Accent)
	muted := lipgloss.Color(p.Colors.Muted)
	sel := lipgloss.Color(p.Colors.Selected)
	status := lipgloss.Color(p.Colors.Status)
	errc := lipgloss.Color(p.Colors.Error)
	return Theme{
		Name:      p.Name,
		App:       lipgloss.NewStyle().Foreground(fg).Background(bg),
		Title:     lipgloss.NewStyle().Bold(true).Foreground(accent).Background(bg),
		Muted:     lipgloss.NewStyle().Foreground(muted).Background(bg),
		Selected:  lipgloss.NewStyle().Foreground(fg).Background(sel).Bold(true).Padding(0, 1),
		Normal:    lipgloss.NewStyle().Foreground(fg).Background(bg).Padding(0, 1),
		Archived:  lipgloss.NewStyle().Foreground(muted).Background(bg).Faint(true).Padding(0, 1),
		StatusBar: lipgloss.NewStyle().Foreground(fg).Background(status).Padding(0, 1),
		Help:      lipgloss.NewStyle().Foreground(fg).Background(bg).Padding(1, 2),
		Error:     lipgloss.NewStyle().Foreground(errc).Background(bg),
		Border:    lipgloss.NewStyle().Foreground(accent),
		Label:     lipgloss.NewStyle().Foreground(muted).Width(14),
		DiffAdd:   lipgloss.NewStyle().Foreground(lipgloss.Color(p.Colors.DiffAdd)).Background(bg),
		DiffDel:   lipgloss.NewStyle().Foreground(lipgloss.Color(p.Colors.DiffDel)).Background(bg),
		DiffHunk:  lipgloss.NewStyle().Foreground(lipgloss.Color(p.Colors.DiffHunk)).Background(bg),
		palette:   p,
	}
}

func defaultDarkPalette() theme.Palette {
	return theme.DefaultDark()
}

func themeByName(themes []Theme, name string) Theme {
	name = strings.TrimSpace(name)
	for _, t := range themes {
		if t.Name == name {
			return t
		}
	}
	if len(themes) > 0 {
		return themes[0]
	}
	return themeFromPalette(defaultDarkPalette())
}

func encodeTheme(t Theme) ([]byte, error) {
	return theme.Encode(t.palette)
}

func (m *Model) applyTheme(name string) {
	m.theme = themeByName(m.themes, name)
	m.cfg.Theme = m.theme.Name
	if err := config.Save(m.configPath, m.cfg); err != nil {
		m.err = i18n.T("Could not save the theme: ") + err.Error()
		return
	}
	m.status = i18n.T("Theme: ") + m.theme.Name
}

func (m *Model) cycleTheme() {
	if len(m.themes) == 0 {
		return
	}
	idx := 0
	for i, t := range m.themes {
		if t.Name == m.theme.Name {
			idx = (i + 1) % len(m.themes)
			break
		}
	}
	m.applyTheme(m.themes[idx].Name)
}

func (m *Model) createUserTheme() {
	dir := m.themeDir
	if dir == "" {
		m.err = i18n.T("No themes folder")
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.err = err.Error()
		return
	}
	name := "custom"
	path := filepath.Join(dir, name+".toml")
	for i := 2; fileExists(path); i++ {
		name = fmt.Sprintf("custom-%d", i)
		path = filepath.Join(dir, name+".toml")
	}
	p := m.theme.palette
	p.Name = name
	t := themeFromPalette(p)
	b, err := encodeTheme(t)
	if err != nil {
		m.err = err.Error()
		return
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		m.err = err.Error()
		return
	}
	m.themes = loadThemes(dir)
	m.applyTheme(name)
	m.status = i18n.T("Custom theme: ") + path
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsLight reports whether the theme has a light background, so Markdown is
// rendered with dark text.
func (t Theme) IsLight() bool {
	return t.palette.IsLight()
}

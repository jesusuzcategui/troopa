package tui

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
)

//go:embed themes/*.toml
var bundledThemes embed.FS

type paletteFile struct {
	Name   string        `toml:"name"`
	Colors paletteColors `toml:"colors"`
}

type paletteColors struct {
	FG       string `toml:"fg"`
	BG       string `toml:"bg"`
	Accent   string `toml:"accent"`
	Muted    string `toml:"muted"`
	Selected string `toml:"selected"`
	Status   string `toml:"status"`
	Error    string `toml:"error"`
	DiffAdd  string `toml:"diff_add"`
	DiffDel  string `toml:"diff_del"`
	DiffHunk string `toml:"diff_hunk"`
}

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

	palette paletteFile
}

func loadThemes(userDir string) []Theme {
	byName := map[string]Theme{}
	order := []string{}
	add := func(t Theme) {
		if t.Name == "" {
			return
		}
		if _, ok := byName[t.Name]; !ok {
			order = append(order, t.Name)
		}
		byName[t.Name] = t
	}
	entries, err := fs.ReadDir(bundledThemes, "themes")
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
				continue
			}
			b, err := bundledThemes.ReadFile("themes/" + e.Name())
			if err != nil {
				continue
			}
			if t, err := parseTheme(b); err == nil {
				add(t)
			}
		}
	}
	if userDir != "" {
		ents, err := os.ReadDir(userDir)
		if err == nil {
			var extra []string
			for _, e := range ents {
				if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".toml") {
					continue
				}
				b, err := os.ReadFile(filepath.Join(userDir, e.Name()))
				if err != nil {
					continue
				}
				t, err := parseTheme(b)
				if err != nil {
					continue
				}
				if t.Name == "" {
					t.Name = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				}
				if _, exists := byName[t.Name]; !exists {
					extra = append(extra, t.Name)
				}
				byName[t.Name] = t
			}
			sort.Strings(extra)
			order = append(order, extra...)
		}
	}
	out := make([]Theme, 0, len(order))
	for _, name := range order {
		if t, ok := byName[name]; ok {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		out = append(out, themeFromPalette(defaultDarkPalette()))
	}
	return out
}

func parseTheme(b []byte) (Theme, error) {
	var pf paletteFile
	if err := toml.Unmarshal(b, &pf); err != nil {
		return Theme{}, err
	}
	if pf.Name == "" {
		return Theme{}, fmt.Errorf("theme missing name")
	}
	if err := pf.Colors.validate(); err != nil {
		return Theme{}, err
	}
	return themeFromPalette(pf), nil
}

func (c paletteColors) validate() error {
	fields := []string{c.FG, c.BG, c.Accent, c.Muted, c.Selected, c.Status, c.Error, c.DiffAdd, c.DiffDel, c.DiffHunk}
	for _, f := range fields {
		if strings.TrimSpace(f) == "" {
			return fmt.Errorf("theme colors incomplete")
		}
	}
	return nil
}

func themeFromPalette(p paletteFile) Theme {
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

func defaultDarkPalette() paletteFile {
	return paletteFile{
		Name: "dark",
		Colors: paletteColors{
			FG: "#E8E4D9", BG: "#1A1916", Accent: "#E8A838", Muted: "#8A8374",
			Selected: "#2A2823", Status: "#25231F", Error: "#E06C75",
			DiffAdd: "#7A9E5A", DiffDel: "#E06C75", DiffHunk: "#6A9BCF",
		},
	}
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
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(t.palette); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

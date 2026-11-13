// Package theme loads and parses Troopa's bundled and user-defined TOML color palettes.
package theme

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

//go:embed themes/*.toml
var bundledThemes embed.FS

// Colors holds the ten palette colors defined in a theme TOML file.
type Colors struct {
	FG       string `toml:"fg" json:"fg"`
	BG       string `toml:"bg" json:"bg"`
	Accent   string `toml:"accent" json:"accent"`
	Muted    string `toml:"muted" json:"muted"`
	Selected string `toml:"selected" json:"selected"`
	Status   string `toml:"status" json:"status"`
	Error    string `toml:"error" json:"error"`
	DiffAdd  string `toml:"diff_add" json:"diff_add"`
	DiffDel  string `toml:"diff_del" json:"diff_del"`
	DiffHunk string `toml:"diff_hunk" json:"diff_hunk"`
}

// Palette represents a named color palette loaded from a bundled or user TOML file.
type Palette struct {
	Name   string `toml:"name" json:"name"`
	Light  bool   `toml:"-" json:"light"`
	Colors Colors `toml:"colors" json:"colors"`
}

// Load returns all bundled themes plus any valid .toml themes in userDir.
// User themes with the same name as a bundled theme override it in place; new
// user themes are appended sorted alphabetically by name.
func Load(userDir string) []Palette {
	byName := map[string]Palette{}
	order := []string{}
	add := func(p Palette) {
		if p.Name == "" {
			return
		}
		if _, ok := byName[p.Name]; !ok {
			order = append(order, p.Name)
		}
		byName[p.Name] = p
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
			if p, err := Parse(b); err == nil {
				add(p)
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
				p, err := Parse(b)
				if err != nil {
					continue
				}
				if p.Name == "" {
					p.Name = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				}
				if _, exists := byName[p.Name]; !exists {
					extra = append(extra, p.Name)
				}
				byName[p.Name] = p
			}
			sort.Strings(extra)
			order = append(order, extra...)
		}
	}
	out := make([]Palette, 0, len(order))
	for _, name := range order {
		if p, ok := byName[name]; ok {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = append(out, DefaultDark())
	}
	return out
}

// Parse decodes and validates a TOML theme definition.
func Parse(b []byte) (Palette, error) {
	var pf Palette
	if err := toml.Unmarshal(b, &pf); err != nil {
		return Palette{}, err
	}
	if pf.Name == "" {
		return Palette{}, fmt.Errorf("theme missing name")
	}
	if err := pf.Colors.Validate(); err != nil {
		return Palette{}, err
	}
	pf.Light = pf.IsLight()
	return pf, nil
}

// Validate checks that every required color field is present.
func (c Colors) Validate() error {
	fields := []string{c.FG, c.BG, c.Accent, c.Muted, c.Selected, c.Status, c.Error, c.DiffAdd, c.DiffDel, c.DiffHunk}
	for _, f := range fields {
		if strings.TrimSpace(f) == "" {
			return fmt.Errorf("theme colors incomplete")
		}
	}
	return nil
}

// DefaultDark returns the fallback dark palette.
func DefaultDark() Palette {
	return Palette{
		Name:  "dark",
		Light: false,
		Colors: Colors{
			FG: "#E8E4D9", BG: "#1A1916", Accent: "#E8A838", Muted: "#8A8374",
			Selected: "#2A2823", Status: "#25231F", Error: "#E06C75",
			DiffAdd: "#7A9E5A", DiffDel: "#E06C75", DiffHunk: "#6A9BCF",
		},
	}
}

// ByName returns the palette matching name, or the first palette if not found.
func ByName(palettes []Palette, name string) Palette {
	name = strings.TrimSpace(name)
	for _, p := range palettes {
		if p.Name == name {
			return p
		}
	}
	if len(palettes) > 0 {
		return palettes[0]
	}
	return DefaultDark()
}

// Encode serializes p as TOML.
func Encode(p Palette) ([]byte, error) {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// IsLight reports whether the palette has a light background.
func (p Palette) IsLight() bool {
	h := strings.TrimPrefix(p.Colors.BG, "#")
	if len(h) != 6 {
		return p.Name == "light"
	}
	var r, g, b int
	if _, err := fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b); err != nil {
		return p.Name == "light"
	}
	return 0.299*float64(r)+0.587*float64(g)+0.114*float64(b) > 140
}

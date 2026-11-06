package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/i18n"
)

// The troop: three robot soldiers, the one in the middle leads. Each robot is
// seven cells wide; the wordmark sits to the right.
var (
	robotRows = []string{"   ▲   ", " [◉_◉] ", " /|█|\\ ", "  / \\  "}
	wordmark  = []string{"▀█▀ █▀█ █▀█ █▀█ █▀█ ▄▀█", " █  █▀▄ █▄█ █▄█ █▀▀ █▀█"}
)

const logoGap = "    "

// renderLogo draws the troop and the wordmark in the theme's colors. Narrow
// screens get the wordmark only.
func (m Model) renderLogo(width int) string {
	th := m.theme
	accent := lipgloss.NewStyle().Foreground(th.Title.GetForeground()).Bold(true)
	body := lipgloss.NewStyle().Foreground(th.Normal.GetForeground())
	dim := lipgloss.NewStyle().Foreground(th.Muted.GetForeground())
	tagline := dim.Render(i18n.T("your AI agent troop in the terminal"))

	if width > 0 && width < 62 {
		return accent.Render(wordmark[0]) + "\n" + accent.Render(wordmark[1]) + "\n\n" + tagline
	}
	var out []string
	for row, part := range robotRows {
		var line strings.Builder
		for r := 0; r < 3; r++ {
			switch {
			case r == 1:
				line.WriteString(accent.Render(part))
			case row == 0:
				line.WriteString(accent.Render(part))
			case row == 1:
				line.WriteString(body.Render(part))
			default:
				line.WriteString(dim.Render(part))
			}
		}
		line.WriteString(logoGap)
		// The wordmark sits on the two middle rows, centered on the robots.
		switch row {
		case 1, 2:
			line.WriteString(accent.Render(wordmark[row-1]))
		case 3:
			line.WriteString(tagline)
		}
		out = append(out, line.String())
	}
	return strings.Join(out, "\n")
}

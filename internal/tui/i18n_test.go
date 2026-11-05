package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
)

// Tests run in English regardless of the machine's LANG.
func TestMain(m *testing.M) {
	i18n.SetLang(i18n.EN)
	os.Exit(m.Run())
}

func TestSpanishInterface(t *testing.T) {
	i18n.SetLang(i18n.ES)
	defer i18n.SetLang(i18n.EN)
	m := newTestModel(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	got := next.(Model)
	if !strings.Contains(got.viewPalette(), "Paleta de comandos") {
		t.Fatalf("palette not in Spanish:\n%s", got.viewPalette())
	}
	got.paletteInput.SetValue("ayuda")
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if got.screen != screenHelp {
		t.Fatalf("screen %v want help", got.screen)
	}
	if !strings.Contains(got.viewHelp(), "Atajos") {
		t.Fatalf("help not in Spanish:\n%s", got.viewHelp())
	}
}

func TestEnglishInterface(t *testing.T) {
	m := newTestModel(t)
	if !strings.Contains(m.viewHelp(), "Shortcuts") {
		t.Fatalf("help not in English:\n%s", m.viewHelp())
	}
	if strings.Contains(listStatus(), "paleta") {
		t.Fatalf("status bar not in English: %s", listStatus())
	}
}

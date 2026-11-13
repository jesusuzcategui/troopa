package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Every plain letter typed on the request-changes screen must land in the
// text box; the image shortcuts live on ctrl/alt.
func TestChangesInputTakesPlainLetters(t *testing.T) {
	m := newTestModel(t)
	m.changesInput = textarea.New()
	m.changesInput.Focus()
	m.screen = screenChanges
	for _, r := range "pDJK pide" {
		next, _ := m.updateChangesKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	if got := m.changesInput.Value(); got != "pDJK pide" {
		t.Fatalf("text box got %q", got)
	}
	if m.changesPathOpen {
		t.Fatal("p must not open the image path prompt")
	}
}

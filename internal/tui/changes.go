package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/clipboard"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
	"github.com/jesusuzcategui/troopa/internal/termimg"
)

// canRequestChanges reports whether the run on screen can go back to its agent.
func canRequestChanges(state string) bool {
	switch state {
	case store.RunReview, store.RunBlocked, store.RunApproved:
		return true
	}
	return false
}

func (m Model) requestChangesFromTask() (tea.Model, tea.Cmd) {
	t, ok := m.selectedTask()
	if !ok {
		m.status = i18n.T("No task selected")
		return m, nil
	}
	run, err := m.store.LatestRun(t.ID)
	if err != nil || !canRequestChanges(run.State) {
		m.err = ""
		m.status = i18n.T("This task has no run to send back to its agent")
		return m, nil
	}
	m.bindRun(t, run)
	return m.openChanges(screenBoard)
}

func (m Model) openChanges(from screen) (tea.Model, tea.Cmd) {
	if !canRequestChanges(m.run.State) {
		m.status = i18n.T("This task has no run to send back to its agent")
		return m, nil
	}
	if m.runTask.Agent == store.AgentMe || m.runTask.Agent == "" {
		m.status = i18n.T("This task has no agent: make the changes yourself with o")
		return m, nil
	}
	if !dirExists(m.runSess.Worktree) {
		m.err = i18n.T("The worktree is gone: discard the run and launch the task again")
		return m, nil
	}
	in := textarea.New()
	in.Placeholder = i18n.T("What should change? For example: add requirements.txt and explain how to create the venv")
	in.SetWidth(max(30, min(80, m.width-6)))
	in.SetHeight(6)
	in.ShowLineNumbers = false
	in.Prompt = ""
	in.Focus()
	m.changesInput = in
	m.changesFrom = from
	m.changesImages = nil
	m.changesImgCursor = 0
	m.changesPathOpen = false
	m.sumOffset = 0
	m.loadRounds()
	m.loadComments()
	m.screen = screenChanges
	m.err = ""
	m.status = changesHelpLine()
	return m, textarea.Blink
}

func changesHelpLine() string {
	return i18n.T("ctrl+s send · p image path · ctrl+v clipboard · J/K images · D remove · esc cancel")
}

func (m Model) updateChangesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.changesPathOpen {
			m.changesPathOpen = false
			m.changesPath.Blur()
			return m, nil
		}
		m.screen = m.changesFrom
		m.changesImages = nil
		m.status = ""
		return m, nil
	case "ctrl+s":
		return m.sendChanges(m.changesInput.Value())
	case "pgdown", "ctrl+d":
		m.sumOffset = min(m.sumOffset+max(1, m.changesAnswerHeight()-1), max(0, len(m.lastAnswerLines())-m.changesAnswerHeight()))
		return m, nil
	case "pgup", "ctrl+u":
		m.sumOffset = max(0, m.sumOffset-max(1, m.changesAnswerHeight()-1))
		return m, nil
	case "p":
		return m.openChangesAttachPath()
	case "ctrl+v":
		return m.changesPasteClipboard()
	case "J":
		m.moveChangesImgCursor(1)
		return m, nil
	case "K":
		m.moveChangesImgCursor(-1)
		return m, nil
	case "D":
		return m.removeChangesImage()
	}
	var cmd tea.Cmd
	m.changesInput, cmd = m.changesInput.Update(msg)
	return m, cmd
}

func (m Model) openChangesAttachPath() (tea.Model, tea.Cmd) {
	m.changesPath.SetValue("")
	m.changesPath.Placeholder = i18n.T("Path to an image")
	m.changesPath.Focus()
	m.changesPathOpen = true
	m.status = i18n.T("Paste a path and press enter")
	return m, textinput.Blink
}

func (m Model) updateChangesAttachPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.changesPathOpen = false
		m.changesPath.Blur()
		m.status = changesHelpLine()
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.changesPath.Value())
		m.changesPathOpen = false
		m.changesPath.Blur()
		return m.changesAddImage(path)
	}
	var cmd tea.Cmd
	m.changesPath, cmd = m.changesPath.Update(msg)
	return m, cmd
}

func (m Model) viewChangesAttachPathOverlay() string {
	th := m.theme
	return th.Title.Render(i18n.T("Attach an image")) + "\n\n" + m.changesPath.View() + "\n\n" + th.Muted.Render(i18n.T("enter attach  esc cancel"))
}

func (m Model) changesAddImage(path string) (Model, tea.Cmd) {
	path = strings.TrimSpace(path)
	if path == "" {
		m.err = i18n.T("File is required")
		return m, nil
	}
	if !termimg.IsImagePath(path) {
		m.err = i18n.T("Not an image: %s", path)
		return m, nil
	}
	if st, err := os.Stat(path); err != nil {
		m.err = i18n.T("File is required")
		if !os.IsNotExist(err) {
			m.err = err.Error()
		}
		return m, nil
	} else if max := m.cfg.MaxAttachmentBytes(); max > 0 && st.Size() > max {
		m.err = fmt.Sprintf("file is too large: %s (limit %s)", store.FormatSize(st.Size()), store.FormatSize(max))
		return m, nil
	}
	m.changesImages = append(m.changesImages, path)
	m.changesImgCursor = len(m.changesImages) - 1
	m.status = i18n.T("Image for this round: ") + filepath.Base(path)
	m.err = ""
	return m, nil
}

func (m Model) changesPasteClipboard() (Model, tea.Cmd) {
	path, err := clipboard.ImageFromClipboard()
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	return m.changesAddImage(path)
}

func (m *Model) moveChangesImgCursor(delta int) {
	n := len(m.changesImages)
	if n == 0 {
		return
	}
	m.changesImgCursor = (m.changesImgCursor + delta + n) % n
}

func (m Model) removeChangesImage() (Model, tea.Cmd) {
	if len(m.changesImages) == 0 {
		m.status = i18n.T("No image selected")
		return m, nil
	}
	i := m.changesImgCursor
	if i < 0 || i >= len(m.changesImages) {
		i = 0
	}
	m.changesImages = append(m.changesImages[:i], m.changesImages[i+1:]...)
	if m.changesImgCursor >= len(m.changesImages) && m.changesImgCursor > 0 {
		m.changesImgCursor--
	}
	m.status = i18n.T("Image removed")
	return m, nil
}

func (m Model) renderChangesImages() string {
	if len(m.changesImages) == 0 {
		return ""
	}
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Images for this round")) + "\n")
	for i, p := range m.changesImages {
		mark := "  "
		if i == m.changesImgCursor {
			mark = th.Border.Render("> ")
		}
		b.WriteString(mark + filepath.Base(p) + th.Muted.Render(" · "+p) + "\n")
	}
	return b.String()
}

// sendChanges writes the feedback to CHANGES.md and starts the same agent
// again in the same worktree, keeping what it already did.
func (m Model) sendChanges(feedback string) (tea.Model, tea.Cmd) {
	m.loadComments()
	imgs := append([]string(nil), m.changesImages...)
	res, err := m.eng().RequestChanges(m.run.ID, feedback, m.pendingComments(), imgs)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.changesImages = nil
	m.loadComments()
	if res.Pair {
		m.run = res.Run
		m.runTask = res.Task
		m.runSess = res.Session
		m.status = res.Message
		m.err = ""
		return m.startPair(res.Task)
	}
	if res.Watch {
		return m.applyEngineLaunch(res)
	}
	return m.applyEngineFinish(res, true)
}

func (m Model) viewChanges() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Request changes")))
	b.WriteString(th.Muted.Render("  ·  " + truncate(m.runTask.Title, 40) + "  ·  " + launch.AgentName(m.runTask.Agent)))
	b.WriteString("\n\n")
	if r := launch.ChangeRounds(m.runSess.Worktree); r > 0 {
		b.WriteString(th.Muted.Render(fmt.Sprintf(i18n.T("Rounds so far: %d (in CHANGES.md)"), r)) + "\n\n")
	}
	b.WriteString(th.Muted.Render(i18n.T("The agent keeps its work in the same worktree and applies what you write here.")) + "\n\n")
	if lines := m.lastAnswerLines(); len(lines) > 0 {
		b.WriteString(th.Title.Render(i18n.T("Last answer from the agent")) + th.Muted.Render("  ·  "+i18n.T("PgUp/PgDn scroll")) + "\n")
		b.WriteString(m.window(lines, m.sumOffset, m.changesAnswerHeight()) + "\n\n")
	}
	if pending := m.pendingComments(); len(pending) > 0 {
		b.WriteString(th.Title.Render(i18n.T("Pending comments")) + th.Muted.Render(fmt.Sprintf("  ·  %d", len(pending))) + "\n")
		prev := ""
		for _, c := range pending {
			if c.Path != prev {
				b.WriteString(th.Muted.Render(c.Path) + "\n")
				prev = c.Path
			}
			b.WriteString(th.Normal.UnsetPadding().Render(fmt.Sprintf("  %s:%d  %s", c.Path, c.Line, firstLine(c.Body))) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(th.Muted.Render(i18n.T("General notes are optional when there are line comments.")) + "\n\n")
	}
	if imgBlock := m.renderChangesImages(); imgBlock != "" {
		b.WriteString(imgBlock + "\n\n")
	}
	b.WriteString(m.changesInput.View())
	b.WriteString("\n\n")
	b.WriteString(th.Muted.Render(changesHelpLine()))
	return th.App.Render(b.String())
}

// lastAnswerLines is the latest round's summary, wrapped to the screen.
func (m Model) lastAnswerLines() []string {
	for i := len(m.rounds) - 1; i >= 0; i-- {
		if sum := strings.TrimSpace(m.rounds[i].Summary); sum != "" {
			return strings.Split(m.renderMarkdownW(sum, max(20, m.width-4)), "\n")
		}
	}
	return nil
}

// changesAnswerHeight leaves room for the title, notes, comments, the input
// box and the help line below the agent's answer.
func (m Model) changesAnswerHeight() int {
	used := 18 + len(m.pendingComments())*2 + len(m.changesImages)*2
	return max(4, m.height-used)
}

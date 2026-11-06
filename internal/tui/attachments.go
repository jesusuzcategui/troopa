package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func (m *Model) loadAttachments(taskID int64) {
	m.attachments = nil
	m.attachCursor = 0
	if taskID == 0 {
		return
	}
	items, err := m.store.ListAttachments(taskID)
	if err != nil {
		return
	}
	m.attachments = items
}

func (m Model) stageTask(worktree string, t store.Task) error {
	atts, err := m.store.ListAttachments(t.ID)
	if err != nil {
		return err
	}
	return launch.StageTaskContext(worktree, t, atts, m.cfg.Launch.Brain, m.boardProject.Slug)
}

func (m Model) attachFromPath(src string) (Model, tea.Cmd) {
	src = strings.TrimSpace(src)
	if src == "" {
		m.err = i18n.T("File is required")
		return m, nil
	}
	if t := m.taskDraft; t.ID == 0 && m.screen != screenBoard {
		if st, err := os.Stat(src); err != nil {
			m.err = i18n.T("File is required")
			if !os.IsNotExist(err) {
				m.err = err.Error()
			}
			return m, nil
		} else if max := m.cfg.MaxAttachmentBytes(); max > 0 && st.Size() > max {
			m.err = fmt.Sprintf("file is too large: %s (limit %s)", store.FormatSize(st.Size()), store.FormatSize(max))
			return m, nil
		}
		m.pendingFiles = append(m.pendingFiles, src)
		m.status = i18n.T("Will attach on save: ") + filepath.Base(src)
		m.err = ""
		return m, nil
	}
	id := m.taskDraft.ID
	if id == 0 {
		if t, ok := m.selectedTask(); ok {
			id = t.ID
		}
	}
	if id == 0 {
		m.err = i18n.T("Save the task before attaching files")
		return m, nil
	}
	dir, err := config.AttachmentsDir(id)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	a, err := m.store.AttachFile(id, src, dir, m.cfg.MaxAttachmentBytes())
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.loadAttachments(id)
	m.status = i18n.T("Attached: ") + a.Name + " · " + store.FormatSize(a.Size)
	m.err = ""
	return m, nil
}

func (m Model) flushPendingFiles(taskID int64) (Model, error) {
	pending := m.pendingFiles
	m.pendingFiles = nil
	for _, src := range pending {
		dir, err := config.AttachmentsDir(taskID)
		if err != nil {
			return m, err
		}
		if _, err := m.store.AttachFile(taskID, src, dir, m.cfg.MaxAttachmentBytes()); err != nil {
			return m, err
		}
	}
	m.loadAttachments(taskID)
	return m, nil
}

func (m Model) openFilePicker() (tea.Model, tea.Cmd) {
	fp := filepicker.New()
	fp.AutoHeight = false
	fp.Height = max(8, m.height-6)
	start := m.boardProject.RepoPath
	if start == "" {
		start, _ = os.UserHomeDir()
	}
	if st, err := os.Stat(start); err != nil || !st.IsDir() {
		start, _ = os.UserHomeDir()
	}
	if start == "" {
		start = "."
	}
	fp.CurrentDirectory = start
	m.picker = fp
	m.pickerFrom = m.screen
	if m.pickerFrom == screenFilePicker {
		m.pickerFrom = screenTaskForm
	}
	m.screen = screenFilePicker
	m.status = i18n.T("enter select  h back  esc cancel")
	return m, m.picker.Init()
}

func (m Model) updateFilePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "esc" || key.String() == "q") {
		m.screen = m.pickerFrom
		m.status = ""
		return m, nil
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	if ok, path := m.picker.DidSelectFile(msg); ok {
		m.screen = m.pickerFrom
		return m.attachFromPath(path)
	}
	return m, cmd
}

func (m Model) viewFilePicker() string {
	th := m.theme
	return th.App.Render(th.Title.Render(i18n.T("Attach a file")) + "\n" + m.picker.CurrentDirectory + "\n\n" + m.picker.View() + "\n" + th.Muted.Render(i18n.T("enter select  h back  esc cancel")))
}

func (m Model) openAttachPath() (tea.Model, tea.Cmd) {
	m.attachPath.SetValue("")
	m.attachPath.Placeholder = i18n.T("Path to a file")
	m.attachPath.Focus()
	m.attachPathOpen = true
	m.status = i18n.T("Paste a path and press enter")
	return m, textinput.Blink
}

func (m Model) updateAttachPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.attachPathOpen = false
		m.attachPath.Blur()
		m.status = ""
		return m, nil
	case "enter":
		path := m.attachPath.Value()
		m.attachPathOpen = false
		m.attachPath.Blur()
		return m.attachFromPath(path)
	}
	var cmd tea.Cmd
	m.attachPath, cmd = m.attachPath.Update(msg)
	return m, cmd
}

func (m Model) viewAttachPathOverlay() string {
	th := m.theme
	return th.Title.Render(i18n.T("Attach a file")) + "\n\n" + m.attachPath.View() + "\n\n" + th.Muted.Render(i18n.T("enter attach  esc cancel"))
}

func (m Model) confirmDetach() (tea.Model, tea.Cmd) {
	var name string
	id := int64(0)
	if m.taskDraft.ID == 0 && m.attachCursor >= 0 && m.attachCursor < len(m.pendingFiles) {
		name = filepath.Base(m.pendingFiles[m.attachCursor])
	} else if m.attachCursor >= 0 && m.attachCursor < len(m.attachments) {
		id = m.attachments[m.attachCursor].ID
		name = m.attachments[m.attachCursor].Name
	} else {
		m.status = i18n.T("No attachment selected")
		return m, nil
	}
	m.popup = &popup{
		kind:    propConfirmDetach,
		title:   i18n.T("Remove attachment «%s»?", name),
		options: []string{"yes", "no"},
		labels:  []string{i18n.T("Yes, remove"), i18n.T("No")},
		cursor:  1,
		taskID:  id,
	}
	return m, nil
}

func (m Model) detachConfirmed(id int64) (tea.Model, tea.Cmd) {
	if m.taskDraft.ID == 0 && id == 0 {
		if m.attachCursor >= 0 && m.attachCursor < len(m.pendingFiles) {
			m.pendingFiles = append(m.pendingFiles[:m.attachCursor], m.pendingFiles[m.attachCursor+1:]...)
			if m.attachCursor >= len(m.pendingFiles) && m.attachCursor > 0 {
				m.attachCursor--
			}
		}
		m.status = i18n.T("Attachment removed")
		return m, nil
	}
	if err := m.store.DeleteAttachment(id); err != nil {
		m.err = err.Error()
		return m, nil
	}
	taskID := m.taskDraft.ID
	if taskID == 0 {
		if t, ok := m.selectedTask(); ok {
			taskID = t.ID
		}
	}
	m.loadAttachments(taskID)
	if m.attachCursor >= len(m.attachments) && m.attachCursor > 0 {
		m.attachCursor--
	}
	m.status = i18n.T("Attachment removed")
	m.err = ""
	return m, nil
}

func (m Model) openSelectedAttachment() (tea.Model, tea.Cmd) {
	var path, mime string
	if m.taskDraft.ID == 0 && m.attachCursor >= 0 && m.attachCursor < len(m.pendingFiles) {
		path = m.pendingFiles[m.attachCursor]
	} else if m.attachCursor >= 0 && m.attachCursor < len(m.attachments) {
		path = m.attachments[m.attachCursor].Path
		mime = m.attachments[m.attachCursor].MIME
	} else {
		m.status = i18n.T("No attachment selected")
		return m, nil
	}
	cmd, err := launch.OpenAttachment(m.cfg.Launch.Editor, path, mime)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.status = i18n.T("Opening: ") + filepath.Base(path)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorClosedMsg{err: err} })
}

func (m Model) renderAttachList() string {
	th := m.theme
	var b strings.Builder
	if m.taskDraft.ID == 0 {
		if len(m.pendingFiles) == 0 {
			b.WriteString(th.Muted.Render(i18n.T("(none yet)")))
			return b.String()
		}
		for i, p := range m.pendingFiles {
			mark := "  "
			if i == m.attachCursor {
				mark = th.Border.Render("> ")
			}
			b.WriteString(mark + filepath.Base(p) + th.Muted.Render(" · "+i18n.T("pending")) + "\n")
		}
		return strings.TrimRight(b.String(), "\n")
	}
	if len(m.attachments) == 0 {
		return th.Muted.Render(i18n.T("(none yet)"))
	}
	for i, a := range m.attachments {
		mark := "  "
		if i == m.attachCursor {
			mark = th.Border.Render("> ")
		}
		kind := a.MIME
		if kind == "" {
			kind = "file"
		}
		b.WriteString(mark + a.Name + th.Muted.Render(fmt.Sprintf(" · %s · %s", kind, store.FormatSize(a.Size))) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) moveAttachCursor(delta int) {
	n := len(m.attachments)
	if m.taskDraft.ID == 0 {
		n = len(m.pendingFiles)
	}
	if n == 0 {
		return
	}
	m.attachCursor = (m.attachCursor + delta + n) % n
}

func mentionFromTextarea(value string, row, col int) (query string, startCol int, ok bool) {
	lines := strings.Split(value, "\n")
	if row < 0 || row >= len(lines) {
		return "", 0, false
	}
	line := []rune(lines[row])
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}
	before := string(line[:col])
	i := strings.LastIndex(before, "@")
	if i < 0 {
		return "", 0, false
	}
	if i > 0 {
		r := []rune(before[:i])
		prev := r[len(r)-1]
		if prev != ' ' && prev != '\t' && prev != '(' && prev != '[' {
			return "", 0, false
		}
	}
	q := before[i+1:]
	if strings.ContainsAny(q, " \t") {
		return "", 0, false
	}
	return q, i, true
}

func (m *Model) refreshMentions() {
	m.mentionHits = nil
	m.mentionQuery = ""
	if m.taskFocus != tfDesc && m.taskFocus != tfCriteria {
		return
	}
	ta := m.taskDesc
	if m.taskFocus == tfCriteria {
		ta = m.taskCrit
	}
	col := ta.LineInfo().StartColumn + ta.LineInfo().ColumnOffset
	q, _, ok := mentionFromTextarea(ta.Value(), ta.Line(), col)
	if !ok {
		return
	}
	m.mentionQuery = q
	m.mentionHits = launch.FilterRepoFiles(m.repoFiles, q)
	if m.mentionCursor >= len(m.mentionHits) {
		m.mentionCursor = 0
	}
}

func (m Model) applyMention() (Model, tea.Cmd) {
	if len(m.mentionHits) == 0 {
		return m, nil
	}
	path := m.mentionHits[m.mentionCursor]
	ta := m.taskDesc
	if m.taskFocus == tfCriteria {
		ta = m.taskCrit
	}
	col := ta.LineInfo().StartColumn + ta.LineInfo().ColumnOffset
	q, start, ok := mentionFromTextarea(ta.Value(), ta.Line(), col)
	if !ok {
		return m, nil
	}
	lines := strings.Split(ta.Value(), "\n")
	row := ta.Line()
	runes := []rune(lines[row])
	end := start + 1 + len([]rune(q))
	if end > len(runes) {
		end = len(runes)
	}
	ins := "@" + path
	lines[row] = string(runes[:start]) + ins + string(runes[end:])
	newVal := strings.Join(lines, "\n")
	ta.SetValue(newVal)
	for ta.Line() > row {
		ta.CursorUp()
	}
	ta.SetCursor(start + len([]rune(ins)))
	if m.taskFocus == tfCriteria {
		m.taskCrit = ta
	} else {
		m.taskDesc = ta
	}
	m.mentionHits = nil
	m.mentionQuery = ""
	return m, nil
}

func (m Model) viewMentions() string {
	if len(m.mentionHits) == 0 {
		return ""
	}
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Muted.Render(i18n.T("Files")) + "\n")
	for i, p := range m.mentionHits {
		mark := "  "
		if i == m.mentionCursor {
			mark = th.Border.Render("> ")
		}
		b.WriteString(mark + p + "\n")
	}
	b.WriteString(th.Muted.Render(i18n.T("tab/enter complete  esc close")))
	return b.String()
}

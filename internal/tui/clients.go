package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/store"
)

type clientField int

const (
	cfName clientField = iota
	cfContact
	cfNotes
	cfCount
)

func newClientInputs() []textinput.Model {
	labels := []string{i18n.T("Name"), i18n.T("Contact"), i18n.T("Notes")}
	out := make([]textinput.Model, cfCount)
	for i := range out {
		ti := textinput.New()
		ti.Placeholder = labels[i]
		ti.CharLimit = 256
		ti.Width = 48
		ti.Prompt = ""
		out[i] = ti
	}
	out[cfNotes].CharLimit = 512
	return out
}

func (m *Model) reloadClients() {
	list, err := m.store.ListClients(m.showArchivedCli)
	if err != nil {
		m.err = err.Error()
		return
	}
	m.clients = list
	if m.clientCursor >= len(m.clients) {
		m.clientCursor = max(0, len(m.clients)-1)
	}
	m.clampClientOffset()
}

func (m *Model) openClients() {
	m.screen = screenClients
	m.err = ""
	m.reloadClients()
	m.status = i18n.T("n new · e edit · a archive · enter board · F filter projects · esc back")
}

func (m Model) updateClientsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b":
		m.screen = screenList
		m.status = listStatus()
		m.reload()
	case "?":
		m.helpFrom = screenClients
		m.screen = screenHelp
	case "t":
		m.cycleTheme()
	case "tab":
		m.showArchivedCli = !m.showArchivedCli
		m.clientCursor = 0
		m.clientOffset = 0
		m.reloadClients()
	case "n":
		m.openClientForm(store.Client{})
	case "e":
		if c, ok := m.selectedClient(); ok {
			m.openClientForm(c)
		}
	case "a":
		if c, ok := m.selectedClient(); ok {
			if c.Archived {
				m.status = i18n.T("Already archived")
				break
			}
			m.pendingClient = c
			m.screen = screenConfirmArchiveClient
		}
	case "enter", "o":
		if c, ok := m.selectedClient(); ok {
			m.filterClientID = c.ID
			m.openClientBoard(c)
		}
	case "F":
		if c, ok := m.selectedClient(); ok {
			m.filterClientID = c.ID
			m.cursor = 0
			m.offset = 0
			m.screen = screenList
			m.reload()
			m.status = i18n.T("Client filter: ") + c.Name
		}
	case "j", "down":
		m.moveClient(1)
	case "k", "up":
		m.moveClient(-1)
	case "pgdown":
		m.moveClient(max(1, m.listHeight))
	case "pgup":
		m.moveClient(-max(1, m.listHeight))
	case "g", "home":
		m.clientCursor = 0
		m.clampClientOffset()
	case "G", "end":
		if len(m.clients) > 0 {
			m.clientCursor = len(m.clients) - 1
			m.clampClientOffset()
		}
	}
	return m, nil
}

func (m *Model) openClientBoard(c store.Client) {
	m.boardProject = store.Project{Name: i18n.T("Client: ") + c.Name}
	m.filterClientID = c.ID
	m.screen = screenBoard
	m.colIndex = 0
	m.cardIndex = 0
	m.search = ""
	m.filterStatus = ""
	m.filterPriority = ""
	m.searchInput.SetValue("")
	m.dragging = false
	m.dragID = 0
	m.err = ""
	m.reloadTasks()
	m.status = i18n.T("Client tasks  C filter  esc projects")
}

func (m Model) updateConfirmArchiveClient(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		if err := m.app.ArchiveClient(m.pendingClient.ID); err != nil {
			m.err = err.Error()
		} else {
			m.status = i18n.T("Archived: ") + m.pendingClient.Name
			m.err = ""
			if m.filterClientID == m.pendingClient.ID {
				m.filterClientID = 0
			}
		}
		m.screen = screenClients
		m.reloadClients()
	case "n", "esc":
		m.screen = screenClients
	}
	return m, nil
}

func (m Model) updateClientFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.screen = screenClients
		m.status = i18n.T("Cancelled")
		return m, nil
	case "?":
		m.helpFrom = screenClientForm
		m.screen = screenHelp
		return m, nil
	case "enter":
		return m.submitClientForm()
	case "tab", "down":
		m.clientFocus = (m.clientFocus + 1) % cfCount
		return m, m.focusClientInput()
	case "shift+tab", "up":
		m.clientFocus = (m.clientFocus - 1 + cfCount) % cfCount
		return m, m.focusClientInput()
	}
	return m.updateClientInputs(msg)
}

func (m Model) submitClientForm() (tea.Model, tea.Cmd) {
	c := store.Client{
		ID:      m.editingClientID,
		Name:    strings.TrimSpace(m.clientInputs[cfName].Value()),
		Contact: strings.TrimSpace(m.clientInputs[cfContact].Value()),
		Notes:   strings.TrimSpace(m.clientInputs[cfNotes].Value()),
	}
	if c.Name == "" {
		m.err = i18n.T("Name is required")
		return m, nil
	}
	var err error
	if c.ID == 0 {
		var created store.Client
		created, err = m.app.CreateClient(c)
		if err == nil {
			m.status = i18n.T("Created: ") + created.Name
		}
	} else {
		err = m.app.UpdateClient(c)
		if err == nil {
			m.status = i18n.T("Saved: ") + c.Name
		}
	}
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.screen = screenClients
	m.err = ""
	m.reloadClients()
	return m, nil
}

func (m Model) updateClientInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for i := range m.clientInputs {
		var cmd tea.Cmd
		m.clientInputs[i], cmd = m.clientInputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) focusClientInput() tea.Cmd {
	for i := range m.clientInputs {
		if clientField(i) == m.clientFocus {
			m.clientInputs[i].Focus()
		} else {
			m.clientInputs[i].Blur()
		}
	}
	return textinput.Blink
}

func (m *Model) openClientForm(c store.Client) {
	m.editingClientID = c.ID
	m.pendingClient = c
	m.err = ""
	vals := []string{c.Name, c.Contact, c.Notes}
	for i := range m.clientInputs {
		m.clientInputs[i].SetValue(vals[i])
	}
	m.clientFocus = cfName
	m.screen = screenClientForm
	_ = m.focusClientInput()
}

func (m *Model) selectedClient() (store.Client, bool) {
	if m.clientCursor < 0 || m.clientCursor >= len(m.clients) {
		return store.Client{}, false
	}
	return m.clients[m.clientCursor], true
}

func (m *Model) moveClient(delta int) {
	if len(m.clients) == 0 {
		return
	}
	m.clientCursor += delta
	if m.clientCursor < 0 {
		m.clientCursor = 0
	}
	if m.clientCursor >= len(m.clients) {
		m.clientCursor = len(m.clients) - 1
	}
	m.clampClientOffset()
}

func (m *Model) clampClientOffset() {
	h := m.listHeight
	if h <= 0 {
		h = 8
	}
	if m.clientCursor < m.clientOffset {
		m.clientOffset = m.clientCursor
	}
	if m.clientCursor >= m.clientOffset+h {
		m.clientOffset = m.clientCursor - h + 1
	}
	if m.clientOffset < 0 {
		m.clientOffset = 0
	}
}

func (m *Model) cycleClientFilter(dir int) {
	ids := []int64{0}
	all, err := m.store.ListClients(true)
	if err != nil {
		m.err = err.Error()
		return
	}
	for _, c := range all {
		if !c.Archived {
			ids = append(ids, c.ID)
		}
	}
	i := 0
	for k, id := range ids {
		if id == m.filterClientID {
			i = k
			break
		}
	}
	n := len(ids)
	m.filterClientID = ids[(i+dir%n+n)%n]
	m.cursor = 0
	m.offset = 0
	m.reload()
	m.status = i18n.T("Client filter: ") + m.clientFilterLabel()
}

func (m *Model) cycleFormClient(dir int) {
	ids := []int64{0}
	for _, c := range m.clientsForAssign() {
		ids = append(ids, c.ID)
	}
	i := 0
	for k, id := range ids {
		if id == m.pending.ClientID {
			i = k
			break
		}
	}
	n := len(ids)
	m.pending.ClientID = ids[(i+dir%n+n)%n]
}

func (m Model) clientsForAssign() []store.Client {
	list, err := m.store.ListClients(false)
	if err != nil {
		return nil
	}
	return list
}

func (m Model) clientFilterLabel() string {
	if m.filterClientID == 0 {
		return i18n.T("all")
	}
	c, err := m.store.GetClient(m.filterClientID)
	if err != nil {
		return fmt.Sprintf("#%d", m.filterClientID)
	}
	return c.Name
}

func (m Model) clientName(id int64) string {
	if id == 0 {
		return i18n.T("no client")
	}
	c, err := m.store.GetClient(id)
	if err != nil {
		return i18n.T("no client")
	}
	return c.Name
}

func (m Model) viewClients() string {
	th := m.theme
	title := th.Title.Render("Troopa") + th.Muted.Render(i18n.T("  ·  clients"))
	if m.showArchivedCli {
		title += th.Muted.Render(i18n.T("  (incl. archived)"))
	}
	listH := m.listHeight
	if listH <= 0 {
		listH = max(3, m.height-6)
	}
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	if len(m.clients) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("  No clients yet. Press n to create one.")))
		b.WriteString("\n")
		return th.App.Render(b.String())
	}
	end := min(len(m.clients), m.clientOffset+listH)
	for i := m.clientOffset; i < end; i++ {
		c := m.clients[i]
		line := pad(c.Name, 22)
		if c.Contact != "" {
			line += "  ·  " + c.Contact
		}
		if c.Archived {
			line += i18n.T("  [archived]")
		}
		style := th.Normal
		if c.Archived {
			style = th.Archived
		}
		if i == m.clientCursor {
			style = th.Selected
		}
		b.WriteString(style.Width(max(0, m.width)).Render(truncate(line, max(1, m.width-2))))
		b.WriteString("\n")
	}
	if c, ok := m.selectedClient(); ok {
		b.WriteString("\n")
		if c.Notes != "" {
			b.WriteString(th.Muted.Render("  " + c.Notes))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("enter board  F filter projects  n new  e edit  a archive  tab archived  esc back")))
	return th.App.Render(b.String())
}

func (m Model) viewClientForm() string {
	th := m.theme
	title := i18n.T("New client")
	if m.editingClientID != 0 {
		title = i18n.T("Edit client")
	}
	var b strings.Builder
	b.WriteString(th.Title.Render(title))
	b.WriteString("\n\n")
	labels := []string{i18n.T("Name"), i18n.T("Contact"), i18n.T("Notes")}
	for i, lab := range labels {
		marker := "  "
		if clientField(i) == m.clientFocus {
			marker = th.Border.Render("> ")
		}
		b.WriteString(marker + th.Label.Render(lab) + " " + m.clientInputs[i].View() + "\n")
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("enter save  tab field  esc cancel")))
	return th.App.Render(b.String())
}

func (m Model) updateClientsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.screen == screenClientForm {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			idx := msg.Y - 4
			if idx >= 0 && idx < int(cfCount) {
				m.clientFocus = clientField(idx)
				return m, m.focusClientInput()
			}
		}
		return m, nil
	}
	wheel := msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown
	if wheel {
		if msg.Button == tea.MouseButtonWheelUp {
			m.moveClient(-1)
		} else {
			m.moveClient(1)
		}
		return m, nil
	}
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		rel := msg.Y - m.listOriginY
		if rel >= 0 && rel < m.listHeight {
			idx := m.clientOffset + rel
			if idx >= 0 && idx < len(m.clients) {
				m.clientCursor = idx
			}
		}
	}
	return m, nil
}

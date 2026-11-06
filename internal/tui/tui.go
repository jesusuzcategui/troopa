package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

type screen int

const (
	screenList screen = iota
	screenForm
	screenConfirmInit
	screenConfirmArchive
	screenHelp
	screenBoard
	screenQuickCreate
	screenTaskForm
	screenSearch
	screenMoveMenu
	screenClients
	screenClientForm
	screenConfirmArchiveClient
	screenLog
	screenReview
	screenPair
	screenMetrics
	screenPalette
	screenTheme
	screenWelcome
)

type field int

const (
	fieldName field = iota
	fieldSlug
	fieldPath
	fieldStack
	fieldDesc
	fieldClient
	fieldCount
)

type Model struct {
	store       *store.Store
	cfg         config.Config
	configPath  string
	theme       Theme
	themes      []Theme
	themeDir    string
	themeCursor int

	paletteFrom   screen
	paletteInput  textinput.Model
	paletteCursor int

	screen screen
	width  int
	height int

	projects     []store.Project
	cursor       int
	offset       int
	showArchived bool
	listOriginY  int
	listHeight   int
	status       string
	err          string
	helpFrom     screen

	clients         []store.Client
	clientCursor    int
	clientOffset    int
	showArchivedCli bool
	filterClientID  int64
	clientInputs    []textinput.Model
	clientFocus     clientField
	editingClientID int64
	pendingClient   store.Client

	inputs    []textinput.Model
	focus     field
	editingID int64
	pending   store.Project
	initPath  string

	boardProject   store.Project
	tasks          []store.Task
	colIndex       int
	cardIndex      int
	search         string
	filterStatus   string
	filterPriority string
	quickInput     textinput.Model
	searchInput    textinput.Model
	dragging       bool
	dragID         int64
	dragOver       string
	moveMenuIndex  int

	taskTitle   textinput.Model
	taskModelIn textinput.Model
	taskDesc    textarea.Model
	taskCrit    textarea.Model
	taskFocus   taskField
	taskDraft   store.Task

	run        store.Run
	runSess    launch.Session
	runTask    store.Task
	runPair    launch.PairProc
	logOffset  int
	diffFiles  []launch.FileDiff
	diffIndex  int
	diffOffset int
}

func New(s *store.Store, cfg config.Config, configPath string) Model {
	m := Model{
		store:       s,
		cfg:         cfg,
		configPath:  configPath,
		themeDir:    config.ThemesDir(configPath),
		screen:      screenList,
		width:       80,
		height:      24,
		listOriginY: 3,
		listHeight:  18,
	}
	m.inputs = newInputs()
	m.quickInput = textinput.New()
	m.quickInput.Placeholder = i18n.T("Task title")
	m.quickInput.CharLimit = 200
	m.quickInput.Width = 40
	m.quickInput.Prompt = ""
	m.searchInput = textinput.New()
	m.searchInput.Placeholder = i18n.T("search title or description")
	m.searchInput.CharLimit = 80
	m.searchInput.Width = 40
	m.searchInput.Prompt = ""
	m.taskTitle, m.taskModelIn, m.taskDesc, m.taskCrit = newTaskInputs()
	m.clientInputs = newClientInputs()
	m.paletteInput = textinput.New()
	m.paletteInput.Placeholder = i18n.T("search command")
	m.paletteInput.CharLimit = 80
	m.paletteInput.Width = 40
	m.paletteInput.Prompt = ""
	m.themes = loadThemes(m.themeDir)
	m.theme = themeByName(m.themes, cfg.NormalizedTheme())
	if !cfg.WelcomeShown {
		m.screen = screenWelcome
	}
	if m.cfg.Launch.Agents == nil {
		dataDir, _, _, err := config.Paths()
		if err != nil {
			dataDir = os.TempDir()
		}
		def := config.DefaultLaunch(dataDir)
		if m.cfg.Launch.Worktrees == "" {
			m.cfg.Launch.Worktrees = def.Worktrees
		}
		m.cfg.Launch.Agents = def.Agents
		if m.cfg.Launch.Prices == nil {
			m.cfg.Launch.Prices = def.Prices
		}
	}
	m.reload()
	m.status = listStatus()
	return m
}

func listStatus() string {
	return i18n.T("Ctrl+K palette · n new · enter board · c clients · F filter · M metrics · t theme · ? help · q quit")
}

func newInputs() []textinput.Model {
	labels := []string{i18n.T("Name"), "Slug", i18n.T("Repo path"), "Stack", i18n.T("Description")}
	out := make([]textinput.Model, int(fieldDesc)+1)
	for i := range out {
		ti := textinput.New()
		ti.Placeholder = labels[i]
		ti.CharLimit = 256
		ti.Width = 48
		ti.Prompt = ""
		out[i] = ti
	}
	out[fieldDesc].CharLimit = 512
	return out
}

func (m *Model) reload() {
	m.reloadClients()
	list, err := m.store.ListFiltered(store.ProjectFilter{
		IncludeArchived: m.showArchived,
		ClientID:        m.filterClientID,
	})
	if err != nil {
		m.err = err.Error()
		return
	}
	m.projects = list
	if m.cursor >= len(m.projects) {
		m.cursor = max(0, len(m.projects)-1)
	}
	m.clampOffset()
}

func Run(s *store.Store, cfg config.Config, configPath string) error {
	p := tea.NewProgram(New(s, cfg, configPath), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.listOriginY = 3
		m.listHeight = max(3, m.height-6)
		m.clampOffset()
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case logTickMsg:
		if m.screen == screenLog {
			return m, tickLog()
		}
		return m, nil
	case agentDoneMsg:
		return m.finishAgent(msg)
	case pairClosedMsg:
		return m.handlePairClosed(msg)
	case editorClosedMsg:
		return m.handleEditorClosed(msg)
	}
	if m.screen == screenForm {
		return m.updateInputs(msg)
	}
	if m.screen == screenClientForm {
		return m.updateClientInputs(msg)
	}
	if m.screen == screenTaskForm {
		return m.updateTaskInputs(msg)
	}
	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.screen == screenWelcome {
		return m.updateWelcomeKey(msg)
	}
	if msg.String() == "ctrl+k" {
		if m.screen == screenPalette {
			m = m.closePalette()
			return m, nil
		}
		return m.openPalette()
	}
	if m.screen == screenPalette {
		return m.updatePaletteKey(msg)
	}
	if m.screen == screenTheme {
		return m.updateThemeKey(msg)
	}
	switch m.screen {
	case screenHelp:
		if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" || msg.String() == "enter" {
			m.screen = m.helpFrom
			if m.screen == screenHelp {
				m.screen = screenList
			}
		}
		return m, nil
	case screenConfirmArchive:
		switch msg.String() {
		case "y", "enter":
			if err := m.store.Archive(m.pending.ID); err != nil {
				m.err = err.Error()
			} else {
				m.status = i18n.T("Archived: ") + m.pending.Name
				m.err = ""
			}
			m.screen = screenList
			m.reload()
		case "n", "esc":
			m.screen = screenList
		}
		return m, nil
	case screenConfirmInit:
		switch msg.String() {
		case "y", "enter":
			if err := gitinit.InitRepo(m.initPath, m.pending.Name); err != nil {
				m.err = err.Error()
				m.screen = screenForm
				return m, nil
			}
			m.status = i18n.T("Git repo created in ") + m.initPath
			return m.savePending()
		case "n":
			m.status = i18n.T("Saved without initializing the repo")
			return m.savePending()
		case "esc":
			m.screen = screenForm
		}
		return m, nil
	case screenForm:
		return m.updateFormKey(msg)
	case screenBoard:
		return m.updateBoardKey(msg)
	case screenQuickCreate:
		return m.updateQuickKey(msg)
	case screenTaskForm:
		return m.updateTaskFormKey(msg)
	case screenSearch:
		return m.updateSearchKey(msg)
	case screenMoveMenu:
		return m.updateMoveMenuKey(msg)
	case screenClients:
		return m.updateClientsKey(msg)
	case screenClientForm:
		return m.updateClientFormKey(msg)
	case screenConfirmArchiveClient:
		return m.updateConfirmArchiveClient(msg)
	case screenLog:
		return m.updateLogKey(msg)
	case screenReview:
		return m.updateReviewKey(msg)
	case screenPair:
		return m.updatePairKey(msg)
	case screenMetrics:
		return m.updateMetricsKey(msg)
	default:
		return m.updateListKey(msg)
	}
}

func (m Model) updateListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.helpFrom = screenList
		m.screen = screenHelp
	case "t":
		m.cycleTheme()
	case "tab":
		m.showArchived = !m.showArchived
		m.cursor = 0
		m.offset = 0
		m.reload()
		if m.showArchived {
			m.status = i18n.T("Showing archived")
		} else {
			m.status = i18n.T("Hiding archived")
		}
	case "n":
		m.openForm(store.Project{})
	case "c":
		m.openClients()
	case "M":
		return m.openMetrics()
	case "F":
		m.cycleClientFilter(1)
	case "enter", "o":
		if p, ok := m.selected(); ok {
			m.openBoard(p)
		}
	case "e":
		if p, ok := m.selected(); ok {
			m.openForm(p)
		}
	case "a":
		if p, ok := m.selected(); ok {
			if p.Archived {
				m.status = i18n.T("Already archived")
				break
			}
			m.pending = p
			m.screen = screenConfirmArchive
		}
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "pgdown":
		m.move(max(1, m.listHeight))
	case "pgup":
		m.move(-max(1, m.listHeight))
	case "g", "home":
		m.cursor = 0
		m.clampOffset()
	case "G", "end":
		if len(m.projects) > 0 {
			m.cursor = len(m.projects) - 1
			m.clampOffset()
		}
	}
	return m, nil
}

func (m Model) updateFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.screen = screenList
		m.status = i18n.T("Cancelled")
		return m, nil
	case "?":
		m.helpFrom = screenForm
		m.screen = screenHelp
		return m, nil
	case "enter":
		if m.focus == fieldClient {
			m.cycleFormClient(1)
			return m, nil
		}
		return m.submitForm()
	case "left":
		if m.focus == fieldClient {
			m.cycleFormClient(-1)
			return m, nil
		}
	case "right":
		if m.focus == fieldClient {
			m.cycleFormClient(1)
			return m, nil
		}
	case "tab", "down":
		m.focus = (m.focus + 1) % fieldCount
		return m, m.focusInput()
	case "shift+tab", "up":
		m.focus = (m.focus - 1 + fieldCount) % fieldCount
		return m, m.focusInput()
	}
	if m.focus == fieldClient {
		return m, nil
	}
	return m.updateInputs(msg)
}

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	p := store.Project{
		ID:          m.editingID,
		Name:        strings.TrimSpace(m.inputs[fieldName].Value()),
		Slug:        strings.TrimSpace(m.inputs[fieldSlug].Value()),
		RepoPath:    store.NormalizeRepoPath(m.inputs[fieldPath].Value()),
		Stack:       strings.TrimSpace(m.inputs[fieldStack].Value()),
		Description: strings.TrimSpace(m.inputs[fieldDesc].Value()),
		ClientID:    m.pending.ClientID,
	}
	if p.Name == "" {
		m.err = i18n.T("Name is required")
		return m, nil
	}
	if p.RepoPath == "" {
		m.err = i18n.T("Repo path is required")
		return m, nil
	}
	if p.Slug == "" {
		p.Slug = store.Slugify(p.Name)
		m.inputs[fieldSlug].SetValue(p.Slug)
	}
	m.pending = p
	m.err = ""
	if _, err := os.Stat(p.RepoPath); os.IsNotExist(err) {
		m.initPath = p.RepoPath
		m.screen = screenConfirmInit
		return m, nil
	}
	return m.savePending()
}

func (m Model) savePending() (tea.Model, tea.Cmd) {
	var err error
	if m.pending.ID == 0 {
		var created store.Project
		created, err = m.store.Create(m.pending)
		if err == nil {
			m.status = i18n.T("Created: ") + created.Name
		}
	} else {
		err = m.store.Update(m.pending)
		if err == nil {
			m.status = i18n.T("Saved: ") + m.pending.Name
		}
	}
	if err != nil {
		m.err = err.Error()
		m.screen = screenForm
		return m, nil
	}
	m.screen = screenList
	m.err = ""
	m.reload()
	return m, nil
}

func (m Model) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) focusInput() tea.Cmd {
	for i := range m.inputs {
		if field(i) == m.focus {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	if m.focus == fieldClient {
		return nil
	}
	return textinput.Blink
}

func (m *Model) openForm(p store.Project) {
	m.editingID = p.ID
	m.pending = p
	m.err = ""
	vals := []string{p.Name, p.Slug, p.RepoPath, p.Stack, p.Description}
	for i := range m.inputs {
		m.inputs[i].SetValue(vals[i])
	}
	m.focus = fieldName
	m.screen = screenForm
	_ = m.focusInput()
}

func (m *Model) selected() (store.Project, bool) {
	if m.cursor < 0 || m.cursor >= len(m.projects) {
		return store.Project{}, false
	}
	return m.projects[m.cursor], true
}

func (m *Model) move(delta int) {
	if len(m.projects) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.projects) {
		m.cursor = len(m.projects) - 1
	}
	m.clampOffset()
}

func (m *Model) clampOffset() {
	h := m.listHeight
	if h <= 0 {
		h = 8
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenHelp:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			m.screen = m.helpFrom
			if m.screen == screenHelp {
				m.screen = screenList
			}
		}
		return m, nil
	case screenConfirmArchive, screenConfirmInit, screenConfirmArchiveClient:
		return m, nil
	case screenClients, screenClientForm:
		return m.updateClientsMouse(msg)
	case screenForm:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// click a field row if within form
			idx := msg.Y - 4
			if idx >= 0 && idx < int(fieldCount) {
				m.focus = field(idx)
				return m, m.focusInput()
			}
		}
		return m, nil
	case screenBoard, screenMoveMenu, screenQuickCreate, screenSearch:
		return m.updateBoardMouse(msg)
	case screenTaskForm:
		return m.updateTaskFormMouse(msg)
	}

	wheel := msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown
	if wheel {
		if msg.Button == tea.MouseButtonWheelUp {
			m.move(-1)
		} else {
			m.move(1)
		}
		return m, nil
	}
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		rel := msg.Y - m.listOriginY
		if rel >= 0 && rel < m.listHeight {
			idx := m.offset + rel
			if idx >= 0 && idx < len(m.projects) {
				m.cursor = idx
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	th := m.theme
	body := ""
	switch m.screen {
	case screenHelp:
		body = m.viewHelp()
	case screenForm:
		body = m.viewForm()
	case screenBoard:
		body = m.viewBoard()
	case screenQuickCreate:
		body = m.viewQuickCreate()
	case screenTaskForm:
		body = m.viewTaskForm()
	case screenSearch:
		body = m.viewSearch()
	case screenMoveMenu:
		body = m.viewMoveMenu()
	case screenConfirmInit:
		body = m.viewConfirm(fmt.Sprintf(i18n.T("The path %s does not exist.\nCreate the folder, git init -b main, a README and an initial commit?"), m.initPath), i18n.T("y yes · n save without init · esc back"))
	case screenConfirmArchive:
		body = m.viewConfirm(fmt.Sprintf(i18n.T("Archive «%s»?"), m.pending.Name), i18n.T("y yes · n no"))
	case screenConfirmArchiveClient:
		body = m.viewConfirm(fmt.Sprintf(i18n.T("Archive «%s»?"), m.pendingClient.Name), i18n.T("y yes · n no"))
	case screenClients:
		body = m.viewClients()
	case screenClientForm:
		body = m.viewClientForm()
	case screenLog:
		body = m.viewLog()
	case screenReview:
		body = m.viewReview()
	case screenPair:
		body = m.viewPair()
	case screenMetrics:
		body = m.viewMetrics()
	case screenPalette:
		body = m.viewPalette()
	case screenTheme:
		body = m.viewThemePicker()
	case screenWelcome:
		body = m.viewWelcome()
	default:
		body = m.viewList()
	}
	status := m.status
	if m.err != "" {
		status = th.Error.Render(m.err)
	}
	bar := th.StatusBar.Width(max(0, m.width)).Render(truncate(status, max(0, m.width-2)))
	innerH := max(1, m.height-1)
	body = lipgloss.NewStyle().Width(m.width).Height(innerH).Background(th.App.GetBackground()).Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, body, bar)
}

func (m Model) viewList() string {
	th := m.theme
	title := th.Title.Render("Troopa") + th.Muted.Render(i18n.T("  ·  projects"))
	if m.showArchived {
		title += th.Muted.Render(i18n.T("  (incl. archived)"))
	}
	if m.filterClientID != 0 {
		title += th.Muted.Render(i18n.T("  client:") + m.clientFilterLabel())
	}
	listH := m.listHeight
	if listH <= 0 {
		listH = max(3, m.height-6)
	}

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	if len(m.projects) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("  No projects yet. Press n to create one.")))
		b.WriteString("\n")
		return th.App.Render(b.String())
	}
	end := min(len(m.projects), m.offset+listH)
	for i := m.offset; i < end; i++ {
		p := m.projects[i]
		line := fmt.Sprintf("%s  %s", pad(p.Name, 22), p.Slug)
		if p.Stack != "" {
			line += "  ·  " + p.Stack
		}
		if p.ClientID != 0 {
			line += "  ·  " + m.clientName(p.ClientID)
		}
		if p.Archived {
			line += i18n.T("  [archived]")
		}
		style := th.Normal
		if p.Archived {
			style = th.Archived
		}
		if i == m.cursor {
			style = th.Selected
		}
		b.WriteString(style.Width(max(0, m.width)).Render(truncate(line, max(1, m.width-2))))
		b.WriteString("\n")
	}
	if p, ok := m.selected(); ok {
		b.WriteString("\n")
		b.WriteString(th.Muted.Render("  " + p.RepoPath))
		if p.Description != "" {
			b.WriteString(th.Muted.Render("  ·  " + p.Description))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("Ctrl+K palette  enter board  c clients  F filter  M metrics  n new  t theme  ? help  q quit")))
	return th.App.Render(b.String())
}

func (m Model) viewForm() string {
	th := m.theme
	title := i18n.T("New project")
	if m.editingID != 0 {
		title = i18n.T("Edit project")
	}
	var b strings.Builder
	b.WriteString(th.Title.Render(title))
	b.WriteString("\n\n")
	labels := []string{i18n.T("Name"), "Slug", i18n.T("Path"), "Stack", i18n.T("Description")}
	for i, lab := range labels {
		marker := "  "
		st := th.Label
		if field(i) == m.focus {
			marker = th.Border.Render("> ")
		}
		b.WriteString(marker + st.Render(lab) + m.inputs[i].View() + "\n")
	}
	marker := "  "
	if m.focus == fieldClient {
		marker = th.Border.Render("> ")
	}
	b.WriteString(marker + th.Label.Render(i18n.T("Client")) + " " + m.clientName(m.pending.ClientID) + "  ←/→\n")
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("enter save  tab field  ←/→ client  esc cancel  ? help")))
	return th.App.Render(b.String())
}

func (m Model) viewConfirm(msg, hints string) string {
	th := m.theme
	return th.App.Render(th.Title.Render(i18n.T("Confirm")) + "\n\n" + msg + "\n\n" + th.Muted.Render(hints))
}

func (m Model) viewHelp() string {
	th := m.theme
	text := i18n.T(i18n.HelpText)
	return th.Help.Render(text)
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

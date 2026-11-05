package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/store"
)

type paletteItem struct {
	ID    string
	Title string
	Hint  string
	Keys  string
}

func (m Model) allCommands() []paletteItem {
	return []paletteItem{
		{ID: "palette", Title: "Paleta de comandos", Hint: "esta paleta", Keys: "ctrl+k"},
		{ID: "help", Title: "Ayuda", Hint: "atajos", Keys: "?"},
		{ID: "quit", Title: "Salir", Hint: "cerrar Troopa", Keys: "q"},
		{ID: "projects", Title: "Ir a proyectos", Hint: "lista principal", Keys: "esc"},
		{ID: "new-project", Title: "Nuevo proyecto", Keys: "n"},
		{ID: "edit-project", Title: "Editar proyecto", Keys: "e"},
		{ID: "archive-project", Title: "Archivar proyecto", Keys: "a"},
		{ID: "open-board", Title: "Abrir tablero", Keys: "enter"},
		{ID: "toggle-archived", Title: "Mostrar u ocultar archivados", Keys: "tab"},
		{ID: "clients", Title: "Vista de clientes", Keys: "c"},
		{ID: "new-client", Title: "Nuevo cliente", Keys: "n"},
		{ID: "edit-client", Title: "Editar cliente", Keys: "e"},
		{ID: "archive-client", Title: "Archivar cliente", Keys: "a"},
		{ID: "client-board", Title: "Tablero del cliente", Keys: "enter"},
		{ID: "filter-client", Title: "Filtrar por cliente", Keys: "F"},
		{ID: "metrics", Title: "Métricas", Keys: "M"},
		{ID: "new-task", Title: "Nueva tarea", Keys: "n"},
		{ID: "edit-task", Title: "Editar tarea", Keys: "e"},
		{ID: "search-tasks", Title: "Buscar tareas", Keys: "/"},
		{ID: "status-menu", Title: "Menú de estado", Keys: "m"},
		{ID: "move-left", Title: "Mover tarea a la izquierda", Keys: "["},
		{ID: "move-right", Title: "Mover tarea a la derecha", Keys: "]"},
		{ID: "filter-status", Title: "Filtro de estado", Keys: "f"},
		{ID: "filter-priority", Title: "Filtro de prioridad", Keys: "p"},
		{ID: "cycle-theme", Title: "Siguiente tema", Keys: "t"},
		{ID: "pick-theme", Title: "Elegir tema", Hint: "lista de paletas TOML"},
		{ID: "new-theme", Title: "Crear tema propio", Hint: "copia la paleta actual a un TOML"},
		{ID: "approve", Title: "Aprobar revisión", Keys: "y"},
		{ID: "discard", Title: "Descartar worktree", Keys: "d"},
		{ID: "finish-pair", Title: "Finish pair", Keys: "f"},
		{ID: "reopen-agent", Title: "Reabrir agente", Keys: "r"},
	}
}

func (m Model) filteredCommands() []paletteItem {
	q := strings.ToLower(strings.TrimSpace(m.paletteInput.Value()))
	all := m.allCommands()
	if q == "" {
		return all
	}
	var out []paletteItem
	for _, c := range all {
		blob := strings.ToLower(c.ID + " " + c.Title + " " + c.Hint + " " + c.Keys)
		if strings.Contains(blob, q) {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) openPalette() (tea.Model, tea.Cmd) {
	m.paletteFrom = m.screen
	m.screen = screenPalette
	m.paletteCursor = 0
	m.paletteInput.SetValue("")
	m.paletteInput.Focus()
	m.status = "Ctrl+K paleta · enter ejecutar · esc cerrar"
	return m, textinput.Blink
}

func (m Model) closePalette() Model {
	if m.paletteFrom == screenPalette || m.paletteFrom == screenWelcome {
		m.paletteFrom = screenList
	}
	m.screen = m.paletteFrom
	return m
}

func (m Model) updatePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m = m.closePalette()
		return m, nil
	case "up", "k", "shift+tab":
		if m.paletteCursor > 0 {
			m.paletteCursor--
		}
		return m, nil
	case "down", "j", "tab":
		n := len(m.filteredCommands())
		if n > 0 && m.paletteCursor < n-1 {
			m.paletteCursor++
		}
		return m, nil
	case "enter":
		items := m.filteredCommands()
		if m.paletteCursor < 0 || m.paletteCursor >= len(items) {
			return m, nil
		}
		return m.runCommand(items[m.paletteCursor].ID)
	}
	var cmd tea.Cmd
	m.paletteInput, cmd = m.paletteInput.Update(msg)
	m.paletteCursor = 0
	return m, cmd
}

func (m Model) runCommand(id string) (tea.Model, tea.Cmd) {
	from := m.paletteFrom
	m = m.closePalette()
	switch id {
	case "palette":
		return m.openPalette()
	case "help":
		m.helpFrom = m.screen
		m.screen = screenHelp
		return m, nil
	case "quit":
		return m, tea.Quit
	case "projects":
		m.screen = screenList
		m.status = listStatus
		m.reload()
		return m, nil
	case "new-project":
		m.openForm(store.Project{})
		return m, nil
	case "edit-project":
		if p, ok := m.selected(); ok {
			m.openForm(p)
		} else {
			m.status = "No hay proyecto seleccionado"
		}
		return m, nil
	case "archive-project":
		if p, ok := m.selected(); ok {
			if p.Archived {
				m.status = "Ya está archivado"
			} else {
				m.pending = p
				m.screen = screenConfirmArchive
			}
		} else {
			m.status = "No hay proyecto seleccionado"
		}
		return m, nil
	case "open-board":
		if p, ok := m.selected(); ok {
			m.openBoard(p)
		} else {
			m.status = "No hay proyecto seleccionado"
		}
		return m, nil
	case "toggle-archived":
		m.showArchived = !m.showArchived
		m.cursor = 0
		m.offset = 0
		m.reload()
		return m, nil
	case "clients":
		m.openClients()
		return m, nil
	case "new-client":
		m.openClients()
		m.openClientForm(store.Client{})
		return m, nil
	case "edit-client":
		m.openClients()
		if c, ok := m.selectedClient(); ok {
			m.openClientForm(c)
		} else {
			m.status = "No hay cliente seleccionado"
		}
		return m, nil
	case "archive-client":
		m.openClients()
		if c, ok := m.selectedClient(); ok {
			if c.Archived {
				m.status = "Ya está archivado"
			} else {
				m.pendingClient = c
				m.screen = screenConfirmArchiveClient
			}
		} else {
			m.status = "No hay cliente seleccionado"
		}
		return m, nil
	case "client-board":
		m.openClients()
		if c, ok := m.selectedClient(); ok {
			m.filterClientID = c.ID
			m.openClientBoard(c)
		} else {
			m.status = "No hay cliente seleccionado"
		}
		return m, nil
	case "filter-client":
		m.cycleClientFilter(1)
		if from == screenBoard {
			m.screen = screenBoard
			m.reloadTasks()
		}
		m.status = "Filtro cliente: " + m.clientFilterLabel()
		return m, nil
	case "metrics":
		return m.openMetrics()
	case "new-task":
		if m.boardProject.ID == 0 {
			if p, ok := m.selected(); ok {
				m.openBoard(p)
			}
		}
		if m.boardProject.ID == 0 {
			m.status = "Abre un proyecto para crear tareas"
			return m, nil
		}
		m.quickInput.SetValue("")
		m.quickInput.Focus()
		m.screen = screenQuickCreate
		m.status = "Título y enter para crear en inbox"
		return m, textinput.Blink
	case "edit-task":
		if t, ok := m.selectedTask(); ok {
			m.openTaskForm(t)
			return m.withTaskFocus()
		}
		m.status = "No hay tarea seleccionada"
		return m, nil
	case "search-tasks":
		if m.boardProject.ID == 0 {
			if p, ok := m.selected(); ok {
				m.openBoard(p)
			}
		}
		m.searchInput.SetValue(m.search)
		m.searchInput.Focus()
		m.screen = screenSearch
		return m, textinput.Blink
	case "status-menu":
		if _, ok := m.selectedTask(); ok {
			m.moveMenuIndex = m.colIndex
			m.screen = screenMoveMenu
		} else {
			m.status = "No hay tarea seleccionada"
		}
		return m, nil
	case "move-left":
		return m.shiftTask(-1)
	case "move-right":
		return m.shiftTask(1)
	case "filter-status":
		m.filterStatus = cycleFilter(m.filterStatus, append([]string{""}, store.Statuses...))
		m.reloadTasks()
		m.status = "Filtro estado: " + filterLabel(m.filterStatus)
		return m, nil
	case "filter-priority":
		m.filterPriority = cycleFilter(m.filterPriority, append([]string{""}, store.Priorities...))
		m.reloadTasks()
		m.status = "Filtro prioridad: " + filterLabel(m.filterPriority)
		return m, nil
	case "cycle-theme":
		m.cycleTheme()
		return m, nil
	case "pick-theme":
		m.themeCursor = 0
		for i, t := range m.themes {
			if t.Name == m.theme.Name {
				m.themeCursor = i
				break
			}
		}
		m.screen = screenTheme
		m.status = "j/k elegir · enter aplicar · esc volver"
		return m, nil
	case "new-theme":
		m.createUserTheme()
		return m, nil
	case "approve":
		if m.screen != screenReview && from != screenReview {
			m.status = "No hay revisión abierta"
			return m, nil
		}
		m.screen = screenReview
		return m.approveRun()
	case "discard":
		if from != screenReview && m.screen != screenReview {
			m.status = "No hay revisión abierta"
			return m, nil
		}
		m.screen = screenReview
		return m.discardRun()
	case "finish-pair":
		if from != screenPair && m.screen != screenPair {
			m.status = "No hay sesión de pareja"
			return m, nil
		}
		m.screen = screenPair
		return m.finishPair()
	case "reopen-agent":
		if from != screenPair && m.screen != screenPair {
			m.status = "No hay sesión de pareja"
			return m, nil
		}
		m.screen = screenPair
		return m.startPair(m.runTask)
	}
	return m, nil
}

func (m Model) viewPalette() string {
	th := m.theme
	items := m.filteredCommands()
	var b strings.Builder
	b.WriteString(th.Title.Render("Comandos") + th.Muted.Render("  Ctrl+K"))
	b.WriteString("\n")
	b.WriteString(th.Normal.Render("> " + m.paletteInput.View()))
	b.WriteString("\n\n")
	if len(items) == 0 {
		b.WriteString(th.Muted.Render("  Sin coincidencias"))
		return th.App.Render(b.String())
	}
	cur := m.paletteCursor
	if cur >= len(items) {
		cur = len(items) - 1
	}
	h := max(5, m.height-8)
	start := 0
	if cur >= h {
		start = cur - h + 1
	}
	end := min(len(items), start+h)
	for i := start; i < end; i++ {
		c := items[i]
		line := c.Title
		if c.Keys != "" {
			line += "  " + c.Keys
		}
		st := th.Normal
		if i == cur {
			st = th.Selected
		}
		b.WriteString(st.Width(max(0, m.width)).Render(truncate(line, max(1, m.width-2))))
		b.WriteString("\n")
	}
	return th.App.Render(b.String())
}

func (m Model) updateThemeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.screen = screenList
		m.status = listStatus
		return m, nil
	case "j", "down":
		if m.themeCursor < len(m.themes)-1 {
			m.themeCursor++
		}
	case "k", "up":
		if m.themeCursor > 0 {
			m.themeCursor--
		}
	case "enter":
		if m.themeCursor >= 0 && m.themeCursor < len(m.themes) {
			m.applyTheme(m.themes[m.themeCursor].Name)
			m.screen = screenList
		}
	}
	return m, nil
}

func (m Model) viewThemePicker() string {
	th := m.theme
	var b strings.Builder
	b.WriteString(th.Title.Render("Temas"))
	b.WriteString(th.Muted.Render("  ·  TOML en " + m.themeDir))
	b.WriteString("\n\n")
	for i, t := range m.themes {
		line := t.Name
		if t.Name == m.theme.Name {
			line += "  (activo)"
		}
		st := th.Normal
		if i == m.themeCursor {
			st = th.Selected
		}
		b.WriteString(st.Width(max(0, m.width)).Render(truncate(line, max(1, m.width-2))))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render("enter elegir  t ciclar  Ctrl+K → Crear tema propio  esc volver"))
	return th.App.Render(b.String())
}

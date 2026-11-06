package i18n

// Long texts shown by the TUI. The English text is the catalog key.

const HelpText = `Shortcuts — projects

  j / k  or  ↑ / ↓   move in the list
  enter / o          open the task board
  e                  edit the project
  n                  create a project
  a                  archive
  c                  clients view
  M                  metrics by agent and project
  F                  filter projects by client
  tab                show or hide archived
  t                  next theme (dark / light / tropic / custom)
  Ctrl+K             command palette (every action)
  ?                  this help
  q / ctrl+c         quit

  Clients
  n / e / a          create, edit, archive
  enter              the client's task board
  F                  filter the project list
  tab                show or hide archived
  esc                back to projects

  Board and list
  v                  switch between board and list
  n                  create a task (title only)
  e                  edit the whole task
  enter              edit, or log/review when an agent ran
  h / l              board: column · list: fold / unfold the group
  z / Z              fold the status · fold every empty status
  [ / ]  or  H / L   move the task to another status
  m / P / A          popup to change status, priority or agent
  tab                task panel (beside the board when the terminal is wide)
  /                  search
  f / p              status / priority filter
  C                  client filter
  M                  metrics by agent and project
  o                  open the worktree (or the project) in your editor
  g                  merge an approved local branch into the base
  c                  clean worktree, keep the branch
  esc                back to projects

  Mouse
  click              select (list or card)
  click a header     fold or unfold the status
  drag a card        change its status
  wheel              scroll · shift+wheel changes column
  click in a popup   pick an option

  Yolo agent (ready, not me)
  y / a              approve (auto: commit+push+PR; none: keeps the worktree)
  g                  merge an approved local branch into the base
  c                  clean worktree, keep the branch
  d                  discard worktree and branch
  o                  open the worktree in your editor

  Pair (pair mode)
  f                  finish pair (checks and review)
  r                  reopen the interactive agent
  o                  open the worktree in your editor
  esc                back to the board (the session keeps going)
  close terminal     pair-paused; enter to resume

  When you save a project whose path does not exist, Troopa offers
  to create the folder with git init -b main, a README and an initial commit.`

const WelcomeText = `Troopa
your AI agent troop in the terminal

  • Projects, board, clients and metrics
  • TOML themes (t cycles · Ctrl+K → Pick theme)
  • Command palette: Ctrl+K
  • Agents in ~/.config/troopa/config.toml

Press enter to start.`

const helpTextES = `Atajos — proyectos

  j / k  o  ↑ / ↓    moverse en la lista
  enter / o          abrir tablero de tareas
  e                  editar el proyecto
  n                  crear proyecto
  a                  archivar
  c                  vista de clientes
  M                  métricas por agente y proyecto
  F                  filtrar proyectos por cliente
  tab                mostrar u ocultar archivados
  t                  siguiente tema (dark / light / tropic / propios)
  Ctrl+K             paleta de comandos (todas las acciones)
  ?                  esta ayuda
  q / ctrl+c         salir

  Clientes
  n / e / a          crear, editar, archivar
  enter              tablero de tareas del cliente
  F                  filtrar la lista de proyectos
  tab                mostrar u ocultar archivados
  esc                volver a proyectos

  Tablero y lista
  v                  cambiar entre tablero y lista
  n                  crear tarea (solo título)
  e                  editar tarea completa
  enter              editar, o log/revisión si hay agente
  h / l              tablero: columna · lista: plegar / desplegar el grupo
  z / Z              plegar el estado · plegar todos los vacíos
  [ / ]  o  H / L    mover tarea de estado
  m / P / A          popup para cambiar estado, prioridad o agente
  tab                panel de la tarea (al lado si la terminal es ancha)
  /                  buscar
  f / p              filtro estado / prioridad
  C                  filtro por cliente
  M                  métricas por agente y proyecto
  o                  abrir el worktree (o el proyecto) en tu editor
  g                  mergear una rama local aprobada en la base
  c                  limpiar worktree, conservar la rama
  esc                volver a proyectos

  Ratón
  clic               seleccionar (lista o tarjeta)
  clic en encabezado plegar o desplegar el estado
  arrastrar tarjeta  cambiar estado
  rueda              scroll · shift+rueda cambia de columna
  clic en popup      elegir una opción

  Agente yolo (ready, no me)
  y / a              aprobar (auto: commit+push+PR; none: deja el worktree)
  g                  mergear una rama local aprobada en la base
  c                  limpiar worktree, conservar la rama
  d                  descartar worktree y rama
  o                  abrir el worktree en tu editor

  Pareja (modo pair)
  f                  terminar la pareja (checks y revisión)
  r                  reabrir el agente interactivo
  o                  abrir el worktree en tu editor
  esc                volver al tablero (la sesión sigue)
  cerrar terminal    pair-paused; enter para retomar

  Al guardar un proyecto, si la ruta no existe se ofrece
  crear la carpeta con git init -b main, README y commit inicial.`

const welcomeTextES = `Troopa
tu tropa de agentes en la terminal

  • Proyectos, tablero, clientes y métricas
  • Temas en TOML (t cicla · Ctrl+K → Elegir tema)
  • Paleta de comandos: Ctrl+K
  • Agentes en ~/.config/troopa/config.toml

Pulsa enter para empezar.`

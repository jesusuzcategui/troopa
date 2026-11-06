package i18n

// Long texts shown by the TUI. The English text is the catalog key.

const HelpText = `Shortcuts — projects

  j / k  or  ↑ / ↓   move in the list
  enter / o          open the task board
  B                  open the second brain
  e                  edit the project
  n                  create a project
  a                  archive
  c                  clients view
  M                  metrics by agent and project
  F                  filter projects by client
  tab                show or hide archived
  t                  next theme
  S                  settings (language, agents, editor, merge, second brain…)
  i                  about Troopa
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
  a                  attach a file (task form attachments section and panel)
  p                  paste a file path (attachments)
  /                  search
  f / p              status / priority filter
  C                  client filter
  M                  metrics by agent and project
  o                  open the worktree (or the project) in your editor
  g                  merge an approved local branch into the base
  c                  clean worktree, keep the branch
  x                  request changes: send the task back to its agent
  B                  open this project's project.md
  w                  plans of this project
  esc                back to projects

  Plans (w on the board)
  n / enter / d      new, open, delete a plan
  a                  add steps (enter adds, esc ends)
  K / J              move the step up / down
  s / p              start or resume / pause the plan
  t                  auto-advance: approve a step and the next one starts
  g                  merge the finished plan (or push and open a PR)

  Profiles and accounts
  S → Profiles       who the agents work for (name, profession, instructions)
  project form       ←/→ picks the account and the profile of the project
  troopa login <account> <agent>   sign an agent in to an account

  Mouse
  click              select (list or card)
  click a header     fold or unfold the status
  drag a card        change its status
  wheel              scroll · shift+wheel changes column
  click in a popup   pick an option

  Yolo agent (ready, not me)
  y / a              approve (auto: commit+push+PR; none: keeps the worktree)
  x                  request changes (same agent, same worktree)
  j / k              move the line cursor on the diff
  n / p              next / previous file
  enter / click      comment on the line
  e / D              edit / delete the comment on the line
  C                  list pending comments
  r                  fold or unfold the selected round
  [ / ] or tab       change round (review and log)
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

  Second brain
  B                  from projects (inbox) · board or task panel (project.md)
  j / k              tree · on a narrow terminal, scroll the note
  h / l              fold / unfold a folder
  enter              open the note or follow the selected link
  tab                cycle [[wiki]] and relative Markdown links
  esc / backspace    history, then the tree, then leave
  /                  search every note (note + line)
  o                  edit this note in your editor
  i                  add a line to inbox.md
  ctrl+s             local commit when brain_git is on
  click / wheel      select a note · scroll

  When you save a project whose path does not exist, Troopa offers
  to create the folder with git init -b main, a README and an initial commit.
  Moving a task to ready asks before its agent starts.
  Agents keep working if you close Troopa; it picks them up when it opens.
  Hooks ([hooks] in config.toml or troopa.toml) run on launch, review,
  approve, merge… and troopa-<name> programs on the PATH run as troopa <name>.
  The second brain folder is chosen in the first-run setup and in Settings.
  troopa setup runs the first-run setup again.`

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
  B                  abrir el segundo cerebro
  e                  editar el proyecto
  n                  crear proyecto
  a                  archivar
  c                  vista de clientes
  M                  métricas por agente y proyecto
  F                  filtrar proyectos por cliente
  tab                mostrar u ocultar archivados
  t                  siguiente tema
  S                  configuración (idioma, agentes, editor, merge, segundo cerebro…)
  i                  acerca de Troopa
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
  a                  adjuntar un archivo (sección Adjuntos del formulario y el panel)
  p                  pegar la ruta de un archivo (adjuntos)
  /                  buscar
  f / p              filtro estado / prioridad
  C                  filtro por cliente
  M                  métricas por agente y proyecto
  o                  abrir el worktree (o el proyecto) en tu editor
  g                  mergear una rama local aprobada en la base
  c                  limpiar worktree, conservar la rama
  x                  pedir cambios: devolver la tarea a su agente
  B                  abrir el project.md de este proyecto
  w                  planes de este proyecto
  esc                volver a proyectos

  Planes (w en el tablero)
  n / enter / d      nuevo, abrir, eliminar un plan
  a                  agregar pasos (enter agrega, esc termina)
  K / J              subir / bajar el paso
  s / p              iniciar o retomar / pausar el plan
  t                  avance automático: apruebas un paso y arranca el siguiente
  g                  merge del plan terminado (o push y PR)

  Perfiles y cuentas
  S → Perfiles       para quién trabajan los agentes (nombre, profesión, instrucciones)
  formulario         ←/→ elige la cuenta y el perfil del proyecto
  troopa login <cuenta> <agente>   iniciar sesión de un agente en una cuenta

  Ratón
  clic               seleccionar (lista o tarjeta)
  clic en encabezado plegar o desplegar el estado
  arrastrar tarjeta  cambiar estado
  rueda              scroll · shift+rueda cambia de columna
  clic en popup      elegir una opción

  Agente yolo (ready, no me)
  y / a              aprobar (auto: commit+push+PR; none: deja el worktree)
  x                  pedir cambios (mismo agente, mismo worktree)
  j / k              mover el cursor de línea en el diff
  n / p              archivo siguiente / anterior
  enter / clic       comentar la línea
  e / D              editar / borrar el comentario de la línea
  C                  listar comentarios pendientes
  r                  plegar o desplegar la ronda seleccionada
  [ / ] o tab        cambiar de ronda (revisión y log)
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

  Segundo cerebro
  B                  desde proyectos (inbox) · tablero o panel (project.md)
  j / k              árbol · en terminal angosta, scroll de la nota
  h / l              plegar / desplegar carpeta
  enter              abrir la nota o seguir el enlace seleccionado
  tab                recorrer [[wiki]] y enlaces Markdown relativos
  esc / backspace    historial, luego el árbol, luego salir
  /                  buscar en todas las notas (nota + línea)
  o                  editar esta nota en tu editor
  i                  añadir una línea a inbox.md
  ctrl+s             commit local si brain_git está activo
  clic / rueda       elegir una nota · scroll

  Al guardar un proyecto, si la ruta no existe se ofrece
  crear la carpeta con git init -b main, README y commit inicial.
  Mover una tarea a ready pregunta antes de lanzar su agente.
  Los agentes siguen trabajando si cierras Troopa; los retoma al abrirlo.
  Los hooks ([hooks] en config.toml o troopa.toml) corren al lanzar, revisar,
  aprobar, mergear… y los programas troopa-<nombre> del PATH corren como troopa <nombre>.
  La carpeta del segundo cerebro se elige en el primer arranque y en Configuración.
  troopa setup repite la configuración inicial.`

const welcomeTextES = `Troopa
tu tropa de agentes en la terminal

  • Proyectos, tablero, clientes y métricas
  • Temas en TOML (t cicla · Ctrl+K → Elegir tema)
  • Paleta de comandos: Ctrl+K
  • Agentes en ~/.config/troopa/config.toml

Pulsa enter para empezar.`

// AboutCreator is the creator blurb in the About window.
const AboutCreator = `Senior web developer who builds tools for people working side by side with AI agents. Troopa is the open-source, terminal-first version of how he runs his own troop of agents: plan the work, hand it out, review it and merge it, without leaving the keyboard.`

const aboutCreatorES = `Desarrollador web senior que crea herramientas para quienes trabajan codo a codo con agentes de IA. Troopa es la versión open source y de terminal de cómo él dirige su propia tropa de agentes: planear el trabajo, repartirlo, revisarlo y hacer el merge, sin soltar el teclado.`

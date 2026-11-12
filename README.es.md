# Troopa

Tu tropa de agentes de IA en la terminal.

Capturas: [docs/demo.tape](docs/demo.tape) (`vhs docs/demo.tape` → `docs/demo.gif`) y [docs/demo.cast](docs/demo.cast) (`asciinema play docs/demo.cast`). Volver a grabar: `asciinema rec docs/demo.cast` y `go run ./cmd/troopa`.

## Instalación

Sitio: https://jesusuzcategui.github.io/troopa/

```bash
# Linux y macOS: última versión, con checksum verificado, en ~/.local/bin
curl -fsSL https://jesusuzcategui.github.io/troopa/install.sh | sh

# O con Go 1.26+
go install github.com/jesusuzcategui/troopa/cmd/troopa@latest
```

Cada [versión](https://github.com/jesusuzcategui/troopa/releases) trae archivos para Linux, macOS y Windows (amd64 y arm64) y paquetes `.deb`, `.rpm`, `.apk` y de Arch.

Desde este repo: `make build` (→ `./bin/troopa`) o `go run ./cmd/troopa`.

## Primer arranque y configuración

La primera vez, Troopa hace una configuración corta: idioma, tema, agente y modelo por defecto, editor, terminal para el modo pareja, estrategia de merge y carpeta del segundo cerebro. Lista los agentes que encuentra en tu computadora (Claude Code, Codex, OpenCode, Pi) con su versión. Puedes repetirla con `troopa setup`.

Después todo queda en Configuración: `S` (o Ctrl+K → Configuración) para cambiar cualquier valor; se guarda al instante en `config.toml`. `i` abre Acerca de. Al mover una tarea a **ready** siempre pregunta antes de lanzar el agente y dice cuál es.

## Idioma

La interfaz está en inglés por defecto. Elige español (o `auto` para seguir el idioma del sistema) en la configuración inicial o en Configuración; se guarda como `language` en `config.toml`. `TROOPA_LANG=en` o `TROOPA_LANG=es` tiene prioridad.

## Temas

Vienen quince paletas. Oscuras: `dark`, `tropic`, `nord`, `dracula`, `gruvbox`, `catppuccin-mocha`, `tokyo-night`, `rose-pine`, `cafe`, `midnight`, `high-contrast`. Claras: `light`, `gruvbox-light`, `catppuccin-latte`, `solarized-light`. `t` las cicla. Nord, Dracula, Gruvbox, Catppuccin, Tokyo Night, Rosé Pine y Solarized son adaptaciones de esas paletas de la comunidad (todas con licencia MIT), con algunos colores ajustados para mejorar el contraste. La paleta de comandos (Ctrl+K) permite elegir uno de la lista o crear una copia del actual en `$XDG_CONFIG_HOME/troopa/themes/`.

## Agentes

Ejemplo de `config.toml` en el README en inglés (`max_attachment_mb = 50`). En el formulario y el panel, `a` abre el selector de archivos y `p` pega una ruta. Los archivos se copian a `$XDG_DATA_HOME/troopa/attachments/<id>/`. `@` en la descripción autocompleta archivos del repo. Al lanzar, los adjuntos van a `.troopa/attachments/` del worktree (fuera de git y del diff) y `TASK.md` los lista con las referencias `@`.

Al pasar a **ready** con un agente distinto de `me` se crea un worktree y `TASK.md`. Yolo corre sin interacción; pair abre el CLI interactivo. Finish pair (`f`) corre checks.

### Después de aprobar

Con remoto, Troopa ya abrió el PR: `c` quita el worktree y conserva la rama. Sin remoto, `g` mergea la rama en la base del proyecto (squash o merge según `config.toml`) o `c` solo limpia el worktree. `d` descarta worktree y rama.

`o` abre el worktree de la tarea en tu editor (o la carpeta del proyecto si el worktree ya no existe o la tarea aún no tiene). El editor sale de `editor` en `config.toml` (por ejemplo `editor = "nvim"` o `"code"`), luego `$VISUAL`, luego `$EDITOR`, luego `nvim`/`vim`/`vi`.

## Pedir cambios

Si el trabajo no quedó bien, pulsa `x` en la revisión (o sobre la tarea en el tablero, la lista o el panel) y escribe qué hay que cambiar. El mismo agente vuelve a trabajar **en el mismo worktree**, conservando lo que ya hizo, con tu comentario en `CHANGES.md` (una ronda por pedido; nunca se commitea). En la revisión, `j`/`k` mueven un cursor de línea sobre el diff (`n`/`p` cambian de archivo); `enter` o un clic comentan esa línea (número real del archivo nuevo, según los encabezados `@@`). Las líneas con comentario se marcan y el texto se ve debajo; `e` edita, `D` borra, `C` lista los pendientes. Pedir cambios incluye esos comentarios como `path:line`, la línea citada y la nota; el texto general es opcional si hay comentarios. Los enviados quedan resueltos y se ven en esa ronda. Cada ronda se guarda aparte: en la revisión y en el panel las anteriores van plegadas y la última abierta (`r` o clic las pliega; `[` `]` o `tab` cambian de ronda). El log se puede filtrar por ronda. Las tareas en modo pareja reabren el agente interactivo con el comentario. El diff sigue siendo el del worktree completo.

Cada `TASK.md` le pide al agente empezar `SUMMARY.md` con una sección **How to run** (preparación, dependencias, comando y cómo probar), así la revisión te dice cómo probar el cambio. También incluye **Project context** con las rutas absolutas de `project.md` e `INDEX.md` y el texto de Current state / Next / Decisions. El agente puede leer el resto del cerebro; no debe modificarlo salvo que la tarea lo pida.

## Segundo cerebro

Notas en markdown que Troopa mantiene junto a SQLite. Carpeta por defecto: `$XDG_DATA_HOME/troopa/brain`. El primer arranque y Configuración (`S`) eligen la carpeta propia de Troopa o un vault existente. Si no existe, Troopa la crea con este árbol:

```
brain/
├── README.md
├── inbox.md
├── today.md
├── projects/
│   ├── INDEX.md
│   └── <slug>/
│       ├── project.md
│       ├── tasks/
│       │   └── <id>-<slug>.md
│       └── guides/
│           └── <YYYY-MM-DD>-<slug>.md
```

Crear o editar un proyecto actualiza `INDEX.md` y el frontmatter YAML de `project.md` sin tocar **Current state**, **Next** ni **Decisions**. Las tareas escriben `projects/<slug>/tasks/<id>-<slug>.md`. Aprobar añade una línea en History (con un id HTML para no duplicar). Un merge local (`g`) añade otra línea y una guía en `projects/<slug>/guides/` con **How to run** de `SUMMARY.md`. `troopa brain sync --json` regenera todo desde SQLite (idempotente). Si `brain_git = true` y la carpeta es un repo git, Troopa hace commit local (`troopa: update <slug>`) y nunca push.

Pulsa `B` en la lista de proyectos, el tablero, el panel de la tarea o la paleta para leer el vault en Troopa. A la izquierda va el árbol (inbox, today, `projects/INDEX`, cada `project.md`, `tasks/` y `guides/`); a la derecha la nota con Glamour. En una terminal angosta, un panel cada vez (`enter` / `esc`). `tab` recorre `[[wiki]]` y enlaces Markdown relativos, `enter` los abre, `esc`/`backspace` vuelven atrás, y cada nota lista backlinks. `/` busca en todas las notas. `o` abre el archivo en tu editor; `i` añade una línea a `inbox.md`. Con `brain_git`, la barra muestra cambios sin commit y `ctrl+s` hace el commit local. El grafo visual no está en la TUI.

Las reglas para todas las tareas de un proyecto van en `troopa.toml`, en la raíz del repo. Los planes con aprobación automática solo aprueban un paso solos cuando hay checks y pasan; sin checks el paso queda en revisión.

```toml
checks = ["python3 -m pytest -q"]
instructions = """
Python: usa un entorno virtual en .venv y mantén requirements.txt al día.
"""
```

## Atajos

`Ctrl+K` paleta de comandos (cualquier acción). La primera vez hay una pantalla de bienvenida.

Proyectos: `enter` tablero · `n` nuevo · `e` editar · `a` archivar · `c` clientes · `F` filtro · `M` métricas · `t` tema · `?` ayuda · `q` salir

Clientes: `n` / `e` / `a` · `enter` tablero · `esc` volver

Tablero y lista: `v` cambia tablero/lista · `z` pliega el estado · `Z` pliega los vacíos · `m`/`P`/`A` popup para cambiar estado, prioridad o agente · `tab` panel de la tarea · `n` tarea rápida · `e` editar · `[` `]` mover · `/` buscar · `f`/`p`/`C` filtros · `g` merge · `c` limpiar worktree · `o` editor · panel `a` adjuntar · `esc` volver

En una terminal angosta el tablero se desplaza de lado siguiendo la columna seleccionada (`◀ 2` / `▶ 3` indican estados ocultos). Desde 120 columnas el panel de la tarea va al lado del tablero; en terminales más chicas `tab` lo abre a pantalla completa. La vista y los estados plegados se guardan como `board_view` y `collapsed` en `config.toml`.

Revisión: `y` aprobar · `x` pedir cambios · `j`/`k` línea · `n`/`p` archivo · `enter` comentar · `e` editar · `D` borrar · `C` comentarios · `r` plegar ronda · `[` `]`/`tab` cambiar ronda · `g` mergear · `c` limpiar worktree · `d` descartar · `o` editor

Pareja: `f` terminar · `r` reabrir agente · `o` editor

## Versiones y sitio web

- `git tag v0.2.0 && git push origin v0.2.0` corre `.github/workflows/release.yml`: pruebas y luego GoReleaser compila todas las plataformas, los paquetes de Linux y `checksums.txt`, y publica la versión en GitHub.
- El sitio está en `github_page/` (HTML, CSS y JavaScript compilados con Vite a `github_page/dist`). `cd github_page && npm install && npm run dev` para trabajarlo; al subir cambios a `main` se publica con `.github/workflows/pages.yml`.

## Licencia

MIT. [CONTRIBUTING.md](CONTRIBUTING.md).

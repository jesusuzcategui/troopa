# Troopa

Tu tropa de agentes de IA en la terminal.

Capturas: [docs/demo.tape](docs/demo.tape) (`vhs docs/demo.tape` → `docs/demo.gif`) y [docs/demo.cast](docs/demo.cast) (`asciinema play docs/demo.cast`). Volver a grabar: `asciinema rec docs/demo.cast` y `go run ./cmd/troopa`.

## Instalación

Go 1.26+ y Git.

```bash
go install github.com/jesusuzcategui/troopa/cmd/troopa@latest
```

O desde este repo:

```bash
make build   # ./bin/troopa
go run ./cmd/troopa
```

Binarios snapshot (Linux, macOS, Windows; no se publican):

```bash
goreleaser build --snapshot --clean
# dist/troopa_<os>_<arch>/troopa
```

Datos: `$XDG_DATA_HOME/troopa/troopa.db` (por defecto `~/.local/share/troopa/`).
Configuración: `$XDG_CONFIG_HOME/troopa/config.toml` (tema, agentes, worktrees, terminal y precios). Troopa solo reescribe las líneas `theme` y `welcome_shown`; el resto queda como lo escribiste.
Checks por proyecto: `troopa.toml` en la raíz del repo.

## Primer arranque y configuración

La primera vez, Troopa hace una configuración corta: idioma, tema, agente y modelo por defecto, editor, terminal para el modo pareja y estrategia de merge. Lista los agentes que encuentra en tu computadora (Claude Code, Codex, OpenCode, Pi) con su versión. Puedes repetirla con `troopa setup`.

Después todo queda en Configuración: `S` (o Ctrl+K → Configuración) para cambiar cualquier valor; se guarda al instante en `config.toml`. `i` abre Acerca de. Al mover una tarea a **ready** siempre pregunta antes de lanzar el agente y dice cuál es.

## Idioma

La interfaz está en inglés por defecto. Elige español (o `auto` para seguir el idioma del sistema) en la configuración inicial o en Configuración; se guarda como `language` en `config.toml`. `TROOPA_LANG=en` o `TROOPA_LANG=es` tiene prioridad.

## Temas

Vienen quince paletas. Oscuras: `dark`, `tropic`, `nord`, `dracula`, `gruvbox`, `catppuccin-mocha`, `tokyo-night`, `rose-pine`, `cafe`, `midnight`, `high-contrast`. Claras: `light`, `gruvbox-light`, `catppuccin-latte`, `solarized-light`. `t` las cicla. Nord, Dracula, Gruvbox, Catppuccin, Tokyo Night, Rosé Pine y Solarized son adaptaciones de esas paletas de la comunidad (todas con licencia MIT), con algunos colores ajustados para mejorar el contraste. La paleta de comandos (Ctrl+K) permite elegir uno de la lista o crear una copia del actual en `$XDG_CONFIG_HOME/troopa/themes/`.

## Agentes

Ejemplo de `config.toml` en el README en inglés. Al pasar a **ready** con un agente distinto de `me` se crea un worktree y `TASK.md`. Yolo corre sin interacción; pair abre el CLI interactivo. Finish pair (`f`) corre checks.

### Después de aprobar

Con remoto, Troopa ya abrió el PR: `c` quita el worktree y conserva la rama. Sin remoto, `g` mergea la rama en la base del proyecto (squash o merge según `config.toml`) o `c` solo limpia el worktree. `d` descarta worktree y rama.

`o` abre el worktree de la tarea en tu editor (o la carpeta del proyecto si el worktree ya no existe o la tarea aún no tiene). El editor sale de `editor` en `config.toml` (por ejemplo `editor = "nvim"` o `"code"`), luego `$VISUAL`, luego `$EDITOR`, luego `nvim`/`vim`/`vi`.

## Pedir cambios

Si el trabajo no quedó bien, pulsa `x` en la revisión (o sobre la tarea en el tablero, la lista o el panel) y escribe qué hay que cambiar. El mismo agente vuelve a trabajar **en el mismo worktree**, conservando lo que ya hizo, con tu comentario en `CHANGES.md` (una ronda por pedido; nunca se commitea). Termina otra vez en la revisión con el diff completo. Las tareas en modo pareja reabren el agente interactivo con el comentario.

Cada `TASK.md` le pide al agente empezar `SUMMARY.md` con una sección **How to run** (preparación, dependencias, comando y cómo probar), así la revisión te dice cómo probar el cambio.

Las reglas para todas las tareas de un proyecto van en `troopa.toml`, en la raíz del repo:

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

Tablero y lista: `v` cambia tablero/lista · `z` pliega el estado · `Z` pliega los vacíos · `m`/`P`/`A` popup para cambiar estado, prioridad o agente · `tab` panel de la tarea · `n` tarea rápida · `e` editar · `[` `]` mover · `/` buscar · `f`/`p`/`C` filtros · `g` merge · `c` limpiar worktree · `o` editor · `esc` volver

En una terminal angosta el tablero se desplaza de lado siguiendo la columna seleccionada (`◀ 2` / `▶ 3` indican estados ocultos). Desde 120 columnas el panel de la tarea va al lado del tablero; en terminales más chicas `tab` lo abre a pantalla completa. La vista y los estados plegados se guardan como `board_view` y `collapsed` en `config.toml`.

Revisión: `y` aprobar · `x` pedir cambios · `g` mergear · `c` limpiar worktree · `d` descartar · `o` editor

Pareja: `f` terminar · `r` reabrir agente · `o` editor

## Licencia

MIT. [CONTRIBUTING.md](CONTRIBUTING.md).

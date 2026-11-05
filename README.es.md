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

## Temas

Van tres paletas (`dark`, `light`, `tropic`). `t` las cicla. La paleta de comandos (Ctrl+K) permite elegir uno de la lista o crear una copia del actual en `$XDG_CONFIG_HOME/troopa/themes/`.

## Agentes

Ejemplo de `config.toml` en el README en inglés. Al pasar a **ready** con un agente distinto de `me` se crea un worktree y `TASK.md`. Yolo corre sin interacción; pair abre el CLI interactivo. Finish pair (`f`) corre checks.

## Atajos

`Ctrl+K` paleta de comandos (cualquier acción). La primera vez hay una pantalla de bienvenida.

Proyectos: `enter` tablero · `n` nuevo · `e` editar · `a` archivar · `c` clientes · `F` filtro · `M` métricas · `t` tema · `?` ayuda · `q` salir

Clientes: `n` / `e` / `a` · `enter` tablero · `esc` volver

Tablero: `n` tarea rápida · `e` editar · `[` `]` mover · `m` menú · `/` buscar · `f`/`p`/`C` filtros · `esc` volver

Revisión: `y` aprobar · `d` descartar

Pareja: `f` terminar · `r` reabrir agente

## Licencia

MIT. [CONTRIBUTING.md](CONTRIBUTING.md).

# Troopa

Your AI agent troop in the terminal.

Captures: [docs/demo.tape](docs/demo.tape) (`vhs docs/demo.tape` → `docs/demo.gif`) and [docs/demo.cast](docs/demo.cast) (`asciinema play docs/demo.cast`). Re-record with `asciinema rec docs/demo.cast` then `go run ./cmd/troopa`.

## Install

Go 1.26+ and Git.

```bash
go install github.com/jesusuzcategui/troopa/cmd/troopa@latest
```

Or from this repo:

```bash
make build   # ./bin/troopa
go run ./cmd/troopa
```

Snapshot binaries (Linux, macOS, Windows; not published):

```bash
goreleaser build --snapshot --clean
# dist/troopa_<os>_<arch>/troopa
```

Data: `$XDG_DATA_HOME/troopa/troopa.db` (default `~/.local/share/troopa/`).
Config: `$XDG_CONFIG_HOME/troopa/config.toml` (theme, agents, worktrees, terminal and token prices). Troopa only rewrites the `theme` and `welcome_shown` lines; everything else stays as you wrote it.
Per-project checks: `troopa.toml` at the repo root (`checks = ["go test ./..."]`).

## Language

The interface is in English or Spanish, picked from `LANG` (`es_*` means Spanish). Override it with `TROOPA_LANG=en` or `TROOPA_LANG=es`. The CLI JSON output and the files written for agents (`TASK.md`, `PAIR.md`) are always in English.

## Themes

Three palettes ship with the binary (`dark`, `light`, `tropic`). `t` cycles them. The command palette (Ctrl+K) can pick one from a list or write a copy of the current one under `$XDG_CONFIG_HOME/troopa/themes/`.

Drop your own TOML there:

```toml
name = "sunset"

[colors]
fg = "#FFE8D6"
bg = "#2B1B17"
accent = "#FF7A59"
muted = "#A8897A"
selected = "#3D2A24"
status = "#35241F"
error = "#FF5D5D"
diff_add = "#8FBF6A"
diff_del = "#FF5D5D"
diff_hunk = "#7EB8FF"
```

## Agents

`~/.config/troopa/config.toml`:

```toml
branch_prefix = ""
worktrees = "~/.local/share/troopa/worktrees"
notify = true
terminal = "kitty -e"
editor = "nvim"            # or "code", "zed", "code --new-window"…

[agents.claude]
command = "claude"
args = ["-p", "{{prompt}}", "--dangerously-skip-permissions", "--output-format", "stream-json"]

[prices.default]
input = 3
output = 15
cache = 0.3
```

Moving a task to **ready** with an agent other than `me` creates a git worktree on `<type>/<slug>` and writes `TASK.md`. **Yolo** runs the CLI non-interactively (live log, then checks and review). **Pair** also writes `PAIR.md` and opens the agent in a tmux split or `terminal`. **Finish pair** (`f`) runs checks. Closing the terminal without finishing leaves **pair-paused**. Approve with `commits=auto` commits, pushes and opens a PR — never merge, never push `main`. `commits=none` leaves the worktree dirty. Discard (`d`) removes worktree and branch.

### After approving

With a remote, Troopa already opened a PR: use `c` to remove the worktree and keep the branch. Without a remote, `g` merges the branch into the project base (squash or merge from `config.toml`) or `c` just cleans the worktree. `d` throws worktree and branch away.

`o` opens the task's worktree in your editor (the project folder when the worktree is gone or the task has none yet). The editor comes from `editor` in `config.toml`, then `$VISUAL`, then `$EDITOR`, then `nvim`/`vim`/`vi`. Terminal editors take over the screen until you quit them; GUI editors open in their own window.

## Keys

`Ctrl+K` command palette (every action). First launch shows a welcome screen.

Projects: `enter` board · `n` new · `e` edit · `a` archive · `c` clients · `F` client filter · `M` metrics · `t` theme · `?` help · `q` quit

Clients: `n` new · `e` edit · `a` archive · `enter` client board · `F` filter projects · `esc` back

Board and list: `v` switch board/list · `z` fold the status · `Z` fold empty statuses · `m`/`P`/`A` popup to change status, priority or agent · `tab` task panel · `n` quick task · `e` edit · `enter` edit or agent log/review · `[` `]` move · `/` search · `f`/`p`/`C` filters · `M` metrics · `g` merge · `c` clean worktree · `o` editor · `esc` back

On a narrow terminal the board scrolls sideways and follows the selected column (`◀ 2` / `▶ 3` show hidden statuses). From 120 columns the task panel sits beside the board; on smaller terminals `tab` opens it full screen. The view and the folded statuses are saved as `board_view` and `collapsed` in `config.toml`.

Review: `y` approve · `g` merge · `c` clean worktree · `d` discard · `n`/`p` file · `j`/`k` scroll · `o` editor

Pair: `f` finish · `r` reopen agent · `o` editor · `esc` board (session keeps running)

Mouse: click to select, drag a card to another column, wheel to scroll.

## License

MIT. See [CONTRIBUTING.md](CONTRIBUTING.md).

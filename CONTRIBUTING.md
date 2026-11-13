# Contributing to Troopa

## Setup

Go 1.26+ and Git. From the repo root:

```bash
make test
make build
```

The TUI is `go run ./cmd/troopa`. Data: `$XDG_DATA_HOME/troopa/` (default `~/.local/share/troopa/`). Config: `$XDG_CONFIG_HOME/troopa/`.

## Pull requests

- Keep diffs scoped to the change. Match existing packages: `internal/tui`, `internal/store`, `internal/launch`, `internal/config`, `internal/cli`.
- Add or extend tests next to the code (`go test ./...`).
- Do not commit `bin/`, `dist/`, databases, or personal paths, emails, or tokens.
- Do not push to `main`. Open a PR.

## Themes

Bundled palettes live in `internal/theme/themes/*.toml`. User palettes go in `$XDG_CONFIG_HOME/troopa/themes/`. Keep the `[colors]` keys listed in the README.

## Release snapshots

Binaries are built locally; nothing is published from this repo:

```bash
goreleaser build --snapshot --clean
```

## License

MIT. See `LICENSE`.

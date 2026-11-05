# Troopa CLI JSON contract

`troopa` exposes a non-interactive JSON contract for integrations. It is versioned independently of the SQLite schema, so clients must use the CLI rather than opening `troopa.db` directly.

Every non-interactive invocation requires `--json`, emits exactly one JSON value on standard output, and includes `schema_version`. Troopa preserves fields and meanings within a schema major version; incompatible changes increment `schema_version`.

## Envelope and errors

Successful results are wrapped in `data`:

```json
{"schema_version":1,"data":[]}
```

Errors are also JSON and use exit code 2 for invalid arguments, 3 for absent resources, and 1 for operational failures:

```json
{"schema_version":1,"error":{"code":"not_found","message":"task not found"}}
```

## Commands

All IDs are positive integers. Text flags accept UTF-8 strings.

```sh
# Projects
 troopa project list --json [--archived] [--client ID]
 troopa project show --json --id ID
 troopa project create --json --name NAME --repo PATH [--slug SLUG --stack STACK --description TEXT --client ID]
 troopa project update --json --id ID [--name NAME --repo PATH --slug SLUG --stack STACK --description TEXT --client ID]
 troopa project archive --json --id ID

# Clients
 troopa client list --json [--archived]
 troopa client show --json --id ID
 troopa client create --json --name NAME [--contact TEXT --notes TEXT]
 troopa client update --json --id ID [--name NAME --contact TEXT --notes TEXT]

# Tasks
 troopa task list --json [--project ID --client ID --status STATUS --priority PRIORITY]
 troopa task show --json --id ID
 troopa task create --json --project ID --title TITLE [--description TEXT --criteria TEXT --status STATUS --priority PRIORITY --agent AGENT --model MODEL --mode MODE --commits MODE --type TYPE]
 troopa task update --json --id ID [same mutable flags]
 troopa task move --json --id ID --status STATUS
 troopa task launch --json --id ID [--status STATUS]
 troopa task approve --json --id ID
 troopa task discard --json --id ID

# Runs
 troopa run list --json [--state STATE]
 troopa run show --json --id ID
```

`task launch` queues a task in `ready` by default (or moves it to an explicit status). The TUI and CLI observe the same SQLite-backed store. `approve` and `discard` update the latest run and task state through the shared store layer.

## Example

```sh
troopa project create --json --name API --repo "$HOME/src/api"
troopa task create --json --project 1 --title 'Add health check' --status ready
```

Example output shape (timestamps are RFC 3339 UTC):

```json
{"schema_version":1,"data":{"id":1,"name":"API","slug":"api","repo_path":"/home/me/src/api","archived":false}}
```

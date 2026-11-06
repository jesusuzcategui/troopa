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
 troopa project create --json --name NAME --repo PATH [--slug SLUG --stack STACK --description TEXT --client ID --account NAME --profile ID]
 troopa project update --json --id ID [--name NAME --repo PATH --slug SLUG --stack STACK --description TEXT --client ID --account NAME --profile ID]
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
 troopa task attach --json --id ID --file PATH
 troopa task attachments --json --id ID
 troopa task detach --json --id ID --attachment ID

# Runs
 troopa run list --json [--state STATE]
 troopa run show --json --id ID
 troopa run rounds --json --id ID
 troopa run merge --json --id ID
 troopa run clean --json --id ID

# Plans
 troopa plan list --json [--project ID]
 troopa plan show --json --id ID
 troopa plan create --json --project ID --title TITLE [--goal TEXT --agent AGENT --model MODEL --manual]
 troopa plan add-step --json --id ID --title TITLE [--description TEXT --criteria TEXT --agent AGENT --model MODEL]
 troopa plan delete --json --id ID

# Profiles
 troopa profile list --json
 troopa profile show --json --id ID
 troopa profile create --json [--name NAME --profession TEXT --about TEXT --instructions TEXT]
 troopa profile update --json --id ID [same flags]
 troopa profile delete --json --id ID

# Second brain
 troopa brain sync --json
```

`plan create` makes a draft; steps are tasks with `plan_id` and `plan_step`. Plans are started, reviewed step by step and merged from the TUI (`w` on the board). `--manual` turns off auto-advance. `plan delete` keeps the steps as ordinary tasks.

`profile` stores who the agents work for; the project's profile (or `default_profile` in config.toml) is written into every TASK.md. `--account` on a project picks an `[accounts.NAME]` login from config.toml.

`task launch` queues a task in `ready` by default (or moves it to an explicit status). The TUI and CLI observe the same SQLite-backed store. `approve` and `discard` update the latest run and task state through the shared store layer.

`task attach` copies the file into `$XDG_DATA_HOME/troopa/attachments/<task-id>/` (default limit 50 MB, `max_attachment_mb` in config.toml) and returns the attachment. `task attachments` lists them. `task detach` removes the stored copy.

`run merge` and `run clean` use the same launch logic as the TUI (`g` / `c`) and return the updated run. Metrics fields are numbers or `null`, never `database/sql` types. The run must be `approved`; otherwise the error code is `invalid_state`. `merge` also requires a project without a remote. `clean` removes the worktree and keeps the branch; uncommitted user files abort the removal.

`brain sync` recreates INDEX.md, project frontmatter and task files from SQLite (idempotent). History lines and merge guides are added without duplicating existing markers. If `brain_git` is on and the folder is a git repo, Troopa makes a local commit (`troopa: sync`) and never pushes.

`run rounds` lists the agent rounds of a run (`launch` then each `x` request). Each item uses numbers or `null` for metrics, `log_from`/`log_to` and `id` (legacy runs without stored rounds appear as a single synthetic round with `"id": null`). `kind` is `launch` or `changes`; `state` is `running`, `review` or `blocked`.

## Example

```sh
troopa project create --json --name API --repo "$HOME/src/api"
troopa task create --json --project 1 --title 'Add health check' --status ready
```

Example output shape (timestamps are RFC 3339 UTC):

```json
{"schema_version":1,"data":{"id":1,"name":"API","slug":"api","repo_path":"/home/me/src/api","archived":false}}
```

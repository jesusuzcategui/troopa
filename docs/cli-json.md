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
 troopa plan create --json --project ID --title TITLE [--goal TEXT --agent AGENT --model MODEL --manual --auto-approve]
 troopa plan update --json --id ID [--title TITLE --goal TEXT --agent AGENT --model MODEL --manual=false --auto-approve]
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

`plan create` makes a draft; steps are tasks with `plan_id` and `plan_step`. Plans are started, reviewed step by step and merged from the TUI (`w` on the board). `--manual` turns off auto-advance; `--auto-approve` approves each step whose checks pass, so the plan runs alone until a step fails. `plan delete` keeps the steps as ordinary tasks.

`profile` stores who the agents work for; the project's profile (or `default_profile` in config.toml) is written into every TASK.md. `--account` on a project picks an `[accounts.NAME]` login from config.toml.

`task launch` queues a task in `ready` by default (or moves it to an explicit status). The TUI and CLI observe the same SQLite-backed store. `approve` and `discard` update the latest run and task state through the shared store layer.

`task attach` copies the file into `$XDG_DATA_HOME/troopa/attachments/<task-id>/` (default limit 50 MB, `max_attachment_mb` in config.toml) and returns the attachment. `task attachments` lists them. `task detach` removes the stored copy.

`run merge` and `run clean` use the same launch logic as the TUI (`g` / `c`) and return the updated run. Metrics fields are numbers or `null`, never `database/sql` types. The run must be `approved`; otherwise the error code is `invalid_state`. `merge` also requires a project without a remote. `clean` removes the worktree and keeps the branch; uncommitted user files abort the removal.

`brain sync` recreates INDEX.md, project frontmatter and task files from SQLite (idempotent). History lines and merge guides are added without duplicating existing markers. If `brain_git` is on and the folder is a git repo, Troopa makes a local commit (`troopa: sync`) and never pushes.

## Daemon

`troopa daemon` is a long-running process without the TUI. It does not use `--json`. Every few seconds it reconciles running agents (`engine.Reconcile`), finishes runs whose agent has exited, and advances running plans that have auto-advance. Logs go to `$XDG_DATA_HOME/troopa/daemon.log`. A pid file keeps a second `troopa daemon` from starting; it exits with a clear message. The TUI keeps working with or without the daemon.

```sh
troopa daemon
troopa daemon status
troopa daemon install
troopa daemon uninstall
```

`install` writes a systemd user unit on Linux (`~/.config/systemd/user/troopa.service`) and a launchd agent on macOS (`~/Library/LaunchAgents/dev.troopa.plist`), then enables it. `uninstall` removes that file. Windows prints that it is not supported yet. `status` prints whether the daemon is running and the runs it watches.

## MCP

`troopa mcp` is an MCP server over stdio in the same binary. It does not use `--json`. Point an MCP client at the `troopa mcp` command. Tools return JSON in the same shapes as the CLI `data` values:

```sh
troopa mcp
troopa mcp install claude    # or codex, opencode, cursor
```

`install` registers the stdio server with that client: Claude Code runs `claude mcp add troopa -- troopa mcp`; Codex writes `[mcp_servers.troopa]` in `~/.codex/config.toml`; OpenCode writes `~/.config/opencode/opencode.json`; Cursor writes `~/.cursor/mcp.json`. If the client binary is not on `PATH`, Troopa prints the snippet instead of writing.

Read tools: `projects_list`, `project_show`, `tasks_list` (filters: `project`, `status`, `plan`), `task_show`, `plans_list`, `plan_show` (steps with status and latest run), `run_show` (summary, checks, state, branch, rounds), `run_diff` (files and patch text, size-limited), `run_log` (tail of `agent.log`), `brain_search`, `brain_read` (paths inside the brain only), `profiles_list`.

Write tools fire the same hooks as the TUI: `task_create`, `task_update`, `task_move`, `plan_create` (title, goal, agent, model, auto, and steps with title, description, criteria in one call), `plan_add_step`, `plan_update`. `plan_start` and `plan_pause` go through the engine. If no daemon is running, `plan_start` watches from the MCP process and the result says that `troopa daemon` keeps plans moving. `run_wait` (`run_id` or `plan_id`, `timeout_seconds` at most 600) blocks until the run leaves running or the plan's current step changes, and returns the new state with summary and checks. On timeout it returns with the state still `running`.

`review_request_changes` (`run_id`, `feedback`, `comments` with `path`, `line`, `side`, `body`) is the same as `x` in the TUI: a new round, notes in `CHANGES.md`, and the same agent continues. `review_approve` (`run_id`) only works when `[mcp] allow_approve = true` in `config.toml` (off by default; Settings has the toggle). The error says how to turn it on. There is no merge tool: merging a plan or a task stays in the TUI (`g`).

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

## Notifications

`troopa notify telegram [--token T --chat C] [--dry-run] [--test]` is meant to run as a hook: it reads the hook event (JSON) on stdin, loads the run, task and plan from the database and sends a Telegram message (HTML) with the agent's summary, checks, time, tokens, cost and the next plan step. `TROOPA_TELEGRAM_TOKEN` and `TROOPA_TELEGRAM_CHAT` replace the flags. `--test` sends the latest run as a sample.

# docker-app-updater

Updates docker compose apps and optionally reports via Gotify push notification.

- **Discovers** compose projects by finding `docker-compose.yml`/`compose.yaml` files.
- **Detects updates** by diffing container/image IDs before and after refresh, not by guessing from uptime.
- **Reports results** via Gotify (if configured).

## Install

```sh
make compile
./bin/docker-app-updater --config ./config/config.yaml
```

## Configuration

Copy [`config/config.example.yaml`](./config/config.example.yaml) to `./config/config.yaml` and adjust for your fleet.

### Path

Checked in order, first found wins:

- `./config/config.yaml`
- `~/.config/docker-app-updater/config.yaml`
- `/etc/docker-app-updater/config.yaml`

Or set an explicit path with `--config`.

### Structure

- **dry_run** (_boolean_, default `false`): log refresh/after commands instead of running them. Docker state is still
  read, so discovery output stays accurate.
- **log_level** (_string_, default `info`): a logrus level.
- **max_threads** (_integer_, default `3`, capped at 100): apps processed concurrently.
- **command_timeout** (_string_, default `15m`): a Go duration each command may run before being killed and treated as a
  failure.
- **discovery.roots** (_[]string_): directories scanned one level deep for compose projects;
  `<root>/<name>/docker-compose.yml` is discovered as app `<name>`.
- **discovery.compose_filenames** (_[]string_, default `docker-compose.yml`, `docker-compose.yaml`, `compose.yml`,
  `compose.yaml`): filenames that mark a compose project.
- **refresh_commands** (_[][]string_, default `docker compose pull` then `docker compose up -d --remove-orphans`):
  commands run in every app's directory.
- **after_commands** (_[][]string_): commands run once, after every app has been processed.
- **excludes** (_[]string_): app names under a discovery root to ignore.
- **skip_if_no_containers** (_boolean_, default `true`): leave an app with zero containers alone rather than starting
  it. See [Skipping empty apps](#skipping-empty-apps).
- **overrides** (_map[string]object_): per-app exceptions, keyed by app name: `refresh_commands`, `after_commands`,
  `profiles` (see [Profiles](#profiles)), `skip_if_no_containers`.
- **apps** (_[]object_): explicit apps outside any discovery root; same shape as an override plus required **name** and
  **path**.
- **gotify.url**, **gotify.token**: literal Gotify credentials. Leave blank to print the report to stdout instead of
  sending. Keep the deployed config out of version control; see [Secrets](#secrets).
- **gotify.priority** (_integer_, optional, default `0`): Gotify's message priority, 0-10.
- **gotify.label** (_string_, optional): bold header prefix (e.g. `**homelab**`) so reports from different hosts can be
  told apart.

#### Commands and arguments

Every token in `refresh_commands`/`after_commands` accepts `{{app.name}}`, `{{app.path}}`, and `{{HOME}}`.

### Profiles

Profiles gate which apps update. Tags are freeform strings, not a built-in vocabulary. An app without profiles always
updates; an app with profiles only updates when `--profile <name>` matches one of them.

```yaml
overrides:
  immich:
    profiles: [heavy]
```

```sh
docker-app-updater
docker-app-updater --profile heavy
```

### Skipping empty apps

An app with zero containers is skipped by default (reported as `Skipped, no containers found`), including apps found via
`discovery.roots`, so a project you took down with `docker compose down` isn't resurrected by a nightly run. Override
with `skip_if_no_containers: false`, globally or per app under `overrides`.

### Secrets

`gotify.url`/`gotify.token` are literal values. Keep the deployed config file out of version control (e.g. under
`/etc/docker-app-updater/`).

### Deploying to a remote host

`make deploy` (or `scripts/deploy.sh`) cross-compiles for linux/amd64 and scp's the binary to a remote host, using
`VPS_HOST`/`VPS_PATH` (and optional `VPS_SSH_OPTS`) from `.env`. Copy `.env.example` to `.env` and fill in your own
values.

## Update detection

Before and after running an app's refresh commands, the tool records each service's container and image ID
(`docker compose ps`/`docker compose images`) and classifies the diff:

| Status      | container ID | image ID | Meaning                                                                 |
| ----------- | ------------ | -------- | ----------------------------------------------------------------------- |
| `updated`   | changed      | changed  | Real update shipped.                                                    |
| `recreated` | changed      | same     | Recreated, image unchanged (config/env/orphans).                        |
| `restarted` | same         | same     | Wasn't already running before this pass.                                |
| `unchanged` | same         | same     | Already up to date. Not reported.                                       |
| `skipped`   | n/a          | n/a      | Zero containers found. See [Skipping empty apps](#skipping-empty-apps). |
| `error`     | n/a          | n/a      | A command failed or timed out.                                          |

A multi-service project rolls up to its most severe status (`error` > `updated` > `recreated` > `restarted` >
`unchanged`). `skipped` is decided before any commands run.

## CLI flags

- `--config <path>`: config file path
- `--profile <name>`: see [Profiles](#profiles)
- `--dry-run`: force dry-run regardless of config
- `--discover-only`: print the resolved app list and exit, without updating anything
- `--no-notify`: skip the Gotify notification; print the report to stdout instead
- `--log-level <level>`: a logrus level, e.g. `debug`/`info`/`warn` (overrides config)
- `--max-threads <n>`: concurrency cap, up to 100 (overrides config)
- `--command-timeout <duration>`: per-command timeout, e.g. `15m` (overrides config)

## Development

```sh
make            # deps, fmt, lint, test, compile, vuln
make test
```

`internal/exec.Executor` fakes `docker`/command output in tests, so orchestration logic is fully testable without a real
Docker daemon.

### Pre-commit hooks

[`pre-commit`](https://pre-commit.com) runs `gofumpt`, `golangci-lint`, `go test -race`, and
[`gitleaks`](https://github.com/gitleaks/gitleaks) (staged files only) on every commit. Install once per clone:

```sh
pre-commit install
```

Run against the whole tree with `pre-commit run --all-files`. Gitleaks rules live in `.gitleaks.toml`; hook definitions
are in `.pre-commit-config.yaml`.

## AI use

Planned and built with AI assistance (Claude Sonnet 5, Anthropic), under a strict human review process: every change was
reviewed, tested, and approved by the engineer before landing.

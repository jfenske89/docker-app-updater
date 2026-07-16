# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A CLI that updates docker compose apps on a host and optionally reports results via a Gotify push notification (Gotify
is unconfigured by default; the report prints to stdout instead, see the invariant below). It discovers compose projects
on disk, runs configured refresh commands for each, detects whether anything _actually_ changed by diffing real Docker
state (container/image IDs) before and after — not by guessing from container uptime — and posts a summary.

## Commands

```sh
make                # full pipeline: deps, fmt, lint, test, compile, vuln
make deps           # go mod tidy
make deps-update    # go get -u ./... && go mod tidy
make fmt            # gofumpt + prettier on *.md
make lint           # golangci-lint run --verbose
make test           # go test -v ./...
make compile        # build ./bin/docker-app-updater
make vuln           # govulncheck
make modernize      # check for modernize-able Go patterns (go/analysis modernize pass)
make modernize-fix  # apply modernize fixes, then re-run test + lint
make deploy         # cross-compile linux/amd64 and scp to a remote host (scripts/deploy.sh)
```

Run a single test or package directly with `go test`, e.g.:

```sh
go test ./internal/update/... -run TestClassify -v
```

**Before considering any code change finished**: it must compile, lint clean, and pass tests, with no new
vulnerabilities — i.e. `make` (or at minimum `go build ./...`, `golangci-lint run`, `go test ./...`,
`govulncheck ./...`) should all succeed. `.golangci.yml` enables `gosec`; suppress a specific false positive with a
`//nolint:gosec // G2xx: <reason>` comment on that line rather than disabling the linter.

`pre-commit` (`.pre-commit-config.yaml`) runs `gofumpt`, `golangci-lint --new-from-rev=HEAD~1`, `go test -race`, and
`gitleaks protect --staged` on every commit — the same checks above, plus a staged-diff secret scan. Rules are in
`.gitleaks.toml` (extends gitleaks' default ruleset). If a commit is ever blocked by gitleaks on a genuine
false-positive (not an actual credential), fix it by adjusting `.gitleaks.toml`, not by bypassing the hook.

## Architecture

Execution flows through a small pipeline, each stage its own package under `internal/`, wired together in
`cmd/docker-app-updater/main.go`:

1. **`config`** — loads and validates YAML (`config.Load`), resolving the file from `--config` or one of three
   well-known paths (`./config/config.yaml`, `~/.config/docker-app-updater/config.yaml`,
   `/etc/docker-app-updater/config.yaml`). Also normalizes/validates values shared with CLI flag overrides (see below).
2. **`discovery`** — `Discover` walks `Discovery.Roots` one level deep for compose files, merges in explicit `cfg.Apps`,
   applies `Excludes`, then layers `Overrides` (keyed by app name) on top. Produces the final, sorted `[]App` list.
   `FilterByProfile` then narrows that list by `--profile`: an app with no `Profiles` always runs; an app with
   `Profiles` only runs when `--profile` matches one of its tags.
3. **`update`** — `Run` orchestrates one app: snapshot Docker state (`dockercli.Snapshot`) → run `RefreshCommands`
   (`runner.RunAll`) → snapshot again → run `AfterCommands` → `Classify` the before/after diff into a `Status`. Apps run
   concurrently (one goroutine per app via `sourcegraph/conc/pool`, bounded by `MaxThreads`) from `main.go`.
4. **`dockercli`** — wraps `docker compose ps`/`docker compose images` to fingerprint each service's container ID, image
   ID, and running state; tolerant of both single-JSON-array and newline-delimited-JSON compose output across versions.
5. **`runner`** — executes a `[][]string` command list via the `exec.Executor` seam, substituting
   `{{app.name}}`/`{{app.path}}`/`{{HOME}}` in every token, applying `CommandTimeoutDuration`, and honoring `dry_run`
   (logs instead of executing).
6. **`report`** — turns `[]update.Result` into the notification message body, grouped by status in fixed display order
   (`updated` → `recreated` → `restarted` → `skipped` → `error`); `unchanged` is never reported. Returns `""` when
   there's nothing to say.
7. **`notify/gotify`** — minimal Gotify REST client: a single `POST {url}/message` with an `X-Gotify-Key` header,
   requesting Markdown rendering via the `extras` field. No DM-channel step, no chunking, no SDK dependency. Entirely
   optional: `main.go` only constructs and calls this client when both `gotify.url` and `gotify.token` are set.

### Key invariants

- **Status classification** (`update.Classify`) is the core domain logic: `updated` (container + image both changed) >
  `recreated` (container changed, image same) > `restarted` (same container, wasn't running before) > `unchanged`, plus
  `error` and `skipped` as special cases decided outside the diff. A multi-service compose project rolls up to its most
  severe per-service status (`severity` map in `internal/update/update.go`).
- **`exec.Executor`** (`internal/exec/exec.go`) is the one seam between orchestration logic and the real world
  (`os/exec`). Every package that shells out takes an `Executor` as a parameter instead of calling `os/exec` directly,
  which is what makes `discovery`/`update`/`runner`/`dockercli` fully testable without a real Docker daemon — tests
  inject a fake `Executor`.
- **CLI flags override config, not the other way around.** `main.go` loads config first, then applies flag overrides
  (`--dry-run`, `--log-level`, `--max-threads`, `--command-timeout`, plus `--profile`/`--discover-only`/`--no-notify` as
  pure CLI behavior). `--max-threads` and `--command-timeout` re-run the same
  `config.NormalizeMaxThreads`/`config.NormalizeCommandTimeout` validation used at load time, so both paths enforce
  identical caps/fallbacks. When adding a new overridable setting, follow this pattern rather than validating in
  `main.go` directly.
- **`skip_if_no_containers`** cascades: global default (`true`) → per-app override (`Overrides[name].SkipIfNoContainers`
  or `App.SkipIfNoContainers`, both `*bool` so nil means "inherit"). `Config.SkipsAppsWithoutContainers()` is the one
  place that resolves the global default; `update.Run` layers the per-app override on top of it.
- **Gotify is optional, off by default, and never an error condition.** Credentials are literal config values, not read
  from the environment. If `gotify.url`/`gotify.token` are unset, or `--no-notify` is passed, or `dry_run` is active,
  the report is printed to stdout instead of sent — never treat a missing/incomplete Gotify config as an error.

### Config files are not to be read as part of exploration

`config/config.yaml` and `.env` are gitignored and may contain real credentials or host-specific paths for whoever is
running this on their own fleet — don't open them while exploring the codebase. `config/config.example.yaml` and
`.env.example` are the safe, checked-in references for their respective shapes.

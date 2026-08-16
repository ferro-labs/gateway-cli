# Changelog

All notable changes to **Ferro Operator Console**, the scriptable CLI and
interactive TUI for Ferro Labs AI Gateway. The binary it installs is `ferro`.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## v0.1.0 — 2026-08-14

First release. One binary, `ferro`, with two surfaces over a single HTTP client:
scriptable commands for automation, and a full-screen console for operators.
Requires AI Gateway v1.4.2 or later.

### Added

- **Commands** — `status`, `models`, `providers`, `mcp`, `plugins`, `services`,
  `sessions`, `audit`, `keys list|get|create|rotate|revoke`,
  `logs list|stats|tail`, `chat`, and `version`. Every one supports
  `--format table|json|yaml`.
- **Console** — run bare `ferro` at a TTY for a full-screen operations view with
  four screens: Home, Request logs, Keys, and Playground. Includes a command
  composer with completion, history, and reverse search (`ctrl+r`).
- **Request-log tail** — follow live traffic in `logs tail` or the console, with
  filters for time, model, provider, stage, and credential.
- **Key management** — list, create, rotate, and revoke API keys, with derived
  `active` / `expired` / `revoked` state. New secrets are printed once, to
  stdout, and never enter the console transcript or history. `rotate` and
  `revoke` are irreversible, so at a terminal each asks for the key id typed
  back before it runs; `--yes` skips the prompt, and is required when stdin is
  not a terminal rather than the verb blocking or proceeding unasked.
- **Playground** — streaming chat against the gateway with `/model` and
  `/clear`, showing token usage plus route and cost when available.
- **Connection profiles** at `os.UserConfigDir()/ferro/config.yaml`. URL
  resolution: `--gateway-url` > `FERRO_URL` > profile > `http://localhost:8080`.
  Key resolution: `FERRO_API_KEY` > profile `api_key_env` > `MASTER_KEY`, the
  last of which applies only when the gateway URL is loopback. `MASTER_KEY` is
  the gateway server's own variable and is not chosen for ferro by anyone, so
  it is the one source that could otherwise be forwarded to a remote host the
  operator named on the command line. Keys are never passed as flags, so they
  stay out of shell history.
- **Global flags** `--gateway-url`, `--profile`, `--format`, `--ascii`, plus
  `NO_COLOR`, TTY, and `TERM=dumb` detection.

### Output contract

- stdout carries data; narration, warnings, and errors go to stderr — so
  `--format json` is always safe to pipe.
- `status` exits 1 only when the gateway is unreachable. A reachable but
  degraded gateway exits 0 and reports its degraded state.
- Missing measurements render `-`, never `0`. Endpoints the gateway does not
  serve disable that panel with a hint instead of failing the command.
- Gateway-supplied text is data, never layout or terminal control. Control
  characters in a provider name, an upstream error message, or a model's answer
  are neutralized on every surface, so a table keeps its columns, a pane keeps
  its line count, and nothing upstream can drive the terminal.

### Distribution

- Release binaries are reproducible: `-trimpath`, and both the build stamp and
  the archive timestamps come from the tagged commit rather than the build
  clock, so two builds of one tag are byte-identical.
- An SPDX SBOM ships beside each archive.
- `go install` builds, which carry no linker stamp, now report their module
  version and commit from the embedded build info instead of `dev`/`none`.

### Known limitations

- The gateway does not report its version over HTTP, so the console's version
  slot renders `—`.
- The traffic panel and route attribution come from the request log, and show as
  unavailable when no request-log store is configured.
- Not yet shipped: config viewer / history / rollback, provider capability
  matrix, doctor checklist, and init wizard.

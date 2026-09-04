# AGENTS.md

## Layout

This directory is the Go module root (`agent-events/server`). The repo root above it contains
no code. Run all Go/Make commands from here, not the repo root.

## Commands (from this directory)

- `make all` — lint → test → build (use this to verify changes)
- `make test` — `go test ./... -race -count=1` (always race + no cache)
- `make lint` / `make fmt` — golangci-lint (v2 config in `.golangci.yml`)
- `make run` — starts the API on `$PORT` (default 8080)
- Single test: `go test ./internal/core/usecase/ -run TestName -race -count=1`

Toolchain (go, golangci-lint) is pinned via `mise.toml`.

## Architecture

Hexagonal / ports-and-adapters, wired with uber/fx in `cmd/api/main.go`:

- `internal/core/` — `domain`, `usecase`, `port` (interfaces). Must not import adapters, transport, or zap.
- `internal/adapters/` — `memory` (in-memory repo), `logger` (zap impl of `port.Logger`).
- `internal/transport/http/` — `controller` (chi router + middleware), `handler`, `dto`.
- `pkg/` — `apperr` (error kinds), `config` (env loading).

Adding a component: write it against a `port` interface, provide the impl in `fx.Provide` in `main.go`
(use `fx.Annotate(..., fx.As(new(port.X)))` for interface binding).

Storage is in-memory only — data is lost on restart; there is no DB or migration flow.

## Conventions

- No code comments — not even doc comments on exported symbols — unless explicitly requested.
- Errors: return `apperr.NotFound` / `apperr.Invalid` / `apperr.Wrap` from use cases; handlers pass them
  to `WriteError`, which maps kind → HTTP status (`pkg/apperr/apperr.go`, `handler/response.go`).
  Repos return sentinel errors (e.g. `domain.ErrEventNotFound`); use cases translate them to apperr.
- Logging: use `port.Logger` with `port.Str/Int/Err/...` fields. Never import zap outside
  `internal/adapters/logger`.
- Config: env vars `PORT`, `ENV`, `LOG_LEVEL` (see `.env.example`); loaded via godotenv + viper,
  so a local `.env` here is picked up automatically. `ENV=production` switches logs to JSON.

## API

`GET /healthz`, CRUD under `/api/v1/events` (`POST/GET /`, `GET/PUT/DELETE /{id}`).

## Maintenance

Keep this file current: when your work changes commands, conventions, or architecture (e.g. a real
storage backend replaces the in-memory repo), update the relevant section instead of leaving it
stale — but always ask the user for permission before editing this file. Remove guidance that no
longer applies; add only what an agent would otherwise get wrong.

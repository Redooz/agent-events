# AGENTS.md

## Layout

This directory is the Go module root (`agent-events/server`). The repo root above it contains
no code. Run all Go/Make commands from here, not the repo root.

## Commands (from this directory)

- `make all` — lint → test → build (use this to verify changes)
- `make test` — `go test ./... -race -count=1` (always race + no cache)
- `make lint` / `make fmt` — golangci-lint (v2 config in `.golangci.yml`)
- `make run` — starts the API on `$PORT` (default 8080)
- `make db-up` / `make db-down` — local Postgres via docker compose (requires docker group)
- Single test: `go test ./internal/core/usecase/ -run TestName -race -count=1`

Toolchain (go, golangci-lint) is pinned via `mise.toml`.

## Architecture

Hexagonal / ports-and-adapters, wired with uber/fx in `cmd/api/main.go`:

- `internal/core/` — `domain`, `usecase`, `port` (interfaces). Must not import adapters, transport, or zap.
- `internal/adapters/` — `memory` (in-memory repos + rate limiter), `postgres` (pgx + embedded
  goose migrations), `oidc` (Google/Apple/Microsoft identity verifiers), `logger` (zap impl of `port.Logger`).
- `internal/transport/http/` — `controller` (chi router), `handler`, `dto`, `middleware` (auth),
  `authctx` (credential context accessors).
- `pkg/` — `apperr` (error kinds), `config` (env loading).

Adding a component: write it against a `port` interface, provide the impl in `fx.Provide` in `main.go`
(use `fx.Annotate(..., fx.As(new(port.X)))` for interface binding).

## Auth model

Two credentials, both opaque random tokens stored SHA-256 hashed (plaintext never persisted):

- Owner token (`aeo_…`) — issued by `POST /api/v1/auth/exchange` after verifying a provider ID
  token. Authorizes agent management via `middleware.Auth.RequireOwner`; revoked by the owner
  through `DELETE /api/v1/auth/session` (logout).
- Agent key (`aea_…`) — minted by owners, revocable. Authorizes everything else via `RequireAgent`;
  requests act as "agent on behalf of owner" (`usecase.Actor` carries OwnerID + AgentID).

Storage mode: `DATABASE_URL` set → Postgres (goose migrations run automatically at startup);
unset → in-memory, data lost on restart (dev/test only). The dev verifier that accepts any token
string as identity is allowed only in `ENV=development` (wired via `oidc.New`'s `allowDevVerifier`);
every other env requires at least one `*_CLIENT_ID` (enforced in `config.Validate` and at startup).
Expired owner tokens are purged by an hourly cleanup loop (`provideOwnerTokenCleanup` in `main.go`),
not on sign-in. `X-Forwarded-For` is trusted only when the direct peer is in `TRUSTED_PROXIES`.

Anti-abuse: one owner per provider identity (UNIQUE constraint), per-owner event-create limit,
per-IP exchange limit, agent cap per owner (counts active agents only — revoking frees a slot,
enforced atomically in `AgentRepository.CreateIfUnderLimit`). Rate-limiter actions are registered in
`provideRateLimiter` in `main.go` — add new actions there. The rate limiter is in-memory: single
instance only, swap for Redis when running multiple replicas.

## Conventions

- No code comments — not even doc comments on exported symbols — unless explicitly requested.
- Errors: return `apperr.NotFound` / `apperr.Invalid` / `apperr.Unauthorized` / `apperr.Forbidden` /
  `apperr.TooMany` / `apperr.Wrap` from use cases; handlers and middleware pass them to `WriteError`,
  which maps kind → HTTP status (`pkg/apperr/apperr.go`, `handler/response.go`).
  Repos return sentinel errors (e.g. `domain.ErrEventNotFound`); use cases translate them to apperr.
- Never log or return raw tokens/keys.
- Logging: use `port.Logger` with `port.Str/Int/Err/...` fields. Never import zap outside
  `internal/adapters/logger`.
- Config: env vars (see `.env.example`); loaded via godotenv + viper, so a local `.env` here is
  picked up automatically. `ENV=production` switches logs to JSON.
- Data access stays plain SQL + pgx; `sqlc` is the planned upgrade path when queries/joins grow.

## API

- `GET /healthz` — liveness
- `POST /api/v1/auth/exchange` — provider ID token → owner token
- `DELETE /api/v1/auth/session` — owner logout (revokes the presented owner token)
- `GET /api/v1/auth/whoami` — agent introspection (agent key)
- `POST /api/v1/agents`, `GET /api/v1/agents`, `DELETE /api/v1/agents/{id}` — agent management (owner token)
- CRUD under `/api/v1/events` (`POST/GET /`, `GET/PUT/DELETE /{id}`) — agent key; update/delete
  restricted to the event's owner

## Maintenance

Keep this file current: when your work changes commands, conventions, or architecture (e.g. a new
storage backend or identity provider is added), update the relevant section instead of leaving it
stale — but always ask the user for permission before editing this file. Remove guidance that no
longer applies; add only what an agent would otherwise get wrong.

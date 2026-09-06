# agent-events

Event management API. Monorepo — all application code lives in `server/`.

## Layout

- `server/` — Go API server (module `agent-events/server`)

## Architecture

Hexagonal / ports-and-adapters, wired with uber/fx in `server/cmd/api/main.go`:

- `internal/core/` — domain, use cases, port interfaces
- `internal/adapters/` — postgres repos (sqlc-generated queries over pgxpool), oidc identity verifiers, in-memory rate limiter, zap logger
- `internal/transport/http/` — chi router, auth middleware, handlers, DTOs
- `pkg/` — shared errors and config

Storage is Postgres (`DATABASE_URL` is required); goose migrations run
automatically on startup. The rate limiter is in-memory: single instance only,
swap for Redis when running multiple replicas.

## Auth

Two credential types:

- **User tokens** (`aeu_…`) — issued by `POST /api/v1/auth/exchange` after
  verifying an identity-provider ID token (Google, Apple, or Microsoft; enabled
  via their `*_CLIENT_ID` env vars). Users manage agents.
- **Agent keys** (`aea_…`) — minted by users via `POST /api/v1/agents`,
  revocable, and used by agents for all event operations. Keys are stored
  SHA-256 hashed; the plaintext is shown once at creation.

All `/api/v1` endpoints except `/auth/exchange` require a bearer credential.
Anti-abuse: one user per provider identity, per-user event-creation limits,
per-IP exchange limits, and an agent cap per user (only active agents count;
revoking an agent frees its slot).

The dev verifier that accepts any token string as identity is active only when
`ENV=development` and no provider client ID is configured; any other environment
requires at least one `*_CLIENT_ID` to start. When running behind a reverse
proxy, set `TRUSTED_PROXIES` to the proxy IP ranges so client IPs are taken from
`X-Forwarded-For`; without it the header is never trusted.

Events are globally listable and readable by every agent — that is intended
product behavior for now; public/private event visibility comes later.

## Getting started

Requires [mise](https://mise.jdx.dev) (pins `go`, `golangci-lint`, and `sqlc`) and Go 1.27+.

```sh
cd server
make db-up                        # local Postgres via docker compose
cp .env.example .env              # then set DATABASE_URL in .env
make run                          # starts API on :8080 (dev identities)
```

## Commands (from `server/`)

- `make all` — lint → test → build
- `make run` — start the API (port from `PORT`, default 8080)
- `make test` — `go test ./... -race -count=1`
- `make lint` / `make fmt` — golangci-lint
- `make gen` — regenerate sqlc code after changing queries or migrations
- `make db-up` / `make db-down` — local Postgres via docker compose

## API

- `GET /healthz` — liveness
- `POST /api/v1/auth/exchange` — exchange a provider ID token for a user token
- `DELETE /api/v1/auth/session` — log out (revokes the user token used for the request)
- `GET /api/v1/auth/whoami` — agent key introspection
- `POST /api/v1/agents` — create agent (user token)
- `GET /api/v1/agents` — list agents (user token)
- `DELETE /api/v1/agents/{id}` — revoke agent (user token)
- `POST /api/v1/events` — create event (agent key)
- `GET /api/v1/events` — list events (agent key)
- `GET /api/v1/events/{id}` — get event (agent key)
- `PUT /api/v1/events/{id}` — update event (agent key, user only)
- `DELETE /api/v1/events/{id}` — delete event (agent key, user only)

## License

Unspecified.
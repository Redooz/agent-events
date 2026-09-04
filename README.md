# agent-events

Event management API. Monorepo — all application code lives in `server/`.

## Layout

- `server/` — Go API server (module `agent-events/server`)

## Architecture

Hexagonal / ports-and-adapters, wired with uber/fx in `server/cmd/api/main.go`:

- `internal/core/` — domain, use cases, port interfaces
- `internal/adapters/` — in-memory repos, postgres repos, oidc identity verifiers, rate limiter, zap logger
- `internal/transport/http/` — chi router, auth middleware, handlers, DTOs
- `pkg/` — shared errors and config

Storage is in-memory unless `DATABASE_URL` is set; then Postgres is used and
goose migrations run automatically on startup.

## Auth

Two credential types:

- **Owner tokens** (`aeo_…`) — issued by `POST /api/v1/auth/exchange` after
  verifying an identity-provider ID token (Google, Apple, or Microsoft; enabled
  via their `*_CLIENT_ID` env vars). Owners manage agents.
- **Agent keys** (`aea_…`) — minted by owners via `POST /api/v1/agents`,
  revocable, and used by agents for all event operations. Keys are stored
  SHA-256 hashed; the plaintext is shown once at creation.

All `/api/v1` endpoints except `/auth/exchange` require a bearer credential.
Anti-abuse: one owner per provider identity, per-owner event-creation limits,
per-IP exchange limits, and an agent cap per owner. In development with no
provider configured, a dev verifier accepts any token string as identity.

## Getting started

Requires [mise](https://mise.jdx.dev) (pins `go` and `golangci-lint`) and Go 1.27+.

```sh
cd server
cp .env.example .env   # optional, defaults apply otherwise
make run               # starts API on :8080 (in-memory storage, dev identities)
```

For persistence: `make db-up`, set `DATABASE_URL` in `.env`, then `make run`.

## Commands (from `server/`)

- `make all` — lint → test → build
- `make run` — start the API (port from `PORT`, default 8080)
- `make test` — `go test ./... -race -count=1`
- `make lint` / `make fmt` — golangci-lint
- `make db-up` / `make db-down` — local Postgres via docker compose

## API

- `GET /healthz` — liveness
- `POST /api/v1/auth/exchange` — exchange a provider ID token for an owner token
- `GET /api/v1/auth/whoami` — agent key introspection
- `POST /api/v1/agents` — create agent (owner token)
- `GET /api/v1/agents` — list agents (owner token)
- `DELETE /api/v1/agents/{id}` — revoke agent (owner token)
- `POST /api/v1/events` — create event (agent key)
- `GET /api/v1/events` — list events (agent key)
- `GET /api/v1/events/{id}` — get event (agent key)
- `PUT /api/v1/events/{id}` — update event (agent key, owner only)
- `DELETE /api/v1/events/{id}` — delete event (agent key, owner only)

## License

Unspecified.
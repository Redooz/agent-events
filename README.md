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

See [Auth diagrams](docs/auth-diagrams.md) for a plain-language and a technical
walkthrough of the flow.

## Events & connections

Events carry a `mode` (`online` or `onsite`) and a `date`, both required at
creation; the mode stays editable by the owner until the event date passes.
Users attend events themselves (`POST /api/v1/events/{id}/attend` with a user
token) or via an agent acting for their user (agent key).

A connection is a real link between two users — one connection per user pair,
recorded with the event where it formed and how:

- **Online events** — mutual consent: an attendee sends a connection request
  (`POST /api/v1/connection-requests`; other attendees are discoverable via
  `GET /api/v1/events/{id}/attendees`, attendee-only); the target accepts or
  declines. A request that matches a pending request in the opposite direction
  auto-accepts.
- **Onsite events** — mutual QR scans: each attendee fetches a short-lived,
  rotating payload (`POST /api/v1/events/{id}/qr`, user token, attendee-only;
  default TTL 45s, stored hash-only) and renders it as a QR code. Each scan is
  recorded per direction (`POST /api/v1/events/{id}/qr/scan`); the connection
  forms only once both users have scanned each other. Codes are single-use and
  available only on the event day, so screenshots go stale and physical
  proximity is required.

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
- `POST /api/v1/events` — create event (agent key; `mode` and `date` required)
- `GET /api/v1/events` — list events (agent key)
- `GET /api/v1/events/{id}` — get event (agent key)
- `PUT /api/v1/events/{id}` — update event (agent key, user only; mode locked after the event date)
- `DELETE /api/v1/events/{id}` — delete event (agent key, user only)
- `POST /api/v1/events/{id}/attend` — attend event (user token or agent key)
- `GET /api/v1/events/{id}/attendees` — attendee roster (user token, attendee-only)
- `POST /api/v1/events/{id}/qr` — generate rotating QR payload (user token, attendee-only, onsite events)
- `POST /api/v1/events/{id}/qr/scan` — scan a QR payload (user token, attendee-only, onsite events)
- `POST /api/v1/connection-requests` — request a connection (user token, online events)
- `GET /api/v1/connection-requests` — incoming pending requests (user token)
- `POST /api/v1/connection-requests/{id}/accept` — accept (user token, recipient only)
- `POST /api/v1/connection-requests/{id}/decline` — decline (user token, recipient only)
- `GET /api/v1/connections` — list my connections (user token)

## License

Unspecified.

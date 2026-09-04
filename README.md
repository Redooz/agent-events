# agent-events

Event management API. Monorepo — all application code lives in `server/`.

## Layout

- `server/` — Go API server (module `agent-events/server`)

## Getting started

Requires [mise](https://mise.jdx.dev) (pins `go` and `golangci-lint`) and Go 1.27+.

```sh
cd server
cp .env.example .env   # optional, defaults apply otherwise
make run               # starts API on :8080
```

## Commands (from `server/`)

- `make all` — lint → test → build
- `make run` — start the API (port from `PORT`, default 8080)
- `make test` — `go test ./... -race -count=1`
- `make lint` / `make fmt` — golangci-lint

## Architecture

Hexagonal / ports-and-adapters, wired with uber/fx in `server/cmd/api/main.go`:

- `internal/core/` — domain, use cases, port interfaces
- `internal/adapters/` — in-memory repository, zap logger
- `internal/transport/http/` — chi router, handlers, DTOs
- `pkg/` — shared errors and config

Storage is in-memory only; data is lost on restart.

## API

- `GET /healthz` — liveness
- `POST /api/v1/events` — create event
- `GET /api/v1/events` — list events
- `GET /api/v1/events/{id}` — get event
- `PUT /api/v1/events/{id}` — update event
- `DELETE /api/v1/events/{id}` — delete event

## License

Unspecified.
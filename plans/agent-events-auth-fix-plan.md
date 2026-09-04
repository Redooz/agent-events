# Plan: Fix the auth review findings (branch `feature/auth`)

This file is a handover for a new session. It explains what to fix, why, and how.
Language is kept simple on purpose.

## Decisions (already made — do not re-open these)

- **H4 rejected.** Global event listing is intended product behavior. Agents
  can list and read all events. No `?mine=true` filter for now. Public/private
  events come later.
- **Rate limiter stays in-memory.** Add a short code comment above the rate
  limiter: single instance only, swap for Redis when running multiple replicas.
  (The project rule is "no comments unless requested" — this one is requested.)
- **Data layer stays plain SQL + pgx.** `sqlc` is the named upgrade path when
  queries/joins grow. Note that in `server/AGENTS.md`.
- **M4 (rate limiter rewrite) is skipped.** H2's eviction bound covers the real
  risk. No background goroutine, no sharding.

## Where things are

- Repo root: `/home/nico/Projects/agent-events`
- All Go code lives in `server/`. Run every Go/Make command from `server/`.
- Check code first: `make test` and `make lint` must pass when you finish.
- Do not commit. The user decides about commits.

## Background

A code review found problems in the new auth code. The findings were checked
again and confirmed real. One finding (H4) was rejected by the product owner:
all agents should see all events. That is a decision, not a bug.

Severity legend used in this file:
- **Must fix** = real bug or real security risk.
- **While we are here** = small improvement in code we touch anyway.

---

## 1. MUST FIX — H2: Rate limit can be bypassed with a fake header

File: `server/internal/transport/http/handler/auth.go` (function `clientIP`, lines ~68-82)

Problem: The server reads the client IP from the `X-Forwarded-For` header.
Anyone can send any value in this header. The rate limit for sign-in
(`/api/v1/auth/exchange`) is keyed on this IP, so an attacker can send a
random IP in every request and never get limited. This is the only
protection on an endpoint that has no login.

Also, the in-memory rate limiter (`server/internal/adapters/memory/rate_limiter.go`)
can grow without limit if many fake IPs arrive.

Fix:
- Add a new config value `TRUSTED_PROXIES` (list of IP ranges, comma
  separated, like `10.0.0.0/8,172.16.0.0/12`). Default: empty.
  Empty means: never trust `X-Forwarded-For`.
- Only read `X-Forwarded-For` when the direct peer (`r.RemoteAddr`) is in
  the trusted list. When reading it, take the right-most entry that is not
  a trusted proxy.
- Pass the trusted list into `NewAuthHandler`. Wire it in `cmd/api/main.go`.
- In `memory/rate_limiter.go`: after `prune`, if the map is still bigger
  than `maxTrackedBuckets`, delete entries until it is small enough. This
  keeps memory bounded. Do NOT rewrite the limiter (no background goroutine).
- Add the requested comment near the limiter: single instance only; swap
  for Redis when running multiple replicas.

Tests:
- Unit test `clientIP`: trusted peer with header, untrusted peer with
  header, broken header values.
- Rate limiter test: flood with many keys, map size stays bounded.

Config/docs: add `TRUSTED_PROXIES` to `server/.env.example` with a comment.

---

## 2. MUST FIX — H3 + M3: Revoked agents still count against the limit

Files:
- `server/internal/core/usecase/auth.go` (function `CreateAgent`, lines ~208-247)
- `server/internal/core/port/agent_repository.go`
- `server/internal/adapters/postgres/agent_repository.go` (`CountByOwner`, line ~83)
- `server/internal/adapters/memory/agent_repository.go` (`CountByOwner`, line ~68)

Problem: The error message says "agent limit reached, revoke an agent
first". But `CountByOwner` counts ALL agents, including revoked ones. So
revoking never frees a slot. After N agents in total (default N=10), the
owner can never create an agent again. Forever.

Also (M3, small): count-then-create is not atomic. Two parallel requests
can both pass the check. Impact is small (11 instead of 10) but we fix it
because we touch this code anyway.

Fix (do both adapters):
- Replace `CountByOwner` + `Create` with one port method:
  `CreateIfUnderLimit(ctx context.Context, agent domain.Agent, max int) (bool, error)`
  Returns `false` when the owner is at the limit. `true` means the agent
  was created.
- Postgres: one transaction. `SELECT ... FOR UPDATE` on the owner row to
  lock it, then `COUNT(*) ... WHERE owner_id=$1 AND revoked_at IS NULL`,
  then insert if under max.
- Memory: count (skipping revoked) and insert while holding the mutex.
- In `CreateAgent`: if the method returns `false`, return
  `apperr.Forbidden("agent limit reached, revoke an agent first")`.
- Remove `CountByOwner` from the port if nothing else uses it.

Tests:
- Usecase test: create up to the limit, revoke one, create again — must
  succeed.

---

## 3. MUST FIX — H5: Wrong ID format gives 500 instead of 404 (Postgres)

Files:
- `server/internal/transport/http/handler/event.go`
- `server/internal/transport/http/handler/agent.go`
- `server/internal/adapters/postgres/event_repository.go` (function `scanEvent`)
- `server/internal/adapters/postgres/agent_repository.go` (function `scanAgent`)

Problem: The `{id}` URL parameter goes straight into SQL queries against
UUID columns. If the id is not a UUID (example: `GET /api/v1/events/missing`),
Postgres returns error `invalid input syntax for type uuid` (SQLSTATE 22P02).
The code only maps "no rows" to 404, so this becomes a 500. The tests
expect 404, but tests use the in-memory repo, so nobody noticed.

Fix (two layers):
- Handler layer: in `get`, `update`, `delete` (event handler) and `revoke`
  (agent handler), check `uuid.Parse(chi.URLParam(r, "id"))`. If it fails,
  return `apperr.NotFound("event not found")` (or "agent not found").
- Repository layer: also map SQLSTATE 22P02 to `domain.ErrEventNotFound`
  / `domain.ErrAgentNotFound` in `scanEvent` and `scanAgent`. Use the pgx
  error type (`*pgconn.PgError`, field `Code`) to check the code.

Tests:
- Handler tests: `GET /api/v1/events/not-a-uuid` and
  `DELETE /api/v1/agents/not-a-uuid` must return 404.

---

## 4. MUST FIX — H1: Dev verifier only allowed in development

Files:
- `server/internal/adapters/oidc/verifier.go` (function `newWithOptions`, lines ~75-83)
- `server/pkg/config/config.go`
- `server/cmd/api/main.go` (function `provideIdentityVerifier`)

Problem: When no identity provider client IDs are set, the server installs
a "dev verifier" that accepts ANY string as identity. This happens in every
environment, not only development. `config.Validate` blocks this only when
`ENV=production` exactly. A deploy with `ENV` unset (default "development")
or `ENV=staging` and no client IDs = anyone can log in as anyone.

Fix:
- Add `allowDevVerifier bool` to `providerOptions` and to `oidc.New(...)`.
- When no providers are configured and `allowDevVerifier` is false, return
  an error (server must not start).
- Add `IsDevelopment()` to config (`Env == "development"`).
- Pass `cfg.IsDevelopment()` from `main.go`. Log a clear warning at startup
  when the dev verifier is active.
- Also update `config.Validate`: require at least one provider client ID
  for every env that is not development (not only "production").

Tests:
- `newWithOptions` with `allowDevVerifier: false` and no providers → error.
- `allowDevVerifier: true` → current dev behavior (adapt the existing test
  `TestMultiVerifierDevModeWhenUnconfigured`).

---

## 5. MUST FIX — M5: Owner logout (revoke owner token)

Files:
- `server/internal/core/port/owner_token_repository.go`
- `server/internal/adapters/postgres/owner_token_repository.go`
- `server/internal/adapters/memory/owner_token_repository.go`
- `server/internal/transport/http/authctx/authctx.go`
- `server/internal/transport/http/middleware/auth.go`
- `server/internal/core/usecase/auth.go`
- `server/internal/transport/http/handler/auth.go`
- `server/internal/transport/http/handler/agent.go`
- `server/internal/transport/http/controller/controller.go`

Problem: There is no way to log out. A leaked owner token (`aeo_...`) is
valid for 30 days and nothing can stop it.

Fix:
- Port: add `DeleteByHash(ctx context.Context, tokenHash string) error`.
  Implement it in memory and postgres.
- `authctx`: store `OwnerIdentity{Owner domain.Owner, TokenHash string}`
  instead of only `domain.Owner`. Update `WithOwner` and `OwnerFrom`.
- `middleware/auth.go` `RequireOwner`: after successful auth, compute
  `hashToken(raw)` and store it in the identity. Note: `hashToken` is in
  package `usecase` and not exported — export it (for example
  `usecase.HashToken`) or move it to a shared place.
- Usecase: add `RevokeOwnerToken(ctx context.Context, tokenHash string) error`.
- Handler: add `Logout` in `handler/auth.go`. It reads the identity from
  context and calls `RevokeOwnerToken`. Returns 204.
- Route: `DELETE /api/v1/auth/session` inside the `RequireOwner` group in
  `controller.go`.
- `handler/agent.go` uses `authctx.OwnerFrom` — update it for the new
  `OwnerIdentity` type (use `.Owner.ID`).

Tests:
- Handler test: exchange → DELETE /api/v1/auth/session → same token now
  gets 401. Agent keys of that owner must still work (agents survive logout).

---

## 6. WHILE WE ARE HERE — M2: Do not write `last_used_at` on every request

File: `server/internal/core/usecase/auth.go` (function `AuthenticateAgent`, ~line 236)

Problem: Every authenticated request does an UPDATE on the agents table.
That is one extra write per request on the busiest path.

Fix: Only call `TouchLastUsed` when `agent.LastUsedAt` is zero or older
than 1 minute.

Test: fake repo counts touch calls. Two quick authentications → one touch.

---

## 7. WHILE WE ARE HERE — M1: Token cleanup must not run on every sign-in

Files:
- `server/internal/adapters/postgres/owner_token_repository.go` (`Create`, line ~21)
- `server/internal/adapters/memory/owner_token_repository.go` (`Create`, lines ~28-32)
- `server/internal/core/port/owner_token_repository.go`
- `server/internal/adapters/postgres/migrations/002_owner_tokens_expires_at.sql` (NEW file)
- `server/cmd/api/main.go`

Problem: Every sign-in runs `DELETE FROM owner_tokens WHERE expires_at < $1`
over the whole table, and there is no index on `expires_at`. Cost grows
with the number of tokens. Attackers can trigger sign-ins.

Fix:
- New migration 002: `CREATE INDEX idx_owner_tokens_expires_at ON owner_tokens (expires_at);`
  Down: drop that index. Follow the goose format of `001_init.sql`.
- Port: add `DeleteExpired(ctx context.Context) error`. Implement in both
  adapters.
- Remove the DELETE from postgres `Create` and the lazy sweep from memory
  `Create`.
- `main.go`: add a small cleanup loop using fx lifecycle — a ticker every
  hour that calls `DeleteExpired`. Stop the ticker on shutdown.

Test: memory repo — create expired tokens, call `DeleteExpired`, they are gone.

---

## 8. WHILE WE ARE HERE — M6: Limit the database connection pool

File: `server/internal/adapters/postgres/db.go` (function `Open`)

Problem: No pool limits. Under load the server can open unlimited
connections and kill the database.

Fix in `Open`, after `sql.Open`:
```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(30 * time.Minute)
```

---

## 9. Docs to update at the end

- `server/.env.example`: add `TRUSTED_PROXIES` with a comment.
- `README.md`: document the dev verifier only in `ENV=development`,
  trusted proxies, agent limit counts only active agents,
  `DELETE /api/v1/auth/session`, and that events are globally listable by
  design (public/private events come later).
- `server/AGENTS.md`: same points where relevant, plus a note that `sqlc`
  is the planned upgrade path when queries/joins grow.

---

## Suggested order of work

1. H3+M3 (agent limit) — section 2
2. H5 (UUID 500) — section 3
3. H1 (dev verifier gate) — section 4
4. H2 (trusted proxies + limiter bound) — section 1
5. M5 (logout) — section 5
6. M2 (touch throttle) — section 6
7. M1 (token cleanup + migration 002) — section 7
8. M6 (pool limits) — section 8
9. Docs — section 9

## Final check (must all pass)

- `cd server && make fmt && make lint && make test`
- `go build ./...`
- If Docker works: `make db-up`, then run the API with `DATABASE_URL` set
  and test by hand: exchange → create agent → create event → logout →
  old owner token gives 401 but agent key still works. This checks the
  new migration and the Postgres-only paths.

## Things explicitly NOT in this plan

- No public/private event visibility (later).
- No `?mine=true` event filter (later — see Decisions).
- No rewrite of the rate limiter (in-memory stays, Redis noted as future).
- No agent scopes/permissions (later).
- No ORM (plain SQL + pgx stays; sqlc noted as upgrade path).
- No commit — the user handles git.
- Do not change existing test expectations except where a fix changes the
  behavior on purpose (sections 2, 4, and the new 404 tests).
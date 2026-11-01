# Architecture

**English** · [Bahasa Indonesia](architecture.id.md)

[← Back to Documentation](README.md)

---

## Overview

rfarm-be is a single Go binary serving a JSON API backed by PostgreSQL. It follows a
**flat, layered structure** rather than a per-domain package tree:

```
HTTP request
    → handlers     parse, validate, authorize, render
    → services     business rules, orchestration, transactions
    → stores       SQL over sqlc, row mapping
    → PostgreSQL
```

The layers are folders, not frameworks. A domain adds one file per layer
(`sale_handler.go`, `sale_service.go`, `sale_store.go`, `sale_dto.go`) — no new
package, no wiring file beyond one block in `internal/routes`.

A new developer reads four files to understand a domain, and a new domain is a copy
of four well-known files.

---

## Directory Responsibilities

```
cmd/app/            Process entry point: load config, open the pool, build the
                    router, start the server, handle shutdown.

api/                api/openapi.yaml — the API contract. Written before the
                    handler, reviewed with the client, generated into Go by
                    make openapi.

db/queries/         *.sql statements compiled by sqlc. The only place SQL is
                    written; never inline SQL in Go.

internal/configs/   Typed application configuration: one struct per section
                    (app, database, redis, auth, cors, log), parsed from config.yaml
                    once and injected.

internal/clients/   Outbound integrations. One interface + one implementation per
                    dependency. Services depend on the interface, never on the SDK.

internal/dtos/      Every struct that crosses a layer: entities, Request, Input,
                    Params, Result. One file per domain: sale_dto.go.

internal/handlers/  HTTP handlers. Parse the request, validate, call one service
                    method, render through pkg/response. No business rules.

internal/routes/    The only place that knows the full graph: constructs stores,
                    services and handlers, and registers routes and middleware.

internal/services/  Business rules: ownership, state transitions, calculations,
                    validation that needs data, orchestration of several stores.

internal/stores/    Data access: interfaces + implementations. Postgres over the
                    sqlc query layer (transactions, row mapping, error mapping)
                    and the Redis session store.

internal/stores/generated/  sqlc output. Generated, read-only — never edited by
                    hand; change db/queries and run make sqlc.

internal/tests/     Shared test helpers: testcontainers PostgreSQL, miniredis,
                    seeds, fixed clock, HTTP decoding helpers. Test-only; never
                    imported by production packages.

migrations/         Schema migrations: NNNNNN_name.up.sql / .down.sql pairs,
                    applied by make migrate-up. No application data.

pkg/cache/          Redis client construction: pool, timeouts, health check.
                    Knows nothing about sessions.

pkg/config/         YAML loading, defaults and environment overrides. Knows
                    nothing about this application's sections.

pkg/logger/         slog construction and context-aware helpers.

pkg/middleware/     Request ID, recover, access log, CORS, auth, roles, farm scope.

pkg/response/       JSON envelope, page meta, RFC 9457 problem details, error type
                    and constructors, the error writer.

pkg/security/       argon2id hashing, JWT signing/parsing. Pure functions.

pkg/utils/          Validation wrapper, query parsing, If-Match parsing, date,
                    decimal and clock helpers.

scripts/            One-off and CI helper scripts (local seed, tooling).

docs/               This documentation. Not compiled, not imported by code.
```

Root files are not packages: `config.example.yaml` (configuration template),
`sqlc.yaml` (sqlc config), `Makefile` (task entry points), `.air.toml` (live reload)
and `go.mod`. The full tree is in [README.md](README.md#folder-structure).

### `internal` vs `pkg`

| Folder | Rule | Test |
|---|---|---|
| `internal/` | Code that knows this application: its tables, its rules, its config. | Could another project import it unchanged? **No** → `internal`. |
| `pkg/` | Code that would work in any Go service with the same stack. | Could another project import it unchanged? **Yes** → `pkg`. |

`pkg` never imports `internal`. `cmd` may import both. `internal` may import `pkg`.

---

## Dependency Rules

```
cmd/app
   │
   ├── internal/configs ──┐
   ├── internal/routes ───┼──> internal/handlers ──> internal/services ──> internal/stores
   │                      │            │                    │                    │
   └── pkg/* <────────────┘            └── internal/dtos <──┴────────────────────┘
                                                                    │
                                                              internal/stores/generated
```

Allowed imports:

| From | May import |
|---|---|
| `cmd/app` | everything |
| `internal/routes` | handlers, services, stores, configs, `pkg/cache`, `pkg/*` |
| `internal/handlers` | services (interfaces), dtos, `pkg/response`, `pkg/utils`, `pkg/middleware` |
| `internal/services` | stores (interfaces, including `SessionStore`), clients (interfaces), dtos, `pkg/response`, `pkg/utils`, `pkg/security`, `pkg/logger` |
| `internal/stores` | dtos, generated, `pkg/response` (error mapping), `pkg/cache`, `pkg/logger`, `redis/go-redis/v9` (session store) |
| `internal/stores/generated` | pgx only |
| `internal/clients` | dtos, `pkg/logger` |
| `internal/configs` | standard library only |
| `internal/dtos` | standard library, `shopspring/decimal`, `google/uuid` |
| `pkg/cache` | standard library, `redis/go-redis/v9` |
| `pkg/*` | standard library and third-party packages only |

Forbidden, and a review blocker:

- `stores → services`, `services → handlers` (upward).
- `handlers → stores` (skipping the service).
- A service using `redis.Client` directly; depend on the `SessionStore` interface.
- `pkg → internal` (breaks reusability).
- Editing anything in `internal/stores/generated`; change `db/queries` and regenerate.
- Two domains importing each other's concrete types; share through `dtos` or inject an
  interface in `routes`.

---

## Startup Sequence

`cmd/app/main.go` is the only place with a `main` function.

```go
func main() {
    if err := run(); err != nil {
        slog.Error("fatal", "err", err)
        os.Exit(1)
    }
}

func run() error {
    cfg := configs.MustLoad()                    // 1. typed config
    log := logger.New(cfg.Log)                   // 2. slog (JSON in prod)
    slog.SetDefault(log)

    decimal.MarshalJSONWithoutQuotes = true      // 3. money as JSON numbers

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    pool, err := pgxpool.New(ctx, cfg.Database.DSN)  // 4. connection pool
    if err != nil {
        return fmt.Errorf("pgxpool: %w", err)
    }
    defer pool.Close()
    if err := pool.Ping(ctx); err != nil {
        return fmt.Errorf("db ping: %w", err)
    }

    rdb, err := cache.NewRedis(ctx, cfg.Redis)   // 5. session store
    if err != nil {
        return fmt.Errorf("redis: %w", err)
    }
    defer rdb.Close()
    if err := rdb.Ping(ctx).Err(); err != nil {
        return fmt.Errorf("redis ping: %w", err)
    }

    router := routes.New(routes.Deps{            // 6. middleware + routes + wiring
        Pool:   pool,
        Redis:  rdb,
        Config: cfg,
        Logger: log,
    })

    srv := &http.Server{                         // 7. server with timeouts
        Addr:              cfg.App.Address(),
        Handler:           router,
        ReadHeaderTimeout: cfg.App.ReadHeaderTimeout,
        ReadTimeout:       cfg.App.ReadTimeout,
        WriteTimeout:      cfg.App.WriteTimeout,
        IdleTimeout:       cfg.App.IdleTimeout,
    }

    errCh := make(chan error, 1)
    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
    }()

    select {                                     // 8. run until signal or failure
    case err := <-errCh:
        return err
    case <-ctx.Done():
    }

    shutCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
    defer cancel()
    return srv.Shutdown(shutCtx)
}
```

Rules:

- No business logic in `main` — it wires, it does not decide.
- The process fails fast: bad config, unreachable database, or an invalid route stops
  startup with a clear message.
- Every resource opened in `main` is closed there (`defer pool.Close()`, `defer rdb.Close()`).
- `run() error` keeps `main` testable and free of `os.Exit` calls in the middle.

### Global middleware order

Order matters; keep it identical everywhere.

```
1. RequestID     → X-Request-ID in the response and in every log line
2. Recover       → panic becomes a logged 500 problem body
3. AccessLog     → method, path, status, latency, request_id, farm_id
4. CORS          → configured origins, methods, headers
5. Auth          → validates the token; puts userID + farmID in the context
6. RequireRole   → 403 for viewer on a write, etc.
7. Handler
```

`Recover` must sit above `AccessLog` and every handler, or a panic bypasses the JSON
error contract.

---

## Request Lifecycle

```
POST /api/v1/sales
  → RequestID        sets a request ID in the context
  → Recover          wraps the rest
  → AccessLog        starts the timer
  → CORS             short-circuits OPTIONS
  → Auth             parses Bearer token → userID + farmID in context
  → RequireRole      checks the route's required role
  → SaleHandler.Create
       utils.ParseAndValidate(r, &req)    → 400 or 422 problem
       svc.CreateSale(ctx, input)         → one service call
            rule checks                    → 422 / 404 / 409
            store.CreateSale (sqlc)        → SQL → 500 on failure
       response.Created(w, sale)           → 201 + envelope
```

Every layer returns `error` upward; only the handler (through `response.Fail`) and the
outer writer produce a body.

---

## Configuration

Configuration lives in one YAML file, parsed once into typed structs and passed down.
There is no global config variable and no `.env` file.

```yaml
# config.example.yaml — committed template; copy to config.yaml (git-ignored)
app:
  env: development
  addr: ":8080"
  readTimeout: 10s
  writeTimeout: 15s
  idleTimeout: 60s
  shutdownTimeout: 10s
  defaultTimezone: Asia/Jakarta
  maxImportBytes: 20971520

database:
  url: postgres://user:pass@localhost:5432/rfarm?sslmode=disable
  maxConns: 10

redis:
  addr: localhost:6379
  password: ""
  db: 0
  dialTimeout: 2s
  readTimeout: 1s
  writeTimeout: 1s
  poolSize: 10

auth:
  jwtSigningKey: change-me
  accessTokenTTL: 15m
  refreshTokenTTL: 720h

cors:
  allowedOrigins:
    - http://localhost:5173

log:
  level: info
```

- The file is `config.yaml`; its path can be changed with the single `CONFIG_PATH`
  environment variable (default `./config.yaml`).
- Loaded by `pkg/config` (YAML decode, defaults, validation) into the structs in
  `internal/configs`; `configs.MustLoad()` stops startup with a message naming the
  file or key that is wrong.
- Precedence: defaults in code < `config.yaml` < environment override.
- Only secrets may be overridden by environment variables — `DATABASE_URL` and
  `JWT_SIGNING_KEY` — so production can keep them out of the file and the image
  (the names come from `specification.md` §5.4).
- Durations are Go duration strings (`10s`, `720h`) parsed at load time; byte limits
  and connection counts are integers.
- `app.defaultTimezone` decides report month boundaries and audit display. Business
  dates stay plain `DATE` — the server never shifts a calendar day.
- `app.maxImportBytes` (20 MB) caps the legacy JSON import, separate from the general
  request body limit.
- `redis.*` points at the session store. It is **required**, not an optional cache:
  startup pings it, and `/readyz` checks Postgres and Redis.
- `auth.refreshTokenTTL` is the TTL of every refresh session in Redis; a rotation
  resets it.
- `app.*Timeout` values are server hardening; the rest is product configuration.
- `config.yaml` is git-ignored; `config.example.yaml` is committed and holds no real
  secret.

---

## Data Access

- One `*pgxpool.Pool` created in `main`, injected into store constructors.
- SQL lives in `db/queries/*.sql`; `sqlc generate` produces the typed functions in
  `internal/stores/generated`.
- The hand-written store wraps generated queries and returns `dtos`, so `pgx` types
  never leak past the store.
- Placeholders follow sqlc (`$1, $2, …`, `sqlc.arg`). Always parameterised.
- Sorting names come from an allowlist, never from the request
  ([List API](standards/list_api.md)).
- Transactions are opened by the store, never passed to the service
  ([Service & Store](standards/service_repo.md)).
- Schema changes live in `migrations/` and are applied by `make migrate-up`. The
  application never creates or alters tables at startup.
- The **session store** is the one non-SQL store: `SessionStore` in
  `internal/stores/session_store.go`, backed by Redis through `pkg/cache`. It holds
  refresh sessions only — no business rules, no Postgres transaction.

### sqlc configuration

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "migrations"
    queries: "db/queries"
    gen:
      go:
        package: "generated"
        out: "internal/stores/generated"
        sql_package: "pgx/v5"
        emit_json_tags: false
        emit_interface: false
        overrides:
          - db_type: "numeric"
            go_type: "github.com/shopspring/decimal.Decimal"
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
```

---

## Tenancy, Roles, Money and Sessions

These rules shape almost every layer, so they are stated once here.

### Farm scope comes from the token

```
JWT claims { sub: userId, farmId, role }
    → middleware.Auth stores them in the request context
    → handlers read middleware.FarmIDFrom(ctx) and pass it in the service input
    → every store query includes `farm_id = $n`
```

- `farm_id` is **never** accepted from a body or a query parameter.
- A row belonging to another farm must look missing: `404`, not `403`.
- The service never trusts an ID it did not get from the context.

### Roles are enforced in middleware

| Role | May |
|---|---|
| `viewer` | read everything in the farm |
| `editor` | read + create/update/delete records |
| `owner` | everything, including settings, accounts, members, import/reset |

Routes are grouped per role in `internal/routes`; the handler assumes the check has
already happened. Handlers do not re-check roles except when the rule depends on the
row (rare — put that in the service).

### Money and quantities are decimal

- `numeric` columns ↔ `decimal.Decimal` in `dtos`.
- Arithmetic happens in the service (`Amount = Qty × Price`), rounded once.
- `float64` never appears in a money path — not in a DTO, not in a calculation, not in
  a test fixture.

### Refresh sessions live in Redis

Access tokens stay stateless: `middleware.Auth` verifies the JWT signature and expiry
and never touches Redis. Redis holds the **refresh session**, which is what makes
logout and rotation possible.

| Item | Choice |
|---|---|
| Refresh token | 32 random bytes, base64url — never stored in plain text |
| Session key | `rfarm:session:<sha256(token)>` → JSON `{ userId, farmId, role, familyId, issuedAt }` |
| TTL | `auth.refreshTokenTTL` (default 30 days), reset on each rotation |
| Family index | `rfarm:family:<familyId>` — set of live token hashes |
| User index | `rfarm:user:<userId>` — set of family IDs, for revoke-all |
| Rotation | delete the old key and issue a new token in the same family, atomically |
| Reuse | a live family with an unknown token hash → revoke the family, `401` |
| Logout | delete the session key; the family lives until its TTL |
| Password change / member removal | revoke every family in the user index |

```
POST /auth/login    → 200 { accessToken, refreshToken, expiresIn }
POST /auth/refresh  → rotate: 200 with a new pair, or 401 if reused or expired
POST /auth/logout   → 204; the refresh session is deleted
```

Rules:

- Hash with SHA-256, not bcrypt: the token is high-entropy random and the session
  store must be fast. Passwords use `argon2id`; tokens do not.
- Rotation is atomic (a Lua script or `WATCH`/`MULTI`); two concurrent refreshes must
  not both succeed.
- After logout the current access token keeps working until it expires (≤ 15 min).
  That is the accepted trade-off; add a `jti` blocklist only if immediate cut-off is
  required.
- Redis is not a cache here: if it is unavailable, login/refresh/logout return `503`
  and `/readyz` fails.
- Rotation resets the TTL (sliding expiration); a session that is never refreshed
  disappears after `auth.refreshTokenTTL`.

---

## Error Flow

```
store      → response.NotFound / Conflict / PreconditionFailed, or a wrapped error
service    → response.Unprocessable / Forbidden / PreconditionFailed, or pass through
handler    → response.Fail(w, r, err)     ← single mapping point
middleware → Recover → response.Fail        ← panic becomes 500
```

Every error carries an HTTP status, a stable code, and a problem body. Details:
[Error Handling](error_handling.md) and [HTTP Status](standards/http_status.md).

---

## Logging

- One `*slog.Logger` built in `main`, injected into middleware, services and clients.
- Structured key/value only: `request_id`, `method`, `path`, `status`,
  `latency_ms`, `user_id`, `farm_id`, `err`.
- Levels: `debug` for developer detail, `info` for lifecycle events, `warn` for
  recoverable oddities, `error` for failures that need attention.
- Never log passwords, tokens, full request bodies, or personal data.
- Log an error **once**, at the boundary that decides the response.

---

## Adding a New Domain

Using `sale` as the example. Nine steps, no new package.

1. **Query** — `db/queries/sales.sql` with named statements (`-- name: CreateSale
   :one`), then `make sqlc`.
2. **Migration** — `migrations/000002_create_sales.up.sql` / `.down.sql`.
3. **DTO** — `internal/dtos/sale_dto.go`: `Sale`, `CreateSaleRequest`,
   `CreateSaleInput`, `ListSalesParams`, …
4. **Store** — `internal/stores/sale_store.go`: interface + `postgresSaleStore` over
   `generated.Queries`, and `NewSaleStore(pool *pgxpool.Pool) SaleStore`.
5. **Service** — `internal/services/sale_service.go`: interface + unexported
   `saleService`, business rules, constructor.
6. **Handler** — `internal/handlers/sale_handler.go`: `http.HandlerFunc` methods and
   `Register(r chi.Router)`.
7. **Contract** — the path and schemas in `api/openapi.yaml` (written before the
   handler in practice).
8. **Wiring** — `internal/routes/routes.go`, one block:

   ```go
   saleStore := stores.NewSaleStore(d.Pool)
   saleSvc := services.NewSaleService(saleStore, d.Logger, utils.SystemClock{})
   saleHandler := handlers.NewSaleHandler(saleSvc, d.Logger)
   saleHandler.Register(api)
   ```

9. **Tests** — `sale_service_test.go`, `sale_handler_test.go`,
   `sale_store_test.go` (integration tag).

> Order matters: query → migration → dto → store → service → handler → route. Each
> step only imports what already exists.

---

## Anti-Patterns

| Anti-pattern | Why it hurts | Do instead |
|---|---|---|
| SQL in a handler or service | Untestable, mixes concerns | Store method |
| Business rule in the store | The rule disappears when the query changes | Service |
| Handler calling the store directly | Skips ownership/state checks | Service |
| `farm_id` from the body or query | Cross-tenant data leak | Token → context |
| Editing `stores/generated` | Regenerated code overwrites it | Change the query, regenerate |
| `float64` for money | Rounding errors in the books | `decimal.Decimal` |
| A package per domain (`internal/modules/...`) | Overkill for this base; more wiring | Flat layer folders |
| Global `pool`/`cfg` variables | Hidden dependencies, race-prone tests | Inject in `routes` |
| `init()` that opens connections | Untestable, order-dependent | `main` |
| `panic` for expected failures | Kills the request, no contract | Typed error |
| Returning `err.Error()` to the client | Leaks internals | `response.Internal(err)` |
| Business logic in middleware | Hidden, hard to test | Service |
| Refresh tokens stored in plain text, or in Postgres | A dump becomes live sessions; the table grows forever | SHA-256 hash in Redis with a TTL |
| `redis.Client` injected into a service | Couples business rules to one store | `SessionStore` interface |
| Session state put inside the JWT | Cannot be revoked before expiry | Keep it in the session store |

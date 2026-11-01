# Development

**English** · [Bahasa Indonesia](development.id.md)

[← Back to Documentation](README.md)

---

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | see `go.mod` (1.23+) | build and test |
| PostgreSQL | 15+ | local database (16 recommended) |
| Redis | 7+ | refresh sessions; the app refuses to start without it |
| Docker | any recent | `testcontainers-go` for integration tests |
| make | any | every task goes through the Makefile |
| Air | latest | live reload (`go install github.com/air-verse/air@latest`) |
| golangci-lint | latest | linting (`brew install golangci-lint`) |
| golang-migrate | latest | schema migrations (`brew install golang-migrate`) |
| sqlc | 1.27+ | typed queries (`brew install sqlc`) |
| oapi-codegen | latest | OpenAPI types/stubs (`go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest`) |

Optional: `delve` for debugging, `psql` for manual queries.

---

## First Run

```bash
# 1. dependencies
go mod download

# 2. local configuration
cp config.example.yaml config.yaml    # config.yaml is git-ignored
# edit database.url and auth.jwtSigningKey

# 3. database and session store
createdb rfarm
make migrate-up
redis-server                     # or: brew services start redis

# 4. generate sqlc + OpenAPI code, then run
make sqlc
make openapi
make dev
```

The API is available at `http://localhost:8080`; health at `/healthz` and
`/readyz`; the contract is served at `/api/v1/openapi.yaml`.

Check it works:

```bash
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
curl -s localhost:8080/api/v1/sales -H "Authorization: Bearer $TOKEN"
```

---

## Configuration

Configuration is one YAML file. `config.example.yaml` is committed as the template
and documents every key; `config.yaml` is local and git-ignored.

```yaml
# config.yaml
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

| Key | Default | Notes |
|---|---|---|
| `app.env` | `development` | `development` \| `staging` \| `production` |
| `app.addr` | `:8080` | listen address |
| `app.readTimeout` | `10s` | server read timeout |
| `app.writeTimeout` | `15s` | server write timeout |
| `app.idleTimeout` | `60s` | keep-alive idle timeout |
| `app.shutdownTimeout` | `10s` | graceful shutdown budget |
| `app.defaultTimezone` | `Asia/Jakarta` | report month boundaries, audit display |
| `app.maxImportBytes` | `20971520` | legacy JSON import cap (20 MB) |
| `database.url` | — | **required**, PostgreSQL DSN |
| `database.maxConns` | `10` | pgxpool max connections |
| `redis.addr` | `localhost:6379` | **required**, Redis host:port for refresh sessions |
| `redis.password` | `""` | Redis AUTH password, empty when unset |
| `redis.db` | `0` | logical database index |
| `redis.dialTimeout` | `2s` | connection timeout |
| `redis.readTimeout` | `1s` | socket read timeout |
| `redis.writeTimeout` | `1s` | socket write timeout |
| `redis.poolSize` | `10` | max pooled connections |
| `auth.jwtSigningKey` | — | **required**, HMAC signing key |
| `auth.accessTokenTTL` | `15m` | access token lifetime |
| `auth.refreshTokenTTL` | `720h` | refresh session TTL in Redis |
| `cors.allowedOrigins` | `["http://localhost:5173"]` | list of origins |
| `log.level` | `info` | `debug` \| `info` \| `warn` \| `error` |

Rules:

- The file path comes from `CONFIG_PATH` (default `./config.yaml`). No other
  environment variable is required to start the app.
- Precedence: defaults in code < `config.yaml` < environment override.
- Only `DATABASE_URL` and `JWT_SIGNING_KEY` may override `database.url` and
  `auth.jwtSigningKey`, so a deployment can keep those two out of the file and the
  image (names from `specification.md` §5.4).
- `redis.*` is a hard dependency, like `database.*`: startup pings Redis and
  `/readyz` reports it. There is no in-memory fallback.
- The app fails fast, naming the file or key, when the file is missing or a required
  value is empty.
- Never commit `config.yaml`, real DSNs, signing keys, or certificates.
- In staging and production the file is rendered from the platform's secret store or
  config map, not baked into the image.

---

## Make Targets

`make` is the only interface you need. If a task is missing, add a target instead of
documenting a raw command in a README or a chat message.

| Target | What it does |
|---|---|
| `make dev` | run with Air live reload |
| `make run` | `go run ./cmd/app` |
| `make build` | build the binary into `bin/rfarm-be` |
| `make sqlc` | regenerate `internal/stores/generated` from `db/queries` |
| `make openapi` | regenerate Go types/stubs from `api/openapi.yaml` |
| `make check-openapi` | fail if `api/openapi.yaml` is not valid OpenAPI 3.1 |
| `make test` | `go test ./... -race -count=1` |
| `make test-cover` | coverage profile + HTML report |
| `make test-integration` | tests with `-tags=integration` (starts Docker containers) |
| `make lint` | `golangci-lint run ./...` |
| `make fmt` | `gofmt -s -w` + `goimports` |
| `make tidy` | `go mod tidy` |
| `make migrate-up` | apply all pending migrations |
| `make migrate-down` | roll back the last migration |
| `make migrate-create name=create_sales` | create a new up/down pair |
| `make seed` | load `scripts/seed.sql` into the local database |
| `make check` | fmt + lint + sqlc diff + openapi check + test — run before every push |

`make check` must fail if `make sqlc` would change a generated file: committed code
and generated code are never allowed to drift.

---

## Everyday Workflow

```bash
make dev                 # terminal 1 — server with reload
make test                # terminal 2 — after each change
make lint
make check               # before commit
```

Recommended loop:

1. Write the SQL in `db/queries/<domain>.sql` and run `make sqlc`.
2. Write or update the migration.
3. Add the DTO, then the store interface + implementation.
4. Add the service rules and its tests.
5. Add the path + schemas to `api/openapi.yaml`, run `make openapi`.
6. Add the handler and handler tests.
7. Wire it in `internal/routes`.
8. Update the docs, run `make check`.

---

## Database Migrations

- Files live in `migrations/`, one pair per change:
  `000002_create_sales.up.sql` and `000002_create_sales.down.sql`.
- Sequential numbering; never reuse or renumber an applied migration.
- Every up has a real down (`DROP TABLE`, `DROP COLUMN`, …).
- Never edit a migration that already ran outside your machine — add a new one.
- Migrations are additive by default: add a column with a default, backfill, then
  tighten constraints in a later migration.
- Reference data (chart of accounts per farm) is seeded by an application command,
  not by a migration; local demo data goes in `scripts/seed.sql`.

```bash
make migrate-create name=create_sales
make migrate-up
make migrate-down          # one step
migrate -path migrations -database "postgres://user:pass@localhost:5432/rfarm?sslmode=disable" version
```

Down migrations are for local development. Production is forward-only: a bad
migration is fixed by a new migration, never by rolling back.

---

## sqlc Workflow

SQL is written in `db/queries/*.sql` with named statements and compiled into Go.

```sql
-- db/queries/sales.sql
-- name: GetSale :one
SELECT id, farm_id, date, buyer, qty, price, amount, version, created_at, updated_at
  FROM sales
 WHERE id = $1 AND farm_id = $2 AND deleted_at IS NULL;
```

```bash
make sqlc
```

Rules:

- `internal/stores/generated` is committed and **never edited by hand**.
- A query that a store needs must exist in `db/queries`; no inline SQL in the store
  except for the rare statement sqlc cannot express (justify it in a comment).
- `numeric` is mapped to `decimal.Decimal` and `uuid` to `uuid.UUID` in `sqlc.yaml`
  ([Architecture](architecture.md)).
- Every select on a record table filters `deleted_at IS NULL` unless the query is
  explicitly about deleted rows.

---

## OpenAPI Workflow

```bash
make openapi        # regenerate types/stubs from api/openapi.yaml
make check-openapi  # validate before committing
```

- `api/openapi.yaml` is the source of truth; there are no annotations in Go code.
- Change the YAML first, review it, then implement.
- Generated files are committed in the same commit as the YAML change.
- Serving `/api/v1/openapi.yaml` from the binary is enough — there is no separate
  Swagger UI in production.

---

## Tests

```bash
make test                                   # unit, fast, no database
make test-integration                       # Docker required
go test ./internal/services -run TestCreateSale -v
go test ./internal/services -coverprofile=cover.out
go tool cover -html=cover.out
```

Rules: see [Unit Testing](standards/unit_test.md). Unit tests never touch the network
or a real database, and never need a running Redis: services take a fake
`SessionStore`, and the session store itself is tested against `miniredis`.
Store tests are tagged `integration` and use `testcontainers-go`, which needs a running
Docker daemon; CI starts containers the same way.

`testcontainers-go` pulls `postgres:16-alpine` once and caches it. The first run is
slow; later runs reuse the image and take seconds.

---

## Dependencies

```bash
go get github.com/example/pkg@v1.2.3
go mod tidy
```

- Pin a version; do not use `@latest` in a commit.
- Justify a new dependency in the PR: what it replaces, its licence, its maintenance
  state.
- Prefer the standard library and the existing stack (`chi`, `pgx`, `sqlc`, `slog`,
  `decimal`, `validator`, `redis/go-redis/v9`) before adding something new.
- `miniredis` is test-only: it appears in no production import path.
- Never vendor unless the team decides to.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `connection refused` on startup | PostgreSQL not running / wrong port | `brew services start postgresql`, check `database.url` |
| `redis: dial tcp … connection refused` | Redis not running / wrong port | `brew services start redis`, check `redis.addr` |
| `redis: WRONGPASS` or `NOAUTH` | `redis.password` missing or wrong | set it in `config.yaml`, or clear it if the server has no AUTH |
| `/readyz` returns `503` with Postgres up | Redis unreachable — it is a hard dependency | start Redis, check `redis.addr` and the network |
| every login/refresh returns `503` | Redis down or `redis.*` misconfigured | same as above; sessions are not cached in memory as a fallback |
| `session expired` right after login | `auth.refreshTokenTTL` too small, or two servers on different Redis DBs | raise the TTL; point every instance at the same `redis.addr`/`redis.db` |
| `password authentication failed` | wrong credentials or a stale `config.yaml` | fix `database.url` (or the `DATABASE_URL` override) |
| `relation "sales" does not exist` | migrations not applied | `make migrate-up` |
| `sqlc: column does not exist` | migration not applied before regenerate | `make migrate-up && make sqlc` |
| generated code out of date in CI | `make sqlc` not run | run it and commit `internal/stores/generated` |
| `port already in use` | an old `make dev` is still running | kill it (`lsof -i :8080`) or change `app.addr` |
| `config file not found` | `config.yaml` missing or `CONFIG_PATH` points elsewhere | `cp config.example.yaml config.yaml` |
| integration tests skip or fail to start | Docker daemon not running | start Docker Desktop, rerun `make test-integration` |
| `permission denied for table …` | migrations ran as a different role | run `make migrate-up` with the app's DSN |
| Air reloads in a loop | it watches its own output | build to `bin/`, keep `tmp/` in `.gitignore` |
| `412` on every update locally | stale `If-Match` from an old response | re-read the record and use its current `version` |

---

## Before a Pull Request

- [ ] `make check` passes (fmt, lint, sqlc diff, OpenAPI check, tests).
- [ ] New behaviour has tests; changed behaviour has updated tests.
- [ ] `api/openapi.yaml` updated and `make openapi` run when a contract changed.
- [ ] Migration added for every schema change, with a working `down`.
- [ ] Doc updated in the same commit, `.id.md` twin included
      ([Documentation Rules](standards/documentation.md)).
- [ ] No secrets, no debug prints, no commented-out code.
- [ ] `config.example.yaml` updated if a new key was introduced.

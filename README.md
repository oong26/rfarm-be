# rfarm-be

Backend API for **R FARM Dashboard** — a small layer-farm (egg) management system
used by the owner and staff in Bondowoso, Indonesia.

One Go binary, one JSON API, one PostgreSQL database. The API replaces the browser
`localStorage` store the React frontend uses today, and adds authentication,
server-side validation, accounting, and reports.

> **Status:** early development. The product contract and engineering documentation
> are complete; the Go implementation is in progress.

---

## Stack

| Concern | Choice |
|---|---|
| Language | Go ≥ 1.23 |
| HTTP | `go-chi/chi` v5 on `net/http` |
| Database | PostgreSQL 15+, `jackc/pgx` v5 + `pgxpool` |
| Queries | `sqlc` — SQL in `db/queries`, generated Go in `internal/stores/generated` |
| Validation | `go-playground/validator` + service-level rules |
| Money / quantities | `shopspring/decimal` — never `float64` |
| Logging | `log/slog`, structured JSON with a request ID |
| API contract | OpenAPI 3.1 in `api/openapi.yaml`, contract-first |
| Migrations | `golang-migrate`, forward-only in production |
| Auth | JWT access token + rotating refresh token, `argon2id` passwords |
| Sessions | `redis/go-redis/v9` — refresh sessions with TTL, rotation, reuse detection |
| Tests | `testing`, `testify`, `testcontainers-go` |

---

## Quick start

Prerequisites: Go 1.23+, PostgreSQL 15+, Redis 7+, `make`, and — for integration
tests — Docker. Tooling (`sqlc`, `golang-migrate`, Air, `golangci-lint`) is listed in
[Development](docs/development.md#prerequisites).

```bash
cp config.example.yaml config.yaml   # then set database.url and auth.jwtSigningKey
make migrate-up
redis-server                           # or: brew services start redis
make dev                               # live reload on http://localhost:8080
```

Check it:

```bash
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
curl -s localhost:8080/api/v1/openapi.yaml
```

Configuration is a YAML file, not `.env`: `config.yaml` is local and git-ignored,
`config.example.yaml` documents every key. Details and all Make targets:
[Development](docs/development.md).

---

## What it covers

| Area | Examples |
|---|---|
| Daily records | eggs, rejects, feed, deaths, birds, notes — one record per day |
| Flock | bird additions, culling, bird sales |
| Feed | alternative feed entries, feed stock movements |
| Sales | egg sales with buyer, quantity, price |
| Expenses | non-feed operating costs |
| Stock | feed and egg stock transactions |
| Health | vaccines, checks, symptoms |
| Accounting | chart of accounts, manual journals, auto-posted journals, statements |
| Reports | dashboard, monthly and annual recaps, population |
| Settings | farm parameters, legacy JSON import, reset |

The authoritative field rules, formulas and endpoint list are in
[`docs/specification.md`](docs/specification.md). Where the spec and the code
disagree, the spec wins — that is a bug to fix.

---

## Project layout

```
cmd/app/            process entry point: config, pool, router, server lifecycle
internal/           application code: handlers → services → stores
  handlers/         parse, validate, render
  services/         business rules and orchestration
  stores/           SQL over sqlc, transactions, row mapping; Redis sessions
  dtos/             structs that cross layers
  routes/           wiring and route groups
  configs/          typed configuration
  clients/          outbound integrations
  tests/            shared test helpers
pkg/                reusable, app-agnostic packages
  cache/            Redis client: pool, timeouts, health check
db/queries/         *.sql compiled by sqlc
migrations/         schema migrations (up/down)
api/openapi.yaml    the API contract
docs/               engineering documentation
```

The rules behind this tree, including which layer may import which, are in
[Architecture](docs/architecture.md).

---

## API conventions

- Base path `/api/v1`, plural kebab-case resources, camelCase JSON.
- Errors are RFC 9457 `application/problem+json` with a stable `code`; messages are
  Indonesian, codes English.
- Lists are paginated by default: `page`, `pageSize` (max 200), `meta.total`; no
  `all=true`.
- Records carry `version`; `PUT`/`DELETE` require `If-Match` and return `412` on a
  stale version.
- `farm_id` always comes from the token, never from the body or query. Another
  farm's row is `404`.
- Money and quantities are decimals end to end — no `float64` in a money path.

Full rules: [HTTP Status](docs/standards/http_status.md),
[List API](docs/standards/list_api.md), [Error Handling](docs/error_handling.md).

---

## Documentation

| Document | What it answers |
|---|---|
| [docs/README.md](docs/README.md) | Documentation map and reading order |
| [docs/specification.md](docs/specification.md) | The product contract: entities, rules, formulas, endpoints |
| [docs/architecture.md](docs/architecture.md) | Layers, directories, request lifecycle, startup |
| [docs/development.md](docs/development.md) | Setup, configuration, Make targets, migrations, troubleshooting |
| [docs/standards/standard.md](docs/standards/standard.md) | Coding standards index |
| [docs/error_handling.md](docs/error_handling.md) | Error taxonomy and the problem-details contract |
| [docs/git_workflow.md](docs/git_workflow.md) | Branches, commits, PRs, releases |

Every document has a Bahasa Indonesia twin (`*.id.md`) with the same structure.
Engineering docs start at [docs/README.md](docs/README.md); the Indonesian entry
point is [docs/README.id.md](docs/README.id.md).

---

## Common commands

```bash
make dev               # run with live reload
make test              # unit tests, no database
make test-integration  # testcontainers PostgreSQL (Docker required)
make lint
make check             # fmt + lint + sqlc diff + OpenAPI check + tests
make sqlc              # regenerate internal/stores/generated
make openapi           # regenerate Go types from api/openapi.yaml
```

The full list, including migrations and seeding, is in
[Development](docs/development.md#make-targets).

---

## Contributing

- Conventional Commits, one logical change per PR, squash merge.
- Run `make check` before pushing.
- A behaviour change updates `api/openapi.yaml`, its tests, and the relevant doc in
  the same commit — including the `.id.md` twin.
- Never edit files under `internal/stores/generated/`; change the query and run
  `make sqlc`.

See [Git Workflow](docs/git_workflow.md) and the
[standards index](docs/standards/standard.md).

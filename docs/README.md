# rfarm-be — Engineering Documentation

**English** · [Bahasa Indonesia](README.id.md)

rfarm-be is the backend for **R FARM Lestari ERP** — a small layer-farm (egg)
management system. It exposes one JSON API over PostgreSQL, built on a flat
layered structure with rules that are the same in every domain.

The product contract (entities, field rules, calculations, endpoints) lives in
[specification.md](specification.md). These documents cover how the code is
structured, written, tested and documented.

> **Rule of thumb:** the specification says *what the product does*; these docs say
> *how we build it*. If a fact is already in the OpenAPI spec or in the code, link
> to it instead of copying it.

---

## Start here

| I want to… | Read |
|---|---|
| Know what the product must do | [specification.md](specification.md) |
| Run the project for the first time | [Development](development.md) |
| Understand the structure and request flow | [Architecture](architecture.md) |
| Write code that matches the rest of the repo | [Standards](standards/standard.md) |
| Return the right status and error body | [HTTP Status](standards/http_status.md), [Error Handling](error_handling.md) |
| Write or update documentation | [Documentation Rules](standards/documentation.md) |
| Commit, branch or open a PR | [Git Workflow](git_workflow.md) |

---

## Stack

| Concern | Choice |
|---|---|
| Language | Go ≥ 1.23 |
| HTTP | `go-chi/chi` v5, `net/http` |
| Database | PostgreSQL 15+, `jackc/pgx` v5 + `pgxpool` |
| Queries | `sqlc` — SQL in `db/queries`, generated Go in `internal/stores/generated` |
| Validation | `go-playground/validator` + domain rules |
| Money / quantities | `shopspring/decimal` — never `float64` |
| Logging | `log/slog` (JSON), request ID on every line |
| API contract | OpenAPI 3.1 in `api/openapi.yaml`, reviewed before code |
| Migrations | `golang-migrate`, forward-only in production |
| Auth | JWT access token (15 min) + rotating refresh token, `argon2id` |
| Sessions | `redis/go-redis/v9` — refresh sessions with TTL, rotation and reuse detection |
| Tests | `testing`, `testify`, `testcontainers-go` for real PostgreSQL |

---

## Documentation map

### Architecture & workflow

| Doc | What it answers |
|---|---|
| [architecture.md](architecture.md) | What each directory is for, which layer may import which, how a request travels, how the app starts and stops |
| [development.md](development.md) | Prerequisites, configuration, Make targets, sqlc, migrations, testing, troubleshooting |
| [error_handling.md](error_handling.md) | How errors are created, wrapped, mapped to status codes, logged, and returned |
| [git_workflow.md](git_workflow.md) | Branch naming, commit messages, PR size and review rules, releases |

### Standards

| Topic | Doc | What it answers |
|---|---|---|
| Index | [standard.md](standards/standard.md) | The ground rules and the full topic list |
| API naming | [api_naming.md](standards/api_naming.md) | Plural vs singular, verbs, nesting, `/admin`, one purpose per endpoint |
| Method naming | [method_naming.md](standards/method_naming.md) | `List{Entity}`, `List{Entity}By{Parent}`, CRUD method names |
| Structs | [struct.md](standards/struct.md) | `Request` / `Input` / `Params`, when to share, where structs live |
| Service & store | [service_repo.md](standards/service_repo.md) | Layer responsibilities, store interfaces, transactions, optimistic locking |
| HTTP status | [http_status.md](standards/http_status.md) | Which status code to return and why, 409 vs 412 vs 422 |
| List API | [list_api.md](standards/list_api.md) | Pagination by default, filters, sorting, meta, exports |
| Unit test | [unit_test.md](standards/unit_test.md) | What to test per layer, file layout, baseline cases, fakes |
| Documentation | [documentation.md](standards/documentation.md) | Where each kind of knowledge lives and how to write it |

---

## Folder structure

```
rfarm-be/
├── api/
│   └── openapi.yaml          # API contract — reviewed before implementation
├── cmd/
│   └── app/                  # entry point: wiring and server lifecycle
├── internal/                 # application code — never imported by other modules
│   ├── clients/              # outbound integrations (HTTP APIs, mail, storage)
│   ├── configs/              # typed application configuration
│   ├── dtos/                 # structs that cross layers (entities, requests, queries)
│   ├── handlers/             # HTTP layer: parse, validate, render
│   ├── routes/               # route table, middleware groups, dependency wiring
│   ├── services/             # business rules and orchestration
│   ├── stores/               # data access: Postgres over sqlc, Redis sessions
│   │   └── generated/        # sqlc-generated code — do not edit
│   └── tests/                # shared test helpers, fixtures, golden files
├── pkg/                      # reusable packages with no knowledge of this app
│   ├── cache/                # Redis client: pool, timeouts, health check
│   ├── config/               # YAML loading, defaults, env overrides
│   ├── logger/               # slog construction
│   ├── middleware/           # request id, recover, access log, CORS, auth, roles
│   ├── response/             # JSON envelope, RFC 9457 problem details, errors
│   ├── security/             # argon2id, JWT signing and parsing
│   └── utils/                # validation, query parsing, pagination, time
├── db/
│   └── queries/              # *.sql statements compiled by sqlc
├── migrations/               # 000001_init.up.sql, …
├── scripts/                  # one-off and CI helper scripts
├── docs/                     # this documentation
├── sqlc.yaml                 # sqlc: db/queries → internal/stores/generated
├── config.example.yaml       # configuration template — copy to config.yaml
├── Makefile                  # developer entry points
├── .air.toml                 # live reload for local development
└── go.mod
```

The rules behind this tree — including what may and may not be imported — are in
[architecture.md](architecture.md).

---

## Recommended reading order

1. [specification.md](specification.md) — the product you are building.
2. [Architecture](architecture.md) — the mental model of the layers.
3. [Development](development.md) — get the project running.
4. [Standards index](standards/standard.md) — the topics that apply to your change.
5. [Documentation rules](standards/documentation.md) — before your first doc or
   OpenAPI change.

---

## Documentation rules at a glance

The full rules live in [documentation.md](standards/documentation.md). The short
version:

1. **One home per fact.** API contract → `api/openapi.yaml`. Product rules →
   `specification.md`. Architecture → `docs/`. Code rule → `docs/standards/`.
   Code quirk → a comment next to the code.
2. **English is canonical.** Every file has an Indonesian twin (`foo.id.md`) with
   the same headings, tables and order — a translation, not a summary.
3. **Same commit as the code.** Behaviour changes update the OpenAPI spec and the
   relevant doc in the same commit, exactly like tests.
4. **Link, don't copy.** Docs point to code by file name; code comments point to
   the doc path. Never duplicate a table in two files.
5. **Every new document is linked** from this README or from the standards index.
   An unlinked doc is an orphan and will not be found.

---

## Keeping these docs true

| Change | Update in the same PR |
|---|---|
| New endpoint or changed request/response | `api/openapi.yaml` + the feature doc if one exists |
| New directory, layer or package | [architecture.md](architecture.md) |
| New rule the team agreed on | The relevant `docs/standards/*.md` + [standard.md](standards/standard.md) index |
| New Make target or tool | [development.md](development.md) |
| New branch/commit/release convention | [git_workflow.md](git_workflow.md) |

Stale documentation is a bug. If a doc contradicts the code, fix it or open a
task — do not leave both versions standing.

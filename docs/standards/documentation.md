# Documentation

**English** · [Bahasa Indonesia](documentation.id.md)

[← Back to Standards](standard.md)

---

## Goal

Every piece of knowledge has **one home**. A reader should find it without
searching, and a writer should never update the same fact in two places.

> **Rule of thumb:** if the OpenAPI contract or the code already says it, don't
> repeat it in a document — link to it.

---

## Where Knowledge Lives

| Where | What goes there | What does **not** |
|---|---|---|
| **`api/openapi.yaml`** | The API contract: paths, parameters, bodies, responses, status codes, error codes | Architecture, history, business rationale |
| **`docs/specification.md`** | The product contract: entities, field rules, formulas, screens, business decisions | Implementation detail |
| **Code comments** | The *why* of one query, rule or quirk; a `TODO`/`FIXME` with an owner and a ticket | Feature overviews, copied rules |
| **`docs/architecture.md`** | Structure, layers, dependency rules, startup, request lifecycle | Endpoint details |
| **`docs/development.md`** | Setup, configuration, Make targets, sqlc, migrations, tools | Code rules |
| **`docs/error_handling.md`** | Error taxonomy, problem details, wrapping, logging | Per-endpoint error lists |
| **`docs/git_workflow.md`** | Branches, commits, PRs, releases | Code style |
| **`docs/standards/*.md`** | How we write code (naming, structs, lists, tests) | Business rules of a feature |
| **`docs/features/<domain>.md`** | One domain's non-obvious rules, flows and decisions | The API contract (that is OpenAPI) |
| **README (repo root)** | What the service is, how to run it in three commands, link to `docs/` | Deep detail |

If a fact fits two rows, it belongs to the more specific one and the other links to
it.

---

## Folder Layout

```
docs/
├── README.md                 ← entry point and documentation map
├── README.id.md
├── specification.md          ← product contract (owned by the product owner)
├── architecture.md           ← cross-cutting: structure and flow
├── development.md            ← cross-cutting: how to build and run
├── error_handling.md         ← cross-cutting: errors end to end
├── git_workflow.md           ← cross-cutting: branches, commits, releases
├── standards/
│   ├── standard.md           ← standards index
│   ├── api_naming.md
│   ├── method_naming.md
│   ├── struct.md
│   ├── service_repo.md
│   ├── http_status.md
│   ├── list_api.md
│   ├── unit_test.md
│   └── documentation.md      ← this file
└── features/
    └── <domain>.md           ← only when a domain needs one (see criteria)

api/
└── openapi.yaml              ← the single API contract
```

Rules:

- File and folder names: lowercase, `snake_case` for standards topics
  (`api_naming.md`), `kebab-case` for feature docs (`daily-records.md`).
- One topic per file. If a file needs more than ~250 lines, it is probably two
  topics — split it and link both from the index.
- Every file is linked from `docs/README.md` or `standards/standard.md`. An
  unlinked file is invisible.
- `specification.md` is owned outside engineering docs: link to it, never rewrite
  it to match the code. If they disagree, that is a bug to raise.

---

## Contract-First: `api/openapi.yaml`

The OpenAPI file is written **before** the handler and reviewed with whoever
consumes the API.

Workflow:

1. Add or change the path, schemas and responses in `api/openapi.yaml`.
2. Review the diff — this is the contract review.
3. Implement the handler against it.
4. `make openapi` regenerates types/stubs; the generated file is committed.

Rules:

- **No annotations in Go code.** No swaggo comments, no `@Summary`, no `@Router`.
  The YAML is the source of truth; the handler never carries contract text.
- Every operation has `operationId`, `summary`, `tags`, and at least the error
  responses it can actually return.
- Schemas are named after the DTO concept (`Sale`, `CreateSaleRequest`,
  `Problem`), not inline blobs.
- One shared `Problem` schema for all errors — never a per-endpoint copy.
- Managed lists use the shared `PageMeta` schema.
- `deprecated: true` before removing anything ([HTTP Status](http_status.md)).

```yaml
paths:
  /sales/{saleId}:
    get:
      operationId: getSale
      summary: Get one sale
      tags: [Sales]
      parameters:
        - name: saleId
          in: path
          required: true
          schema: { type: string, format: uuid }
      responses:
        "200":
          description: The sale
          content:
            application/json:
              schema:
                type: object
                properties:
                  data: { $ref: '#/components/schemas/Sale' }
        "401": { $ref: '#/components/responses/Unauthorized' }
        "404": { $ref: '#/components/responses/NotFound' }
        "500": { $ref: '#/components/responses/Internal' }
```

---

## Document Types

### 1. Standard (rule)

A rule the team applies while writing code. Lives in `docs/standards/`.

```markdown
# <Topic>

**English** · [Bahasa Indonesia](<topic>.id.md)

[← Back to Standards](standard.md)

---

## Core Rule

One paragraph: the rule and why it exists.

---

## Examples

### Correct
### Avoid

---

## Quick Reference

| Rule | Correct | Avoid |
|---|---|---|
```

### 2. Architecture (explanation)

How the system is put together: layers, directories, flow, startup. One file,
updated whenever the structure changes.

### 3. Development (how-to)

Steps a person follows: install, configure, run, migrate, test, troubleshoot.
Ordered instructions, copy-pasteable commands.

### 4. Feature / domain doc (reference)

One domain's non-obvious knowledge. Write one **only** when at least one is true:

- the domain has rules the code cannot express (state machines, quotas, the
  accounting model);
- it integrates with an external system or an async job (import, export);
- a deviation from an obvious design was agreed and must be recorded.

Plain CRUD gets the OpenAPI entry and nothing else.

```markdown
# <Domain>

One paragraph: what it covers, which screens call it, what exists today.
Code: `internal/handlers/<domain>_handler.go` and siblings.
Contract: `api/openapi.yaml#/paths/~1<domain>`.

## Rules
Business rules with the reason, one per line (R-<DOMAIN>-001 …).

## Flows
Non-trivial flows: steps, decisions, failure handling.

## Endpoints
Table: method + path → handler → notes. Detail stays in OpenAPI.

## Decisions
What was chosen, what was rejected, and why (date).

## Open questions
One line each, with the owner.
```

### 5. ADR (decision record)

Optional. Use for a decision that will be questioned later: a new dependency, a
storage choice, a public contract. One file per decision under
`docs/decisions/ADR-NNN-<slug>.md`, with context, decision, consequences, status.

---

## English and Indonesian Twins

Every document has two files:

- `foo.md` — English, **canonical**.
- `foo.id.md` — Bahasa Indonesia, same headings, same tables, same order.

Rules:

1. Edit the English file first, then the twin **in the same PR**.
2. The Indonesian file is a **translation**, not a summary. Section order and
   anchors must match so readers can switch mid-document.
3. Code blocks, paths, commands and identifiers stay in English in both files.
4. Language switcher under the title:

   ```markdown
   **English** · [Bahasa Indonesia](foo.id.md)   <!-- in foo.md -->
   [English](foo.md) · **Bahasa Indonesia**       <!-- in foo.id.md -->
   ```

5. Links inside `.id.md` point to `.id.md` where a twin exists.
6. A missing twin is a review blocker; a stale twin is a bug.
7. `specification.md` is the exception: it is a product document and has no twin.

Terminology — keep these words in English inside the Indonesian text: handler,
service, store, endpoint, request, response, commit, branch, pull request, merge,
deploy, migration, payload, farm, journal.

| English | Bahasa Indonesia |
|---|---|
| rule | aturan |
| layer | lapisan |
| query | kueri |
| pagination | paginasi |
| error | galat (or "error" when naming a code) |
| test | pengujian |
| setup | penyiapan |
| example | contoh |

---

## Code Comments

Write a comment when the code cannot say it:

```go
// Sales are soft-deleted: deleted_at is set instead of removing the row, so the
// auto-generated journal entry keeps a valid foreign key.
```

Do not write:

```go
// Get sale by id.
func (s *saleService) GetSale(...)
```

Markers:

```go
// TODO(<owner>): replace the N+1 query with a single join. TICKET-123
// FIXME(<owner>): the legacy importer writes local dates; normalise before v2. TICKET-456
```

A marker without an owner or a ticket is noise.

---

## Rules

1. **Same commit as the code.** A behaviour change updates `api/openapi.yaml` and
   the relevant doc in the same commit — like tests.
2. **Record decisions, not guesses.** A deviation is documented after it is agreed,
   with the date.
3. **Link, don't copy.** Docs point to code by file name; code comments point to
   the doc path. Never duplicate a table.
4. **Every new doc is linked** from `docs/README.md` or the standards index.
5. **English canonical, Indonesian twin in sync.**
6. **Lean.** Short tables beat long prose. Delete what is no longer true.
7. **No secrets, no customer data, no internal URLs** in docs.
8. **Money is documented as a decimal.** Every example shows an amount as a JSON
   number (`264000`, `2200.5`), never a quoted string — matching
   `shopspring/decimal` with `MarshalJSONWithoutQuotes`.

---

## Checklist

- [ ] `api/openapi.yaml` updated; `make openapi` run and generated files committed
- [ ] The relevant doc updated (architecture, standards, feature) — or a new one
      created if the criteria above apply
- [ ] `.id.md` twin updated with the same structure and headings
- [ ] New doc linked from `docs/README.md` / `standards/standard.md`
- [ ] Code comments referencing a doc use the current path
- [ ] No duplicated fact: each table lives in exactly one file
- [ ] Markdown renders: code fences closed, tables aligned, links resolve

# API Naming & Nesting

**English** · [Bahasa Indonesia](api_naming.id.md)

[← Back to Standards](standard.md)

---

## Core Rule

The URL names the **resource**, the HTTP method names the **operation**, and the
path structure reflects **ownership** — never the database schema.

```
GET    /api/v1/sales                list
POST   /api/v1/sales                create
GET    /api/v1/sales/{saleId}      detail
PUT    /api/v1/sales/{saleId}      update
DELETE /api/v1/sales/{saleId}      soft delete
```

---

## 1. Versioning

Every business endpoint lives under `/api/v1`.

```
/api/v1/daily-records
/api/v1/sales/{saleId}
```

- Infrastructure endpoints sit outside the version: `/healthz`, `/readyz`, `/metrics`.
  The contract itself is served at `/api/v1/openapi.yaml`.
- A breaking change means `/api/v2` — never a silent change to a `/v1` contract.
- Additive changes (new optional field, new endpoint) stay in `/v1`.

---

## 2. Plural vs Singular

### Plural for resources (collections)

A resource is something you can create, list or own.

```
GET  /api/v1/sales
GET  /api/v1/sales/{saleId}
GET  /api/v1/daily-records
GET  /api/v1/expenses
GET  /api/v1/journal-entries
```

### Singular for module namespaces

A namespace groups related endpoints under a domain; it is not a collection.

```
/api/v1/stock/movements
/api/v1/accounting/accounts
/imports               ← a process, not a collection of "import"
/backup                ← one document of the whole farm
```

### How to tell the difference

Ask: *"Can I create or list one of these?"*

```
/sales       → yes, I create and list sales        → plural   ✓
/accounting  → no, accounting is a domain label    → singular ✓
```

---

## 3. Abbreviations Follow Their Role

No special case for abbreviations — the same plural/singular logic applies.

| Term | As a resource | As a namespace |
|---|---|---|
| HDP (hen-day production) | `/hdp` (a calculated report) | `/reports/hdp` |
| COA (chart of accounts) | `/accounts` | `/accounting/accounts` |

Spell the word out in the path unless the short form is the name the team and the UI
actually use.

---

## 4. CRUD Uses HTTP Methods, Not Action Names

| Operation | Correct | Avoid |
|---|---|---|
| List | `GET /sales` | `GET /get-sales` |
| Create | `POST /sales` | `POST /sales/create` |
| Detail | `GET /sales/{saleId}` | `GET /sales/detail/{saleId}` |
| Replace/update | `PUT /sales/{saleId}` | `PUT /sales/update/{saleId}` |
| Soft delete | `DELETE /sales/{saleId}` | `POST /sales/{saleId}/delete` |

`PUT` replaces the editable representation. The client sends `If-Match: "<version>"`
for optimistic locking ([Service & Store](service_repo.md)).

---

## 5. Business Actions May Use Verbs

When an operation is not a plain CRUD on the resource, an action verb is correct.

```
POST /api/v1/imports                     start a legacy JSON import
POST /api/v1/sales/{saleId}/void         cancel a sale, keep the row
POST /api/v1/journal-entries/{id}/post    post a manual journal
POST /api/v1/exports                      start an async export job
```

A verb is justified when the operation:

- changes state along a workflow (void, post, approve), or
- triggers a side effect that is not the resource itself (import, export, send).

Do not invent a verb for something HTTP already expresses. `create`, `update`,
`delete`, `get` are never path segments.

---

## 6. Nesting — Keep It Shallow

### When to nest

Nest when the parent genuinely **scopes or owns** the child.

```
/sales/{saleId}/lines                 ← lines only exist inside a sale
/accounts/{accountId}/journal-lines   ← lines only make sense per account
```

### Maximum three resource levels

```
/sales/{saleId}/lines                          ← 2 levels ✓
/journal-entries/{entryId}/lines               ← 2 levels ✓
/farms/{farmId}/sales/{saleId}/lines          ← 4 levels ✗ (farm is in the token)
```

The farm is identified by the token, not the URL — every authenticated request is
already scoped. Never add `/farms/{farmId}` to a business path.

### When to flatten

If a hierarchy exists only in the schema, flatten the route and use query
parameters.

```
❌ /farms/{farmId}/accounts/{accountId}/journal-entries
✓  /journal-entries?account_id=…&month=2026-10
```

---

## 7. Path Follows the Resource Returned

Name the path after what the endpoint returns, and group it with the screen or flow
that calls it.

```
GET /api/v1/reports/hdp                  ← returns the HDP report
GET /api/v1/settings                     ← returns farm settings
❌ GET /api/v1/farm/settings              ← the resource is settings, farm is context
```

---

## 8. Roles Are Not in the URL

Access is decided by the token's role (`viewer`, `editor`, `owner`) and enforced in
middleware — not by a path prefix.

```
✅ DELETE /api/v1/sales/{saleId}        editor and owner
✅ PUT    /api/v1/settings               owner only
❌ /admin/settings                       do not encode the role
```

- The same resource has one path for every role.
- A role with no access gets `403` from middleware, not `404`.
- Do not create a second endpoint just because the permission differs.

---

## 9. Filters and Pagination

| Concern | Where | Example |
|---|---|---|
| Identify a resource | Path | `/sales/{saleId}` |
| Scope to a parent | Path | `/sales/{saleId}/lines` |
| Date range | Query | `?from=2026-10-01&to=2026-10-31` |
| Month shortcut | Query | `?month=2026-10` |
| Search | Query | `?q=bondowoso` |
| Sorting | Query | `?sort=-date` (leading `-` = descending) |
| Paging | Query | `?page=1&pageSize=25` |
| Business state change | Path + verb | `/journal-entries/{id}/post` |

Never put a filter in the path as a pseudo-resource (`/sales/october`), and never put
an identifier in the query string when it identifies the resource itself.

---

## 10. One Purpose Per Endpoint

One endpoint does one thing. Do not switch behaviour with a query flag; give each
purpose its own route.

```
❌ GET /sales?view=detail            ← two payloads on one route
✓ GET /sales/{saleId}              ← detail
✓ GET /sales                        ← list

❌ GET /reports?type=hdp&export=true
✓ GET /reports/hdp
✓ POST /exports                     ← async export job
```

---

## 11. JSON Conventions

| Concern | Rule | Example |
|---|---|---|
| Field names | `camelCase` | `pageSize`, `feedBagPrice`, `createdAt` |
| Money / quantity | JSON number, ≤ 2 decimals, `decimal` on the server — never a quoted string | `2200.5` |
| Business dates | `YYYY-MM-DD`, no timezone | `"2026-10-31"` |
| Timestamps | ISO-8601 UTC | `"2026-10-31T04:12:00Z"` |
| IDs | UUID string (UUIDv7) | `"0192f3…"` |
| Enums | stable English `snake_case` | `"vaccine_medicine"` |
| Lists | always an object with `data` | `{ "data": [] }` |

---

## Quick Reference

| Rule | Correct | Avoid |
|---|---|---|
| Version prefix | `/api/v1/sales` | `/sales` |
| Resource collection | `/sales`, `/expenses` | `/sale`, `/expense` |
| Module namespace | `/accounting/accounts` | `/accounting/account` |
| CRUD | `DELETE /sales/{id}` | `POST /sales/{id}/delete` |
| Business action | `POST /journal-entries/{id}/post` | `POST /journal-entries/post/{id}` |
| Nesting depth | ≤ 3 levels | 4+ levels mirroring the schema |
| Farm scope | token | `/farms/{farmId}/…` in the path |
| Roles | middleware + `403` | `/admin/...` |
| Returned resource in the path | `/reports/hdp` | `/reports?type=hdp` |
| One purpose | separate routes | `?view=`, `?mode=` switching behaviour |
| Casing | kebab-case path, camelCase JSON | snake_case in paths, PascalCase JSON |
| Dates | `YYYY-MM-DD` | timestamps for business dates |

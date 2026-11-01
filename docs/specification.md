# R FARM Lestari ERP — Backend Specification

**Target stack:** Go + PostgreSQL · **Version:** 1.0 (draft for review) · **Date:** 2026-10-31
**Audience:** backend engineers (and the frontend engineer who integrates with them)
**Companion files:** `sql/schema.sql`, `sql/accounting_views.sql`, `sql/reference_queries.sql`, `sql/golden.sql`
(All SQL was executed against PostgreSQL 16 and the results verified — see [§13 Testing](#13-testing--quality).)

---

## 1. Purpose and scope

R FARM Lestari ERP is a small-farm (layer chicken / egg) management app used by the owner and staff in Bondowoso, Indonesia. Today it is a **frontend-only React app** that keeps all data in the browser (`localStorage`). This document specifies the **backend** that replaces that storage:

- a **Go REST API** backed by **PostgreSQL**,
- authentication and multi-user access,
- server-side validation, and calculations that are currently done in the browser,
- a **one-time migration** from the existing browser data (JSON backup) into PostgreSQL.

### In scope
CRUD for seven operational modules, farm settings, derived reports (dashboard, monthly/annual, stock, population), the accounting module (chart of accounts, manual journals, auto-posted journals, financial statements), backup/export/import, authentication/authorization, audit trail.

### Out of scope (v1)
Payments or bank integration, inventory valuation methods (FIFO/average), payroll, tax filing, mobile native apps, offline-first sync (see decision **D-8**), multi-currency.

### Success criteria
1. Every number shown in the current app can be reproduced by the API (parity), verified with the golden test vectors in [§7](#7-calculations).
2. An existing user can import their old JSON backup and see identical data.
3. No data is lost on concurrent edits (optimistic locking), and every change is auditable.

---

## 2. Current system (what exists today)

| Aspect | Today |
|---|---|
| Frontend | React 18 SPA (Vite), Zustand store, React Router (hash routes). Single HTML file. |
| Storage | `localStorage` key `rfarm_lestari_erp_v1`, one JSON document. |
| IDs | Client generated random strings (`Math.random().toString(36)…`). |
| Calculations | All done in the browser from the full data set (`src/lib/calc.js`, `src/lib/accounting.js`). |
| Users | None. Single implicit user. |
| Backup | “Download JSON” = the whole stored document; “Import” replaces everything. |
| Language/locale | UI in Indonesian, currency IDR, dates `YYYY-MM-DD`, timezone Asia/Jakarta. |

### Modules (menus)

| Module | Frontend route | Purpose |
|---|---|---|
| Daily input (Input Harian) | `/daily` | One record per day: birds counted, eggs, rejects, feed, deaths, conditions |
| Flock (Manajemen Ayam) | `/flock` | Bird additions, culling, bird sales |
| Alternative feed (Pakan Alternatif) | `/feedalt` | Corn, bran, azolla, etc. given or kept in stock |
| Egg sales (Penjualan Telur) | `/sales` | Sales transactions |
| Expenses (Biaya & Pembelian) | `/expenses` | Non-feed operating expenses |
| Stock (Stok Pakan & Telur) | `/inventory` | Feed/egg stock movements |
| Health (Kesehatan & Vaksin) | `/health` | Vaccines, checks, symptoms |
| Reports (Laporan 12 Bulan) | `/reports` | Annual recap by month |
| Accounting (Akuntansi) | `/accounting`, `/accounting/journal` | Statements + journals |
| Settings (Pengaturan) | `/settings` | Farm parameters, backup, reset |
| Dashboard | `/` | Monthly KPIs |

Each CRUD module has **three screens**: list (`/x`), create (`/x/new`), edit (`/x/:id/edit`).

---

## 3. Goals and non-goals

**Goals**
- Replace `localStorage` with PostgreSQL as the single source of truth.
- Keep the JSON field names the frontend already uses (camelCase) to minimise frontend changes.
- Move business calculations server-side (dashboard/reports/accounting) so the browser no longer needs the entire dataset.
- Provide a clean, documented, versioned REST API (OpenAPI 3.1, contract-first).
- Be small, boring and maintainable: one Go service, one database, one binary.

**Non-goals**
- Microservices, message queues, event sourcing, GraphQL.
- Changing business rules. Where current behaviour looks questionable it is **kept for parity** and listed in [Appendix C](#appendix-c--known-quirks-of-current-behaviour) as a decision.

---

## 4. Technology stack (recommended)

| Concern | Choice | Notes |
|---|---|---|
| Language | Go ≥ 1.23 | |
| HTTP router | `go-chi/chi` v5 | `net/http` compatible; Echo or Gin are acceptable if the team prefers |
| DB driver | `jackc/pgx` v5 + `pgxpool` | |
| Queries | `sqlc` (generated, type-safe) | Raw SQL files, no ORM |
| Migrations | `golang-migrate` or `goose` | Forward-only, versioned, run on deploy |
| Decimals | `shopspring/decimal` | **Never** use `float64` for money/quantities |
| Validation | `go-playground/validator` + domain-level checks | |
| Auth | JWT access token (15 min) + rotating refresh token; `argon2id` password hashing | |
| IDs | UUIDv7 (`google/uuid`) generated in the app | time-ordered, index friendly |
| Logging | `log/slog` (JSON) | request ID, user ID, farm ID on every line |
| API contract | OpenAPI 3.1 (`oapi-codegen` for server stubs/types) | spec lives in repo, reviewed first |
| Tests | `testing`, `testify`, `testcontainers-go` (real PostgreSQL) | |
| Packaging | Multi-stage Dockerfile, `docker-compose` for local dev | |
| Observability | `/healthz`, `/readyz`, Prometheus metrics (optional OpenTelemetry) | |

PostgreSQL **15 or newer** (tested on 16). Extensions: `citext`, `pg_trgm`.

---

## 5. Architecture

### 5.1 Layers

```
HTTP handler  →  service (business rules, transactions)  →  repository (sqlc queries)  →  PostgreSQL
   (DTO, auth, validation mapping)      (pure Go domain logic)
```

- **Handlers** parse/validate input, enforce authorization, map errors to HTTP problems. No SQL here.
- **Services** own transactions and business rules (population, stock, accounting). Pure functions for calculations so they are unit-testable without a database.
- **Repositories** are generated by `sqlc`; every query takes `farm_id`.

### 5.2 Suggested repository layout

```
backend/
  cmd/api/main.go
  api/openapi.yaml
  internal/
    config/            # env parsing
    http/              # router, middleware (auth, request id, recover, cors, rate limit), handlers
    auth/              # password hashing, JWT, refresh tokens
    domain/            # entities, enums, validation, calculations (pure)
      calc/            # dailyMetrics, monthStats, population, stock
      accounting/      # chart of accounts, auto journals, statements
    service/
    store/             # sqlc generated + hand-written queries
    importer/          # legacy JSON importer
  db/migrations/       # 0001_init.up.sql ...
  db/queries/          # *.sql for sqlc
  deploy/              # Dockerfile, compose, k8s (optional)
```

### 5.3 Tenancy

The current app has one farm. The schema is **multi-tenant ready** (every business row has `farm_id`; users belong to farms through `farm_members` with a role) but v1 only needs **one farm per deployment**. All repository queries MUST filter by `farm_id` taken from the authenticated principal — never from the request body.

### 5.4 Configuration (environment variables)

`DATABASE_URL`, `HTTP_ADDR`, `JWT_SIGNING_KEY` (or key file), `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `CORS_ALLOWED_ORIGINS`, `LOG_LEVEL`, `DEFAULT_TIMEZONE=Asia/Jakarta`, `MAX_IMPORT_BYTES=20971520`.

---

## 6. Data model

The complete, executable DDL is in **`sql/schema.sql`** (validated on PostgreSQL 16). Summary:

```
farms ─┬─ farm_settings (1:1)
       ├─ farm_members ── users ── refresh_tokens
       ├─ daily_records        (unique per farm + date)
       ├─ flock_events
       ├─ feed_alt_entries
       ├─ sales
       ├─ expenses
       ├─ stock_transactions
       ├─ health_records
       ├─ accounts             (chart of accounts, seeded per farm)
       ├─ journal_entries ── journal_lines   (MANUAL journals only; balanced by trigger)
       └─ audit_log
```

### 6.1 Conventions

- **Primary keys:** `uuid` (UUIDv7 generated by the app).
- **Business dates:** `DATE` (no timezone). The user’s calendar day is what matters; the server never shifts dates.
- **Audit timestamps:** `timestamptz` in UTC.
- **Money:** `numeric(14,2)` / `numeric(16,2)`; quantities `numeric(10,2)`/`(12,2)`; counts `integer`.
- **Row metadata on every record table:** `created_by`, `created_at`, `updated_at`, `version` (optimistic lock), `deleted_at` (soft delete), `legacy_id` (id from the old localStorage data, used by the importer for idempotency).
- **Soft delete:** user-initiated deletes set `deleted_at` (queries must filter `deleted_at IS NULL`; partial indexes already do). A hard purge is only done by the “reset farm data” operation. (Decision **D-4**.)
- **Enumerations:** stable English snake_case codes stored in PostgreSQL enum types. Indonesian display labels live in the frontend. Full mapping in [Appendix A](#appendix-a--enum-mapping-legacy--new).
- **Derived data is not stored:** auto-posted journals, monthly stats, stock levels and population are computed on demand (views/queries/Go), so they can never drift out of sync with the source records.

### 6.2 Resources and field rules

All rules below are enforced **twice**: by the API (returns field-level errors) and by DB constraints (last line of defence).
JSON names are what the API accepts/returns (they match the existing frontend form fields).

#### Settings — `farm_settings` (one per farm)

| JSON | Type | Rule | Default |
|---|---|---|---|
| `farmName` | string | required, ≤120 (stored in `farms.name`) | R FARM Lestari |
| `location` | string | ≤200 (`farms.location`) | Bondowoso, Jawa Timur |
| `housing` | enum | `battery` \| `postal` \| `other` | battery |
| `birds` | int | ≥ 0 — **initial** flock size | 30 |
| `feedBagPrice` | decimal | ≥ 0 (Rp per bag) | 400000 |
| `feedBagKg` | decimal | ≥ 1 | 50 |
| `feedTarget` | int | ≥ 0 (g / bird / day) | 110 |
| `eggPrice` | decimal | ≥ 0 (default Rp per egg) | 2200 |
| `eggWeight` | decimal | ≥ 1 (g) | 60 |
| `hdpTarget` | decimal | 0–100 (%) | 85 |
| `feedStock` | decimal | ≥ 0 — **opening** feed stock (kg) | 50 |
| `eggStock` | int | ≥ 0 — opening egg stock | 0 |

#### Daily record — `daily_records`

| JSON | Type | Rule |
|---|---|---|
| `id` | uuid | server generated |
| `date` | date | **required**, **unique per farm** (409 on duplicate) |
| `birds` | int | **required**, 0–10000. Counted live birds on that day = HDP denominator (a snapshot, *not* the system population) |
| `eggs` | int | **required**, ≥ 0 |
| `rejects` | int | ≥ 0, **≤ `eggs`** (cracked/culled eggs) |
| `feedGiven` | decimal | **required**, ≥ 0 (kg) |
| `feedLeft` | decimal | ≥ 0, **≤ `feedGiven`** (kg) |
| `eggWeight` | decimal \| null | > 0 (g); `null` ⇒ use farm default |
| `deaths` | int | ≥ 0 |
| `water` | enum | `yes` \| `no` \| `unchecked` |
| `condition` | enum | `normal` \| `hot` \| `humid` \| `ammonia` \| `needs_repair` |
| `notes` | string | ≤ 2000 |

#### Flock event — `flock_events`

| JSON | Type | Rule |
|---|---|---|
| `date` | date | required |
| `type` | enum | `addition` \| `cull` \| `sale` |
| `qty` | int | required, ≥ 1 |
| `price` | decimal | ≥ 0 (Rp per bird) |
| `party` | string | counterparty (supplier/buyer) |
| `batch` | string | group/age label |
| `payment` | enum | payment method (default `cash`; **new in v7** — old data has none ⇒ `cash`) |
| `notes` | string | |

#### Alternative feed — `feed_alt_entries`

| JSON | Type | Rule |
|---|---|---|
| `date` | date | required |
| `ingredient` | enum | `ground_corn` \| `bran` \| `dried_tofu_dregs` \| `azolla` \| `concentrate` \| `premix_mineral` \| `other` |
| `qty` | decimal | required, ≥ 0.01 (kg) |
| `price` | decimal | ≥ 0 (Rp / kg) |
| `source` | string | |
| `status` | enum | `given` (counts as consumed feed) \| `stock` (kept in store, no effect) |
| `notes` | string | |

#### Sale — `sales`

| JSON | Type | Rule |
|---|---|---|
| `date` | date | required |
| `qty` | int | required, ≥ 1 (eggs sold) |
| `price` | decimal | required, ≥ 0 (Rp / egg) |
| `returns` | int | ≥ 0 (returned/damaged eggs; reduce egg stock, **not** revenue) |
| `buyer` | string | |
| `payment` | enum | `cash` \| `transfer` \| `qris` \| `credit` |
| `notes` | string | |

#### Expense — `expenses`

| JSON | Type | Rule |
|---|---|---|
| `date` | date | required |
| `category` | enum | `vaccine_medicine` \| `disinfectant` \| `packaging` \| `transport` \| `utilities` \| `equipment` \| `labor` \| `coop_repair` \| `other` |
| `amount` | decimal | required, ≥ 1 (Rp) |
| `vendor` | string | |
| `payment` | enum | payment method |
| `description` | string | |
| `notes` | string | |

> Feed purchases are **not** expenses — they are stock movements (below). The UI warns about this to avoid double-counting.

#### Stock transaction — `stock_transactions`

| JSON | Type | Rule |
|---|---|---|
| `date` | date | required |
| `item` | enum | `feed` \| `egg` |
| `type` | enum | `in` \| `out` \| `adjust_plus` \| `adjust_minus` |
| `qty` | decimal | required, ≥ 0.01 |
| `unit` | string | **read-only**: `kg` for feed, `butir` for egg (generated column) |
| `price` | decimal | ≥ 0 (Rp per unit) |
| `source` | string | |
| `notes` | string | |

#### Health record — `health_records`

| JSON | Type | Rule |
|---|---|---|
| `date` | date | required |
| `type` | enum | `vaccination` \| `checkup` \| `symptom` \| `medication` \| `vitamin` \| `biosecurity` \| `death` \| `other` |
| `group` | string | flock/group label (default “N ekor”) |
| `action` | string | product / action |
| `person` | string | responsible person / vet |
| `nextDate` | date \| null | follow-up, must be ≥ `date` |
| `notes` | string | dose per label, result, follow-up |

#### Manual journal — `journal_entries` + `journal_lines`

The frontend form posts a **two-line** journal (one debit account, one credit account, one amount). The model supports N lines so future screens can post compound entries.

| JSON | Type | Rule |
|---|---|---|
| `date` | date | required |
| `ref` | string | unique per farm; if empty the server generates `MAN-<yyyymmdd>-<seq>` |
| `memo` | string | required |
| `notes` | string | |
| `cashFlowClass` | enum | `operating` \| `investing` \| `financing` (default `financing`) — **new**; used only for lines touching cash/bank (decision **D-6**) |
| `lines[]` | array | ≥ 2 lines; each `{ account, debit, credit }`, exactly one of debit/credit > 0; **Σ debit = Σ credit** |

**Compatibility shortcut:** the API also accepts the legacy flat form `{ debit: "1010", credit: "3010", amount: 1000000 }` and expands it to two lines.

Integrity guarantees: the DB has a **deferred constraint trigger** that rejects any unbalanced or <2-line journal at `COMMIT` (verified in testing).

#### Chart of accounts — `accounts`

31 default accounts (codes 1010…7010) are seeded per farm from `seed_default_accounts()` (listed in `sql/schema.sql`). Types: `asset`, `contra_asset`, `liability`, `equity`, `draw`, `revenue`, `expense`. v1: read-only list; v2: owner can add/deactivate accounts.

---

## 7. Calculations

These are the business formulas currently in the browser. The backend must reproduce them exactly. Reference SQL (verified) is in `sql/reference_queries.sql`. Implement them as **pure Go functions** (unit tested) *or* with the provided SQL — either is acceptable as long as the golden vectors pass.

### 7.1 Definitions

Let `S` = farm settings, and for a daily record `r`:

```
feedPricePerKg      = S.feedBagPrice / S.feedBagKg
commercialFeed(r)   = max(0, r.feedGiven − r.feedLeft)
altFeedGiven(date)  = Σ feed_alt.qty  where date = r.date AND status = 'given'
altFeedCost(date)   = Σ feed_alt.qty × price  (same filter)
consumed(r)         = commercialFeed(r) + altFeedGiven(r.date)
layable(r)          = max(0, r.eggs − r.rejects)
weight(r)           = r.eggWeight (if set) else S.eggWeight          [grams]
eggMassKg(r)        = layable(r) × weight(r) / 1000
HDP(r)              = r.eggs / r.birds   (0 if birds = 0)
FCR(r)              = consumed(r) / eggMassKg(r)   (0 if mass = 0)
```

### 7.2 Monthly statistics (`calcMonth`)

For all daily records `d`, sales `s`, expenses `e` whose date falls in the month:

```
eggs, rejects     = Σ over d
feed              = Σ consumed(d)                 (commercial + alternative, kg)
commercial        = Σ commercialFeed(d)
mass              = Σ eggMassKg(d)
birdDays          = Σ d.birds
HDP (month)       = eggs / birdDays               ← ratio of sums, NOT average of daily HDP
FCR (month)       = feed / mass
eggsSold          = Σ s.qty
revenue           = Σ s.qty × s.price
otherCosts        = Σ e.amount
feedCost          = commercial × feedPricePerKg + Σ altFeedCost (alt entries in month, status='given')
operatingProfit   = revenue − feedCost − otherCosts       (“Sisa operasional”)
daysRecorded      = count(d)
deaths            = Σ d.deaths
```

*Operating profit excludes depreciation, bird purchases and owner labour — this is a deliberate simplification shown in the UI.*

### 7.3 Population (`liveBirdsAt(date)`)

```
population(date) = max(0,
    S.birds
  + Σ flock.qty  × (+1 if type='addition' else −1)   for events with event_date ≤ date
  − Σ daily.deaths                                    for records with record_date ≤ date )
```

The daily `birds` field is **not** used here (it is only the HDP denominator).

### 7.4 Stock levels

```
feedStock = S.feedStock
          + Σ stock(item=feed).qty × (+1 for in/adjust_plus, −1 for out/adjust_minus)
          − Σ commercialFeed(all daily records)                       ← alternative feed is NOT deducted

eggStock  = S.eggStock
          + Σ stock(item=egg).qty × (±1 as above)
          + Σ (daily.eggs − daily.rejects)
          − Σ (sales.qty + sales.returns)
```

Negative stock is allowed and shown as is (parity).

### 7.5 Annual report

12 × the monthly statistics for `year`, plus totals: Σ of `eggs, feed, mass, revenue, feedCost, otherCosts, profit, eggsSold`; annual FCR = Σfeed / Σmass. Months without data are returned as zeros (not omitted).

### 7.6 Golden test vectors (must pass)

Dataset (`sql/golden.sql` loads exactly this):

| Table | Row |
|---|---|
| settings | birds 30, bag price 400 000, bag 50 kg, egg weight 60 g, opening feed 50 kg |
| daily | 2026-10-01: birds 30, eggs 24, rejects 2, feedGiven 3.3, feedLeft 0.3, deaths 1 |
| sales | 2026-10-02: qty 20 @ Rp 2 200, cash |
| flock | 2026-10-03: addition 10 @ Rp 50 000, cash |

Expected:

| Value | Result |
|---|---|
| Month 2026-10: eggs / feed kg / egg mass kg | **24** / **3** / **1.32** |
| Feed cost (3 kg × 8 000) | **Rp 24 000** |
| Revenue / other costs / operating profit | **Rp 44 000** / 0 / **Rp 20 000** |
| HDP / FCR | **0.80** (80 %) / **2.27** |
| Population on 2026-10-31 | **39** (30 − 1 + 10) |
| Feed stock / egg stock | **47 kg** / **2** (24−2 −20) |
| Auto journals | `SALE` Dr 1010 / Cr 4010 = 44 000 · `FEED` Dr 5010 / Cr 1040 = 24 000 · `BIRD` Dr 1210 / Cr 1010 = 500 000 |
| P&L Oct: revenue / expenses / profit | 44 000 / 24 000 / **20 000** |

The main values (eggs, feed, feed cost, revenue, profit, population, stock) are also asserted in the frontend unit tests (`tests/calc.test.js`) — both sides must agree.

---

## 8. Accounting rules

Double-entry bookkeeping over a fixed chart of accounts (SAK EMKM-style, simplified). **Operational records generate journals automatically (not stored); manual journals are stored.** `sql/accounting_views.sql` implements the auto-posting as view `v_auto_journal_lines` and `v_all_journal_lines` (verified: every journal balanced, balance-sheet difference = 0).

### 8.1 Auto-posting rules

“Settle account” = the account that settles the transaction by payment method: `cash`/`qris` → **1010 Kas**; `transfer` → **1020 Bank**; `credit` → receivable **1030** (sales, flock sales) or payable **2010** (expenses, flock purchases).

| Source | Condition | Debit | Credit | Amount |
|---|---|---|---|---|
| Egg sale | qty × price > 0 | settle (1030 if credit) | **4010** Penjualan Telur | qty × price |
| Expense | amount > 0 | by category (below) | settle (2010 if credit) | amount |
| Stock `in`/`adjust_plus`, with price | qty × price > 0 | **1040** (feed) / **1050** (egg) | **1010** Kas (always cash) | qty × price |
| Daily feed usage | cost > 0 | **5010** Beban Pakan | **1040** Persediaan Pakan | `round(commercialFeed × pricePerKg + altFeedCost, 2)` |
| Flock `addition` | qty × price > 0 | **1210** Ayam Produktif | settle (2010 if credit) | qty × price |
| Flock `sale` | qty × price > 0 | settle (1030 if credit) | **4020** Penjualan Ayam Afkir | qty × price |
| Flock `cull` | — | *no journal* | | |

Expense category → expense account: `vaccine_medicine`→5020 · `disinfectant`→5030 · `packaging`→5040 · `transport`→5050 · `utilities`→5060 · `labor`→5070 · `coop_repair`→5080 · `equipment`/`other`→5090.

Auto journal references are `SALE-<id>`, `EXP-<id>`, `STK-<id>`, `FEED-<id>`, `BIRD-<id>`; they are read-only in the UI (badge “Otomatis”).

### 8.2 Statements (for month `M`)

Account balance conventions: debit-normal = `asset`, `expense`, `draw`; credit-normal = `liability`, `equity`, `revenue`, `contra_asset`.

- **Income statement:** revenue = Σ(credit − debit) on revenue accounts dated in `M`; expenses = Σ(debit − credit) on expense accounts dated in `M`; net profit = revenue − expenses. Return every account line (including zeros) grouped by `group_name`.
- **Balance sheet (as of end of `M`, i.e. `entry_date < first day of M+1`):**
  - Assets = Σ asset balances − contra_asset balances
  - Liabilities = Σ liability balances
  - Equity = Σ equity balances − draw balance **+ retained earnings (net profit of all months before `M`) + net profit of `M`**
  - `difference = Assets − Liabilities − Equity` must be 0 (`balanced: true` when |diff| < 1). A non-zero difference is returned, not hidden.
  - No period-closing entries exist; retained earnings are computed.
- **Cash flow (direct method, month `M`)** — list of cash movements classified:

| Movement | Class | Amount |
|---|---|---|
| Egg sales not on credit | operating | +qty×price |
| Expenses not on credit | operating | −amount |
| Stock `in`/`adjust_plus` | operating | −qty×price |
| Flock `addition` not on credit | investing | −qty×price |
| Flock `sale` not on credit | investing | +qty×price |
| Manual journal lines on 1010/1020 | `cashFlowClass` of the journal (default financing) | debit − credit |

  Totals: operating, investing, financing, net change in cash.

> The statements are a **bookkeeping aid, not an audit opinion**. Keep the disclaimer text in the API response (`disclaimer`) so every client displays it.

---

## 9. API specification

Contract-first: write `api/openapi.yaml` first, review it with the frontend, then generate server types. Below is the required surface; request/response shapes follow §6.2.

### 9.1 Conventions

- Base path `/api/v1`. JSON only (`Content-Type: application/json; charset=utf-8`). All timestamps ISO-8601 UTC; business dates `YYYY-MM-DD`.
- **Naming:** camelCase JSON keys, plural kebab-case resource paths.
- **Decimals:** JSON numbers with ≤ 2 decimal places (server uses `decimal` internally; never `float64` arithmetic) — decision **D-7**.
- **Auth:** `Authorization: Bearer <access token>`. All endpoints except `/auth/*`, `/healthz`, `/readyz` require it.
- **Authorization roles:** `viewer` = read; `editor` = read + create/update/delete records; `owner` = everything incl. settings, journals’ accounts, import/reset, members.
- **List endpoints** support: `month=YYYY-MM` | `from`/`to` (dates), `q` (search), `sort` (`-date` default), `page` (default 1), `pageSize` (default 25, max 200). Response:

```json
{ "data": [ … ], "meta": { "page": 1, "pageSize": 25, "total": 137 } }
```

- **Optimistic locking:** every record has `version` (int). `PUT`/`DELETE` require header `If-Match: "<version>"`; mismatch ⇒ `412 Precondition Failed`. Responses carry `ETag`.
- **Idempotency:** `POST` accepts optional `Idempotency-Key` header (stored 24 h) to make client retries safe.
- **Errors:** `application/problem+json` (RFC 9457) with field-level detail that the frontend maps onto form fields:

```json
{
  "type": "https://rfarm.example/problems/validation",
  "title": "Validation failed",
  "status": 422,
  "code": "validation_failed",
  "requestId": "01J…",
  "errors": [
    { "field": "date", "code": "duplicate_date", "message": "Tanggal ini sudah memiliki input harian." },
    { "field": "rejects", "code": "exceeds_eggs", "message": "Telur afkir tidak boleh melebihi telur terkumpul." }
  ]
}
```

| Status | Used for |
|---|---|
| 400 | malformed JSON / query |
| 401 | missing/invalid/expired token |
| 403 | role not allowed |
| 404 | not found (also when it belongs to another farm) |
| 409 | uniqueness conflict (daily date, journal ref) |
| 412 | version mismatch |
| 422 | validation errors (field-level) |
| 429 | rate limited |
| 500 | unexpected (never leak internals) |

Stable **error codes** the frontend relies on (messages are Indonesian, codes are English): `required`, `min`, `max`, `invalid_enum`, `duplicate_date`, `exceeds_eggs`, `exceeds_feed_given`, `journal_unbalanced`, `same_account`, `invalid_account`, `next_before_date`, `version_conflict`.

### 9.2 Endpoints

#### Auth
| Method | Path | Notes |
|---|---|---|
| POST | `/auth/login` | `{ email, password }` → `{ accessToken, refreshToken, expiresIn, user, farm }` |
| POST | `/auth/refresh` | rotate refresh token (reuse of a revoked token revokes the whole family) |
| POST | `/auth/logout` | revoke refresh token |
| GET | `/me` | current user, farm, role |

#### Settings
| Method | Path | Role |
|---|---|---|
| GET | `/settings` | viewer |
| PUT | `/settings` | owner (`If-Match`) |

#### Record resources (same CRUD shape)
`/daily-records`, `/flock-events`, `/feed-alt-entries`, `/sales`, `/expenses`, `/stock-transactions`, `/health-records`

| Method | Path | Role | Notes |
|---|---|---|---|
| GET | `/{resource}` | viewer | list + filters above |
| POST | `/{resource}` | editor | 201 + `Location` |
| GET | `/{resource}/{id}` | viewer | |
| PUT | `/{resource}/{id}` | editor | full replacement, `If-Match` |
| DELETE | `/{resource}/{id}` | editor | soft delete, `If-Match`, 204 |
| GET | `/daily-records/by-date/{date}` | viewer | convenience; 404 if none |

Search `q` matches: daily → date, notes; flock → party, batch, notes, type; feed-alt → ingredient, source, notes; sales → buyer, payment; expenses → category, description, vendor; stock → item, type, source; health → type, action, notes.

Example — create a daily record:

```http
POST /api/v1/daily-records
{
  "date": "2026-10-31", "birds": 30, "eggs": 22, "rejects": 2,
  "feedGiven": 3.3, "feedLeft": 0, "eggWeight": null, "deaths": 0,
  "water": "yes", "condition": "normal", "notes": ""
}
→ 201
{ "id": "0192…", "version": 1, "date": "2026-10-31", "birds": 30, "eggs": 22, "rejects": 2,
  "feedGiven": 3.3, "feedLeft": 0, "eggWeight": null, "deaths": 0, "water": "yes", "condition": "normal",
  "notes": "", "metrics": { "consumedKg": 3.3, "hdp": 0.7333, "fcr": 2.75, "eggMassKg": 1.2 },
  "createdAt": "2026-10-31T09:30:00Z", "updatedAt": "2026-10-31T09:30:00Z" }
```

List rows of daily records also include the derived `metrics` object (the list screen shows consumption, HDP, FCR).

#### Derived / reporting (read-only)
| Method | Path | Returns |
|---|---|---|
| GET | `/dashboard?month=YYYY-MM` | monthly stats (§7.2), `population`, `stock {feedKg, eggs}`, `flock {added, removed, deaths}`, `recent[]` (6 latest activities), `chart {daily:[{day, eggs\|null}], targetEggs}`, `today {hasDailyRecord}` |
| GET | `/reports/monthly?month=YYYY-MM` | monthly statistics |
| GET | `/reports/annual?year=YYYY` | 12 months + totals (§7.5); `availableYears[]` |
| GET | `/reports/annual.csv?year=YYYY` | same as CSV (UTF-8 BOM, `;` separator — matches current export) |
| GET | `/stock/levels` | `{ feedKg, eggs }` |
| GET | `/flock/population?asOf=YYYY-MM-DD` | `{ asOf, birds }` (default: today in farm timezone) |

`targetEggs` for the dashboard chart = `population × hdpTarget / 100`.

#### Accounting
| Method | Path | Role | Notes |
|---|---|---|---|
| GET | `/accounting/accounts` | viewer | chart of accounts |
| GET | `/accounting/journals?month=&q=&page=` | viewer | auto **and** manual journals, each `{ id, ref, date, memo, source, auto, lines[] }`; `auto:true` rows are read-only |
| POST | `/accounting/journals` | editor | manual journal (lines or legacy flat form) |
| GET/PUT/DELETE | `/accounting/journals/{id}` | editor | **manual only**; auto ids → 404/405 |
| GET | `/accounting/statements?month=YYYY-MM` | viewer | `{ kpis, incomeStatement, balanceSheet {balanced, difference}, cashFlow {summary, details[]}, disclaimer }` |
| GET | `/accounting/journals.csv?month=YYYY-MM` | viewer | CSV export (Tanggal, Referensi, Keterangan, Sumber, Akun, Debit, Kredit) |

#### Backup, import, reset (owner only)
| Method | Path | Notes |
|---|---|---|
| GET | `/backup` | JSON in the **legacy-compatible shape** (Appendix B) so it can be re-imported into the old and new app |
| POST | `/import?mode=replace\|merge&dryRun=true\|false` | body = legacy JSON (see §10). Returns an import report |
| GET | `/export.csv` | all records as `MODUL;DATA JSON` (current “Ekspor semua data CSV”) |
| POST | `/farm/reset` | header `X-Confirm: DELETE-ALL-DATA`; hard-deletes records, keeps users/settings defaults |

#### Operational
`GET /healthz` (liveness), `GET /readyz` (DB reachable), `GET /metrics`, `GET /api/v1/openapi.yaml`.

---

## 10. Data migration (localStorage → PostgreSQL)

### 10.1 Source format

The legacy document (Appendix B) is exactly what “Backup JSON” downloads today. Older versions (ERP v4, v5, v6.1) produce the same shape with fewer collections; **missing collections are treated as empty**.

### 10.2 Import algorithm (`POST /import`)

1. **Parse & size-check** (≤ `MAX_IMPORT_BYTES`). Reject unless `settings` is an object and `daily` is an array (same check the current app performs).
2. **Normalise** each row: map enum labels (Appendix A), coerce numbers (empty → 0, but `eggWeight` 0/empty → `NULL`), trim strings, default missing `payment` on flock rows to `cash`.
3. **Validate** every row with the *same rules as the API*, applying the **legacy tolerance rules** in §10.2a first. Collect all problems into the report; do not stop at the first.
4. **Resolve duplicates:**
   - `daily` rows with the same date: keep the **last** one, report the others as `skipped_duplicate`.
   - Rows are idempotent by `(farm_id, legacy_id)` — re-running the same import never creates duplicates.
5. **Manual journals:** legacy rows have `lines:[{account,debit,credit},…]`; verify balance; unknown account codes ⇒ error. Duplicate `ref` values (the old app allowed them) ⇒ keep all, suffix later ones `-2`, `-3`… and report a warning.
6. **Write** inside **one transaction**: `replace` mode first hard-deletes the farm’s records; `merge` mode upserts by `legacy_id`. Settings are updated in both modes (owner chose to import them).
7. **Dry run** (`dryRun=true`) executes steps 1-5 plus a rollback-only transaction and returns the same report — the UI shows it before the user confirms.
8. Write an `audit_log` row (`action='import'`) with the report.

### 10.2a Legacy tolerance rules

The old app had **less validation** than the new API. A real backup can therefore contain rows the new rules would reject. The importer must not fail the whole import for these; it repairs them **visibly** (every repair is listed in the report as a `warning` with the original value):

| Legacy situation | Importer action | Report |
|---|---|---|
| `rejects` > `eggs` | clamp `rejects = eggs` | warning `clamped` |
| `feedLeft` > `feedGiven` | clamp `feedLeft = feedGiven` (old code already used `max(0, given − left)`, so consumption is unchanged) | warning `clamped` |
| Expense with `amount` = 0 (old form allowed it; it produced no journal) | skip the row | warning `skipped_zero_amount` |
| Health `nextDate` earlier than `date` | set `nextDate = NULL`, keep original text in `notes` | warning `next_before_date` |
| Flock row without `payment` | `cash` | none (expected for pre-v7 data) |
| `eggWeight` 0 / empty | `NULL` | none |
| Stock row where `unit` disagrees with `item` | derive unit from `item` | none |
| Two `daily` rows, same date | keep the last, skip the others | warning `skipped_duplicate` |

**Blocking errors** (nothing is imported): missing/invalid `date`, unknown enum label, `qty` ≤ 0 where a quantity is required, unknown account code, unbalanced manual journal, wrong top-level shape.

### 10.3 Import report

```json
{
  "mode": "replace", "dryRun": true, "ok": false,
  "counts": { "daily": {"read": 120, "imported": 118, "skipped": 2, "errors": 0},
              "sales": {"read": 40, "imported": 39, "skipped": 0, "errors": 1}, "...": {} },
  "issues": [
    { "collection": "sales", "legacyId": "k3j2h1", "row": 7, "field": "qty", "code": "min", "message": "qty must be ≥ 1" }
  ]
}
```

If any blocking `error` exists, **nothing is written** (all-or-nothing). `skipped` and repaired (`clamped`) entries are warnings only.

### 10.4 Cut-over plan

1. Deploy backend + Postgres, create the owner user and farm.
2. In the old app (browser that holds the data): **Settings → Download backup JSON**. Keep this file.
3. Log into the new app → Settings → Import → choose file → **dry run** → fix issues → import.
4. Compare the Dashboard/Reports for 2–3 months against the old app (numbers must be identical — §7 parity).
5. Freeze the old app (read-only) for one week, then retire it. The downloaded JSON remains the rollback.

---

## 11. Security requirements

- **Passwords:** `argon2id` (memory ≥ 64 MiB, time ≥ 3), per-user salt; minimum 10 characters; constant-time comparison. Login rate limit (e.g. 5 attempts / minute / IP+email) with exponential back-off.
- **Tokens:** access JWT (EdDSA or HS256 with ≥ 256-bit key) 15 min; refresh tokens random 256-bit, stored **hashed**, rotated on every use, revocable, 30-day expiry.
- **Transport:** TLS only (HSTS); secure, `HttpOnly`, `SameSite=Strict` cookie if the refresh token is delivered by cookie (then CSRF protection is required); otherwise keep refresh tokens out of `localStorage` where possible.
- **Authorization:** role check in middleware; **`farm_id` always from the token**; every SQL statement filters by it. Optional defence in depth: PostgreSQL Row-Level Security keyed on `current_setting('app.farm_id')`.
- **Input:** strict JSON decoding (`DisallowUnknownFields`), body size limit (1 MiB; import 20 MiB), parameterised queries only (sqlc), no string-built SQL, output encoding is the frontend’s job but the API must never reflect raw HTML.
- **CORS:** allow-list of the frontend origin(s) only.
- **Secrets:** environment/secret manager only; never in the repo or the image.
- **Audit:** every create/update/delete/import/reset writes `audit_log` (who, what, before/after JSON, when). Retain ≥ 2 years.
- **Privacy:** the data is business-sensitive (finance). Backups encrypted at rest; no PII beyond user email/name.
- **Dependencies:** `govulncheck` and `go mod verify` in CI; pinned base image, non-root container user.

---

## 12. Non-functional requirements

| Area | Requirement |
|---|---|
| Data volume | ≈ 1 farm × < 5 000 rows per table per year. Aggregates must answer in **< 200 ms (p95)** for 5 years of data. |
| Latency | CRUD < 100 ms p95 on a small VM (1 vCPU / 1 GB). |
| Availability | Single instance is acceptable (target 99.5 %); stateless service so it can scale horizontally. |
| Consistency | All writes transactional. Reads see committed data (default `READ COMMITTED`). Multi-row operations (import, reset, journal with lines) in one transaction. |
| Time & locale | Business dates are `DATE`. “Today” default for `population`/`asOf` is computed in the farm timezone (`farms.timezone`, default `Asia/Jakarta`). Currency IDR, formatted by the client. |
| Backups | `pg_dump` nightly + WAL archiving if possible; test a restore monthly. RPO ≤ 24 h, RTO ≤ 4 h. |
| Observability | JSON logs with request id; metrics (RPS, latency, error rate, DB pool); alerts on 5xx rate and DB down. |
| Compatibility | API is versioned (`/api/v1`); additive changes only within v1; breaking changes ⇒ `/v2`. |
| Internationalisation | Error `code` is stable English; `message` is Indonesian (`Accept-Language: id`), English acceptable as fallback. |
| Accessibility of errors | Field-level errors always include `field` so the UI can focus the invalid input. |

---

## 13. Testing & quality

1. **Unit tests** (pure Go): calculations (§7) and accounting (§8) with the golden vectors **and** table-driven edge cases: zero birds, zero egg mass, negative stock, `credit` payments, unbalanced manual journal, cull events, month boundaries (28/29/30/31 days, year end), duplicate dates.
2. **Database tests** (`testcontainers-go`, real PostgreSQL 16): migrations apply from scratch; constraint behaviour (verified manually already: unbalanced journal rejected at commit, duplicate daily date rejected, `rejects ≤ eggs`, generated `unit` column).
3. **API contract tests:** generate a client from `openapi.yaml`, run every endpoint including error cases; fail CI on spec drift.
4. **Parity test:** load `sql/golden.sql`; assert the API returns the §7.6 values. Run the same JSON dataset through the **frontend** calculation library and compare — both must match to the cent.
5. **Import tests:** fixtures from real v4, v5, v6.1 backups; idempotent re-import; invalid files; dry-run leaves no rows.
6. **Security tests:** cross-farm access attempts return 404; role matrix; expired/rotated tokens; rate limiting.
7. **Quality gates in CI:** `go vet`, `staticcheck`, `golangci-lint`, `govulncheck`, race detector (`go test -race`), coverage ≥ 80 % on `domain/` and `service/`.

**Already verified for this document** (PostgreSQL 16): `schema.sql` and `accounting_views.sql` apply cleanly; `reference_queries.sql` returns the §7.6 values (24 eggs, 3 kg feed, Rp 24 000 feed cost, Rp 44 000 revenue, Rp 20 000 profit, HDP 0.80, FCR 2.27, population 39, stock 47 kg / 2 eggs); auto journals balance and the balance-sheet difference is 0.

---

## 14. Deployment & operations

- **Container:** multi-stage build → distroless/alpine, non-root, read-only filesystem, `HEALTHCHECK` on `/healthz`.
- **Local dev:** `docker compose up` brings up `postgres:16` + the API with hot-reload (`air`) and seed data (`golden.sql`).
- **Environments:** `dev`, `staging` (copy of prod structure, anonymised data), `prod`.
- **Migrations:** run automatically at start-up by a one-shot job *before* the new version takes traffic; migrations must be backward compatible for one release (expand → migrate → contract).
- **CI/CD:** lint → test (with Postgres service) → build image → push → deploy staging → smoke test → manual approval → prod.
- **Runbook** (deliver with v1): restore from backup, rotate JWT key, revoke all sessions, run import, reset a user’s password.

---

## 15. Frontend integration (changes needed on the React side)

For coordination — the backend team does not implement these, but the contract must support them.

| Area | Change |
|---|---|
| Storage | Replace Zustand `localStorage` persistence with an API client. Keep Zustand for UI state (selected month, toast, menu); use **TanStack Query** for server data (cache, retry, invalidation). |
| Auth | Add login screen, token handling, 401 → refresh → retry, logout. Hide write actions for `viewer`. |
| Lists | Switch list pages to server-side search, month filter and pagination (`q`, `month`, `page`). |
| Calculations | Dashboard, Reports, Accounting pages call `/dashboard`, `/reports/*`, `/accounting/statements` instead of `src/lib/calc.js` / `accounting.js`. Keep those libs as the **reference implementation for parity tests** until the backend is accepted, then remove. |
| Enums | Forms send English codes; keep Indonesian labels in `config/modules.jsx` via `{ value, label }` options (already supported by the `Field` component). |
| Forms | Map `errors[].field` from problem+json onto field errors; send `If-Match` with `version` on edit; handle 409/412 with a “reload” message. |
| Flock | Form already has `payment`. |
| Journal | Add a `cashFlowClass` select (default financing) to the manual journal form. |
| Backup/Import | Settings page calls `/backup`, `/import?dryRun=true` (show report) then `/import`. |
| Offline | The single-file offline mode goes away (requires network) — see **D-8**. |
| Config | `VITE_API_BASE_URL`. |

---

## 16. Delivery plan and acceptance criteria

| Milestone | Scope | Acceptance criteria |
|---|---|---|
| **M1 Foundation** | Repo, CI, Docker compose, migrations (`schema.sql` as `0001`), config, logging, health, OpenAPI skeleton, auth (login/refresh/logout/me), farm + settings | Can log in; `GET/PUT /settings` works; CI green; migrations apply from empty DB |
| **M2 Records CRUD** | 7 resources with validation, search, pagination, optimistic locking, soft delete, audit log, role checks | All field rules in §6.2 enforced (API + DB); contract tests pass; cross-farm isolation test passes |
| **M3 Derived data** | Dashboard, monthly/annual reports, stock levels, population, CSV exports | Golden vectors §7.6 pass; annual report for 5 years < 200 ms p95 |
| **M4 Accounting** | Accounts, manual journals (balanced trigger), auto journals, statements, journal CSV | P&L, balance sheet (difference 0) and cash flow match the frontend for the same dataset; unbalanced journal rejected |
| **M5 Migration** | Backup, import (dry-run, merge/replace, idempotent), reset, importer fixtures | Real v4/v6.1 backups import without loss; dashboard numbers identical to old app |
| **M6 Hardening & release** | Rate limits, security tests, load test, backups/restore drill, runbook, docs | Penetration checklist clear; restore drill successful; v1.0 tagged |

Definition of done for every endpoint: documented in OpenAPI, unit + integration tests, authorization tested, audit logged (writes), error codes from the shared list.

---

## 17. Open decisions (please answer before M2)

| # | Decision | Recommendation |
|---|---|---|
| **D-1** | One farm per deployment, or true multi-tenant now? | Schema is ready for multi-tenant; **ship single-farm** (one seed farm) |
| **D-2** | Who logs in? Owner only, or owner + staff + read-only viewer? | Three roles as in §9.1 |
| **D-3** | API/DB enum values: English codes (recommended) vs. keep the current Indonesian strings? | **English codes**; frontend keeps labels. Importer maps legacy strings |
| **D-4** | Soft delete vs. hard delete of records | Soft delete + audit log (financial traceability) |
| **D-5** | Server-side vs. client-side calculations | Server-side (this spec); keep client libs only as test oracle |
| **D-6** | Add `cashFlowClass` to manual journals | Yes (cash-flow statement is otherwise always “financing” for manual cash lines) |
| **D-7** | Decimals as JSON numbers or strings? | Numbers with ≤ 2 dp (IDR amounts and kg fit exactly); revisit if sub-cent precision is ever needed |
| **D-8** | Offline use at the coop (poor connectivity)? | Not in v1. If needed: PWA + queued writes with `Idempotency-Key`, add in v1.1 |
| **D-9** | Refresh token in `HttpOnly` cookie vs. body | Cookie (`SameSite=Strict`) + CSRF header, if frontend and API share a site |
| **D-10** | Hosting (VPS / managed Postgres / cloud) and region (Indonesia latency, data residency) | To be decided by the owner |
| **D-11** | Keep or fix the quirks in Appendix C | Keep for parity in v1, fix in v1.1 with owner sign-off |

---

## Appendix A — Enum mapping (legacy → new)

| Field | Legacy label (stored today) | New code |
|---|---|---|
| payment (sales, expenses, flock) | Tunai · Transfer · QRIS · Tempo | `cash` · `transfer` · `qris` · `credit` |
| housing | Kandang baterai · Kandang postal · Lainnya | `battery` · `postal` · `other` |
| daily.water | Ya · Tidak · Tidak diperiksa | `yes` · `no` · `unchecked` |
| daily.condition | Normal · Panas · Lembap · Bau amonia · Perlu perbaikan | `normal` · `hot` · `humid` · `ammonia` · `needs_repair` |
| flock.type | Penambahan ayam · Pengurangan/afkir · Penjualan ayam | `addition` · `cull` · `sale` |
| feedAlt.status | Diberikan · Persediaan | `given` · `stock` |
| feedAlt.ingredient | Jagung giling · Dedak/bekatul · Ampas tahu kering · Azolla · Konsentrat · Premiks/mineral · Bahan lain | `ground_corn` · `bran` · `dried_tofu_dregs` · `azolla` · `concentrate` · `premix_mineral` · `other` |
| expense.category | Vaksin/obat · Desinfektan · Kemasan · Transportasi · Listrik/air · Peralatan · Tenaga kerja · Perbaikan kandang · Lainnya | `vaccine_medicine` · `disinfectant` · `packaging` · `transport` · `utilities` · `equipment` · `labor` · `coop_repair` · `other` |
| stock.item | Pakan · Telur | `feed` · `egg` |
| stock.type | Masuk · Keluar · Penyesuaian + · Penyesuaian - | `in` · `out` · `adjust_plus` · `adjust_minus` |
| health.type | Vaksin · Pemeriksaan · Gejala · Obat · Vitamin · Biosekuriti · Kematian · Lainnya | `vaccination` · `checkup` · `symptom` · `medication` · `vitamin` · `biosecurity` · `death` · `other` |

Unknown legacy values ⇒ import issue `invalid_enum` (not silently coerced), except `payment` missing on flock rows ⇒ `cash`.

## Appendix B — Legacy JSON shape (input to `/import`, output of `/backup`)

```json
{
  "settings": {
    "farmName": "R FARM Lestari", "location": "Bondowoso, Jawa Timur", "housing": "Kandang baterai",
    "birds": 30, "feedBagPrice": 400000, "feedBagKg": 50, "feedTarget": 110,
    "eggPrice": 2200, "eggWeight": 60, "hdpTarget": 85, "feedStock": 50, "eggStock": 0
  },
  "daily": [
    { "id": "k3j2h1x9", "date": "2026-10-31", "birds": 30, "eggs": 22, "rejects": 2,
      "feedGiven": 3.3, "feedLeft": 0, "eggWeight": 0, "deaths": 0,
      "water": "Ya", "condition": "Normal", "notes": "" }
  ],
  "sales":    [ { "id": "…", "date": "2026-10-31", "qty": 40, "price": 2200, "returns": 0, "buyer": "Warung A", "payment": "Tunai", "notes": "" } ],
  "expenses": [ { "id": "…", "date": "2026-10-31", "category": "Vaksin/obat", "amount": 75000, "vendor": "", "payment": "Tunai", "description": "", "notes": "" } ],
  "stock":    [ { "id": "…", "date": "2026-10-30", "item": "Pakan", "type": "Masuk", "qty": 50, "unit": "kg", "price": 8000, "source": "Toko", "notes": "" } ],
  "health":   [ { "id": "…", "date": "2026-10-20", "type": "Vaksin", "group": "30 ekor", "action": "ND-IB", "person": "Drh. Budi", "nextDate": "2026-11-20", "notes": "" } ],
  "flock":    [ { "id": "…", "date": "2026-10-03", "type": "Penambahan ayam", "qty": 10, "price": 50000, "party": "Pemasok", "batch": "pullet 17 mg", "payment": "Tunai", "notes": "" } ],
  "feedAlt":  [ { "id": "…", "date": "2026-10-31", "ingredient": "Jagung giling", "qty": 1.5, "price": 6000, "source": "", "status": "Diberikan", "notes": "" } ],
  "manualJournals": [
    { "id": "…", "date": "2026-10-01", "ref": "JRN-001", "memo": "Setoran modal", "notes": "", "source": "Manual",
      "lines": [ { "account": "1010", "debit": 1000000, "credit": 0 }, { "account": "3010", "debit": 0, "credit": 1000000 } ] }
  ]
}
```

Notes: `id` values are arbitrary strings (stored as `legacy_id`; the new UUIDs are generated). `eggWeight: 0` means “use default” ⇒ `NULL`. Numeric fields may arrive as numbers; strings of digits must be accepted.

## Appendix C — Known quirks of current behaviour

Documented so nobody “fixes” them by accident. Default for v1 = **keep (parity)**; each one needs an owner decision (D-11).

| # | Behaviour today | Consequence / suggestion |
|---|---|---|
| Q1 | Population = *current* `settings.birds` + events − deaths; changing the initial flock in Settings retroactively changes every past population. | Suggest storing an opening-balance event instead. |
| Q2 | Stock `out`/`adjust_minus` movements are subtracted **in addition to** automatic consumption (daily feed) and sales (eggs). | Possible double-counting if users also log those outflows manually. |
| Q3 | Feed deduction in stock ignores alternative feed; only commercial feed is deducted. | Alt feed has its own cost but no stock. |
| Q4 | Stock purchases always credit **Cash 1010**, regardless of how the user paid. | Journal may not match the bank/credit reality. |
| Q5 | `cull` flock events produce **no journal** (no loss in 5110). | Book value of culled birds is never written off. |
| Q6 | Egg sale *returns* reduce stock but not revenue. | Returns are not refunded in the books. |
| Q7 | No period-closing entries; retained earnings are computed from all prior journals. | Fine for small farms; document for the accountant. |
| Q8 | `QRIS` is booked to **Kas** (1010), not Bank. | Configurable mapping later. |
| Q9 | Cash-flow class for manual journals was always “financing” (the form had no field). | Fixed by D-6. |
| Q10 | “Today” in the old app used UTC for some defaults; the new frontend uses local date. Server must use the farm timezone. | Already specified in §12. |
| Q11 | Month-end filter in old code compared strings with `YYYY-MM-31`. | Backend uses `date >= first_of_month AND date < first_of_next_month`. |
| Q12 | Daily-record uniqueness was enforced only by the UI (a second save replaced the first). | Now a DB constraint; API returns 409 `duplicate_date`. |

## Appendix D — Companion files

| File | Purpose |
|---|---|
| `sql/schema.sql` | Enums, tables, constraints, indexes, balance trigger, default chart of accounts seed function |
| `sql/accounting_views.sql` | `settle_account()`, `v_auto_journal_lines`, `v_all_journal_lines` + statement formulas |
| `sql/reference_queries.sql` | Monthly stats, population, stock level queries (parameters `:farm`, `:m`, `:asof`) |
| `sql/golden.sql` | The §7.6 golden dataset |

Quick start for the backend team:

```bash
createdb rfarm
psql rfarm -v ON_ERROR_STOP=1 -f sql/schema.sql -f sql/accounting_views.sql
psql rfarm -f sql/golden.sql
psql rfarm -c "select seed_default_accounts('00000000-0000-0000-0000-000000000001')"
psql rfarm -v farm=00000000-0000-0000-0000-000000000001 -v m=2026-10-01 -v asof=2026-10-31 -f sql/reference_queries.sql
```
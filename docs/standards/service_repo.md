# Service & Store Logic

**English** · [Bahasa Indonesia](service_repo.id.md)

[← Back to Standards](standard.md)

---

## Principle

**Service owns business logic. Store owns data access.**

| Layer | Responsible for | Never does |
|---|---|---|
| **Handler** (`internal/handlers`) | Parse, validate, authorize, call one service method, render the response | Business rules, SQL |
| **Service** (`internal/services`) | Business rules, orchestration, transactions, state transitions, error decisions | SQL, `pgxpool.Pool`, `http.ResponseWriter` |
| **Store** (`internal/stores`) | SQL over sqlc, row mapping, transactions it opens, returning data or raw DB errors | Business decisions, HTTP status, client-facing messages |

The store answers *"did the query succeed, and what did it return?"*. The service
decides *"what does that mean for the business?"*.

> `stores/` is the repository layer. `stores/generated/` is sqlc output — treat it as
> read-only and never edit it by hand. The one non-SQL store is `SessionStore`, which
> reads and writes refresh sessions in Redis.

---

## Layer Walkthrough

```go
// internal/handlers/sale_handler.go — HTTP only
func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req dtos.CreateSaleRequest
    if err := utils.ParseAndValidate(r, &req); err != nil {
        response.Fail(w, r, err)              // 422 with field details
        return
    }

    sale, err := h.svc.CreateSale(r.Context(), dtos.CreateSaleInput{
        FarmID:  middleware.FarmIDFrom(r.Context()),
        Date:    req.Date,
        Qty:     req.Qty,
        Price:   req.Price,
        Buyer:   req.Buyer,
        Payment: req.Payment,
    })
    if err != nil {
        response.Fail(w, r, err)              // 409 / 500 / …
        return
    }

    response.Created(w, sale)
}
```

```go
// internal/services/sale_service.go — business rules
func (s *saleService) CreateSale(ctx context.Context, in dtos.CreateSaleInput) (*dtos.Sale, error) {
    if !in.Qty.IsPositive() {
        return nil, response.Unprocessable("qty must be greater than 0", nil)
    }
    if in.Price.IsNegative() {
        return nil, response.Unprocessable("price must be greater than or equal to 0", nil)
    }

    in.ID = uuid.Must(uuid.NewV7()).String()
    in.Amount = in.Qty.Mul(in.Price)

    sale := &dtos.Sale{ /* map fields, Version: 1 */ }
    if err := s.store.CreateSale(ctx, in); err != nil {
        return nil, err
    }
    return sale, nil
}
```

```go
// internal/stores/sale_store.go — SQL only
func (r *postgresSaleStore) CreateSale(ctx context.Context, in dtos.CreateSaleInput) error {
    err := r.q.InsertSale(ctx, generated.InsertSaleParams{
        ID:        in.ID,
        FarmID:    in.FarmID,
        Date:      in.Date,
        Qty:       in.Qty,
        Price:     in.Price,
        Amount:    in.Amount,
        CreatedBy: in.CreatedBy,
    })
    if err != nil {
        return mapDBError(err, "sale")        // unique/FK violations only
    }
    return nil
}
```

---

## Store Interfaces Over sqlc

SQL lives in `db/queries/*.sql`; `sqlc generate` produces
`internal/stores/generated`. The hand-written store file defines the interface the
service consumes and implements it over the generated queries.

```go
// internal/stores/sale_store.go
type SaleStore interface {
    ListSales(ctx context.Context, p dtos.ListSalesParams) ([]dtos.Sale, int64, error)
    GetSaleByID(ctx context.Context, farmID, saleID string) (*dtos.Sale, error)
    CreateSale(ctx context.Context, in dtos.CreateSaleInput) error
    UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) (int64, error)
    SoftDeleteSale(ctx context.Context, in dtos.DeleteSaleInput) (int64, error)
}

type postgresSaleStore struct {
    pool *pgxpool.Pool
    q    *generated.Queries
}

func NewSaleStore(pool *pgxpool.Pool) SaleStore {
    return &postgresSaleStore{pool: pool, q: generated.New(pool)}
}
```

Rules:

- One interface per entity, declared in `stores`, so the service only knows the
  contract.
- A service depends on the smallest interface it needs; a read-only feature gets a
  narrow interface, not the whole store.
- Constructors return the interface; the struct stays unexported.
- `farm_id` is a parameter of every method — never a field the caller can forget.
- No business methods like `CreateSaleAndPostJournal` unless the whole thing is one
  transaction; orchestration belongs to the service.

---

## Transactions

A transaction is a **data-access concern**. The service decides *that* an operation
must be atomic; the store implements it.

- One store method that touches several tables opens its own transaction and commits
  or rolls back inside that method.
- The service never receives a `pgx.Tx` and never commits anything.
- If a business flow can tolerate partial progress, call several store methods
  instead of one big transaction.

```go
func (r *postgresSaleStore) CreateSaleWithJournal(ctx context.Context, in dtos.CreateSaleInput) error {
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

    q := r.q.WithTx(tx)
    if err := q.InsertSale(ctx, toInsertSaleParams(in)); err != nil {
        return mapDBError(err, "sale")
    }
    if err := q.InsertAutoJournal(ctx, toAutoJournalParams(in)); err != nil {
        return mapDBError(err, "journal")
    }

    return tx.Commit(ctx)
}
```

> Rule of thumb: if the second statement must not survive without the first, they
> belong in the same store method and the same transaction.

---

## Session Store (Redis)

`SessionStore` is the one store that is not SQL. It lives in
`internal/stores/session_store.go`, talks to Redis through `pkg/cache`, and has no
sqlc query and no Postgres transaction.

```go
// internal/stores/session_store.go
type SessionStore interface {
    Create(ctx context.Context, s dtos.RefreshSession, ttl time.Duration) error
    Rotate(ctx context.Context, oldHash string, next dtos.RefreshSession, ttl time.Duration) error
    Delete(ctx context.Context, hash string) error
    RevokeUser(ctx context.Context, userID string) error
}

type redisSessionStore struct {
    rdb *redis.Client
}

func NewSessionStore(rdb *redis.Client) SessionStore {
    return &redisSessionStore{rdb: rdb}
}
```

Rules:

- Tokens are generated and hashed by the auth service (`pkg/security`); the store
  only ever sees a SHA-256 hash. A raw refresh token never reaches Redis or a log.
- `Rotate` is **atomic** — one Lua script or `WATCH`/`MULTI` roundtrip that deletes
  the old hash, writes the new one, and updates the family index. Two concurrent
  refreshes must not both succeed.
- A hash that is not live while its family is → treat it as reuse: revoke the family
  and return `response.Unauthorized("session expired")`.
- TTL comes from the caller (`cfg.Auth.RefreshTokenTTL`), reset on every rotation. The
  store never reads configuration.
- Redis has no transactions to hand to a service: there is no `pgx.Tx` equivalent
  here, and the service never sees the `redis.Client`.
- Session existence is a fact of the store (`redis.Nil` → not found). Whether that
  means `401`, "revoke the family", or nothing is the service's decision — except the
  atomic reuse check above.
- A transport error is returned unchanged; the error mapping turns it into `503`.
  `redis.Nil` is never a `500`.

---

## Context Rules

- `ctx context.Context` is the first parameter of every service and store method.
- Never store a context in a struct; never use `context.Background()` inside a
  service or store — take the request context.
- The HTTP layer sets the deadline; add a per-call timeout only around external
  I/O, in the client.
- Always pass `ctx` to pgx (`Query`, `QueryRow`, `Exec`) — no exceptions.
- Never let a goroutine outlive the request with the request's context.

---

## Optimistic Locking

### Why

Every record table carries `version int NOT NULL`. The client reads it from the
`ETag` header and sends it back in `If-Match` on update and delete. If the row moved
on, the request fails with **`412 Precondition Failed`** — not `409`.

| Case | Allowed? |
|---|---|
| Service makes several store calls for genuine multi-step business logic | ✓ Yes |
| Store combines the version check with the write in one statement or CTE | ✓ Yes, preferred |
| Service calls `GetByID`, compares the version, then calls a plain update | ✓ Yes, fallback |
| Service calls `GetByID`, decides, then updates **without** passing the version | ✗ Lost update |

### Pattern A — service-level check *(fallback)*

```
Service.UpdateSale()
  ├─ store.GetSaleByID()          → no row → NotFound
  ├─ current.Version != in.Version → 412 Precondition Failed
  └─ store.UpdateSale()           → plain UPDATE, 0 rows is not mapped to 404
```

```go
func (s *saleService) UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) error {
    current, err := s.store.GetSaleByID(ctx, in.FarmID, in.ID)
    if err != nil {
        return err
    }

    if current.Version != in.Version {
        return response.PreconditionFailed("sale has been modified by another user")
    }

    _, err = s.store.UpdateSale(ctx, in) // plain UPDATE, no version condition
    return err
}
```

**Limitation:** a small race window remains between the read and the write. Use B or
C for anything that matters.

### Pattern B — `WHERE version = ?` *(preferred, single lock condition)*

```sql
-- db/queries/sales.sql
-- name: UpdateSale :execrows
UPDATE sales
   SET date = $3, qty = $4, price = $5, amount = $6,
       version = version + 1, updated_at = now()
 WHERE id = $1 AND farm_id = $2 AND version = $7 AND deleted_at IS NULL;
```

```
Service.UpdateSale()
  └─ store.UpdateSale(id, farm_id, ..., version)
        ├─ 1 row affected  → success
        └─ 0 rows affected → store.GetSaleByID to disambiguate
              ├─ no row              → NotFound
              └─ row, version differs → 412 Precondition Failed
```

```go
func (s *saleService) UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) error {
    n, err := s.store.UpdateSale(ctx, in)
    if err != nil {
        return err
    }
    if n == 0 {
        if _, err := s.store.GetSaleByID(ctx, in.FarmID, in.ID); err != nil {
            return err // NotFound
        }
        return response.PreconditionFailed("sale has been modified by another user")
    }
    return nil
}
```

Happy path is one roundtrip and the update is atomic.

### Pattern C — atomic CTE *(recommended for new tables)*

Use when the update needs more than one check at once (existence, uniqueness, and
the version) resolved in a single atomic roundtrip.

```sql
WITH target AS (
    SELECT id, version FROM sales
     WHERE id = $1 AND farm_id = $2 AND deleted_at IS NULL
),
duplicate AS (
    SELECT 1 FROM sales, target
     WHERE sales.reference = $3 AND sales.id <> target.id
       AND sales.farm_id = $2 AND sales.deleted_at IS NULL
),
updated AS (
    UPDATE sales
       SET reference = $3, qty = $4, price = $5, amount = $6,
           version = version + 1, updated_at = now()
      FROM target
     WHERE sales.id = target.id
       AND target.version = $7
       AND NOT EXISTS (SELECT 1 FROM duplicate)
    RETURNING 1
)
SELECT CASE
    WHEN EXISTS (SELECT 1 FROM updated)    THEN 'updated'
    WHEN NOT EXISTS (SELECT 1 FROM target) THEN 'not_found'
    WHEN EXISTS (SELECT 1 FROM duplicate)  THEN 'duplicate'
    ELSE 'version_conflict'
END;
```

```go
func (r *postgresSaleStore) UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) error {
    var status string
    if err := r.pool.QueryRow(ctx, updateSaleSQL, in.ID, in.FarmID, in.Reference, in.Qty, in.Price, in.Amount, in.Version).Scan(&status); err != nil {
        return err
    }

    switch status {
    case "updated":
        return nil
    case "not_found":
        return response.NotFound("sale")
    case "duplicate":
        return response.Conflict("sale reference already exists")
    default: // "version_conflict"
        return response.PreconditionFailed("sale has been modified by another user")
    }
}

// service — thin pass-through, no separate GetByID needed
func (s *saleService) UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) error {
    return s.store.UpdateSale(ctx, in)
}
```

**B vs C:** Pattern B is enough when existence is the only check (0 rows affected).
Choose C when a second check — uniqueness, ownership, another rule — must be
resolved atomically with the write.

---

## Farm Scope and Soft Delete

Two rules every query follows:

```sql
WHERE farm_id = $1
  AND deleted_at IS NULL
```

- `farm_id` comes from the token (`middleware.FarmIDFrom(ctx)`), never from the
  body or a query parameter.
- A row of another farm must look exactly like a missing row: `404`, never `403`
  (that would confirm it exists).
- `DELETE` sets `deleted_at`; only the "reset farm data" operation hard-deletes.
- Migrations add partial indexes where `deleted_at IS NULL`.

---

## Error Ownership

| Situation | Owner | Returns |
|---|---|---|
| Row not found by ID | Store | `response.NotFound("sale")` |
| Unique key violation | Store | `response.Conflict("… already exists")` |
| FK violation while deleting | Store | `response.Conflict("… still in use")` |
| Version mismatch | Service (A) or store (B/C) | `response.PreconditionFailed(...)` |
| Invalid state transition | Service | `response.Conflict("…")` |
| Field-level validation | Handler / service | `response.Unprocessable(msg, fields)` |
| Missing/invalid session | Middleware | `response.Unauthorized(...)` |
| Refresh token reused or expired | Store (atomic check) | `response.Unauthorized("session expired")` |
| Redis unreachable | Any | `response.ServiceUnavailable(err)`, mapped to `503` |
| Role not allowed | Middleware | `response.Forbidden(...)` |
| Unexpected failure | Any | `response.Internal(err)`, logged once |

Store rules:

- `Get{Entity}ByID` returns `NotFound` when no row exists — row existence is a fact
  of the data, not a business decision.
- An atomic CTE (Pattern C) may return a named error directly.
- A **plain** `UPDATE`/`DELETE` returns `(rowsAffected, error)`. The service
  disambiguates 0 rows — the store must not guess `NotFound`.

---

## Quick Reference

| Decision | Layer |
|---|---|
| Parse the body, validate tags | Handler |
| Map `Request` → `Input` (add farm scope, version) | Handler |
| Choose the HTTP status | `pkg/response`, called by the handler |
| "Does this record exist?" | Store (`GetByID`, or the CTE checker) |
| "Was it modified by someone else?" | Compare `version`; B/C in the query |
| "Is this state transition allowed?" | Service |
| "Must these writes be atomic?" | Service decides, store implements |
| Run the SQL / open the transaction | Store |
| Generate, hash, and rotate a refresh token | Service decides; `SessionStore` persists atomically |
| Set the refresh session TTL | Service passes `cfg.Auth.RefreshTokenTTL`; store applies it |
| Which error to return | Service — unless already resolved atomically in the store |
| Log the error | Once, at the boundary, with `request_id` |

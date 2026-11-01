# Unit Testing

**English** · [Bahasa Indonesia](unit_test.id.md)

[← Back to Standards](standard.md)

---

## Philosophy

Tests exist to **protect behaviour**, not to maximise a coverage number. A small
suite that guards every meaningful decision branch is worth more than a large suite
written to hit a percentage.

> **Rule of thumb:** one test case = one meaningful behaviour.

---

## Principles

- Cover every **behaviour-changing branch**.
- Test behaviour, not implementation details.
- Don't write separate tests for inputs that produce identical behaviour.
- **Critical business and security rules are always covered**: farm scoping, money
  arithmetic, journal balance, role checks, optimistic locking, session rotation and
  reuse detection.
- Tests must be deterministic: no real clock, no network, no shared database state.
- Every test fails for exactly one reason and says which.

---

## What to Test Where

```
Handler (internal/handlers)
  ├── Request parsing and validation → status code and problem body.
  ├── Farm scope and version taken from context/headers, never from the body.
  └── Do not repeat business rules here.

Service (internal/services)              ← primary unit-test target
  ├── Every business rule and decision branch.
  ├── Every dependency (store/client) failure path.
  ├── Money and quantity arithmetic with decimal.
  └── Ownership and state-transition checks.

Store (internal/stores)
  ├── Integration tests against a real PostgreSQL (testcontainers).
  ├── Filters, pagination, sorting, soft delete, version locking, CTE outcomes.
  └── Session store: unit tests with miniredis (no Docker needed).

Middleware (pkg/middleware)
  └── Auth, role enforcement and the exact status returned.
```

A service test uses a fake store, never a database. A store test uses a real
database, never a fake. The session store is the exception: its unit tests use
`miniredis` and carry no `integration` build tag.

---

## File Layout

One test file per source file, named after it:

| Source | Test |
|---|---|
| `sale_service.go` | `sale_service_test.go` |
| `sale_handler.go` | `sale_handler_test.go` |
| `sale_store.go` | `sale_store_test.go` (`//go:build integration`) |
| `session_store.go` | `session_store_test.go` (miniredis, no build tag) |
| `sale_fakes_test.go` | shared fakes for the sale package |

Rules:

- Package `_test` (black box) by default: tests use the exported interface.
- `_internal_test.go` (white box) only when an unexported helper must be tested —
  the suffix makes the exception visible.
- `//go:build integration` for anything that needs PostgreSQL; `go test ./...` must
  stay fast and offline.
- Shared fakes and helpers: `<entity>_fakes_test.go`, one file per package, so an
  interface change is fixed in one place.

```
internal/services/
├── sale_service.go
├── sale_service_test.go
├── sale_fakes_test.go          ← fakeSaleStore, newTestService
└── journal_service.go
    └── journal_service_test.go
```

---

## Test Naming

```go
func TestCreateSale(t *testing.T)
func TestListSales(t *testing.T)
func TestUpdateSale(t *testing.T)
func TestSaleHandler_Create(t *testing.T)
func TestSaleStore_UpdateSale_VersionConflict(t *testing.T)
```

- `Test{Method}` for a service or store.
- `Test{Type}_{Method}` when several types share a method name.
- Add a suffix only for the behaviour the case pins down, not for every case.

Table-driven with named cases:

```go
tests := []struct {
    name      string
    input     dtos.CreateSaleInput
    setup     func(*fakeSaleStore)
    wantCode  string // stable error code, "" means success
    wantField string // optional: which field the error points at
}{
    {name: "success", input: validInput},
    {name: "store fails", input: validInput, setup: func(s *fakeSaleStore) {
        s.createErr = errors.New("db down")
    }, wantCode: "internal"},
    {name: "qty not positive", input: zeroQty, wantCode: "validation_failed", wantField: "qty"},
    {name: "duplicate date", input: duplicateDate, wantCode: "duplicate_date"},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { /* … */ })
}
```

Assert the **code**, not the wording: messages may be reworded, codes are the
contract.

---

## Baseline Cases

| # | Case |
|---|---|
| 1 | Happy path |
| 2 | Dependency (store/client) error |
| 3 | The core business rule for that operation |
| 4 | Farm isolation — a row of another farm behaves as missing (`404`) |

```text
CreateSale
├── success
├── store error
├── qty ≤ 0                          ← field rule
└── duplicate daily record           ← uniqueness

UpdateSale
├── success
├── store error
├── version mismatch                 ← 412
└── row belongs to another farm      ← 404, not 403
```

There is no fixed number. The count reflects the **complexity of the logic**.

---

## Fakes

Hand-written fakes, one per interface, in `<entity>_fakes_test.go`. No mocking
framework is required.

Services that call `SessionStore` get a `fakeSessionStore`:

```go
type fakeSessionStore struct {
    createErr  error
    rotateErr  error
    deleteErr  error
    revokeErr  error
    reuse      bool   // set true to simulate token reuse → 401
}

func (f *fakeSessionStore) Create(_ context.Context, _ dtos.RefreshSession, _ time.Duration) error {
    return f.createErr
}
func (f *fakeSessionStore) Rotate(_ context.Context, _ string, _ dtos.RefreshSession, _ time.Duration) error {
    if f.reuse {
        return response.Unauthorized("Sesi sudah berakhir. Silakan masuk kembali.")
    }
    return f.rotateErr
}
func (f *fakeSessionStore) Delete(_ context.Context, _ string) error { return f.deleteErr }
func (f *fakeSessionStore) RevokeUser(_ context.Context, _ string) error { return f.revokeErr }
```

Session store itself is tested with `miniredis`:

```go
// internal/stores/session_store_test.go  — no build tag
func TestSessionStore_Rotate_Reuse(t *testing.T) {
    mr := miniredis.RunT(t)
    rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

    store := stores.NewSessionStore(rdb)
    ttl := 30 * 24 * time.Hour

    sess := dtos.RefreshSession{UserID: "u1", FarmID: "f1", FamilyID: "fam1"}
    hash := sha256Hex("token-A")
    require.NoError(t, store.Create(context.Background(), sess, ttl))

    // Rotate once — succeeds
    sess2 := dtos.RefreshSession{UserID: "u1", FarmID: "f1", FamilyID: "fam1"}
    require.NoError(t, store.Rotate(context.Background(), hash, sess2, ttl))

    // Rotate with old hash again — reuse detected
    err := store.Rotate(context.Background(), hash, sess2, ttl)
    var appErr *response.AppError
    require.ErrorAs(t, err, &appErr)
    assert.Equal(t, "unauthorized", appErr.Code)
}
```

Rules for fakes:

```go
type fakeSaleStore struct {
    created   *dtos.CreateSaleInput
    createErr error

    item  *dtos.Sale
    getErr error

    updated   *dtos.UpdateSaleInput
    updateN   int64
    updateErr error
}

func (f *fakeSaleStore) CreateSale(_ context.Context, in dtos.CreateSaleInput) error {
    if f.createErr != nil {
        return f.createErr
    }
    f.created = &in
    return nil
}

func (f *fakeSaleStore) GetSaleByID(_ context.Context, farmID, saleID string) (*dtos.Sale, error) {
    return f.item, f.getErr
}

func (f *fakeSaleStore) UpdateSale(_ context.Context, in dtos.UpdateSaleInput) (int64, error) {
    if f.updateErr != nil {
        return 0, f.updateErr
    }
    f.updated = &in
    return f.updateN, nil
}
```

- The fake implements the whole store interface; unused methods return zero values.
- Record what was passed in a field when the test needs to prove it (`f.created`).
- Never fake the type under test. Never mock `context.Context`.
- `fakeSessionStore` covers: Redis down (set `createErr`/`rotateErr`), reuse detection,
  revoke-all path.

---

## Handler Tests

Use `httptest` + the real `chi` router — no network listener.

```go
func TestSaleHandler_Create(t *testing.T) {
    h := handlers.NewSaleHandler(fakeSaleService{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
    r := chi.NewRouter()
    h.Register(r)

    body := strings.NewReader(`{"date":"2026-10-31","qty":-1}`)
    req := httptest.NewRequest(http.MethodPost, "/sales", body)
    req.Header.Set("Content-Type", "application/json")

    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)

    require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
    require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

    var p response.Problem
    require.NoError(t, json.NewDecoder(rec.Body).Decode(&p))
    assert.Equal(t, "validation_failed", p.Code)
    assert.Equal(t, "qty", p.Errors[0].Field)
}
```

Cover: success status and body, validation failure (`422`), not found (`404`),
version mismatch (`412`), missing token (`401`), role denial (`403`).

Farm scope is asserted explicitly — a handler that reads `farmId` from the body must
fail the test.

---

## Store Tests (integration)

`testcontainers-go` starts a throwaway PostgreSQL, applies the real migrations, and
gives every test its own transaction or truncation.

```go
//go:build integration

package stores_test

func TestSaleStore_ListSales(t *testing.T) {
    pool := tests.NewPostgres(t)          // migrated + truncated, closed by t.Cleanup
    s := stores.NewSaleStore(pool)

    farmA, farmB := tests.SeedFarm(t, pool), tests.SeedFarm(t, pool)
    tests.SeedSale(t, pool, farmA.ID, "2026-10-01", "100")
    tests.SeedSale(t, pool, farmA.ID, "2026-10-31", "200")
    tests.SeedSale(t, pool, farmB.ID, "2026-10-15", "300")

    got, total, err := s.ListSales(ctx, dtos.ListSalesParams{
        FarmID: farmA.ID, Offset: 0, Limit: 25, SortField: "date", SortDesc: true,
    })

    require.NoError(t, err)
    assert.EqualValues(t, 2, total)        // farmB row excluded
    require.Len(t, got, 2)
    assert.Equal(t, "2026-10-31", got[0].Date) // newest first
}
```

Cover at least: pagination boundaries, sort allowlist, every filter, soft-deleted
rows excluded, other-farm rows excluded, unique and FK violations mapped to `409`,
and the version-locking outcomes of patterns B/C.

Run them with `make test-integration`.

---

## Test Helpers

`internal/tests` holds everything shared between packages:

| File | Contents |
|---|---|
| `postgres.go` | `NewPostgres(t)` — container, migrations, cleanup |
| `seed.go` | `SeedFarm`, `SeedSale`, `SeedDailyRecord`, … |
| `http.go` | request builders, `DecodeProblem(t, rec)`, `DecodeData[T]` |
| `clock.go` | fixed clock used by services instead of `time.Now` |
| `redis.go` | `NewMiniredis(t)` — miniredis instance + `*redis.Client`, closed by cleanup |

- Helpers return `*testing.T`-registered cleanups, never leak containers.
- Helpers assert nothing about the test's intent — they only prepare state.
- A helper used by a single package stays in that package's `_test.go` file.

---

## Commands

```bash
make test               # go test ./... -race -count=1
make test-cover         # coverage profile + HTML report
make test-integration   # go test ./... -tags=integration -count=1
go test ./internal/services -run TestCreateSale -v
go test ./internal/stores -tags=integration -run TestSaleStore_UpdateSale -v
```

---

## Coverage

Coverage is a measurement, not a goal.

| Signal | Meaning |
|---|---|
| High coverage, meaningful cases | Good suite |
| High coverage, trivial cases | False confidence |
| Lower coverage, all key behaviours covered | Acceptable |
| Missing critical business/security rules | **Not acceptable** |

CI enforces a **floor** (e.g. 60% on `internal/services`), never a target to chase.
A drop on changed lines is explained in the PR, not hidden with trivial tests.

---

## Don't Test

- Getters, setters, constructors with no logic.
- Third-party libraries and sqlc-generated code.
- SQL string formatting — assert behaviour against a real database instead.
- The same branch twice through different entry points.
- `time.Now()` output; inject a clock instead.

---

## Quick Reference

| Layer | Tool | Database | Speed |
|---|---|---|---|
| Service | fake store/client (incl. `fakeSessionStore`) | none | ms |
| Handler | `httptest` + chi router + fake service | none | ms |
| Store (Postgres) | `testcontainers-go` PostgreSQL, `integration` tag | yes | seconds |
| Store (session) | `miniredis`, no build tag | no | ms |
| Middleware | `httptest` + chi router | none | ms |

| Rule | Correct | Avoid |
|---|---|---|
| One file per source | `sale_service_test.go` | `all_test.go` |
| Black box default | `package services_test` | reaching into privates |
| Assert the contract | error code `version_conflict` | exact message text |
| Table-driven | named subtests | copy-pasted bodies |
| Fake dependencies | hand-written fake | mock the type under test |
| Session store tests | `miniredis`, no Docker | `testcontainers-go` for Redis |
| Farm isolation | explicit test | assuming the query is scoped |
| Deterministic | fixed clock / seed | `time.Now()` in assertions |

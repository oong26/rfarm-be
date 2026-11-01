# Method Naming

**English** · [Bahasa Indonesia](method_naming.id.md)

[← Back to Standards](standard.md)

---

## Core Rule

A method name consists of an **operation**, an **entity**, and — only when
mandatory — the **scope** it belongs to.

```
Verb + Entity (+ By{Parent})
```

The same shape is used in every layer, so a reader can follow one operation from
the handler down to the SQL without guessing names.

---

## List Methods

| Condition | Convention | Example |
|---|---|---|
| No required scope | `List{Entity}` | `ListSales(ctx, params)` |
| Must be scoped to a parent | `List{Entity}By{Parent}` | `ListLinesBySale(ctx, saleID)` |

`By{Parent}` means the caller **must** supply the parent ID — the entity cannot be
listed without it. Do not add `By{Parent}` when the parent is only an optional
filter; use a filter field instead.

```go
ListSales(ctx, params)                 // all sales of the farm, filters optional
ListLinesBySale(ctx, saleID)           // lines cannot exist without a sale
```

`farm_id` is not a `By{Farm}` suffix: it is present on every method as a scope
parameter, because no query may run without it.

---

## Single-Entity CRUD

Single-entity operations never carry a `By{Parent}` suffix — the entity ID already
identifies the record.

```go
GetSale(ctx, farmID, saleID)
CreateSale(ctx, input)
UpdateSale(ctx, input)
DeleteSale(ctx, farmID, saleID)
```

Not `GetSaleById`, not `GetSaleByFarm`. The identifiers are part of the signature,
not the name.

---

## Handler Methods

Handlers are named after the use case only; the layer already says `Handler`.

```go
func (h *SaleHandler) List(w http.ResponseWriter, r *http.Request)
func (h *SaleHandler) Get(w http.ResponseWriter, r *http.Request)
func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request)
func (h *SaleHandler) Update(w http.ResponseWriter, r *http.Request)
func (h *SaleHandler) Delete(w http.ResponseWriter, r *http.Request)
```

- No `Handle`, `Do`, `Process` prefixes.
- One method per route; the router is the mapping (`GET /sales → h.List`).
- The signature is always `(http.ResponseWriter, *http.Request)`, so it can be
  passed to chi directly.
- The path parameter is read with `chi.URLParam(r, "sale_id")` inside the method.

---

## Service Methods

Service methods describe the business operation. They mirror the handler name and
add the scope only when the rule requires it.

```go
type SaleService interface {
    ListSales(ctx context.Context, in dtos.ListSalesInput) (dtos.ListSalesResult, error)
    ListLinesBySale(ctx context.Context, farmID, saleID string) ([]dtos.SaleLine, error)
    GetSale(ctx context.Context, farmID, saleID string) (*dtos.Sale, error)
    CreateSale(ctx context.Context, in dtos.CreateSaleInput) (*dtos.Sale, error)
    UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) error
    DeleteSale(ctx context.Context, in dtos.DeleteSaleInput) error
    VoidSale(ctx context.Context, in dtos.VoidSaleInput) error
}
```

- `ctx context.Context` is always the first parameter.
- `error` is always the last return value.
- A verb that is a business action keeps its name (`VoidSale`, `PostJournal`), never
  `ChangeSaleStatus`.
- Create returns the created entity so the handler can answer `201` with a body.

---

## Store Methods

Store methods answer one data question each, and use the same operation + entity
shape. The PostgreSQL implementation sits on top of sqlc-generated queries.

```go
type SaleStore interface {
    ListSales(ctx context.Context, p dtos.ListSalesParams) ([]dtos.Sale, int64, error)
    GetSaleByID(ctx context.Context, farmID, saleID string) (*dtos.Sale, error)
    CreateSale(ctx context.Context, in dtos.CreateSaleInput) error
    UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) (int64, error)
    SoftDeleteSale(ctx context.Context, in dtos.DeleteSaleInput) (int64, error)
}
```

- `Get{Entity}ByID` for a single row — the `ByID` suffix distinguishes it from a
  business-level `Get{Entity}` that may assemble several queries.
- `List{Entity}` returns rows **and** the total count together; both come from one
  consistent read.
- `SoftDelete{Entity}` rather than `Delete{Entity}` when the row is kept
  (`deleted_at`), which is the default here.
- Plain writes return `(rowsAffected int64, error)` so the service can decide
  whether zero rows means "not found" or "version conflict".

---

## Constructors and Interfaces

| Kind | Convention | Example |
|---|---|---|
| Constructor | `New{Type}` | `NewSaleHandler`, `NewSaleService`, `NewSaleStore` |
| Interface | `{Entity}{Layer}` | `SaleService`, `SaleStore` |
| Implementation | `{layer}{Entity}` (unexported) | `saleService`, `postgresSaleStore` |
| Fake (tests) | `fake{Type}` | `fakeSaleStore` |
| File | `{entity}_{layer}.go` | `sale_service.go`, `sale_handler.go`, `sale_store.go` |

The constructor returns the interface, the struct stays unexported. That is what
makes the layer replaceable in tests without touching the caller.

```go
func NewSaleService(st stores.SaleStore) SaleService {
    return &saleService{store: st}
}
```

---

## Booleans, Getters and Prefixes

| Rule | Correct | Avoid |
|---|---|---|
| Boolean reads as a statement | `isPosted`, `hasStock` | `postedFlag` |
| Boolean method | `CanVoid(ctx, sale) (bool, error)` | `CheckVoid` |
| No `Get` on a plain field accessor | `sale.Qty` | `sale.GetQty()` |
| `Get` only for I/O | `GetSale(ctx, farmID, id)` | `FetchSaleData` |
| No layer name inside the name | `VoidSale` | `ServiceVoidSale` |
| No abbreviations the domain does not use | `GetSale` | `GetSl` |

---

## Full Example

```go
// internal/handlers/sale_handler.go
type SaleHandler struct {
    svc services.SaleService
    log *slog.Logger
}

func NewSaleHandler(svc services.SaleService, log *slog.Logger) *SaleHandler {
    return &SaleHandler{svc: svc, log: log}
}

func (h *SaleHandler) Register(r chi.Router) {
    r.Route("/sales", func(r chi.Router) {
        r.Get("/", h.List)
        r.Post("/", h.Create)
        r.Route("/{saleId}", func(r chi.Router) {
            r.Get("/", h.Get)
            r.Put("/", h.Update)
            r.Delete("/", h.Delete)
        })
    })
}
```

```go
// internal/services/sale_service.go
type SaleService interface {
    ListSales(ctx context.Context, in dtos.ListSalesInput) (dtos.ListSalesResult, error)
    GetSale(ctx context.Context, farmID, saleID string) (*dtos.Sale, error)
    CreateSale(ctx context.Context, in dtos.CreateSaleInput) (*dtos.Sale, error)
    UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) error
    DeleteSale(ctx context.Context, in dtos.DeleteSaleInput) error
}
```

```go
// internal/stores/sale_store.go
type SaleStore interface {
    ListSales(ctx context.Context, p dtos.ListSalesParams) ([]dtos.Sale, int64, error)
    GetSaleByID(ctx context.Context, farmID, saleID string) (*dtos.Sale, error)
    CreateSale(ctx context.Context, in dtos.CreateSaleInput) error
    UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) (int64, error)
    SoftDeleteSale(ctx context.Context, in dtos.DeleteSaleInput) (int64, error)
}
```

A reader can now trace `DELETE /api/v1/sales/{id}` → `SaleHandler.Delete` →
`SaleService.DeleteSale` → `SaleStore.SoftDeleteSale` → one SQL statement.

---

## Quick Reference

| Operation | Handler | Service | Store |
|---|---|---|---|
| List (farm-wide) | `List` | `ListSales` | `ListSales` |
| List (scoped) | `List` | `ListLinesBySale` | `ListLinesBySale` |
| Detail | `Get` | `GetSale` | `GetSaleByID` |
| Create | `Create` | `CreateSale` | `CreateSale` |
| Update | `Update` | `UpdateSale` | `UpdateSale` |
| Soft delete | `Delete` | `DeleteSale` | `SoftDeleteSale` |
| Business action | `Void` | `VoidSale` | `SetSaleVoided` |

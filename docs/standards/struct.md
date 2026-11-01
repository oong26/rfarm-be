# Struct Naming

**English** · [Bahasa Indonesia](struct.id.md)

[← Back to Standards](standard.md)

---

## Core Rule

Create a new struct only when there is a change in **responsibility** or **data
shape**. Do not create a new struct just because the layer is different — share
when the contract is identical, separate when it diverges.

All structs that cross layers live in `internal/dtos`, one file per entity:
`internal/dtos/sale_dto.go`.

---

## Postfix Convention

| Postfix | Layer | Purpose |
|---|---|---|
| `Entity` (no postfix) | All | The domain record as the API sees it: `Sale`, `DailyRecord` |
| `Request` | HTTP handler | Parses and validates incoming JSON/query from the client |
| `Input` | Service | Clean, validated data passed from handler to business logic |
| `Params` | Store | Adds fields the query needs: `farm_id`, paging, sorting |
| `Result` | Service → handler | A response that is more than the entity (list + meta) |

The name always starts with the operation and the entity: `CreateSaleRequest`,
`UpdateSaleInput`, `ListSalesParams`. Never `SaleCreateRequest`.

---

## Decision Flow

```
Is the data shape different between layers?
│
├── No  → Share the struct.
│
└── Yes → Does the store need extra fields (farm scope, paging, sorting)?
          │
          ├── Yes → Add a Params struct.
          │
          └── No  → Request → Input is enough. Stop there.
```

---

## Sharing a Struct Across Layers

When the contract is identical, **do not copy the struct into every layer**. Reuse
the upper layer's struct, following the call order **handler → service → store**,
and keep that struct's name.

| Situation | What to do | Name |
|---|---|---|
| Handler, service and store need exactly the same fields | One struct, passed down as-is | `*Request` |
| Service needs different data than the handler | Add a struct for the service | `*Input` |
| Store needs different data than the service | Add a struct for the store | `*Params` |

A new postfix appears only at the layer where the shape actually changes, and
layers below keep reusing that one until it changes again.

```
Handler          Service          Store
CreateXRequest → CreateXRequest → CreateXRequest        ← identical: 1 struct

CreateXRequest → CreateXInput   → CreateXInput          ← service differs

CreateXRequest → CreateXInput   → CreateXParams         ← store differs too
```

```go
// Identical in every layer — service and store accept the handler's struct.
func (s *saleService) CreateSale(ctx context.Context, in dtos.CreateSaleRequest) error
func (r *postgresSaleStore) CreateSale(ctx context.Context, in dtos.CreateSaleRequest) error
```

> Avoid field-by-field mapping between two structs that carry the same data.
> Mapping boilerplate is the cost this rule exists to remove — introduce a new
> struct only when the shape differs.

---

## Tag Ownership

| Struct | Allowed tags | Never |
|---|---|---|
| `Request` | `json`, `query`, `validate` | `db` |
| `Input` | none | `json`, `validate`, `db` |
| `Params` | none | `json`, `validate` |
| `Entity` | `json` | `validate` (validation is a transport concern) |

Database column tags belong to the sqlc-generated models in
`internal/stores/generated`, not to the DTOs. The store maps a generated row to a
DTO; that mapping is allowed, it is the one place where the DB shape meets the API
shape.

---

## Money, IDs and Timestamps

| Kind | Go type | JSON | Note |
|---|---|---|---|
| Money / quantity | `decimal.Decimal` | number, ≤ 2 decimals | never `float64` |
| ID | `string` | UUIDv7 string | generated in the service via `uuid.NewV7()` |
| Business date | `string` (`YYYY-MM-DD`) or `civil.Date` | `"2026-10-31"` | no timezone |
| Audit timestamp | `time.Time` | `"2026-10-31T04:12:00Z"` | UTC |
| Version | `int64` | number | optimistic lock, sent back as `ETag` |

`decimal.Decimal` marshals as a **string** by default. Set
`decimal.MarshalJSONWithoutQuotes = true` once in `main` so the API emits JSON
numbers as the contract requires.

Validation tags cannot compare decimals; check them in the service/domain:

```go
if in.Price.IsNegative() {
    return response.Unprocessable("price must be greater than or equal to 0", nil)
}
```

---

## Examples

### 1. Typical — handler validates, service consumes clean data

```go
// Transport contract: validation and JSON names live here.
type CreateSaleRequest struct {
    Date    string          `json:"date"    validate:"required,datetime=2006-01-02"`
    Qty     decimal.Decimal `json:"qty"`     // validated in the service
    Price   decimal.Decimal `json:"price"`
    Buyer   string          `json:"buyer"   validate:"omitempty,max=120"`
    Payment string          `json:"payment" validate:"omitempty,oneof=cash transfer qris"`
}

// Service contract: plain data. ID, farm scope, amount and audit stamps are
// created by the service, so they are not part of the request.
type CreateSaleInput struct {
    ID        string
    FarmID    string
    Date      string
    Qty       decimal.Decimal
    Price     decimal.Decimal
    Amount    decimal.Decimal
    Buyer     string
    Payment   string
    CreatedBy string
}
```

### 2. IDOR protection — handler sees part, service adds farm scope

`farm_id` comes from the authenticated principal, never from the body.

```go
type UpdateSaleRequest struct {
    Date    string          `json:"date"    validate:"required,datetime=2006-01-02"`
    Qty     decimal.Decimal `json:"qty"`
    Price   decimal.Decimal `json:"price"`
}

type UpdateSaleInput struct {
    ID      string
    FarmID  string // from the token — never from the client
    Version int64  // from If-Match — never from the body
    Date    string
    Qty     decimal.Decimal
    Price   decimal.Decimal
}
```

```go
func (h *SaleHandler) Update(w http.ResponseWriter, r *http.Request) {
    var req dtos.UpdateSaleRequest
    if err := utils.ParseAndValidate(r, &req); err != nil {
        response.Fail(w, r, err)
        return
    }

    version, err := utils.IfMatchVersion(r)
    if err != nil {
        response.Fail(w, r, err)
        return
    }

    err = h.svc.UpdateSale(r.Context(), dtos.UpdateSaleInput{
        ID:      chi.URLParam(r, "sale_id"),
        FarmID:  middleware.FarmIDFrom(r.Context()),
        Version: version,
        Date:    req.Date,
        Qty:     req.Qty,
        Price:   req.Price,
    })
    if err != nil {
        response.Fail(w, r, err)
        return
    }

    response.NoContent(w)
}
```

This difference in **shape** — not only in layer — is why `Request` and `Input` are
separate here.

### 3. List — one request, three shapes

```go
// Handler: query tags.
type ListSalesRequest struct {
    Month    string `query:"month"    validate:"omitempty,datetime=2006-01"`
    Q        string `query:"q"        validate:"omitempty,max=120"`
    Page     int    `query:"page"     validate:"omitempty,min=1"`
    PageSize int    `query:"pageSize" validate:"omitempty,min=1,max=200"`
}

// Service: parsed values plus the farm scope.
type ListSalesInput struct {
    FarmID   string
    Month    string
    Q        string
    Page     int
    PageSize int
}

// Store: ready for SQL.
type ListSalesParams struct {
    FarmID   string
    Month    string
    Q        string
    Sort     string
    Offset   int
    Limit    int
}
```

### 4. Nested entity — the child has its own identity

Name a struct after its **domain entity**, not its position inside a response.

```go
type JournalEntry struct {
    ID     string       `json:"id"`
    Date   string       `json:"date"`
    Lines  []JournalLine `json:"lines"` // ✅ uses JournalLine's own name
}

type JournalLine struct {
    ID        string          `json:"id"`
    AccountID string          `json:"accountId"`
    Debit     decimal.Decimal `json:"debit"`
    Credit    decimal.Decimal `json:"credit"`
}
```

Use `ParentChild` **only** when the struct represents a junction/relationship
record — its own table, with attributes that belong to the combination rather than
to either entity:

```go
// farm_members: the role belongs to the user-farm pair.
type FarmMember struct {
    FarmID string `json:"farmId"`
    UserID string `json:"userId"`
    Role   string `json:"role"`
}
```

| Entity has its own CRUD / lifecycle? | Convention | Example |
|---|---|---|
| Yes | Entity's own name | `[]JournalLine` ✅ |
| No — junction with relational attributes | `ParentChild` | `FarmMember` ✅ |

### 5. Optional and nullable fields

- Optional request field → pointer with `omitempty` (`*string`, `*decimal.Decimal`).
- A nullable column → pointer in the DTO, `nil` in JSON.
- Never use `0` or `""` to mean "not set" when the value is legitimately zero.

```go
type DailyRecordRequest struct {
    EggWeight *decimal.Decimal `json:"eggWeight"` // nil ⇒ use the farm default
    Notes     string           `json:"notes"      validate:"omitempty,max=2000"`
}
```

---

## Quick Reference

| Rule | Correct | Avoid |
|---|---|---|
| Naming order | `CreateSaleRequest` | `SaleCreateRequest` |
| Share identical shapes | one struct through all layers | three copies with mapping |
| Add `Input` only on change | service shape differs | copy because "layers" |
| Add `Params` only on change | store needs scope/paging | params on every method |
| Tags per layer | `validate` on `Request` | `validate` on `Input` |
| Money | `decimal.Decimal` | `float64` |
| IDs | `uuid.NewV7()` string | auto-increment int |
| Dates | `YYYY-MM-DD` string | `time.Time` for a business date |
| Optional value | `*T` + `omitempty` | `0` as "unset" |
| File location | `internal/dtos/sale_dto.go` | a struct per layer folder |

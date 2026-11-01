# List API — Pagination, Filters & Sorting

**English** · [Bahasa Indonesia](list_api.id.md)

[← Back to Standards](standard.md)

---

## Core Rules

1. **One resource = one list endpoint.** Never add `/all`, `/paginated` or `/list`.
2. **Every list is paginated.** `page` + `pageSize`, always. There is no
   `all=true` — no endpoint ever returns an unbounded set.
3. **The response always carries `meta`.** `{ "data": [...], "meta": { "page",
   "pageSize", "total" } }`.
4. **Filters are declared per resource**, validated, and farm-scoped.
5. **The service owns parsing defaults; the store owns LIMIT/OFFSET and the count.**

Large datasets are read page by page. When someone needs *everything*, it is an
**export job**, not a list parameter.

---

## Endpoint Shape

```
GET /api/v1/sales?page=1&pageSize=25            ← default page
GET /api/v1/sales?month=2026-10                 ← one month
GET /api/v1/sales?from=2026-10-01&to=2026-10-31 ← explicit range
GET /api/v1/sales?q=bondowoso&sort=-date        ← search + sort
```

Never:

```
GET /api/v1/sales/all            ✗
GET /api/v1/sales/paginated      ✗
GET /api/v1/sales/list           ✗
GET /api/v1/sales?all=true       ✗
```

---

## Query Parameters

| Param | Type | Default | Rules |
|---|---|---|---|
| `page` | int | `1` | min 1 |
| `pageSize` | int | `25` | min 1, **max 200**; above 200 → `422` (`max`) |
| `month` | string | — | `YYYY-MM`; expands to a date range in the service |
| `from` / `to` | date | — | `YYYY-MM-DD`, inclusive; `from > to` → `422` |
| `q` | string | — | free-text search; the store decides which columns it covers |
| `sort` | string | `-date` | field name, optional leading `-` for descending; allowlisted |

Rules:

- `month` and `from`/`to` are alternatives. Sending both → `422` (`invalid_query`).
- Every filter is optional; no filter means "all rows of this farm".
- Unknown `sort` values → `422` (`invalid_enum`).
- Unknown query parameters are ignored, not rejected — but never used.

---

## The Page Helper

Defaults, clamping and offset live in **one** place. Handlers never compute them.

```go
// internal/dtos/page.go
const (
    DefaultPageSize = 25
    MaxPageSize     = 200
)

type Page struct {
    Number int
    Size   int
}

func NewPage(number, size int) Page {
    if number < 1 {
        number = 1
    }
    if size < 1 {
        size = DefaultPageSize
    }
    if size > MaxPageSize {
        size = MaxPageSize
    }
    return Page{Number: number, Size: size}
}

func (p Page) Offset() int { return (p.Number - 1) * p.Size }
```

`NewPage` clamps as a safety net. The request validator is what rejects `pageSize > 200`
with `422` — a clamped value never reaches the store from a client request.

---

## Struct Chain

### Handler — `Request` (query tags + validation)

```go
type ListSalesRequest struct {
    Month    string `query:"month"    validate:"omitempty,datetime=2006-01"`
    From     string `query:"from"     validate:"omitempty,datetime=2006-01-02"`
    To       string `query:"to"       validate:"omitempty,datetime=2006-01-02"`
    Q        string `query:"q"        validate:"omitempty,max=120"`
    Sort     string `query:"sort"     validate:"omitempty,oneof=date buyer amount createdAt -date -buyer -amount -createdAt"`
    Page     int    `query:"page"     validate:"omitempty,min=1"`
    PageSize int    `query:"pageSize" validate:"omitempty,min=1,max=200"`
}
```

### Service — `Input` (farm scope added, no tags)

```go
type ListSalesInput struct {
    FarmID   string // from the token — never from the query
    Month    string
    From     string
    To       string
    Q        string
    Sort     string
    Page     Page
}
```

### Store — `Params` (ready for SQL)

```go
type ListSalesParams struct {
    FarmID    string
    From      string // resolved range, month already expanded
    To        string
    Q         string
    SortField string // allowlisted column, never raw input
    SortDesc  bool
    Offset    int
    Limit     int
}
```

### Result

```go
type ListSalesResult struct {
    Data []Sale
    Meta response.Meta // { page, pageSize, total }
}
```

---

## Service

```go
func (s *saleService) ListSales(ctx context.Context, in dtos.ListSalesInput) (dtos.ListSalesResult, error) {
    from, to, err := resolveRange(in.Month, in.From, in.To)
    if err != nil {
        return dtos.ListSalesResult{}, err // 422
    }

    field, desc, err := parseSort(in.Sort) // allowlist lookup
    if err != nil {
        return dtos.ListSalesResult{}, err // 422
    }

    page := dtos.NewPage(in.Page.Number, in.Page.Size)

    items, total, err := s.store.ListSales(ctx, dtos.ListSalesParams{
        FarmID:    in.FarmID,
        From:      from,
        To:        to,
        Q:         in.Q,
        SortField: field,
        SortDesc:  desc,
        Offset:    page.Offset(),
        Limit:     page.Size,
    })
    if err != nil {
        return dtos.ListSalesResult{}, err
    }

    return dtos.ListSalesResult{
        Data: items,
        Meta: response.NewMeta(page, total),
    }, nil
}
```

`resolveRange` and `parseSort` are unexported helpers with their own tests — they
are the only place where a query string becomes SQL input.

---

## Store

Rows and the total must come from **one consistent read**: either a single query
with a window count, or one transaction.

```sql
-- name: ListSales :many
SELECT id, date, buyer, qty, price, amount, version, created_at,
       COUNT(*) OVER() AS total
  FROM sales
 WHERE farm_id = $1
   AND deleted_at IS NULL
   AND ($2::date IS NULL OR date >= $2::date)
   AND ($3::date IS NULL OR date <= $3::date)
   AND ($4 = '' OR buyer ILIKE '%' || $4 || '%')
 ORDER BY
   CASE WHEN @sort_field = 'date'   AND NOT @sort_desc THEN date END ASC,
   CASE WHEN @sort_field = 'date'   AND     @sort_desc THEN date END DESC,
   CASE WHEN @sort_field = 'buyer'  AND NOT @sort_desc THEN buyer END ASC,
   CASE WHEN @sort_field = 'buyer'  AND     @sort_desc THEN buyer END DESC,
   id
 LIMIT $5 OFFSET $6;
```

- The sort is expressed with `sqlc.arg` fields and explicit `CASE` branches — never
  by concatenating `ORDER BY` text.
- `total` comes from `COUNT(*) OVER()`, so page 2 and the count cannot disagree.
- `LIMIT 0` is treated as "no limit" by PostgreSQL — the service never sends 0,
  `NewPage` guarantees a positive size.
- Because a `page` beyond the last one returns zero rows, the total would be lost.
  In that case run the count query for the same filters; the summary below shows
  the shape.

```go
func (r *postgresSaleStore) ListSales(ctx context.Context, p dtos.ListSalesParams) ([]dtos.Sale, int64, error) {
    rows, err := r.q.ListSales(ctx, generated.ListSalesParams{ /* … */ })
    if err != nil {
        return nil, 0, fmt.Errorf("stores.ListSales: %w", err)
    }

    var total int64
    if len(rows) > 0 {
        total = rows[0].Total
    } else {
        total, err = r.q.CountSales(ctx, generated.CountSalesParams{ /* same filters */ })
        if err != nil {
            return nil, 0, fmt.Errorf("stores.CountSales: %w", err)
        }
    }

    return mapSales(rows), total, nil
}
```

---

## Handler

```go
func (h *SaleHandler) List(w http.ResponseWriter, r *http.Request) {
    var req dtos.ListSalesRequest
    if err := utils.ParseAndValidate(r, &req); err != nil {
        response.Fail(w, r, err)
        return
    }

    result, err := h.svc.ListSales(r.Context(), dtos.ListSalesInput{
        FarmID: middleware.FarmIDFrom(r.Context()),
        Month:  req.Month,
        From:   req.From,
        To:     req.To,
        Q:      req.Q,
        Sort:   req.Sort,
        Page:   dtos.Page{Number: req.Page, Size: req.PageSize},
    })
    if err != nil {
        response.Fail(w, r, err)
        return
    }

    response.List(w, result.Data, result.Meta)
}
```

Always `response.List` for list endpoints — never `response.OK` with a hand-built
envelope.

---

## Response Contract

```json
{
  "data": [
    {
      "id": "0192f3c4-…",
      "date": "2026-10-31",
      "buyer": "Pak Slamet",
      "qty": 120,
      "price": 2200,
      "amount": 264000,
      "version": 4
    }
  ],
  "meta": { "page": 1, "pageSize": 25, "total": 137 }
}
```

| Rule | Detail |
|---|---|
| `data` | always an array, never `null` |
| `meta.page` | echoed, already defaulted |
| `meta.pageSize` | echoed, already clamped |
| `meta.total` | rows matching the filters, not the page |
| `totalPages` | not returned — the client computes `ceil(total / pageSize)` |

---

## Paging Past the End

`?page=999` on a 137-row result returns:

```json
{ "data": [], "meta": { "page": 999, "pageSize": 25, "total": 137 } }
```

`200`, never `404`. The client uses `total` to render "no more data".

---

## Sorting

- The wire format is one parameter: `sort=-date`.
- `-` prefix means descending; no prefix means ascending.
- Only fields in the allowlist may be sorted; each map entry has a fixed direction
  set (`sortable: ["date", "buyer", "amount"]`).
- The default sort for record lists is `-date` (newest first), with `id` as a
  tiebreaker so pages never overlap.

| Good | Bad |
|---|---|
| `?sort=-date` | `?sort=date desc` |
| `?sort=buyer` | `?sort=unknown_column` |

---

## Large Datasets — Exports

Do not raise `pageSize` and do not add `all=true`. Use an async job:

```
POST /api/v1/exports                 ← { resource: "sales", filters: {…} } → 202 + job
GET  /api/v1/exports/{exportId}     ← poll status, then download
```

The job runs in the background, writes a file, and records its status so the
operator can see a failure. A list endpoint never blocks on a large scan.

---

## Empty Results and Errors

| Case | Response |
|---|---|
| No rows match | `200`, `data: []`, `meta.total: 0` |
| `pageSize` > 200 | `422`, `errors[].code: "max"` |
| `from` after `to` | `422`, `errors[].code: "invalid_query"` |
| `month` together with `from`/`to` | `422`, `errors[].code: "invalid_query"` |
| Unknown `sort` field | `422`, `errors[].code: "invalid_enum"` |
| `page` past the end | `200`, `data: []`, real `total` |

---

## Quick Reference

| Rule | Correct | Avoid |
|---|---|---|
| One list endpoint | `GET /sales` | `/sales/all`, `/sales/list` |
| Pagination | `?page=1&pageSize=25` | unpaginated default |
| Page size ceiling | `max=200` → `422` | silently returning everything |
| Whole dataset | export job | `all=true` |
| Meta on every list | `response.List(w, data, meta)` | bare array |
| Count + rows | `COUNT(*) OVER()` / count query | two unrelated queries |
| Sorting | allowlist in the query | raw `ORDER BY` from input |
| Range filters | `month` or `from`/`to`, not both | both at once |
| Farm scope | token | query parameter |

# List API — Paginasi, Filter & Sorting

[English](list_api.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Aturan Inti

1. **Satu resource = satu endpoint list.** Jangan menambahkan `/all`, `/paginated`,
   atau `/list`.
2. **Setiap list berpaginasi.** `page` + `pageSize`, selalu. Tidak ada `all=true` —
   tidak ada endpoint yang mengembalikan himpunan tanpa batas.
3. **Respons selalu membawa `meta`.** `{ "data": [...], "meta": { "page",
   "pageSize", "total" } }`.
4. **Filter dideklarasikan per resource**, divalidasi, dan ter-scope farm.
5. **Service memiliki default parsing; store memiliki LIMIT/OFFSET dan count.**

Dataset besar dibaca halaman per halaman. Bila seseorang membutuhkan *semuanya*,
itu **job export**, bukan parameter list.

---

## Bentuk Endpoint

```
GET /api/v1/sales?page=1&pageSize=25            ← halaman default
GET /api/v1/sales?month=2026-10                 ← satu bulan
GET /api/v1/sales?from=2026-10-01&to=2026-10-31 ← rentang eksplisit
GET /api/v1/sales?q=bondowoso&sort=-date        ← pencarian + sort
```

Jangan pernah:

```
GET /api/v1/sales/all            ✗
GET /api/v1/sales/paginated      ✗
GET /api/v1/sales/list           ✗
GET /api/v1/sales?all=true       ✗
```

---

## Query Parameter

| Param | Tipe | Default | Aturan |
|---|---|---|---|
| `page` | int | `1` | minimal 1 |
| `pageSize` | int | `25` | minimal 1, **maksimal 200**; di atas 200 → `422` (`max`) |
| `month` | string | — | `YYYY-MM`; diperluas menjadi rentang tanggal di service |
| `from` / `to` | date | — | `YYYY-MM-DD`, inklusif; `from > to` → `422` |
| `q` | string | — | pencarian teks bebas; store menentukan kolom yang dicakup |
| `sort` | string | `-date` | nama field, prefix `-` opsional untuk menurun; allowlist |

Aturan:

- `month` dan `from`/`to` adalah alternatif. Mengirim keduanya → `422`
  (`invalid_query`).
- Setiap filter opsional; tanpa filter berarti "semua baris farm ini".
- Nilai `sort` yang tidak dikenal → `422` (`invalid_enum`).
- Query parameter yang tidak dikenal diabaikan, bukan ditolak — tetapi tidak pernah
  dipakai.

---

## Helper Page

Default, pembatasan, dan offset berada di **satu** tempat. Handler tidak pernah
menghitungnya.

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

`NewPage` membatasi sebagai jaring pengaman. Validator request yang menolak
`pageSize > 200` dengan `422` — nilai yang sudah dibatasi tidak pernah sampai ke store
dari request client.

---

## Rantai Struct

### Handler — `Request` (tag query + validasi)

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

### Service — `Input` (scope farm ditambahkan, tanpa tag)

```go
type ListSalesInput struct {
    FarmID   string // dari token — tidak pernah dari query
    Month    string
    From     string
    To       string
    Q        string
    Sort     string
    Page     Page
}
```

### Store — `Params` (siap untuk SQL)

```go
type ListSalesParams struct {
    FarmID    string
    From      string // rentang hasil resolve, month sudah diperluas
    To        string
    Q         string
    SortField string // kolom allowlist, tidak pernah input mentah
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

    field, desc, err := parseSort(in.Sort) // pencarian allowlist
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
        Meta: response.NewMeta(page.Number, page.Size, total),
    }, nil
}
```

`resolveRange` dan `parseSort` adalah helper yang tidak diekspor dan punya test
sendiri — hanya di sanalah query string berubah menjadi input SQL.

---

## Store

Baris dan total harus berasal dari **satu pembacaan yang konsisten**: baik satu query
dengan window count, atau satu transaksi.

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

- Sort diungkapkan dengan field `sqlc.arg` dan cabang `CASE` eksplisit — tidak
  pernah dengan menyambung teks `ORDER BY`.
- `total` berasal dari `COUNT(*) OVER()`, sehingga halaman 2 dan count tidak dapat
  berbeda.
- `LIMIT 0` diperlakukan PostgreSQL sebagai "tanpa limit" — service tidak pernah
  mengirim 0, `NewPage` menjamin ukuran positif.
- Karena `page` di luar halaman terakhir mengembalikan nol baris, total akan hilang.
  Dalam kasus itu jalankan query count untuk filter yang sama; ringkasannya
  menunjukkan bentuknya.

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
        total, err = r.q.CountSales(ctx, generated.CountSalesParams{ /* filter sama */ })
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

Selalu `response.List` untuk endpoint list — jangan `response.OK` dengan envelope
yang dirakit manual.

---

## Kontrak Respons

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

| Aturan | Detail |
|---|---|
| `data` | selalu array, tidak pernah `null` |
| `meta.page` | dikembalikan apa adanya, sudah diberi default |
| `meta.pageSize` | dikembalikan apa adanya, sudah dibatasi |
| `meta.total` | baris yang cocok dengan filter, bukan isi halaman |
| `totalPages` | tidak dikembalikan — client menghitung `ceil(total / pageSize)` |

---

## Paging Melewati Akhir

`?page=999` pada hasil 137 baris mengembalikan:

```json
{ "data": [], "meta": { "page": 999, "pageSize": 25, "total": 137 } }
```

`200`, bukan `404`. Client memakai `total` untuk menampilkan "tidak ada data lagi".

---

## Sorting

- Format di wire adalah satu parameter: `sort=-date`.
- Prefix `-` berarti menurun; tanpa prefix berarti menaik.
- Hanya field di allowlist yang boleh di-sort; setiap entri map punya set arah tetap
  (`sortable: ["date", "buyer", "amount"]`).
- Sort default untuk daftar record adalah `-date` (terbaru dulu), dengan `id`
  sebagai penentu akhir agar halaman tidak pernah tumpang tindih.

| Benar | Salah |
|---|---|
| `?sort=-date` | `?sort=date desc` |
| `?sort=buyer` | `?sort=unknown_column` |

---

## Dataset Besar — Export

Jangan menaikkan `pageSize` dan jangan menambahkan `all=true`. Gunakan job asinkron:

```
POST /api/v1/exports                 ← { resource: "sales", filters: {…} } → 202 + job
GET  /api/v1/exports/{exportId}     ← polling status, lalu unduh
```

Job berjalan di background, menulis file, dan mencatat statusnya sehingga operator
dapat melihat kegagalan. Endpoint list tidak pernah memblokir pada scan besar.

---

## Hasil Kosong dan Error

| Kasus | Respons |
|---|---|
| Tidak ada baris yang cocok | `200`, `data: []`, `meta.total: 0` |
| `pageSize` > 200 | `422`, `errors[].code: "max"` |
| `from` setelah `to` | `422`, `errors[].code: "invalid_query"` |
| `month` bersama `from`/`to` | `422`, `errors[].code: "invalid_query"` |
| Field `sort` tidak dikenal | `422`, `errors[].code: "invalid_enum"` |
| `page` melewati akhir | `200`, `data: []`, `total` sebenarnya |

---

## Referensi Cepat

| Aturan | Benar | Hindari |
|---|---|---|
| Satu endpoint list | `GET /sales` | `/sales/all`, `/sales/list` |
| Paginasi | `?page=1&pageSize=25` | default tanpa paginasi |
| Batas page size | `max=200` → `422` | diam-diam mengembalikan semua |
| Seluruh dataset | job export | `all=true` |
| Meta di setiap list | `response.List(w, data, meta)` | array telanjang |
| Count + baris | `COUNT(*) OVER()` / query count | dua query tak berhubungan |
| Sorting | allowlist di query | `ORDER BY` mentah dari input |
| Filter rentang | `month` atau `from`/`to`, bukan keduanya | keduanya sekaligus |
| Scope farm | token | query parameter |

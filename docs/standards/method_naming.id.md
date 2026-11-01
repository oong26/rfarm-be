# Penamaan Method

[English](method_naming.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Aturan Inti

Nama method terdiri dari **operasi**, **entity**, dan — hanya bila wajib — **scope**
tempat method itu berada.

```
Verb + Entity (+ By{Parent})
```

Bentuk yang sama dipakai di setiap lapisan, sehingga pembaca dapat mengikuti satu
operasi dari handler sampai ke SQL tanpa menebak nama.

---

## Method List

| Kondisi | Konvensi | Contoh |
|---|---|---|
| Tidak ada scope wajib | `List{Entity}` | `ListSales(ctx, params)` |
| Wajib ter-scope ke parent | `List{Entity}By{Parent}` | `ListLinesBySale(ctx, saleID)` |

`By{Parent}` berarti pemanggil **wajib** menyertakan ID parent — entity tidak dapat
didaftar tanpa itu. Jangan menambahkan `By{Parent}` bila parent hanya filter
opsional; gunakan field filter.

```go
ListSales(ctx, params)                 // semua penjualan farm, filter opsional
ListLinesBySale(ctx, saleID)           // line tidak ada tanpa sale
```

`farm_id` bukan akhiran `By{Farm}`: ia hadir di setiap method sebagai parameter
scope, karena tidak ada query yang boleh berjalan tanpanya.

---

## CRUD Satu Entity

Operasi satu entity tidak pernah memakai akhiran `By{Parent}` — ID entity sudah
mengidentifikasi record.

```go
GetSale(ctx, farmID, saleID)
CreateSale(ctx, input)
UpdateSale(ctx, input)
DeleteSale(ctx, farmID, saleID)
```

Bukan `GetSaleById`, bukan `GetSaleByFarm`. Identifier adalah bagian dari signature,
bukan bagian dari nama.

---

## Method Handler

Handler dinamai sesuai use case saja; lapisannya sudah terlihat dari `Handler`.

```go
func (h *SaleHandler) List(w http.ResponseWriter, r *http.Request)
func (h *SaleHandler) Get(w http.ResponseWriter, r *http.Request)
func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request)
func (h *SaleHandler) Update(w http.ResponseWriter, r *http.Request)
func (h *SaleHandler) Delete(w http.ResponseWriter, r *http.Request)
```

- Tanpa prefix `Handle`, `Do`, `Process`.
- Satu method per route; router adalah pemetaannya (`GET /sales → h.List`).
- Signature selalu `(http.ResponseWriter, *http.Request)`, sehingga bisa diteruskan
  ke chi langsung.
- Path parameter dibaca dengan `chi.URLParam(r, "sale_id")` di dalam method.

---

## Method Service

Method service menggambarkan operasi bisnis. Namanya mengikuti handler dan
menambahkan scope hanya bila aturan memang membutuhkannya.

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

- `ctx context.Context` selalu parameter pertama.
- `error` selalu nilai kembali terakhir.
- Verb yang merupakan aksi bisnis mempertahankan namanya (`VoidSale`,
  `PostJournal`), bukan `ChangeSaleStatus`.
- Create mengembalikan entity yang dibuat agar handler dapat menjawab `201` dengan
  body.

---

## Method Store

Method store menjawab satu pertanyaan data masing-masing, dengan bentuk operasi +
entity yang sama. Implementasi PostgreSQL berada di atas query hasil generate sqlc.

```go
type SaleStore interface {
    ListSales(ctx context.Context, p dtos.ListSalesParams) ([]dtos.Sale, int64, error)
    GetSaleByID(ctx context.Context, farmID, saleID string) (*dtos.Sale, error)
    CreateSale(ctx context.Context, in dtos.CreateSaleInput) error
    UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) (int64, error)
    SoftDeleteSale(ctx context.Context, in dtos.DeleteSaleInput) (int64, error)
}
```

- `Get{Entity}ByID` untuk satu baris — akhiran `ByID` membedakannya dari
  `Get{Entity}` tingkat bisnis yang mungkin merangkai beberapa query.
- `List{Entity}` mengembalikan baris **dan** total count sekaligus; keduanya berasal
  dari satu pembacaan yang konsisten.
- `SoftDelete{Entity}` alih-alih `Delete{Entity}` ketika baris disimpan
  (`deleted_at`), yang merupakan default di sini.
- Write biasa mengembalikan `(rowsAffected int64, error)` agar service dapat
  memutuskan apakah nol baris berarti "tidak ditemukan" atau "version conflict".

---

## Konstruktor dan Interface

| Jenis | Konvensi | Contoh |
|---|---|---|
| Konstruktor | `New{Type}` | `NewSaleHandler`, `NewSaleService`, `NewSaleStore` |
| Interface | `{Entity}{Layer}` | `SaleService`, `SaleStore` |
| Implementasi | `{layer}{Entity}` (tidak diekspor) | `saleService`, `postgresSaleStore` |
| Fake (test) | `fake{Type}` | `fakeSaleStore` |
| File | `{entity}_{layer}.go` | `sale_service.go`, `sale_handler.go`, `sale_store.go` |

Konstruktor mengembalikan interface, struct tetap tidak diekspor. Itulah yang
membuat lapisan dapat diganti saat test tanpa menyentuh pemanggilnya.

```go
func NewSaleService(st stores.SaleStore) SaleService {
    return &saleService{store: st}
}
```

---

## Boolean, Getter, dan Prefix

| Aturan | Benar | Hindari |
|---|---|---|
| Boolean dibaca sebagai pernyataan | `isPosted`, `hasStock` | `postedFlag` |
| Method boolean | `CanVoid(ctx, sale) (bool, error)` | `CheckVoid` |
| Tanpa `Get` untuk akses field | `sale.Qty` | `sale.GetQty()` |
| `Get` hanya untuk I/O | `GetSale(ctx, farmID, id)` | `FetchSaleData` |
| Tanpa nama lapisan di dalam nama | `VoidSale` | `ServiceVoidSale` |
| Tanpa singkatan yang tidak dipakai domain | `GetSale` | `GetSl` |

---

## Contoh Lengkap

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

Pembaca sekarang dapat melacak `DELETE /api/v1/sales/{id}` → `SaleHandler.Delete` →
`SaleService.DeleteSale` → `SaleStore.SoftDeleteSale` → satu statement SQL.

---

## Referensi Cepat

| Operasi | Handler | Service | Store |
|---|---|---|---|
| List (seluruh farm) | `List` | `ListSales` | `ListSales` |
| List (ter-scope) | `List` | `ListLinesBySale` | `ListLinesBySale` |
| Detail | `Get` | `GetSale` | `GetSaleByID` |
| Create | `Create` | `CreateSale` | `CreateSale` |
| Update | `Update` | `UpdateSale` | `UpdateSale` |
| Soft delete | `Delete` | `DeleteSale` | `SoftDeleteSale` |
| Aksi bisnis | `Void` | `VoidSale` | `SetSaleVoided` |

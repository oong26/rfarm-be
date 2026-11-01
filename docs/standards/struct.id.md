# Penamaan Struct

[English](struct.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Aturan Inti

Buat struct baru hanya ketika ada perubahan **tanggung jawab** atau **bentuk data**.
Jangan membuat struct baru hanya karena lapisannya berbeda — pakai bersama bila
kontraknya identik, pisahkan bila berbeda.

Semua struct yang melintasi lapisan disimpan di `internal/dtos`, satu file per
entity: `internal/dtos/sale_dto.go`.

---

## Konvensi Akhiran

| Akhiran | Lapisan | Tujuan |
|---|---|---|
| `Entity` (tanpa akhiran) | Semua | Record domain sebagaimana dilihat API: `Sale`, `DailyRecord` |
| `Request` | Handler HTTP | Mem-parsing dan memvalidasi JSON/query dari client |
| `Input` | Service | Data bersih dan tervalidasi dari handler ke logika bisnis |
| `Params` | Store | Menambah field yang dibutuhkan query: `farm_id`, paging, sorting |
| `Result` | Service → handler | Respons yang lebih dari entity (list + meta) |

Nama selalu dimulai dengan operasi dan entity: `CreateSaleRequest`,
`UpdateSaleInput`, `ListSalesParams`. Bukan `SaleCreateRequest`.

---

## Alur Keputusan

```
Apakah bentuk data berbeda antar lapisan?
│
├── Tidak → Pakai struct yang sama.
│
└── Ya   → Apakah store butuh field tambahan (scope farm, paging, sorting)?
          │
          ├── Ya   → Tambahkan struct Params.
          │
          └── Tidak → Request → Input sudah cukup. Berhenti di sini.
```

---

## Berbagi Struct Antar Lapisan

Bila kontraknya identik, **jangan salin struct ke setiap lapisan**. Pakai ulang
struct dari lapisan atas, mengikuti urutan pemanggilan **handler → service →
store**, dan pertahankan namanya.

| Situasi | Yang dilakukan | Nama |
|---|---|---|
| Handler, service, dan store butuh field yang persis sama | Satu struct, diteruskan apa adanya | `*Request` |
| Service butuh data berbeda dari handler | Tambahkan struct untuk service | `*Input` |
| Store butuh data berbeda dari service | Tambahkan struct untuk store | `*Params` |

Akhiran baru muncul hanya di lapisan tempat bentuknya benar-benar berubah, dan
lapisan di bawahnya terus memakai struct itu sampai berubah lagi.

```
Handler          Service          Store
CreateXRequest → CreateXRequest → CreateXRequest        ← identik: 1 struct

CreateXRequest → CreateXInput   → CreateXInput          ← service berbeda

CreateXRequest → CreateXInput   → CreateXParams         ← store juga berbeda
```

```go
// Identik di setiap lapisan — service dan store menerima struct dari handler.
func (s *saleService) CreateSale(ctx context.Context, in dtos.CreateSaleRequest) error
func (r *postgresSaleStore) CreateSale(ctx context.Context, in dtos.CreateSaleRequest) error
```

> Hindari mapping field-per-field antara dua struct yang membawa data sama.
> Boilerplate mapping adalah biaya yang ingin dihapus oleh aturan ini — buat struct
> baru hanya ketika bentuknya berbeda.

---

## Kepemilikan Tag

| Struct | Tag yang boleh | Tidak boleh |
|---|---|---|
| `Request` | `json`, `query`, `validate` | `db` |
| `Input` | tidak ada | `json`, `validate`, `db` |
| `Params` | tidak ada | `json`, `validate` |
| `Entity` | `json` | `validate` (validasi adalah urusan transport) |

Tag kolom database milik model hasil generate sqlc di
`internal/stores/generated`, bukan milik DTO. Store memetakan baris hasil generate ke
DTO; mapping itu diperbolehkan, itulah satu-satunya tempat bentuk DB bertemu bentuk
API.

---

## Uang, ID, dan Timestamp

| Jenis | Tipe Go | JSON | Catatan |
|---|---|---|---|
| Uang / kuantitas | `decimal.Decimal` | number, ≤ 2 desimal | tidak pernah `float64` |
| ID | `string` | string UUIDv7 | dibuat service via `uuid.NewV7()` |
| Tanggal bisnis | `string` (`YYYY-MM-DD`) atau `civil.Date` | `"2026-10-31"` | tanpa timezone |
| Timestamp audit | `time.Time` | `"2026-10-31T04:12:00Z"` | UTC |
| Version | `int64` | number | optimistic lock, dikirim balik sebagai `ETag` |

`decimal.Decimal` secara default di-marshal sebagai **string**. Set
`decimal.MarshalJSONWithoutQuotes = true` sekali di `main` agar API mengeluarkan
number JSON sesuai kontrak.

Tag validasi tidak dapat membandingkan decimal; periksa di service/domain:

```go
if in.Price.IsNegative() {
    return response.Unprocessable("price must be greater than or equal to 0", nil)
}
```

---

## Contoh

### 1. Umum — handler memvalidasi, service menerima data bersih

```go
// Kontrak transport: validasi dan nama JSON ada di sini.
type CreateSaleRequest struct {
    Date    string          `json:"date"    validate:"required,datetime=2006-01-02"`
    Qty     decimal.Decimal `json:"qty"`     // divalidasi di service
    Price   decimal.Decimal `json:"price"`
    Buyer   string          `json:"buyer"   validate:"omitempty,max=120"`
    Payment string          `json:"payment" validate:"omitempty,oneof=cash transfer qris"`
}

// Kontrak service: data biasa. ID, scope farm, amount, dan stempel audit dibuat
// service, jadi bukan bagian dari request.
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

### 2. Proteksi IDOR — handler melihat sebagian, service menambah scope farm

`farm_id` berasal dari principal yang terautentikasi, tidak pernah dari body.

```go
type UpdateSaleRequest struct {
    Date    string          `json:"date"    validate:"required,datetime=2006-01-02"`
    Qty     decimal.Decimal `json:"qty"`
    Price   decimal.Decimal `json:"price"`
}

type UpdateSaleInput struct {
    ID      string
    FarmID  string // dari token — tidak pernah dari client
    Version int64  // dari If-Match — tidak pernah dari body
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

Perbedaan **bentuk** — bukan hanya perbedaan lapisan — inilah alasan `Request` dan
`Input` dipisahkan di sini.

### 3. List — satu request, tiga bentuk

```go
// Handler: tag query.
type ListSalesRequest struct {
    Month    string `query:"month"    validate:"omitempty,datetime=2006-01"`
    Q        string `query:"q"        validate:"omitempty,max=120"`
    Page     int    `query:"page"     validate:"omitempty,min=1"`
    PageSize int    `query:"pageSize" validate:"omitempty,min=1,max=200"`
}

// Service: nilai ter-parse plus scope farm.
type ListSalesInput struct {
    FarmID   string
    Month    string
    Q        string
    Page     int
    PageSize int
}

// Store: siap untuk SQL.
type ListSalesParams struct {
    FarmID   string
    Month    string
    Q        string
    Sort     string
    Offset   int
    Limit    int
}
```

### 4. Entity bersarang — child punya identitas sendiri

Namai struct sesuai **entity domain**, bukan posisinya di dalam respons.

```go
type JournalEntry struct {
    ID    string        `json:"id"`
    Date  string        `json:"date"`
    Lines []JournalLine `json:"lines"` // ✅ memakai nama JournalLine sendiri
}

type JournalLine struct {
    ID        string          `json:"id"`
    AccountID string          `json:"accountId"`
    Debit     decimal.Decimal `json:"debit"`
    Credit    decimal.Decimal `json:"credit"`
}
```

Gunakan `ParentChild` **hanya** ketika struct merepresentasikan record junction/
relasi — tabelnya sendiri, dengan atribut milik kombinasi, bukan milik salah satu
entity:

```go
// farm_members: role milik pasangan user-farm.
type FarmMember struct {
    FarmID string `json:"farmId"`
    UserID string `json:"userId"`
    Role   string `json:"role"`
}
```

| Entity punya CRUD / siklus hidup sendiri? | Konvensi | Contoh |
|---|---|---|
| Ya | Nama entity sendiri | `[]JournalLine` ✅ |
| Tidak — junction dengan atribut relasi | `ParentChild` | `FarmMember` ✅ |

### 5. Field opsional dan nullable

- Field request opsional → pointer dengan `omitempty` (`*string`,
  `*decimal.Decimal`).
- Kolom nullable → pointer di DTO, `nil` di JSON.
- Jangan memakai `0` atau `""` untuk berarti "belum diisi" bila nilainya memang
  bisa nol.

```go
type DailyRecordRequest struct {
    EggWeight *decimal.Decimal `json:"eggWeight"` // nil ⇒ pakai default farm
    Notes     string           `json:"notes"      validate:"omitempty,max=2000"`
}
```

---

## Referensi Cepat

| Aturan | Benar | Hindari |
|---|---|---|
| Urutan penamaan | `CreateSaleRequest` | `SaleCreateRequest` |
| Bentuk identik | satu struct di semua lapisan | tiga salinan dengan mapping |
| `Input` hanya saat berubah | bentuk service berbeda | salin karena "lapisan" |
| `Params` hanya saat berubah | store butuh scope/paging | params di setiap method |
| Tag per lapisan | `validate` di `Request` | `validate` di `Input` |
| Uang | `decimal.Decimal` | `float64` |
| ID | string `uuid.NewV7()` | int auto-increment |
| Tanggal | string `YYYY-MM-DD` | `time.Time` untuk tanggal bisnis |
| Nilai opsional | `*T` + `omitempty` | `0` sebagai "belum diisi" |
| Lokasi file | `internal/dtos/sale_dto.go` | satu struct per folder lapisan |

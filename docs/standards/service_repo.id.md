# Logika Service & Store

[English](service_repo.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Prinsip

**Service memiliki logika bisnis. Store memiliki akses data.**

| Lapisan | Bertanggung jawab atas | Tidak pernah |
|---|---|---|
| **Handler** (`internal/handlers`) | Parse, validasi, otorisasi, panggil satu method service, render respons | Aturan bisnis, SQL |
| **Service** (`internal/services`) | Aturan bisnis, orkestrasi, transaksi, transisi state, keputusan error | SQL, `pgxpool.Pool`, `http.ResponseWriter` |
| **Store** (`internal/stores`) | SQL di atas sqlc, mapping baris, transaksi yang dibukanya, data atau error DB mentah | Keputusan bisnis, status HTTP, pesan untuk client |

Store menjawab *"apakah query berhasil, dan apa hasilnya?"*. Service memutuskan
*"apa artinya itu bagi bisnis?"*.

> `stores/` adalah lapisan repository. `stores/generated/` adalah hasil sqlc —
> perlakukan sebagai read-only dan jangan pernah diedit manual. Satu-satunya store
> non-SQL adalah `SessionStore`, yang membaca dan menulis sesi refresh di Redis.

---

## Penelusuran Lapisan

```go
// internal/handlers/sale_handler.go — hanya HTTP
func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req dtos.CreateSaleRequest
    if err := utils.ParseAndValidate(r, &req); err != nil {
        response.Fail(w, r, err)              // 422 dengan detail field
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
// internal/services/sale_service.go — aturan bisnis
func (s *saleService) CreateSale(ctx context.Context, in dtos.CreateSaleInput) (*dtos.Sale, error) {
    if !in.Qty.IsPositive() {
        return nil, response.Unprocessable("qty must be greater than 0", nil)
    }
    if in.Price.IsNegative() {
        return nil, response.Unprocessable("price must be greater than or equal to 0", nil)
    }

    in.ID = uuid.Must(uuid.NewV7()).String()
    in.Amount = in.Qty.Mul(in.Price)

    sale := &dtos.Sale{ /* map field, Version: 1 */ }
    if err := s.store.CreateSale(ctx, in); err != nil {
        return nil, err
    }
    return sale, nil
}
```

```go
// internal/stores/sale_store.go — hanya SQL
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
        return mapDBError(err, "sale")        // hanya pelanggaran unique/FK
    }
    return nil
}
```

---

## Interface Store di Atas sqlc

SQL berada di `db/queries/*.sql`; `sqlc generate` menghasilkan
`internal/stores/generated`. File store tulisan tangan mendefinisikan interface yang
dipakai service dan mengimplementasikannya di atas query hasil generate.

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

Aturan:

- Satu interface per entity, dideklarasikan di `stores`, sehingga service hanya
  mengenal kontraknya.
- Service bergantung pada interface terkecil yang dibutuhkannya; fitur read-only
  mendapat interface sempit, bukan seluruh store.
- Konstruktor mengembalikan interface; struct tetap tidak diekspor.
- `farm_id` adalah parameter setiap method — bukan field yang bisa terlupa pemanggil.
- Tidak ada method bisnis seperti `CreateSaleAndPostJournal` kecuali seluruhnya satu
  transaksi; orkestrasi milik service.

---

## Transaksi

Transaksi adalah **urusan akses data**. Service memutuskan *bahwa* sebuah operasi
harus atomik; store mengimplementasikannya.

- Satu method store yang menyentuh beberapa tabel membuka transaksinya sendiri dan
  commit atau rollback di dalam method itu.
- Service tidak pernah menerima `pgx.Tx` dan tidak pernah commit apa pun.
- Bila alur bisnis boleh berjalan sebagian, panggil beberapa method store alih-alih
  satu transaksi besar.

```go
func (r *postgresSaleStore) CreateSaleWithJournal(ctx context.Context, in dtos.CreateSaleInput) error {
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx) //nolint:errcheck // rollback setelah commit tidak berefek

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

> Patokan: bila statement kedua tidak boleh bertahan tanpa statement pertama,
> keduanya berada di method store dan transaksi yang sama.

---

## Session Store (Redis)

`SessionStore` adalah satu-satunya store yang bukan SQL. Ia berada di
`internal/stores/session_store.go`, berbicara ke Redis lewat `pkg/cache`, dan tidak
punya query sqlc maupun transaksi Postgres.

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

Aturan:

- Token dibuat dan di-hash oleh auth service (`pkg/security`); store hanya pernah
  melihat hash SHA-256. Refresh token mentah tidak pernah sampai ke Redis atau log.
- `Rotate` bersifat **atomik** — satu skrip Lua atau roundtrip `WATCH`/`MULTI` yang
  menghapus hash lama, menulis hash baru, dan memperbarui indeks family. Dua refresh
  bersamaan tidak boleh sama-sama berhasil.
- Hash yang tidak hidup sementara family-nya masih ada → anggap reuse: cabut family
  dan kembalikan `response.Unauthorized("session expired")`.
- TTL berasal dari pemanggil (`cfg.Auth.RefreshTokenTTL`), diulang setiap rotasi.
  Store tidak pernah membaca konfigurasi.
- Redis tidak punya transaksi yang bisa diserahkan ke service: tidak ada padanan
  `pgx.Tx` di sini, dan service tidak pernah melihat `redis.Client`.
- Keberadaan sesi adalah fakta store (`redis.Nil` → tidak ditemukan). Apakah itu
  berarti `401`, "cabut family", atau bukan apa-apa adalah keputusan service — kecuali
  pengecekan reuse atomik di atas.
- Error transport dikembalikan apa adanya; pemetaan error mengubahnya menjadi `503`.
  `redis.Nil` tidak pernah menjadi `500`.

---

## Aturan Context

- `ctx context.Context` adalah parameter pertama setiap method service dan store.
- Jangan menyimpan context di struct; jangan memakai `context.Background()` di
  dalam service atau store — ambil context dari request.
- Lapisan HTTP menetapkan deadline; tambahkan timeout per panggilan hanya di sekitar
  I/O eksternal, di client.
- Selalu teruskan `ctx` ke pgx (`Query`, `QueryRow`, `Exec`) — tanpa pengecualian.
- Jangan biarkan goroutine hidup lebih lama dari request dengan context request.

---

## Optimistic Locking

### Mengapa

Setiap tabel record membawa `version int NOT NULL`. Client membacanya dari header
`ETag` dan mengirimkannya kembali di `If-Match` saat update dan delete. Bila baris
sudah berubah, request gagal dengan **`412 Precondition Failed`** — bukan `409`.

| Kasus | Diperbolehkan? |
|---|---|
| Service melakukan beberapa panggilan store untuk logika multi-langkah yang nyata | ✓ Ya |
| Store menggabungkan pengecekan versi dengan penulisan dalam satu statement/CTE | ✓ Ya, lebih disukai |
| Service memanggil `GetByID`, membandingkan versi, lalu memanggil update biasa | ✓ Ya, fallback |
| Service memanggil `GetByID`, memutuskan, lalu update **tanpa** mengirim versi | ✗ Lost update |

### Pola A — pengecekan di service *(fallback)*

```
Service.UpdateSale()
  ├─ store.GetSaleByID()           → tidak ada baris → NotFound
  ├─ current.Version != in.Version → 412 Precondition Failed
  └─ store.UpdateSale()            → UPDATE biasa, 0 baris bukan 404
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

    _, err = s.store.UpdateSale(ctx, in) // UPDATE biasa, tanpa kondisi versi
    return err
}
```

**Keterbatasan:** masih ada celah race kecil antara baca dan tulis. Gunakan B atau C
untuk data yang penting.

### Pola B — `WHERE version = ?` *(disukai, satu kondisi lock)*

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
        ├─ 1 baris terpengaruh → sukses
        └─ 0 baris terpengaruh → store.GetSaleByID untuk membedakan
              ├─ tidak ada baris     → NotFound
              └─ baris ada, versi beda → 412 Precondition Failed
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

Happy path hanya satu roundtrip dan update bersifat atomik.

### Pola C — CTE atomik *(disarankan untuk tabel baru)*

Dipakai bila update membutuhkan lebih dari satu pengecekan sekaligus (keberadaan,
keunikan, dan versi) yang harus selesai dalam satu roundtrip atomik.

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

// service — meneruskan saja, tidak perlu GetByID terpisah
func (s *saleService) UpdateSale(ctx context.Context, in dtos.UpdateSaleInput) error {
    return s.store.UpdateSale(ctx, in)
}
```

**B vs C:** Pola B cukup bila keberadaan adalah satu-satunya pengecekan (0 baris
terpengaruh). Pilih C ketika pengecekan kedua — keunikan, kepemilikan, aturan lain —
harus diselesaikan atomik bersama penulisan.

---

## Scope Farm dan Soft Delete

Dua aturan yang diikuti setiap query:

```sql
WHERE farm_id = $1
  AND deleted_at IS NULL
```

- `farm_id` berasal dari token (`middleware.FarmIDFrom(ctx)`), tidak pernah dari body
  atau query parameter.
- Baris milik farm lain harus terlihat sama seperti baris yang tidak ada: `404`,
  bukan `403` (itu akan mengonfirmasi keberadaannya).
- `DELETE` mengisi `deleted_at`; hanya operasi "reset farm data" yang menghapus
  permanen.
- Migrasi menambahkan partial index di mana `deleted_at IS NULL`.

---

## Kepemilikan Error

| Situasi | Pemilik | Mengembalikan |
|---|---|---|
| Baris tidak ditemukan berdasarkan ID | Store | `response.NotFound("sale")` |
| Pelanggaran unique key | Store | `response.Conflict("… already exists")` |
| Pelanggaran FK saat delete | Store | `response.Conflict("… still in use")` |
| Version mismatch | Service (A) atau store (B/C) | `response.PreconditionFailed(...)` |
| Transisi state tidak valid | Service | `response.Conflict("…")` |
| Validasi tingkat field | Handler / service | `response.Unprocessable(msg, fields)` |
| Session tidak ada/tidak valid | Middleware | `response.Unauthorized(...)` |
| Refresh token dipakai ulang atau kedaluwarsa | Store (pengecekan atomik) | `response.Unauthorized("session expired")` |
| Redis tidak terjangkau | Mana pun | `response.ServiceUnavailable(err)`, dipetakan ke `503` |
| Role tidak diizinkan | Middleware | `response.Forbidden(...)` |
| Kegagalan tak terduga | Mana pun | `response.Internal(err)`, dicatat sekali |

Aturan store:

- `Get{Entity}ByID` mengembalikan `NotFound` bila baris tidak ada — keberadaan baris
  adalah fakta data, bukan keputusan bisnis.
- CTE atomik (Pola C) boleh mengembalikan error bernama secara langsung.
- `UPDATE`/`DELETE` **biasa** mengembalikan `(rowsAffected, error)`. Service yang
  membedakan arti 0 baris — store tidak boleh menebak `NotFound`.

---

## Referensi Cepat

| Keputusan | Lapisan |
|---|---|
| Parse body, validasi tag | Handler |
| Mapping `Request` → `Input` (tambah scope farm, version) | Handler |
| Memilih status HTTP | `pkg/response`, dipanggil handler |
| "Apakah record ini ada?" | Store (`GetByID`, atau checker CTE) |
| "Apakah sudah diubah orang lain?" | Bandingkan `version`; B/C di query |
| "Apakah transisi state ini boleh?" | Service |
| "Apakah write ini harus atomik?" | Service memutuskan, store mengimplementasikan |
| Menjalankan SQL / membuka transaksi | Store |
| Membuat, me-hash, dan merotasi refresh token | Service memutuskan; `SessionStore` menyimpan secara atomik |
| Menetapkan TTL sesi refresh | Service mengirim `cfg.Auth.RefreshTokenTTL`; store menerapkannya |
| Error mana yang dikembalikan | Service — kecuali sudah diselesaikan atomik di store |
| Mencatat error | Sekali, di batas, dengan `request_id` |

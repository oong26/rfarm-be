# Unit Testing

[English](unit_test.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Filosofi

Test ada untuk **melindungi perilaku**, bukan memaksimalkan angka coverage. Suite
kecil yang menjaga setiap cabang keputusan penting lebih berharga daripada suite
besar yang ditulis hanya untuk mengejar persentase.

> **Patokan:** satu test case = satu perilaku bermakna.

---

## Prinsip

- Cakup setiap **cabang yang mengubah perilaku**.
- Uji perilaku, bukan detail implementasi.
- Jangan menulis test terpisah untuk input yang menghasilkan perilaku identik.
- **Aturan bisnis dan keamanan yang kritis selalu dicakup**: scope farm, aritmetika
  uang, keseimbangan jurnal, pengecekan role, optimistic locking, rotasi sesi, dan
  deteksi reuse.
- Test harus deterministik: tanpa jam nyata, tanpa jaringan, tanpa state database
  bersama.
- Setiap test gagal karena satu alasan, dan alasannya jelas.

---

## Apa yang Diuji di Mana

```
Handler (internal/handlers)
  ├── Parsing dan validasi request → status code dan body problem.
  ├── Scope farm dan version diambil dari context/header, bukan dari body.
  └── Jangan mengulang aturan bisnis di sini.

Service (internal/services)              ← target unit test utama
  ├── Setiap aturan bisnis dan cabang keputusan.
  ├── Setiap jalur kegagalan dependensi (store/client).
  ├── Aritmetika uang dan kuantitas dengan decimal.
  └── Pengecekan kepemilikan dan transisi state.

Store (internal/stores)
  ├── Integration test terhadap PostgreSQL nyata (testcontainers).
  ├── Filter, paginasi, sorting, soft delete, locking versi, hasil CTE.
  └── Session store: unit test dengan miniredis (tidak butuh Docker).

Middleware (pkg/middleware)
  └── Auth, penegakan role, dan status yang dikembalikan.
```

Test service memakai fake store, tidak pernah database. Test store memakai database
nyata, tidak pernah fake. Session store adalah pengecualian: unit test-nya memakai
`miniredis` dan tidak membawa build tag `integration`.

---

## Tata Letak File

Satu file test per file sumber, dinamai mengikuti sumbernya:

| Sumber | Test |
|---|---|
| `sale_service.go` | `sale_service_test.go` |
| `sale_handler.go` | `sale_handler_test.go` |
| `sale_store.go` | `sale_store_test.go` (`//go:build integration`) |
| `session_store.go` | `session_store_test.go` (miniredis, tanpa build tag) |
| `sale_fakes_test.go` | fake bersama untuk paket sale |

Aturan:

- Paket `_test` (black box) secara default: test memakai interface yang diekspor.
- `_internal_test.go` (white box) hanya bila helper tidak diekspor perlu diuji —
  akhiran itu membuat pengecualian terlihat.
- `//go:build integration` untuk apa pun yang membutuhkan PostgreSQL; `go test ./...`
  harus tetap cepat dan offline.
- Fake dan helper bersama: `<entity>_fakes_test.go`, satu file per paket, agar
  perubahan interface diperbaiki di satu tempat.

```
internal/services/
├── sale_service.go
├── sale_service_test.go
├── sale_fakes_test.go          ← fakeSaleStore, newTestService
└── journal_service.go
    └── journal_service_test.go
```

---

## Penamaan Test

```go
func TestCreateSale(t *testing.T)
func TestListSales(t *testing.T)
func TestUpdateSale(t *testing.T)
func TestSaleHandler_Create(t *testing.T)
func TestSaleStore_UpdateSale_VersionConflict(t *testing.T)
```

- `Test{Method}` untuk service atau store.
- `Test{Type}_{Method}` bila beberapa tipe berbagi nama method.
- Tambahkan akhiran hanya untuk perilaku yang diikat kasus itu, bukan untuk setiap
  kasus.

Table-driven dengan kasus bernama:

```go
tests := []struct {
    name      string
    input     dtos.CreateSaleInput
    setup     func(*fakeSaleStore)
    wantCode  string // kode error stabil, "" berarti sukses
    wantField string // opsional: field yang ditunjuk error
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

Assert **kode**, bukan kata-katanya: pesan boleh diperhalus, kode adalah kontrak.

---

## Kasus Dasar

| # | Kasus |
|---|---|
| 1 | Happy path |
| 2 | Error dependensi (store/client) |
| 3 | Aturan bisnis inti untuk operasi itu |
| 4 | Isolasi farm — baris farm lain berperilaku seperti tidak ada (`404`) |

```text
CreateSale
├── success
├── store error
├── qty ≤ 0                          ← aturan field
└── daily record duplikat            ← keunikan

UpdateSale
├── success
├── store error
├── version mismatch                 ← 412
└── baris milik farm lain            ← 404, bukan 403
```

Tidak ada jumlah pasti. Jumlahnya mencerminkan **kompleksitas logika**.

---

## Fake

Fake ditulis tangan, satu per interface, di `<entity>_fakes_test.go`. Framework
mocking tidak diperlukan.

Service yang memanggil `SessionStore` mendapat `fakeSessionStore`:

```go
type fakeSessionStore struct {
    createErr  error
    rotateErr  error
    deleteErr  error
    revokeErr  error
    reuse      bool   // set true untuk simulasi token reuse → 401
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

Session store sendiri diuji dengan `miniredis`:

```go
// internal/stores/session_store_test.go  — tanpa build tag
func TestSessionStore_Rotate_Reuse(t *testing.T) {
    mr := miniredis.RunT(t)
    rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

    store := stores.NewSessionStore(rdb)
    ttl := 30 * 24 * time.Hour

    sess := dtos.RefreshSession{UserID: "u1", FarmID: "f1", FamilyID: "fam1"}
    hash := sha256Hex("token-A")
    require.NoError(t, store.Create(context.Background(), sess, ttl))

    // Rotasi sekali — berhasil
    sess2 := dtos.RefreshSession{UserID: "u1", FarmID: "f1", FamilyID: "fam1"}
    require.NoError(t, store.Rotate(context.Background(), hash, sess2, ttl))

    // Rotasi lagi dengan hash lama — reuse terdeteksi
    err := store.Rotate(context.Background(), hash, sess2, ttl)
    var appErr *response.AppError
    require.ErrorAs(t, err, &appErr)
    assert.Equal(t, "unauthorized", appErr.Code)
}
```

Aturan untuk fake:

```go
type fakeSaleStore struct {
    created   *dtos.CreateSaleInput
    createErr error

    item   *dtos.Sale
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

- Fake mengimplementasi seluruh interface store; method yang tidak dipakai
  mengembalikan nilai nol.
- Catat nilai yang diteruskan di field bila test perlu membuktikannya (`f.created`).
- Jangan mem-fake tipe yang sedang diuji. Jangan mem-mock `context.Context`.
- `fakeSessionStore` mencakup: Redis mati (set `createErr`/`rotateErr`), deteksi
  reuse, jalur revoke-all.

---

## Test Handler

Gunakan `httptest` + router `chi` nyata — tanpa listener jaringan.

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

Cakup: status dan body sukses, kegagalan validasi (`422`), tidak ditemukan (`404`),
version mismatch (`412`), token hilang (`401`), penolakan role (`403`).

Scope farm diuji eksplisit — handler yang membaca `farmId` dari body harus gagal
test.

---

## Test Store (integrasi)

`testcontainers-go` menjalankan PostgreSQL sekali pakai, menerapkan migrasi nyata,
dan memberi setiap test transaksi atau truncation sendiri.

```go
//go:build integration

package stores_test

func TestSaleStore_ListSales(t *testing.T) {
    pool := tests.NewPostgres(t)          // sudah dimigrasi + di-truncate, ditutup t.Cleanup
    s := stores.NewSaleStore(pool)

    farmA, farmB := tests.SeedFarm(t, pool), tests.SeedFarm(t, pool)
    tests.SeedSale(t, pool, farmA.ID, "2026-10-01", "100")
    tests.SeedSale(t, pool, farmA.ID, "2026-10-31", "200")
    tests.SeedSale(t, pool, farmB.ID, "2026-10-15", "300")

    got, total, err := s.ListSales(ctx, dtos.ListSalesParams{
        FarmID: farmA.ID, Offset: 0, Limit: 25, SortField: "date", SortDesc: true,
    })

    require.NoError(t, err)
    assert.EqualValues(t, 2, total)        // baris farmB tidak ikut
    require.Len(t, got, 2)
    assert.Equal(t, "2026-10-31", got[0].Date) // terbaru dulu
}
```

Cakup minimal: batas paginasi, allowlist sort, setiap filter, baris soft-deleted
tidak ikut, baris farm lain tidak ikut, pelanggaran unique dan FK dipetakan ke `409`,
serta hasil locking versi pola B/C.

Jalankan dengan `make test-integration`.

---

## Helper Test

`internal/tests` menyimpan semua yang dipakai bersama antar paket:

| File | Isi |
|---|---|
| `postgres.go` | `NewPostgres(t)` — container, migrasi, cleanup |
| `seed.go` | `SeedFarm`, `SeedSale`, `SeedDailyRecord`, … |
| `http.go` | pembangun request, `DecodeProblem(t, rec)`, `DecodeData[T]` |
| `clock.go` | jam tetap yang dipakai service, bukan `time.Now` |
| `redis.go` | `NewMiniredis(t)` — instance miniredis + `*redis.Client`, ditutup lewat cleanup |

- Helper mendaftarkan cleanup lewat `*testing.T`, tidak pernah membocorkan container.
- Helper tidak meng-assert maksud test — ia hanya menyiapkan state.
- Helper yang dipakai satu paket tetap di file `_test.go` paket itu.

---

## Perintah

```bash
make test               # go test ./... -race -count=1
make test-cover         # profil coverage + laporan HTML
make test-integration   # go test ./... -tags=integration -count=1
go test ./internal/services -run TestCreateSale -v
go test ./internal/stores -tags=integration -run TestSaleStore_UpdateSale -v
```

---

## Coverage

Coverage adalah alat ukur, bukan tujuan.

| Sinyal | Arti |
|---|---|
| Coverage tinggi, kasus bermakna | Suite bagus |
| Coverage tinggi, kasus sepele | Kepercayaan palsu |
| Coverage lebih rendah, semua perilaku kunci tercakup | Dapat diterima |
| Aturan bisnis/keamanan kritis hilang | **Tidak dapat diterima** |

CI menegakkan **batas bawah** (mis. 60% pada `internal/services`), bukan target yang
dikejar. Penurunan pada baris yang berubah dijelaskan di PR, bukan disembunyikan
dengan test sepele.

---

## Jangan Diuji

- Getter, setter, konstruktor tanpa logika.
- Library pihak ketiga dan kode hasil generate sqlc.
- Format string SQL — uji perilakunya terhadap database nyata.
- Cabang yang sama dua kali lewat pintu masuk berbeda.
- Output `time.Now()`; suntikkan jam sebagai gantinya.

---

## Referensi Cepat

| Lapisan | Alat | Database | Kecepatan |
|---|---|---|---|
| Service | fake store/client (termasuk `fakeSessionStore`) | tidak | ms |
| Handler | `httptest` + router chi + fake service | tidak | ms |
| Store (Postgres) | PostgreSQL `testcontainers-go`, tag `integration` | ya | detik |
| Store (session) | `miniredis`, tanpa build tag | tidak | ms |
| Middleware | `httptest` + router chi | tidak | ms |

| Aturan | Benar | Hindari |
|---|---|---|
| Satu file per sumber | `sale_service_test.go` | `all_test.go` |
| Black box default | `package services_test` | mengakses private |
| Assert kontrak | kode error `version_conflict` | teks pesan persis |
| Table-driven | subtest bernama | body hasil salin-tempel |
| Fake dependensi | fake tulisan tangan | mock tipe yang diuji |
| Test session store | `miniredis`, tanpa Docker | `testcontainers-go` untuk Redis |
| Isolasi farm | test eksplisit | menganggap query sudah ter-scope |
| Deterministik | jam/seed tetap | `time.Now()` di assertion |

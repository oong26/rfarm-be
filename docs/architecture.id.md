# Arsitektur

[English](architecture.md) · **Bahasa Indonesia**

[← Kembali ke Dokumentasi](README.id.md)

---

## Gambaran Umum

rfarm-be adalah satu binary Go yang menyajikan JSON API dengan PostgreSQL. Proyek ini
memakai **struktur berlapis yang datar**, bukan pohon paket per domain:

```
Request HTTP
    → handlers     parse, validasi, otorisasi, render
    → services     aturan bisnis, orkestrasi, transaksi
    → stores       SQL di atas sqlc, mapping baris
    → PostgreSQL
```

Lapisan adalah folder, bukan framework. Satu domain menambahkan satu file per lapisan
(`sale_handler.go`, `sale_service.go`, `sale_store.go`, `sale_dto.go`) — tanpa paket
baru, tanpa file wiring selain satu blok di `internal/routes`.

Developer baru membaca empat file untuk memahami satu domain, dan domain baru adalah
salinan dari empat file yang sudah dikenal.

---

## Tanggung Jawab Direktori

```
cmd/app/            Entry point proses: muat config, buka pool, bangun router,
                    jalankan server, tangani shutdown.

api/                api/openapi.yaml — kontrak API. Ditulis sebelum handler,
                    direview bersama client, digenerate ke Go oleh make openapi.

db/queries/         Statement *.sql yang dikompilasi sqlc. Satu-satunya tempat SQL
                    ditulis; tidak pernah SQL inline di Go.

internal/configs/   Konfigurasi aplikasi bertipe: satu struct per bagian
                    (app, database, redis, auth, cors, log), di-parse dari config.yaml
                    sekali dan diinjeksi.

internal/clients/   Integrasi keluar. Satu interface + satu implementasi per
                    dependensi. Service bergantung pada interface, bukan pada SDK.

internal/dtos/      Setiap struct yang melintasi lapisan: entity, Request, Input,
                    Params, Result. Satu file per domain: sale_dto.go.

internal/handlers/  Handler HTTP. Parse request, validasi, panggil satu method
                    service, render lewat pkg/response. Tanpa aturan bisnis.

internal/routes/    Satu-satunya tempat yang mengenal graf lengkap: membangun store,
                    service, handler, dan mendaftarkan route serta middleware.

internal/services/  Aturan bisnis: kepemilikan, transisi state, perhitungan, validasi
                    yang butuh data, orkestrasi beberapa store.

internal/stores/    Akses data: interface + implementasi. Postgres di atas lapisan
                    query sqlc (transaksi, mapping baris, pemetaan error) dan
                    session store Redis.

internal/stores/generated/  Hasil sqlc. Hasil generate, read-only — jangan pernah
                    diedit manual; ubah db/queries lalu jalankan make sqlc.

internal/tests/     Helper test bersama: PostgreSQL testcontainers, miniredis, seed,
                    jam tetap, helper decoding HTTP. Hanya untuk test; tidak pernah
                    diimpor paket produksi.

migrations/         Migrasi skema: pasangan NNNNNN_name.up.sql / .down.sql,
                    diterapkan oleh make migrate-up. Tanpa data aplikasi.

pkg/cache/          Konstruksi client Redis: pool, timeout, health check. Tidak tahu
                    apa pun tentang session.

pkg/config/         Pemuatan YAML, default, dan override environment. Tidak tahu
                    apa pun tentang bagian konfigurasi aplikasi ini.

pkg/logger/         Konstruksi slog dan helper yang sadar context.

pkg/middleware/     Request ID, recover, access log, CORS, auth, role, scope farm.

pkg/response/       Envelope JSON, meta halaman, problem details RFC 9457, tipe error
                    dan konstruktornya, penulis error.

pkg/security/       Hashing argon2id, signing/parsing JWT. Fungsi murni.

pkg/utils/          Wrapper validasi, parsing query, parsing If-Match, helper tanggal,
                    decimal, dan jam.

scripts/            Skrip sekali jalan dan helper CI (seed lokal, tooling).

docs/               Dokumentasi ini. Tidak dikompilasi, tidak diimpor kode.
```

File root bukan paket: `config.example.yaml` (template konfigurasi), `sqlc.yaml`
(konfigurasi sqlc), `Makefile` (pintu masuk tugas), `.air.toml` (live reload), dan
`go.mod`. Pohon lengkap ada di [README.id.md](README.id.md#struktur-folder).

### `internal` vs `pkg`

| Folder | Aturan | Uji |
|---|---|---|
| `internal/` | Kode yang mengenal aplikasi ini: tabelnya, aturannya, konfigurasinya. | Bisakah proyek lain mengimpornya tanpa perubahan? **Tidak** → `internal`. |
| `pkg/` | Kode yang akan berfungsi di layanan Go mana pun dengan stack yang sama. | Bisakah proyek lain mengimpornya tanpa perubahan? **Ya** → `pkg`. |

`pkg` tidak pernah mengimpor `internal`. `cmd` boleh mengimpor keduanya. `internal`
boleh mengimpor `pkg`.

---

## Aturan Dependensi

```
cmd/app
   │
   ├── internal/configs ──┐
   ├── internal/routes ───┼──> internal/handlers ──> internal/services ──> internal/stores
   │                      │            │                    │                    │
   └── pkg/* <────────────┘            └── internal/dtos <──┴────────────────────┘
                                                                    │
                                                              internal/stores/generated
```

Impor yang diizinkan:

| Dari | Boleh mengimpor |
|---|---|
| `cmd/app` | semuanya |
| `internal/routes` | handlers, services, stores, configs, `pkg/cache`, `pkg/*` |
| `internal/handlers` | services (interface), dtos, `pkg/response`, `pkg/utils`, `pkg/middleware` |
| `internal/services` | stores (interface, termasuk `SessionStore`), clients (interface), dtos, `pkg/response`, `pkg/utils`, `pkg/security`, `pkg/logger` |
| `internal/stores` | dtos, generated, `pkg/response` (pemetaan error), `pkg/cache`, `pkg/logger`, `redis/go-redis/v9` (session store) |
| `internal/stores/generated` | hanya pgx |
| `internal/clients` | dtos, `pkg/logger` |
| `internal/configs` | standard library saja |
| `internal/dtos` | standard library, `shopspring/decimal`, `google/uuid` |
| `pkg/cache` | standard library, `redis/go-redis/v9` |
| `pkg/*` | standard library dan paket pihak ketiga saja |

Dilarang, dan menjadi penghalang review:

- `stores → services`, `services → handlers` (ke atas).
- `handlers → stores` (melewati service).
- Service memakai `redis.Client` langsung; bergantunglah pada interface `SessionStore`.
- `pkg → internal` (merusak reusability).
- Mengedit apa pun di `internal/stores/generated`; ubah `db/queries` lalu regenerate.
- Dua domain saling mengimpor tipe konkret; berbagi lewat `dtos` atau injeksi interface
  di `routes`.

---

## Urutan Startup

`cmd/app/main.go` adalah satu-satunya tempat dengan fungsi `main`.

```go
func main() {
    if err := run(); err != nil {
        slog.Error("fatal", "err", err)
        os.Exit(1)
    }
}

func run() error {
    cfg := configs.MustLoad()                    // 1. config bertipe
    log := logger.New(cfg.Log)                   // 2. slog (JSON di prod)
    slog.SetDefault(log)

    decimal.MarshalJSONWithoutQuotes = true      // 3. uang sebagai number JSON

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    pool, err := pgxpool.New(ctx, cfg.Database.DSN)  // 4. connection pool
    if err != nil {
        return fmt.Errorf("pgxpool: %w", err)
    }
    defer pool.Close()
    if err := pool.Ping(ctx); err != nil {
        return fmt.Errorf("db ping: %w", err)
    }

    rdb, err := cache.NewRedis(ctx, cfg.Redis)   // 5. session store
    if err != nil {
        return fmt.Errorf("redis: %w", err)
    }
    defer rdb.Close()
    if err := rdb.Ping(ctx).Err(); err != nil {
        return fmt.Errorf("redis ping: %w", err)
    }

    router := routes.New(routes.Deps{            // 6. middleware + route + wiring
        Pool:   pool,
        Redis:  rdb,
        Config: cfg,
        Logger: log,
    })

    srv := &http.Server{                         // 7. server dengan timeout
        Addr:              cfg.App.Address(),
        Handler:           router,
        ReadHeaderTimeout: cfg.App.ReadHeaderTimeout,
        ReadTimeout:       cfg.App.ReadTimeout,
        WriteTimeout:      cfg.App.WriteTimeout,
        IdleTimeout:       cfg.App.IdleTimeout,
    }

    errCh := make(chan error, 1)
    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
    }()

    select {                                     // 8. berjalan sampai sinyal atau gagal
    case err := <-errCh:
        return err
    case <-ctx.Done():
    }

    shutCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
    defer cancel()
    return srv.Shutdown(shutCtx)
}
```

Aturan:

- Tanpa logika bisnis di `main` — ia merangkai, bukan memutuskan.
- Proses gagal cepat: config buruk, database tak terjangkau, atau route tidak valid
  menghentikan startup dengan pesan jelas.
- Setiap resource yang dibuka di `main` ditutup di sana (`defer pool.Close()`, `defer rdb.Close()`).
- `run() error` menjaga `main` tetap dapat diuji dan bebas `os.Exit` di tengah.

### Urutan middleware global

Urutan penting; jaga identik di mana pun.

```
1. RequestID     → X-Request-ID di respons dan di setiap baris log
2. Recover       → panic menjadi 500 yang tercatat dengan body problem
3. AccessLog     → method, path, status, latency, request_id, farm_id
4. CORS          → origin, method, header yang dikonfigurasi
5. Auth          → memvalidasi token; menaruh userID + farmID di context
6. RequireRole   → 403 untuk viewer pada operasi tulis, dan sejenisnya
7. Handler
```

`Recover` harus berada di atas `AccessLog` dan semua handler, atau panic akan melewati
kontrak error JSON.

---

## Siklus Request

```
POST /api/v1/sales
  → RequestID        menetapkan request ID di context
  → Recover          membungkus sisanya
  → AccessLog        memulai timer
  → CORS             short-circuit OPTIONS
  → Auth             mem-parsing Bearer token → userID + farmID di context
  → RequireRole      memeriksa role yang dibutuhkan route
  → SaleHandler.Create
       utils.ParseAndValidate(r, &req)    → 400 atau 422 problem
       svc.CreateSale(ctx, input)         → satu panggilan service
            pengecekan aturan              → 422 / 404 / 409
            store.CreateSale (sqlc)        → SQL → 500 bila gagal
       response.Created(w, sale)           → 201 + envelope
```

Setiap lapisan mengembalikan `error` ke atas; hanya handler (lewat `response.Fail`)
dan penulis error terluar yang menghasilkan body.

---

## Konfigurasi

Konfigurasi berada di satu file YAML, di-parse sekali menjadi struct bertipe lalu
diteruskan ke bawah. Tidak ada variabel config global dan tidak ada file `.env`.

```yaml
# config.example.yaml — template yang di-commit; salin ke config.yaml (git-ignored)
app:
  env: development
  addr: ":8080"
  readTimeout: 10s
  writeTimeout: 15s
  idleTimeout: 60s
  shutdownTimeout: 10s
  defaultTimezone: Asia/Jakarta
  maxImportBytes: 20971520

database:
  url: postgres://user:pass@localhost:5432/rfarm?sslmode=disable
  maxConns: 10

redis:
  addr: localhost:6379
  password: ""
  db: 0
  dialTimeout: 2s
  readTimeout: 1s
  writeTimeout: 1s
  poolSize: 10

auth:
  jwtSigningKey: change-me
  accessTokenTTL: 15m
  refreshTokenTTL: 720h

cors:
  allowedOrigins:
    - http://localhost:5173

log:
  level: info
```

- File-nya `config.yaml`; path dapat diubah lewat satu environment variable
  `CONFIG_PATH` (default `./config.yaml`).
- Dimuat oleh `pkg/config` (decode YAML, default, validasi) ke struct di
  `internal/configs`; `configs.MustLoad()` menghentikan startup dengan pesan yang
  menyebut file atau key yang salah.
- Prioritas: default di kode < `config.yaml` < override environment.
- Hanya secret yang boleh di-override environment — `DATABASE_URL` dan
  `JWT_SIGNING_KEY` — agar produksi dapat menyimpannya di luar file dan image
  (nama mengikuti `specification.md` §5.4).
- Durasi berupa string durasi Go (`10s`, `720h`) yang di-parse saat load; batas byte
  dan jumlah koneksi berupa integer.
- `app.defaultTimezone` menentukan batas bulan laporan dan tampilan audit. Tanggal
  bisnis tetap `DATE` biasa — server tidak pernah menggeser hari kalender.
- `app.maxImportBytes` (20 MB) membatasi import JSON legacy, terpisah dari batas body
  request umum.
- `redis.*` menunjuk session store. Sifatnya **wajib**, bukan cache opsional: startup
  melakukan ping, dan `/readyz` memeriksa Postgres dan Redis.
- `auth.refreshTokenTTL` adalah TTL setiap sesi refresh di Redis; rotasi mengulang
  TTL-nya.
- Nilai `app.*Timeout` adalah pengerasan server; sisanya konfigurasi produk.
- `config.yaml` di-ignore git; `config.example.yaml` di-commit dan tidak memuat
  secret nyata.

---

## Akses Data

- Satu `*pgxpool.Pool` dibuat di `main`, diinjeksi ke konstruktor store.
- SQL berada di `db/queries/*.sql`; `sqlc generate` menghasilkan fungsi bertipe di
  `internal/stores/generated`.
- Store tulisan tangan membungkus query hasil generate dan mengembalikan `dtos`, agar
  tipe pgx tidak bocor melewati store.
- Placeholder mengikuti sqlc (`$1, $2, …`, `sqlc.arg`). Selalu memakai parameter,
  tidak pernah menyambung string SQL.
- Nama kolom sort berasal dari allowlist, tidak pernah dari request
  ([List API](standards/list_api.id.md)).
- Transaksi dibuka store, tidak pernah diteruskan ke service
  ([Service & Store](standards/service_repo.id.md)).
- Perubahan skema berada di `migrations/` dan diterapkan oleh `make migrate-up`.
  Aplikasi tidak pernah membuat atau mengubah tabel saat startup.
- **Session store** adalah satu-satunya store non-SQL: `SessionStore` di
  `internal/stores/session_store.go`, didukung Redis lewat `pkg/cache`. Isinya hanya
  sesi refresh — tanpa aturan bisnis, tanpa transaksi Postgres.

### Konfigurasi sqlc

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "migrations"
    queries: "db/queries"
    gen:
      go:
        package: "generated"
        out: "internal/stores/generated"
        sql_package: "pgx/v5"
        emit_json_tags: false
        emit_interface: false
        overrides:
          - db_type: "numeric"
            go_type: "github.com/shopspring/decimal.Decimal"
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
```

---

## Tenancy, Role, Uang, dan Session

Aturan-aturan ini membentuk hampir setiap lapisan, jadi dinyatakan sekali di sini.

### Scope farm berasal dari token

```
JWT claims { sub: userId, farmId, role }
    → middleware.Auth menyimpannya di context request
    → handler membaca middleware.FarmIDFrom(ctx) dan meneruskannya di input service
    → setiap query store menyertakan `farm_id = $n`
```

- `farm_id` **tidak pernah** diterima dari body atau query parameter.
- Baris milik farm lain harus terlihat tidak ada: `404`, bukan `403`.
- Service tidak pernah memercayai ID yang tidak berasal dari context.

### Role ditegakkan di middleware

| Role | Boleh |
|---|---|
| `viewer` | membaca semua data farm |
| `editor` | membaca + membuat/mengubah/menghapus record |
| `owner` | semuanya, termasuk settings, akun, anggota, import/reset |

Route dikelompokkan per role di `internal/routes`; handler menganggap pengecekan sudah
terjadi. Handler tidak memeriksa role lagi kecuali aturannya bergantung pada baris
(jarang — taruh di service).

### Uang dan kuantitas adalah decimal

- Kolom `numeric` ↔ `decimal.Decimal` di `dtos`.
- Aritmetika terjadi di service (`Amount = Qty × Price`), dibulatkan sekali.
- `float64` tidak pernah muncul di jalur uang — bukan di DTO, bukan di perhitungan,
  bukan di fixture test.

### Sesi refresh hidup di Redis

Access token tetap stateless: `middleware.Auth` memverifikasi tanda tangan dan masa
berlaku JWT dan tidak pernah menyentuh Redis. Redis menyimpan **sesi refresh**, yang
memungkinkan logout dan rotasi.

| Item | Pilihan |
|---|---|
| Refresh token | 32 byte acak, base64url — tidak pernah disimpan sebagai teks biasa |
| Key sesi | `rfarm:session:<sha256(token)>` → JSON `{ userId, farmId, role, familyId, issuedAt }` |
| TTL | `auth.refreshTokenTTL` (default 30 hari), diulang setiap rotasi |
| Indeks family | `rfarm:family:<familyId>` — set hash token yang masih hidup |
| Indeks user | `rfarm:user:<userId>` — set ID family, untuk revoke-all |
| Rotasi | hapus key lama dan terbitkan token baru di family yang sama, secara atomik |
| Reuse | family masih hidup tetapi hash token tidak dikenal → cabut family, `401` |
| Logout | hapus key sesi; family bertahan sampai TTL-nya |
| Ganti password / hapus anggota | cabut semua family di indeks user |

```
POST /auth/login    → 200 { accessToken, refreshToken, expiresIn }
POST /auth/refresh  → rotasi: 200 dengan pasangan baru, atau 401 bila reuse/kedaluwarsa
POST /auth/logout   → 204; sesi refresh dihapus
```

Aturan:

- Hash dengan SHA-256, bukan bcrypt: token berentropi tinggi dan session store harus
  cepat. Password memakai `argon2id`; token tidak.
- Rotasi bersifat atomik (skrip Lua atau `WATCH`/`MULTI`); dua refresh bersamaan tidak
  boleh sama-sama berhasil.
- Setelah logout, access token yang sedang dipakai tetap berlaku sampai kedaluwarsa
  (≤ 15 menit). Itu trade-off yang diterima; tambahkan blocklist `jti` hanya bila
  pemutusan instan dibutuhkan.
- Redis bukan cache di sini: bila tidak tersedia, login/refresh/logout mengembalikan
  `503` dan `/readyz` gagal.
- Rotasi mengulang TTL (sliding expiration); sesi yang tidak pernah di-refresh hilang
  setelah `auth.refreshTokenTTL`.

---

## Alur Error

```
store      → response.NotFound / Conflict / PreconditionFailed, atau error terbungkus
service    → response.Unprocessable / Forbidden / PreconditionFailed, atau teruskan
handler    → response.Fail(w, r, err)     ← satu titik pemetaan
middleware → Recover → response.Fail        ← panic menjadi 500
```

Setiap error membawa status HTTP, kode yang stabil, dan body problem. Detail:
[Error Handling](error_handling.id.md) dan
[HTTP Status](standards/http_status.id.md).

---

## Logging

- Satu `*slog.Logger` dibangun di `main`, diinjeksi ke middleware, service, dan client.
- Hanya key/value terstruktur: `request_id`, `method`, `path`, `status`,
  `latency_ms`, `user_id`, `farm_id`, `err`.
- Level: `debug` untuk detail developer, `info` untuk peristiwa siklus hidup, `warn`
  untuk keanehan yang pulih, `error` untuk kegagalan yang perlu perhatian.
- Jangan pernah mencatat password, token, body request penuh, atau data pribadi.
- Catat error **sekali**, di batas yang memutuskan respons.

---

## Menambah Domain Baru

Dengan `sale` sebagai contoh. Sembilan langkah, tanpa paket baru.

1. **Query** — `db/queries/sales.sql` dengan statement bernama (`-- name: CreateSale
   :one`), lalu `make sqlc`.
2. **Migrasi** — `migrations/000002_create_sales.up.sql` / `.down.sql`.
3. **DTO** — `internal/dtos/sale_dto.go`: `Sale`, `CreateSaleRequest`,
   `CreateSaleInput`, `ListSalesParams`, …
4. **Store** — `internal/stores/sale_store.go`: interface + `postgresSaleStore` di atas
   `generated.Queries`, dan `NewSaleStore(pool *pgxpool.Pool) SaleStore`.
5. **Service** — `internal/services/sale_service.go`: interface + `saleService` yang
   tidak diekspor, aturan bisnis, konstruktor.
6. **Handler** — `internal/handlers/sale_handler.go`: method `http.HandlerFunc` dan
   `Register(r chi.Router)`.
7. **Kontrak** — path dan schema di `api/openapi.yaml` (dalam praktik ditulis sebelum
   handler).
8. **Wiring** — `internal/routes/routes.go`, satu blok:

   ```go
   saleStore := stores.NewSaleStore(d.Pool)
   saleSvc := services.NewSaleService(saleStore, d.Logger, utils.SystemClock{})
   saleHandler := handlers.NewSaleHandler(saleSvc, d.Logger)
   saleHandler.Register(api)
   ```

9. **Test** — `sale_service_test.go`, `sale_handler_test.go`, `sale_store_test.go`
   (tag integration).

> Urutan penting: query → migrasi → dto → store → service → handler → route. Setiap
> langkah hanya mengimpor yang sudah ada.

---

## Anti-Pola

| Anti-pola | Mengapa merugikan | Lakukan ini |
|---|---|---|
| SQL di handler atau service | Tidak dapat diuji, mencampur tanggung jawab | Method store |
| Aturan bisnis di store | Aturan hilang saat query berubah | Service |
| Handler memanggil store langsung | Melewati pengecekan kepemilikan/state | Service |
| `farm_id` dari body atau query | Kebocoran data antar-tenant | Token → context |
| Mengedit `stores/generated` | Kode hasil generate menimpanya | Ubah query, regenerate |
| `float64` untuk uang | Kesalahan pembulatan di pembukuan | `decimal.Decimal` |
| Satu paket per domain (`internal/modules/...`) | Berlebihan untuk basis ini; wiring lebih banyak | Folder lapisan datar |
| Variabel global `pool`/`cfg` | Dependensi tersembunyi, test rawan race | Injeksi di `routes` |
| `init()` yang membuka koneksi | Tidak dapat diuji, bergantung urutan | `main` |
| `panic` untuk kegagalan yang diperkirakan | Mematikan request, tanpa kontrak | Error bertipe |
| Mengembalikan `err.Error()` ke client | Membocorkan internal | `response.Internal(err)` |
| Logika bisnis di middleware | Tersembunyi, sulit diuji | Service |
| Refresh token disimpan sebagai teks biasa, atau di Postgres | Dump database menjadi sesi hidup; tabel tumbuh tanpa batas | Hash SHA-256 di Redis dengan TTL |
| `redis.Client` diinjeksi ke service | Mengikat aturan bisnis ke satu store | Interface `SessionStore` |
| State sesi dimasukkan ke JWT | Tidak dapat dicabut sebelum kedaluwarsa | Simpan di session store |

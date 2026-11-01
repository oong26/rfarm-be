# Development

[English](development.md) · **Bahasa Indonesia**

[← Kembali ke Dokumentasi](README.id.md)

---

## Prasyarat

| Tool | Versi | Kegunaan |
|---|---|---|
| Go | lihat `go.mod` (1.23+) | build dan test |
| PostgreSQL | 15+ | database lokal (disarankan 16) |
| Redis | 7+ | sesi refresh; aplikasi menolak start tanpanya |
| Docker | versi terbaru | `testcontainers-go` untuk integration test |
| make | apa pun | semua tugas lewat Makefile |
| Air | terbaru | live reload (`go install github.com/air-verse/air@latest`) |
| golangci-lint | terbaru | linting (`brew install golangci-lint`) |
| golang-migrate | terbaru | migrasi skema (`brew install golang-migrate`) |
| sqlc | 1.27+ | query bertipe (`brew install sqlc`) |
| oapi-codegen | terbaru | tipe/stub OpenAPI (`go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest`) |

Opsional: `delve` untuk debugging, `psql` untuk query manual.

---

## Menjalankan Pertama Kali

```bash
# 1. dependensi
go mod download

# 2. konfigurasi lokal
cp config.example.yaml config.yaml    # config.yaml di-ignore git
# edit database.url dan auth.jwtSigningKey

# 3. database dan session store
createdb rfarm
make migrate-up
redis-server                     # atau: brew services start redis

# 4. generate sqlc + kode OpenAPI, lalu jalankan
make sqlc
make openapi
make dev
```

API tersedia di `http://localhost:8080`; health di `/healthz` dan `/readyz`; kontrak
disajikan di `/api/v1/openapi.yaml`.

Cek berhasil:

```bash
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
curl -s localhost:8080/api/v1/sales -H "Authorization: Bearer $TOKEN"
```

---

## Konfigurasi

Konfigurasi berupa satu file YAML. `config.example.yaml` di-commit sebagai template dan
mendokumentasikan setiap key; `config.yaml` bersifat lokal dan di-ignore git.

```yaml
# config.yaml
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

| Key | Default | Catatan |
|---|---|---|
| `app.env` | `development` | `development` \| `staging` \| `production` |
| `app.addr` | `:8080` | alamat listen |
| `app.readTimeout` | `10s` | timeout baca server |
| `app.writeTimeout` | `15s` | timeout tulis server |
| `app.idleTimeout` | `60s` | timeout idle keep-alive |
| `app.shutdownTimeout` | `10s` | jatah graceful shutdown |
| `app.defaultTimezone` | `Asia/Jakarta` | batas bulan laporan, tampilan audit |
| `app.maxImportBytes` | `20971520` | batas import JSON legacy (20 MB) |
| `database.url` | — | **wajib**, DSN PostgreSQL |
| `database.maxConns` | `10` | koneksi maksimum pgxpool |
| `redis.addr` | `localhost:6379` | **wajib**, host:port Redis untuk sesi refresh |
| `redis.password` | `""` | password AUTH Redis, kosong bila tidak diset |
| `redis.db` | `0` | indeks database logis |
| `redis.dialTimeout` | `2s` | timeout koneksi |
| `redis.readTimeout` | `1s` | timeout baca socket |
| `redis.writeTimeout` | `1s` | timeout tulis socket |
| `redis.poolSize` | `10` | koneksi maksimum di pool |
| `auth.jwtSigningKey` | — | **wajib**, kunci signing HMAC |
| `auth.accessTokenTTL` | `15m` | masa hidup access token |
| `auth.refreshTokenTTL` | `720h` | TTL sesi refresh di Redis |
| `cors.allowedOrigins` | `["http://localhost:5173"]` | daftar origin |
| `log.level` | `info` | `debug` \| `info` \| `warn` \| `error` |

Aturan:

- Path file berasal dari `CONFIG_PATH` (default `./config.yaml`). Tidak ada environment
  variable lain yang wajib untuk menjalankan aplikasi.
- Prioritas: default di kode < `config.yaml` < override environment.
- Hanya `DATABASE_URL` dan `JWT_SIGNING_KEY` yang boleh meng-override `database.url`
  dan `auth.jwtSigningKey`, agar deployment dapat menyimpan keduanya di luar file dan
  image (nama dari `specification.md` §5.4).
- `redis.*` adalah dependensi keras, seperti `database.*`: startup melakukan ping ke
  Redis dan `/readyz` melaporkannya. Tidak ada fallback in-memory.
- Aplikasi gagal cepat, menyebut file atau key, bila file hilang atau nilai wajib
  kosong.
- Jangan pernah commit `config.yaml`, DSN nyata, kunci signing, atau sertifikat.
- Di staging dan produksi file dirender dari secret store atau config map platform,
  tidak dipanggang ke dalam image.

---

## Target Make

`make` adalah satu-satunya antarmuka yang dibutuhkan. Bila sebuah tugas belum ada,
tambahkan target alih-alih mendokumentasikan perintah mentah di README atau chat.

| Target | Fungsinya |
|---|---|
| `make dev` | jalankan dengan Air live reload |
| `make run` | `go run ./cmd/app` |
| `make build` | build binary ke `bin/rfarm-be` |
| `make sqlc` | regenerasi `internal/stores/generated` dari `db/queries` |
| `make openapi` | regenerasi tipe/stub Go dari `api/openapi.yaml` |
| `make check-openapi` | gagal bila `api/openapi.yaml` bukan OpenAPI 3.1 yang valid |
| `make test` | `go test ./... -race -count=1` |
| `make test-cover` | profil coverage + laporan HTML |
| `make test-integration` | test dengan `-tags=integration` (menjalankan container Docker) |
| `make lint` | `golangci-lint run ./...` |
| `make fmt` | `gofmt -s -w` + `goimports` |
| `make tidy` | `go mod tidy` |
| `make migrate-up` | terapkan semua migrasi tertunda |
| `make migrate-down` | rollback migrasi terakhir |
| `make migrate-create name=create_sales` | buat pasangan up/down baru |
| `make seed` | muat `scripts/seed.sql` ke database lokal |
| `make check` | fmt + lint + diff sqlc + cek openapi + test — jalankan sebelum setiap push |

`make check` harus gagal bila `make sqlc` akan mengubah file hasil generate: kode yang
di-commit dan kode hasil generate tidak boleh berbeda.

---

## Alur Kerja Harian

```bash
make dev                 # terminal 1 — server dengan reload
make test                # terminal 2 — setelah setiap perubahan
make lint
make check               # sebelum commit
```

Siklus yang disarankan:

1. Tulis SQL di `db/queries/<domain>.sql` dan jalankan `make sqlc`.
2. Tulis atau perbarui migrasi.
3. Tambahkan DTO, lalu interface store + implementasinya.
4. Tambahkan aturan service dan test-nya.
5. Tambahkan path + schema ke `api/openapi.yaml`, jalankan `make openapi`.
6. Tambahkan handler dan test handler.
7. Wiring di `internal/routes`.
8. Perbarui dokumen, jalankan `make check`.

---

## Migrasi Database

- File berada di `migrations/`, satu pasang per perubahan:
  `000002_create_sales.up.sql` dan `000002_create_sales.down.sql`.
- Penomoran berurutan; jangan pernah memakai ulang atau menomori ulang migrasi yang
  sudah diterapkan.
- Setiap up punya down yang nyata (`DROP TABLE`, `DROP COLUMN`, …).
- Jangan pernah mengedit migrasi yang sudah berjalan di luar mesin Anda — buat yang
  baru.
- Migrasi bersifat aditif secara default: tambah kolom dengan default, backfill, lalu
  perketat constraint di migrasi berikutnya.
- Data referensi (chart of accounts per farm) di-seed oleh perintah aplikasi, bukan
  migrasi; data demo lokal berada di `scripts/seed.sql`.

```bash
make migrate-create name=create_sales
make migrate-up
make migrate-down          # satu langkah
migrate -path migrations -database "postgres://user:pass@localhost:5432/rfarm?sslmode=disable" version
```

Migrasi down untuk pengembangan lokal. Produksi bersifat forward-only: migrasi yang
salah diperbaiki dengan migrasi baru, bukan dengan rollback.

---

## Alur sqlc

SQL ditulis di `db/queries/*.sql` dengan statement bernama lalu dikompilasi menjadi Go.

```sql
-- db/queries/sales.sql
-- name: GetSale :one
SELECT id, farm_id, date, buyer, qty, price, amount, version, created_at, updated_at
  FROM sales
 WHERE id = $1 AND farm_id = $2 AND deleted_at IS NULL;
```

```bash
make sqlc
```

Aturan:

- `internal/stores/generated` di-commit dan **tidak pernah diedit manual**.
- Query yang dibutuhkan store harus ada di `db/queries`; tanpa SQL inline di store
  kecuali statement langka yang tidak dapat diungkapkan sqlc (jelaskan di komentar).
- `numeric` dipetakan ke `decimal.Decimal` dan `uuid` ke `uuid.UUID` di `sqlc.yaml`
  ([Arsitektur](architecture.id.md)).
- Setiap select pada tabel record memfilter `deleted_at IS NULL` kecuali query memang
  tentang baris yang sudah dihapus.

---

## Alur OpenAPI

```bash
make openapi        # regenerasi tipe/stub dari api/openapi.yaml
make check-openapi  # validasi sebelum commit
```

- `api/openapi.yaml` adalah sumber kebenaran; tidak ada anotasi di kode Go.
- Ubah YAML lebih dulu, review, lalu implementasikan.
- File hasil generate di-commit pada commit yang sama dengan perubahan YAML.
- Menyajikan `/api/v1/openapi.yaml` dari binary sudah cukup — tidak ada Swagger UI
  terpisah di produksi.

---

## Test

```bash
make test                                   # unit, cepat, tanpa database
make test-integration                       # butuh Docker
go test ./internal/services -run TestCreateSale -v
go test ./internal/services -coverprofile=cover.out
go tool cover -html=cover.out
```

Aturan: lihat [Unit Testing](standards/unit_test.id.md). Unit test tidak pernah
menyentuh jaringan atau database nyata, dan tidak perlu Redis yang berjalan: service
memakai `SessionStore` palsu, dan session store sendiri diuji dengan `miniredis`.
Test store diberi tag `integration` dan memakai `testcontainers-go`, yang membutuhkan
Docker daemon berjalan; CI menjalankan container dengan cara yang sama.

`testcontainers-go` menarik `postgres:16-alpine` sekali dan menyimpannya di cache.
Run pertama lambat; run berikutnya memakai ulang image dan hanya butuh detik.

---

## Dependensi

```bash
go get github.com/example/pkg@v1.2.3
go mod tidy
```

- Pin versi; jangan memakai `@latest` di commit.
- Jelaskan dependensi baru di PR: apa yang digantikan, lisensinya, status
  pemeliharaannya.
- Utamakan standard library dan stack yang sudah ada (`chi`, `pgx`, `sqlc`, `slog`,
  `decimal`, `validator`, `redis/go-redis/v9`) sebelum menambah sesuatu yang baru.
- `miniredis` hanya untuk test: ia tidak muncul di jalur impor produksi.
- Jangan vendor kecuali tim memutuskan demikian.

---

## Troubleshooting

| Gejala | Kemungkinan penyebab | Perbaikan |
|---|---|---|
| `connection refused` saat startup | PostgreSQL tidak berjalan / port salah | `brew services start postgresql`, cek `database.url` |
| `redis: dial tcp … connection refused` | Redis tidak berjalan / port salah | `brew services start redis`, cek `redis.addr` |
| `redis: WRONGPASS` atau `NOAUTH` | `redis.password` hilang atau salah | isi di `config.yaml`, atau kosongkan bila server tanpa AUTH |
| `/readyz` mengembalikan `503` padahal Postgres hidup | Redis tidak terjangkau — ia dependensi keras | jalankan Redis, cek `redis.addr` dan jaringan |
| setiap login/refresh mengembalikan `503` | Redis mati atau `redis.*` salah konfigurasi | sama seperti di atas; sesi tidak di-cache di memori sebagai fallback |
| `session expired` tepat setelah login | `auth.refreshTokenTTL` terlalu kecil, atau dua server memakai DB Redis berbeda | naikkan TTL; arahkan semua instance ke `redis.addr`/`redis.db` yang sama |
| `password authentication failed` | kredensial salah atau `config.yaml` basi | perbaiki `database.url` (atau override `DATABASE_URL`) |
| `relation "sales" does not exist` | migrasi belum diterapkan | `make migrate-up` |
| `sqlc: column does not exist` | migrasi belum diterapkan sebelum regenerate | `make migrate-up && make sqlc` |
| kode hasil generate basi di CI | `make sqlc` belum dijalankan | jalankan dan commit `internal/stores/generated` |
| `port already in use` | `make dev` lama masih berjalan | matikan (`lsof -i :8080`) atau ubah `app.addr` |
| `config file not found` | `config.yaml` hilang atau `CONFIG_PATH` menunjuk tempat lain | `cp config.example.yaml config.yaml` |
| integration test skip atau gagal start | Docker daemon tidak berjalan | jalankan Docker Desktop, ulangi `make test-integration` |
| `permission denied for table …` | migrasi dijalankan dengan role berbeda | jalankan `make migrate-up` dengan DSN aplikasi |
| Air reload berulang | ia memantau output-nya sendiri | build ke `bin/`, masukkan `tmp/` ke `.gitignore` |
| `412` pada setiap update di lokal | `If-Match` basi dari respons lama | baca ulang record dan pakai `version` terkininya |

---

## Sebelum Pull Request

- [ ] `make check` lulus (fmt, lint, diff sqlc, cek OpenAPI, test).
- [ ] Perilaku baru punya test; perilaku yang berubah punya test yang diperbarui.
- [ ] `api/openapi.yaml` diperbarui dan `make openapi` dijalankan saat kontrak
      berubah.
- [ ] Migrasi ditambahkan untuk setiap perubahan skema, dengan `down` yang berfungsi.
- [ ] Dokumen diperbarui pada commit yang sama, termasuk kembaran `.id.md`
      ([Aturan Dokumentasi](standards/documentation.id.md)).
- [ ] Tidak ada secret, debug print, atau kode yang dikomentari.
- [ ] `config.example.yaml` diperbarui bila ada key baru.

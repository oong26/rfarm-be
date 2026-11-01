# rfarm-be — Dokumentasi Teknis

[English](README.md) · **Bahasa Indonesia**

rfarm-be adalah backend untuk **R FARM Lestari ERP** — sistem manajemen peternakan
ayam petelur skala kecil. Layanan ini menyediakan satu JSON API di atas PostgreSQL,
dibangun dengan struktur berlapis yang datar dan aturan yang sama di setiap domain.

Kontrak produk (entity, aturan field, perhitungan, endpoint) berada di
[specification.md](specification.md). Dokumen ini membahas bagaimana kode disusun,
ditulis, diuji, dan didokumentasikan.

> **Patokan:** spesifikasi mengatakan *apa yang dilakukan produk*; dokumen ini
> mengatakan *bagaimana kita membangunnya*. Jika sebuah fakta sudah ada di OpenAPI
> atau di kode, tautkan saja — jangan disalin.

---

## Mulai dari sini

| Saya ingin… | Baca |
|---|---|
| Tahu apa yang harus dilakukan produk | [specification.md](specification.md) |
| Menjalankan proyek untuk pertama kali | [Development](development.id.md) |
| Memahami struktur dan alur request | [Architecture](architecture.id.md) |
| Menulis kode yang sesuai dengan repo | [Standards](standards/standard.id.md) |
| Mengembalikan status dan body error yang benar | [HTTP Status](standards/http_status.id.md), [Error Handling](error_handling.id.md) |
| Menulis atau memperbarui dokumentasi | [Aturan Dokumentasi](standards/documentation.id.md) |
| Commit, branch, atau membuka PR | [Git Workflow](git_workflow.id.md) |

---

## Stack

| Urusan | Pilihan |
|---|---|
| Bahasa | Go ≥ 1.23 |
| HTTP | `go-chi/chi` v5, `net/http` |
| Database | PostgreSQL 15+, `jackc/pgx` v5 + `pgxpool` |
| Query | `sqlc` — SQL di `db/queries`, Go hasil generate di `internal/stores/generated` |
| Validasi | `go-playground/validator` + aturan domain |
| Uang / kuantitas | `shopspring/decimal` — tidak pernah `float64` |
| Logging | `log/slog` (JSON), request ID di setiap baris |
| Kontrak API | OpenAPI 3.1 di `api/openapi.yaml`, direview sebelum kode |
| Migrasi | `golang-migrate`, forward-only di produksi |
| Auth | JWT access token (15 menit) + rotating refresh token, `argon2id` |
| Session | `redis/go-redis/v9` — sesi refresh dengan TTL, rotasi, dan deteksi reuse |
| Test | `testing`, `testify`, `testcontainers-go` untuk PostgreSQL nyata |

---

## Peta dokumentasi

### Arsitektur & alur kerja

| Dokumen | Menjawab apa |
|---|---|
| [architecture.md](architecture.id.md) | Fungsi setiap direktori, lapisan mana boleh mengimpor mana, perjalanan request, cara aplikasi start dan stop |
| [development.md](development.id.md) | Prasyarat, konfigurasi, target Make, sqlc, migrasi, pengujian, troubleshooting |
| [error_handling.md](error_handling.id.md) | Cara error dibuat, dibungkus, dipetakan ke status code, dicatat, dan dikembalikan |
| [git_workflow.md](git_workflow.id.md) | Penamaan branch, pesan commit, ukuran PR dan aturan review, rilis |

### Standar

| Topik | Dokumen | Menjawab apa |
|---|---|---|
| Indeks | [standard.md](standards/standard.id.md) | Aturan dasar dan daftar topik lengkap |
| Penamaan API | [api_naming.md](standards/api_naming.id.md) | Plural vs singular, verb, nesting, `/admin`, satu tujuan per endpoint |
| Penamaan method | [method_naming.md](standards/method_naming.id.md) | `List{Entity}`, `List{Entity}By{Parent}`, nama method CRUD |
| Struct | [struct.md](standards/struct.id.md) | `Request` / `Input` / `Params`, kapan dishare, di mana struct disimpan |
| Service & store | [service_repo.md](standards/service_repo.id.md) | Tanggung jawab lapisan, interface store, transaksi, optimistic locking |
| Status HTTP | [http_status.md](standards/http_status.id.md) | Status code mana yang dipakai dan mengapa, 409 vs 412 vs 422 |
| List API | [list_api.md](standards/list_api.id.md) | Paginasi sebagai default, filter, sorting, meta, export |
| Unit test | [unit_test.md](standards/unit_test.id.md) | Apa yang diuji per lapisan, tata letak file, kasus dasar, fake |
| Dokumentasi | [documentation.md](standards/documentation.id.md) | Di mana setiap jenis pengetahuan disimpan dan cara menulisnya |

---

## Struktur folder

```
rfarm-be/
├── api/
│   └── openapi.yaml          # kontrak API — direview sebelum implementasi
├── cmd/
│   └── app/                  # entry point: wiring dan siklus hidup server
├── internal/                 # kode aplikasi — tidak pernah diimpor modul lain
│   ├── clients/              # integrasi keluar (HTTP API, email, storage)
│   ├── configs/              # konfigurasi aplikasi bertipe
│   ├── dtos/                 # struct yang melintasi lapisan (entity, request, query)
│   ├── handlers/             # lapisan HTTP: parse, validasi, render
│   ├── routes/               # tabel route, grup middleware, wiring dependensi
│   ├── services/             # aturan bisnis dan orkestrasi
│   ├── stores/               # akses data: Postgres lewat sqlc, sesi Redis
│   │   └── generated/        # kode hasil sqlc — jangan diedit
│   └── tests/                # helper test bersama, fixture, golden file
├── pkg/                      # paket yang bisa dipakai ulang, tanpa pengetahuan aplikasi
│   ├── cache/                # client Redis: pool, timeout, health check
│   ├── config/               # pemuatan YAML, default, override env
│   ├── logger/               # konstruksi slog
│   ├── middleware/           # request id, recover, access log, CORS, auth, role
│   ├── response/             # envelope JSON, problem details RFC 9457, error
│   ├── security/             # argon2id, signing/parsing JWT
│   └── utils/                # validasi, parsing query, paginasi, waktu
├── db/
│   └── queries/              # statement *.sql yang dikompilasi sqlc
├── migrations/               # 000001_init.up.sql, …
├── scripts/                  # skrip bantu sekali pakai dan CI
├── docs/                     # dokumentasi ini
├── sqlc.yaml                 # sqlc: db/queries → internal/stores/generated
├── config.example.yaml       # template konfigurasi — salin ke config.yaml
├── Makefile                  # pintu masuk untuk developer
├── .air.toml                 # live reload untuk pengembangan lokal
└── go.mod
```

Aturan di balik struktur ini — termasuk apa yang boleh dan tidak boleh diimpor —
ada di [architecture.md](architecture.id.md).

---

## Urutan bacaan yang disarankan

1. [specification.md](specification.md) — produk yang sedang dibangun.
2. [Architecture](architecture.id.md) — model mental lapisannya.
3. [Development](development.id.md) — jalankan proyeknya.
4. [Indeks standar](standards/standard.id.md) — topik yang relevan dengan perubahan
   Anda.
5. [Aturan dokumentasi](standards/documentation.id.md) — sebelum mengubah dokumen
   atau OpenAPI pertama Anda.

---

## Aturan dokumentasi secara ringkas

Aturan lengkapnya ada di [documentation.md](standards/documentation.id.md). Versi
singkatnya:

1. **Satu fakta, satu tempat.** Kontrak API → `api/openapi.yaml`. Aturan produk →
   `specification.md`. Arsitektur → `docs/`. Aturan kode → `docs/standards/`.
   Keanehan kode → komentar di samping kode.
2. **Bahasa Inggris adalah acuan.** Setiap file punya kembaran Bahasa Indonesia
   (`foo.id.md`) dengan heading, tabel, dan urutan yang sama — terjemahan, bukan
   ringkasan.
3. **Satu commit dengan kodenya.** Perubahan perilaku memperbarui OpenAPI dan
   dokumen terkait pada commit yang sama, sama seperti test.
4. **Tautkan, jangan salin.** Dokumen menunjuk kode lewat nama file; komentar kode
   menunjuk path dokumen. Jangan pernah menduplikasi tabel di dua file.
5. **Setiap dokumen baru ditautkan** dari README ini atau dari indeks standar.
   Dokumen tanpa tautan adalah orphan dan tidak akan ditemukan.

---

## Menjaga dokumentasi tetap benar

| Perubahan | Perbarui pada PR yang sama |
|---|---|
| Endpoint baru atau request/response berubah | `api/openapi.yaml` + dokumen fitur bila ada |
| Direktori, lapisan, atau paket baru | [architecture.md](architecture.id.md) |
| Aturan baru yang disepakati tim | File `docs/standards/*.md` terkait + indeks [standard.md](standards/standard.id.md) |
| Target Make atau tool baru | [development.md](development.id.md) |
| Konvensi branch/commit/rilis baru | [git_workflow.md](git_workflow.id.md) |

Dokumentasi yang basi adalah bug. Jika dokumen bertentangan dengan kode, perbaiki
atau buka task — jangan biarkan dua versi hidup berdampingan.

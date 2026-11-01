# Dokumentasi

[English](documentation.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Tujuan

Setiap pengetahuan punya **satu tempat**. Pembaca menemukannya tanpa mencari, dan
penulis tidak perlu memperbarui fakta yang sama di dua tempat.

> **Patokan:** bila kontrak OpenAPI atau kode sudah mengatakannya, jangan diulang di
> dokumen — tautkan saja.

---

## Di Mana Pengetahuan Disimpan

| Tempat | Isinya | Bukan untuk |
|---|---|---|
| **`api/openapi.yaml`** | Kontrak API: path, parameter, body, respons, status code, kode error | Arsitektur, histori, alasan bisnis |
| **`docs/specification.md`** | Kontrak produk: entity, aturan field, formula, layar, keputusan bisnis | Detail implementasi |
| **Komentar kode** | *Mengapa* dari satu query, aturan, atau keanehan; `TODO`/`FIXME` dengan pemilik dan tiket | Gambaran fitur, aturan yang disalin |
| **`docs/architecture.md`** | Struktur, lapisan, aturan dependensi, startup, siklus request | Detail endpoint |
| **`docs/development.md`** | Setup, konfigurasi, target Make, sqlc, migrasi, tool | Aturan kode |
| **`docs/error_handling.md`** | Taksonomi error, problem details, pembungkusan, logging | Daftar error per endpoint |
| **`docs/git_workflow.md`** | Branch, commit, PR, rilis | Gaya kode |
| **`docs/standards/*.md`** | Cara menulis kode (penamaan, struct, list, test) | Aturan bisnis sebuah fitur |
| **`docs/features/<domain>.md`** | Aturan, alur, dan keputusan satu domain yang tidak jelas dari kode | Kontrak API (itu OpenAPI) |
| **README (root repo)** | Layanan ini apa, cara menjalankannya dalam tiga perintah, tautan ke `docs/` | Detail mendalam |

Bila sebuah fakta cocok di dua baris, simpan di baris yang lebih spesifik dan baris
lain menautkannya.

---

## Tata Letak Folder

```
docs/
├── README.md                 ← pintu masuk dan peta dokumentasi
├── README.id.md
├── specification.md          ← kontrak produk (milik product owner)
├── architecture.md           ← lintas topik: struktur dan alur
├── development.md            ← lintas topik: cara build dan run
├── error_handling.md         ← lintas topik: error dari ujung ke ujung
├── git_workflow.md           ← lintas topik: branch, commit, rilis
├── standards/
│   ├── standard.md           ← indeks standar
│   ├── api_naming.md
│   ├── method_naming.md
│   ├── struct.md
│   ├── service_repo.md
│   ├── http_status.md
│   ├── list_api.md
│   ├── unit_test.md
│   └── documentation.md      ← file ini
└── features/
    └── <domain>.md           ← hanya bila sebuah domain membutuhkannya

api/
└── openapi.yaml              ← satu-satunya kontrak API
```

Aturan:

- Nama file dan folder: huruf kecil, `snake_case` untuk topik standar
  (`api_naming.md`), `kebab-case` untuk dokumen fitur (`daily-records.md`).
- Satu topik per file. Bila file perlu lebih dari ~250 baris, kemungkinan itu dua
  topik — pisahkan dan tautkan keduanya dari indeks.
- Setiap file ditautkan dari `docs/README.md` atau `standards/standard.md`. File tanpa
  tautan tidak terlihat.
- `specification.md` dimiliki di luar dokumentasi engineering: tautkan, jangan
  menulisnya ulang agar sesuai kode. Bila keduanya berbeda, itu bug yang harus
  diangkat.

---

## Contract-First: `api/openapi.yaml`

File OpenAPI ditulis **sebelum** handler dan direview bersama pihak yang memakai API.

Alur kerja:

1. Tambahkan atau ubah path, schema, dan respons di `api/openapi.yaml`.
2. Review diff-nya — inilah review kontrak.
3. Implementasikan handler sesuai kontrak.
4. `make openapi` meregenerasi tipe/stub; file hasil generate di-commit.

Aturan:

- **Tanpa anotasi di kode Go.** Tanpa komentar swaggo, tanpa `@Summary`, tanpa
  `@Router`. YAML adalah sumber kebenaran; handler tidak pernah membawa teks
  kontrak.
- Setiap operasi punya `operationId`, `summary`, `tags`, dan minimal respons error
  yang benar-benar dapat dikembalikan.
- Schema dinamai sesuai konsep DTO (`Sale`, `CreateSaleRequest`, `Problem`), bukan
  blob inline.
- Satu schema `Problem` bersama untuk semua error — tidak pernah salinan per
  endpoint.
- List terkelola memakai schema `PageMeta` bersama.
- `deprecated: true` sebelum menghapus apa pun
  ([HTTP Status](http_status.id.md)).

```yaml
paths:
  /sales/{saleId}:
    get:
      operationId: getSale
      summary: Get one sale
      tags: [Sales]
      parameters:
        - name: saleId
          in: path
          required: true
          schema: { type: string, format: uuid }
      responses:
        "200":
          description: The sale
          content:
            application/json:
              schema:
                type: object
                properties:
                  data: { $ref: '#/components/schemas/Sale' }
        "401": { $ref: '#/components/responses/Unauthorized' }
        "404": { $ref: '#/components/responses/NotFound' }
        "500": { $ref: '#/components/responses/Internal' }
```

---

## Jenis Dokumen

### 1. Standar (aturan)

Aturan yang diterapkan tim saat menulis kode. Berada di `docs/standards/`.

```markdown
# <Topik>

[English](<topic>.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Aturan Inti

Satu paragraf: aturannya dan mengapa aturan itu ada.

---

## Contoh

### Benar
### Hindari

---

## Referensi Cepat

| Aturan | Benar | Hindari |
|---|---|---|
```

### 2. Arsitektur (penjelasan)

Bagaimana sistem disusun: lapisan, direktori, alur, startup. Satu file, diperbarui
setiap kali struktur berubah.

### 3. Development (panduan)

Langkah yang diikuti orang: instal, konfigurasi, jalankan, migrasi, test,
troubleshooting. Instruksi berurutan, perintah siap salin-tempel.

### 4. Dokumen fitur / domain (referensi)

Pengetahuan yang tidak jelas dari satu domain. Tulis **hanya** bila minimal satu
benar:

- domain punya aturan yang tidak dapat diungkapkan kode (state machine, kuota, model
  akuntansi);
- terintegrasi dengan sistem eksternal atau job asinkron (import, export);
- ada penyimpangan dari desain yang jelas, dan itu disepakati serta perlu dicatat.

CRUD biasa cukup entri OpenAPI, tidak lebih.

```markdown
# <Domain>

Satu paragraf: cakupannya, layar yang memanggilnya, apa yang sudah ada.
Kode: `internal/handlers/<domain>_handler.go` dan file sejenisnya.
Kontrak: `api/openapi.yaml#/paths/~1<domain>`.

## Aturan
Aturan bisnis beserta alasannya, satu per baris (R-<DOMAIN>-001 …).

## Alur
Alur tidak sepele: langkah, keputusan, penanganan kegagalan.

## Endpoint
Tabel: method + path → handler → catatan. Detail tetap di OpenAPI.

## Keputusan
Apa yang dipilih, apa yang ditolak, dan mengapa (tanggal).

## Pertanyaan terbuka
Satu baris per pertanyaan, dengan pemiliknya.
```

### 5. ADR (catatan keputusan)

Opsional. Dipakai untuk keputusan yang akan dipertanyakan kemudian: dependensi baru,
pilihan penyimpanan, kontrak publik. Satu file per keputusan di
`docs/decisions/ADR-NNN-<slug>.md`, berisi konteks, keputusan, konsekuensi, status.

---

## Kembaran Inggris dan Indonesia

Setiap dokumen punya dua file:

- `foo.md` — Bahasa Inggris, **acuan**.
- `foo.id.md` — Bahasa Indonesia, heading, tabel, dan urutan yang sama.

Aturan:

1. Edit file Inggris lebih dulu, lalu kembarannya **pada PR yang sama**.
2. File Indonesia adalah **terjemahan**, bukan ringkasan. Urutan bagian dan anchor
   harus sama agar pembaca dapat berpindah di tengah dokumen.
3. Blok kode, path, perintah, dan identifier tetap Bahasa Inggris di kedua file.
4. Pengalih bahasa di bawah judul:

   ```markdown
   **English** · [Bahasa Indonesia](foo.id.md)   <!-- di foo.md -->
   [English](foo.md) · **Bahasa Indonesia**       <!-- di foo.id.md -->
   ```

5. Tautan di dalam `.id.md` mengarah ke `.id.md` bila kembarannya ada.
6. Kembaran yang hilang adalah penghalang review; kembaran yang basi adalah bug.
7. `specification.md` adalah pengecualian: itu dokumen produk dan tidak punya
   kembaran.

Terminologi — pertahankan kata berikut dalam Bahasa Inggris di dalam teks Indonesia:
handler, service, store, endpoint, request, response, commit, branch, pull request,
merge, deploy, migration, payload, farm, journal.

| Inggris | Bahasa Indonesia |
|---|---|
| rule | aturan |
| layer | lapisan |
| query | kueri |
| pagination | paginasi |
| error | galat (atau "error" saat menamai kode) |
| test | pengujian |
| setup | penyiapan |
| example | contoh |

---

## Komentar Kode

Tulis komentar ketika kode tidak dapat mengatakannya:

```go
// Sales are soft-deleted: deleted_at is set instead of removing the row, so the
// auto-generated journal entry keeps a valid foreign key.
```

Jangan menulis:

```go
// Get sale by id.
func (s *saleService) GetSale(...)
```

Penanda:

```go
// TODO(<owner>): replace the N+1 query with a single join. TICKET-123
// FIXME(<owner>): the legacy importer writes local dates; normalise before v2. TICKET-456
```

Penanda tanpa pemilik atau tiket adalah kebisingan.

---

## Aturan

1. **Satu commit dengan kodenya.** Perubahan perilaku memperbarui `api/openapi.yaml`
   dan dokumen terkait pada commit yang sama — seperti test.
2. **Catat keputusan, bukan dugaan.** Penyimpangan didokumentasikan setelah
   disepakati, dengan tanggal.
3. **Tautkan, jangan salin.** Dokumen menunjuk kode lewat nama file; komentar kode
   menunjuk path dokumen. Jangan pernah menduplikasi tabel.
4. **Setiap dokumen baru ditautkan** dari `docs/README.md` atau indeks standar.
5. **Bahasa Inggris adalah acuan, kembaran Indonesia tetap sinkron.**
6. **Ringkas.** Tabel pendek lebih baik daripada prosa panjang. Hapus yang sudah
   tidak benar.
7. **Tanpa secret, data pelanggan, atau URL internal** di dokumen.
8. **Uang didokumentasikan sebagai decimal.** Setiap contoh menampilkan nominal
   sebagai number JSON (`264000`, `2200.5`), bukan string berkutip — sesuai
   `shopspring/decimal` dengan `MarshalJSONWithoutQuotes`.

---

## Checklist

- [ ] `api/openapi.yaml` diperbarui; `make openapi` dijalankan dan file hasil
      generate di-commit
- [ ] Dokumen terkait diperbarui (arsitektur, standar, fitur) — atau dibuat baru bila
      kriteria di atas terpenuhi
- [ ] Kembaran `.id.md` diperbarui dengan struktur dan heading yang sama
- [ ] Dokumen baru ditautkan dari `docs/README.md` / `standards/standard.id.md`
- [ ] Komentar kode yang menunjuk dokumen memakai path terkini
- [ ] Tidak ada fakta terduplikasi: setiap tabel hanya ada di satu file
- [ ] Markdown ter-render: code fence tertutup, tabel rapi, tautan berfungsi

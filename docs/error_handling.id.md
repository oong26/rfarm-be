# Error Handling

[English](error_handling.md) · **Bahasa Indonesia**

[← Kembali ke Dokumentasi](README.id.md)

---

## Prinsip

1. **Error adalah bagian dari kontrak API.** Ia membawa status, kode yang stabil, dan
   pesan yang aman untuk client.
2. **Buat error di tempat pengetahuannya.** Store tahu baris tidak ada, service tahu
   state salah, handler tahu field tidak valid.
3. **Catat atau kembalikan — jangan keduanya.** Error yang dikembalikan dicatat
   sekali, di batas.
4. **Jangan bocorkan internal.** Tanpa SQL, error driver, stack trace, atau ID farm
   lain di body respons.
5. **Bungkus dengan konteks, bukan pengulangan.** `%w` menjaga rantai; pesan
   menambahkan yang belum ada.
6. **Kode berbahasa Inggris dan stabil; pesan berbahasa Indonesia.** Frontend
   bercabang pada kode, pengguna membaca pesan.

---

## Taksonomi

| Jenis | Dibuat oleh | Status | `code` |
|---|---|---|---|
| Request tidak dapat di-parse | Handler (parse) | 400 | `bad_request` |
| Query rusak | Handler / service | 400 | `invalid_query` |
| Token hilang/tidak valid | Middleware | 401 | `unauthorized` |
| Refresh token dipakai ulang, kedaluwarsa, atau dicabut | Service / `SessionStore` | 401 | `unauthorized` |
| Role tidak diizinkan | Middleware | 403 | `forbidden` |
| Baris hilang, soft-deleted, farm lain | Store | 404 | `not_found` |
| Nilai unik duplikat | Store | 409 | `conflict` |
| Delete terhalang FK | Store | 409 | `conflict` |
| Transisi state tidak valid | Service | 409 | `conflict` |
| `If-Match` versi mismatch | Service atau store | 412 | `version_conflict` |
| `If-Match` hilang | Handler | 428 | `precondition_required` |
| Validasi field / lintas field | Handler atau service | 422 | `validation_failed` |
| Endpoint dihapus / tautan sekali pakai | Handler | 410 | `gone` |
| Rate limit | Middleware | 429 | `too_many_requests` |
| Kegagalan tak terduga | Mana pun | 500 | `internal` |
| Dependensi tidak tersedia | Client, `SessionStore` | 503 | `service_unavailable` |
| Dependensi timeout | Client, `SessionStore` | 504 | `gateway_timeout` |

Detail pemetaan: [HTTP Status](standards/http_status.id.md).

---

## Tipe Error

`pkg/response/app_error.go`.

```go
type AppError struct {
    Status  int     // status HTTP
    Code    string  // kode snake_case stabil, aman untuk logika client
    Title   string  // ringkasan singkat Bahasa Inggris, tampil di problem.title
    Message string  // Bahasa Indonesia, untuk pengguna, tanpa internal
    Fields  []Field // error field opsional
    cause   error   // hanya internal — tidak pernah diserialisasi
}

type Field struct {
    Field   string `json:"field"`
    Code    string `json:"code"`
    Message string `json:"message"`
}

func (e *AppError) Error() string { return e.Code + ": " + e.Message }
func (e *AppError) Unwrap() error { return e.cause }
```

`pkg/response/problem.go` memegang bentuk wire. `Message` menjadi `detail`, sehingga
error tanpa detail field tetap membawa kalimat Bahasa Indonesia:

```go
type Problem struct {
    Type      string  `json:"type"`
    Title     string  `json:"title"`
    Status    int     `json:"status"`
    Code      string  `json:"code"`
    Detail    string  `json:"detail,omitempty"`
    RequestID string  `json:"requestId"`
    Errors    []Field `json:"errors,omitempty"`
}
```

```go
func BadRequest(msg string) *AppError
func InvalidQuery(msg string) *AppError
func Unauthorized(msg string) *AppError
func Forbidden(msg string) *AppError
func NotFound(resource string) *AppError            // → "Data <resource> tidak ditemukan."
func Conflict(msg string) *AppError
func Gone(msg string) *AppError
func PreconditionFailed(msg string) *AppError        // 412, code version_conflict
func PreconditionRequired() *AppError                // 428, If-Match hilang
func UnsupportedMedia(msg string) *AppError
func Unprocessable(fields ...Field) *AppError        // 422, code validation_failed
func TooManyRequests(msg string) *AppError
func Internal(cause error) *AppError                 // pesan selalu generik
func ServiceUnavailable(msg string) *AppError
func GatewayTimeout(msg string) *AppError
```

Aturan:

- Konstruktor mengembalikan `*AppError`; service dan store mengembalikannya sebagai
  `error`.
- `Internal` menyimpan cause untuk logging dan tidak pernah mengeksposnya.
- `Code` tidak pernah berubah setelah client bergantung padanya. Menambah kode boleh;
  mengganti nama adalah breaking change.
- `Fields` terstruktur dan terbatas, tidak pernah `error`, tidak pernah map berisi
  nilai mentah.
- `Message` ditulis Bahasa Indonesia; `Title` dan `code` tetap Bahasa Inggris.

---

## Di Mana Error Dibuat

| Lapisan | Membuat |
|---|---|
| **Handler** | `BadRequest`, `InvalidQuery`, `PreconditionRequired`, `UnsupportedMedia`, validasi field lewat `utils.ParseAndValidate` |
| **Service** | `Forbidden`, `Conflict`, `Unprocessable` (aturan lintas field), `PreconditionFailed`, `Internal` untuk invariant yang seharusnya mustahil |
| **Store** | `NotFound` dari `GetByID`, `Conflict` dari pelanggaran unique/FK, `PreconditionFailed` dari status CTE atomik |
| **Session store** | `Unauthorized` dari refresh token kedaluwarsa/dipakai ulang; `ServiceUnavailable` saat Redis tidak terjangkau |
| **Client** | `ServiceUnavailable`, `GatewayTimeout`, error transport terbungkus |
| **Middleware** | `Unauthorized`, `Forbidden`, `TooManyRequests` |

Handler tidak pernah memeriksa error untuk memilih kode. Ia memanggil
`response.Fail` lalu selesai.

---

## Pembungkusan

```go
// store — tambahkan konteks operasi, pertahankan rantai
if err := r.pool.QueryRow(ctx, q, id, farmID).Scan(&s); err != nil {
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, response.NotFound("penjualan")
    }
    return nil, fmt.Errorf("stores.GetSaleByID: %w", err)
}
```

- Pakai `%w`, jangan `%v`, ketika pemanggil mungkin perlu `errors.Is`/`errors.As`.
- Awali dengan `package.Method` — itu hal pertama yang terlihat di baris log.
- Pesan huruf kecil, tanpa titik di akhir, tanpa prefix `error:`.
- Jangan membungkus `*AppError` di dalam `*AppError` lain; kembalikan apa adanya.

```go
// ✗ pemetaan ganda — menyembunyikan 404 atau 412 sebagai 500
if err != nil {
    return response.Internal(err)
}

// ✓ teruskan, bungkus hanya error tak dikenal
if err != nil {
    return err
}
```

---

## Batas Handler

```go
func (h *SaleHandler) Update(w http.ResponseWriter, r *http.Request) {
    var req dtos.UpdateSaleRequest
    if err := utils.ParseAndValidate(r, &req); err != nil {
        response.Fail(w, r, err)
        return
    }

    version, err := utils.IfMatchVersion(r)
    if err != nil {
        response.Fail(w, r, err)   // 428 bila header hilang
        return
    }

    err = h.svc.UpdateSale(r.Context(), dtos.UpdateSaleInput{ /* … Version: version */ })
    if err != nil {
        response.Fail(w, r, err)
        return
    }

    response.NoContent(w)
}
```

```go
// pkg/response/fail.go
func Fail(w http.ResponseWriter, r *http.Request, err error) {
    var appErr *AppError
    if !errors.As(err, &appErr) {
        appErr = Internal(err)          // tak dikenal → 500, cause disimpan untuk log
    }

    if appErr.Status >= http.StatusInternalServerError {
        LoggerFrom(r.Context()).Error("request failed",
            "request_id", RequestIDFrom(r.Context()),
            "method", r.Method,
            "path", r.URL.Path,
            "err", appErr.Unwrap(),
        )
    }

    writeProblem(w, r, appErr)
}
```

`writeProblem` adalah satu-satunya fungsi yang menyerialisasi error. Ia mengisi
`Content-Type: application/problem+json` dan menulis body RFC 9457.

---

## Body Problem — RFC 9457

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/problem+json
X-Request-ID: 01J8Z3Q7…
```

```json
{
  "type": "https://rfarm.example/problems/validation",
  "title": "Validation failed",
  "status": 422,
  "code": "validation_failed",
  "requestId": "01J8Z3Q7…",
  "errors": [
    { "field": "rejects", "code": "exceeds_eggs",
      "message": "Telur afkir tidak boleh melebihi telur terkumpul." },
    { "field": "date", "code": "required",
      "message": "Tanggal wajib diisi." }
  ]
}
```

| Field | Aturan |
|---|---|
| `type` | `https://rfarm.example/problems/<code>` — stabil per kode |
| `title` | satu kalimat pendek Bahasa Inggris per kode |
| `status` | selalu sama dengan status HTTP |
| `code` | kode `snake_case` yang stabil |
| `detail` | `Message` Bahasa Indonesia; dihilangkan bila `errors[]` membawa teksnya |
| `requestId` | dari request ID; sertakan saat melaporkan bug |
| `errors[]` | hanya ada untuk error bergaya validasi |

---

## Validasi Field dan Lintas Field (422)

Error field memakai daftar kode stabil dari spesifikasi:

`required`, `min`, `max`, `invalid_enum`, `duplicate_date`, `exceeds_eggs`,
`exceeds_feed_given`, `journal_unbalanced`, `same_account`, `invalid_account`,
`next_before_date`, `version_conflict`.

```go
// service — aturan yang membutuhkan baris lain
if in.Rejects.GreaterThan(in.Eggs) {
    return response.Unprocessable(response.Field{
        Field:   "rejects",
        Code:    "exceeds_eggs",
        Message: "Telur afkir tidak boleh melebihi telur terkumpul.",
    })
}
```

```go
// dua field yang tidak dapat divalidasi sendiri-sendiri
if in.DebitAccountID == in.CreditAccountID {
    return response.Unprocessable(response.Field{
        Field:   "creditAccountId",
        Code:    "same_account",
        Message: "Akun debit dan kredit tidak boleh sama.",
    })
}
```

- `utils.ParseAndValidate` mengubah `validator.ValidationErrors` menjadi `[]Field`,
  memetakan `required`/`min`/`max`/`oneof` ke kode stabil.
- `field` adalah key JSON yang dikirim client (`pageSize`, bukan `page_size`).
- Jangan pernah mengembalikan `validator.ValidationErrors` atau string error Go.
- Body yang tidak dapat di-parse, atau content type salah, adalah `400` — bukan `422`.

---

## Error Database

Petakan error driver sekali, di helper store bersama.

```go
func mapDBError(err error, resource string) error {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "23505": // unique_violation
            return response.Conflict("Data " + resource + " dengan nilai tersebut sudah ada.")
        case "23503": // foreign_key_violation
            return response.Conflict("Data " + resource + " masih dipakai oleh data lain.")
        case "23502": // not_null_violation
            return response.Internal(err) // bug di kode kita, bukan kesalahan client
        case "22P02": // invalid_text_representation
            return response.BadRequest("Format ID tidak valid.")
        case "40001": // serialization_failure
            return response.Conflict("Data sedang diubah. Silakan coba lagi.")
        case "57014": // query_canceled (statement_timeout)
            return response.GatewayTimeout("Permintaan melebihi batas waktu.")
        }
    }
    return fmt.Errorf("stores: %w", err)
}
```

| PostgreSQL | Arti | Respons |
|---|---|---|
| `23505` | unique violation | `409 Conflict` |
| `23503` | FK violation | `409 Conflict` |
| `23502` | NOT NULL violation | `500 Internal` (bug kita) |
| `22P02` | sintaks input tidak valid (mis. uuid) | `400 Bad Request` |
| `40001` | serialization failure | coba ulang internal, jika tidak `409` |
| `57014` | statement timeout | `504 Gateway Timeout` |
| `pgx.ErrNoRows` | tidak ada baris | `404 Not Found` (hanya dari `GetByID`) |

`UPDATE`/`DELETE` biasa yang mengembalikan nol baris **bukan** error di tingkat store —
kembalikan count dan biarkan service memutuskan apakah itu `404` atau `412`
([Service & Store](standards/service_repo.id.md)).

---

## Redis dan Error Sesi

Sesi refresh hidup di Redis, sehingga endpoint auth memiliki dependensi kedua.

| Hasil Redis | Arti | Respons |
|---|---|---|
| key ditemukan, hash cocok | sesi hidup | lanjutkan |
| `redis.Nil` saat lookup | tidak ada sesi: kedaluwarsa, logout, atau tidak pernah ada | `401 Unauthorized` |
| hash token tidak dikenal tapi family masih hidup | token sudah dirotasi — anggap pencurian | cabut family, `401 Unauthorized` |
| `context.DeadlineExceeded` | Redis lebih lambat dari timeout yang dikonfigurasi | `504 Gateway Timeout` |
| error dial/read/write, `WRONGPASS`, `NOAUTH` | Redis tidak terjangkau atau salah konfigurasi | `503 Service Unavailable` |

```go
// stores — kegagalan transport tetap sebagai transport failure
if err := r.rdb.Get(ctx, sessionKey(hash)).Scan(&s); err != nil {
    if errors.Is(err, redis.Nil) {
        return nil, response.Unauthorized("Sesi sudah berakhir. Silakan masuk kembali.")
    }
    return nil, fmt.Errorf("stores.GetSession: %w", err) // dipetakan ke 503 di batas
}
```

- `redis.Nil` adalah **cache miss**, bukan error — jangan pernah bungkus sebagai `Internal`.
- Redis mati bukan degradasi cache: login, refresh, logout, dan `/readyz` gagal dengan
  `503`. Tidak ada fallback in-memory.
- Jangan pernah menaruh token mentah di error, baris log, atau body problem.
- Refresh token yang dipakai ulang adalah event keamanan: log di level `warn` dengan
  `user_id` dan ID family, cabut family sebelum mengembalikan `401`.

---

## 409 vs 412

Keduanya berarti "request valid, tetapi data menolak". Bedanya adalah apa yang harus
dilakukan client selanjutnya.

| | 409 Conflict | 412 Precondition Failed |
|---|---|---|
| Arti | aturan keunikan atau state dilanggar | baris berubah sejak client membacanya |
| Penyebab | tanggal duplikat, FK masih direferensikan, transisi tidak valid | `If-Match` ≠ `version` saat ini |
| Tindakan client | ubah payload | baca ulang, gabungkan, coba lagi dengan versi baru |
| Kode | `conflict` | `version_conflict` |

```go
// 409 — aturannya tentang data, bukan tentang konkurensi
return response.Conflict("Catatan harian untuk tanggal tersebut sudah ada.")

// 412 — snapshot milik client sudah basi
return response.PreconditionFailed("Data sudah diubah pengguna lain. Muat ulang lalu coba lagi.")
```

Jangan pernah mengubah version mismatch menjadi `409`: frontend menampilkan pesan
berbeda dan mengambil tindakan berbeda.

---

## Logging

| Level | Dipakai untuk |
|---|---|
| `debug` | nilai berguna saat pengembangan; tanpa data pribadi |
| `info` | startup, shutdown, background job selesai |
| `warn` | keanehan yang pulih, retry, endpoint deprecated dipakai |
| `error` | 5xx, background job gagal, inkonsistensi data |

Aturan:

- **Catat sekali.** Handler mengembalikan; `response.Fail` (atau middleware) mencatat.
  Service tidak boleh mencatat error yang juga dikembalikannya.
- Setiap baris log menyertakan `request_id`, `user_id`, dan `farm_id` bila diketahui.
- Catat cause (`appErr.Unwrap()`), jangan hanya `appErr.Message`.
- 4xx bukan log level error — itu kesalahan client, terlihat di access log.
- Jangan pernah mencatat token, password, body penuh, atau data pribadi.

---

## Panic

`middleware.Recover` berada di atas setiap route.

```go
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    log.Error("panic recovered",
                        "request_id", RequestIDFrom(r.Context()),
                        "panic", rec,
                        "stack", string(debug.Stack()),
                    )
                    response.Fail(w, r, response.Internal(fmt.Errorf("panic: %v", rec)))
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

- Panic hanya untuk kesalahan programmer (invariant nil, state mustahil), tidak pernah
  untuk hasil bisnis yang diperkirakan.
- Client selalu mendapat body problem 500 standar.
- Stack masuk ke log, tidak pernah ke respons.
- Middleware recover harus terdaftar sebelum apa pun yang dapat panic, termasuk access
  log.

---

## Error Asinkron dan Background

- Goroutine harus memulihkan panic-nya sendiri; crash di sana mematikan proses.
- Lampirkan `request_id` asal (atau `job_id` baru) di setiap baris log.
- Jangan pernah menelan error dengan `_ =`; tangani atau catat dengan konteks.
- Background job yang gagal harus meninggalkan state yang terlihat oleh operator
  (kolom status, penghitung retry), bukan hanya baris log.

```go
go func() {
    defer func() {
        if rec := recover(); rec != nil {
            log.Error("export job panicked", "panic", rec, "job_id", jobID)
        }
    }()
    if err := exporter.Run(ctx, jobID); err != nil {
        log.Error("export job failed", "job_id", jobID, "err", err)
    }
}()
```

---

## Anti-Pola

| Anti-pola | Mengapa merugikan | Lakukan ini |
|---|---|---|
| `if err != nil { return err }` untuk error tak dikenal, langsung ke client | Membocorkan detail SQL/driver | Bungkus, biarkan batas memetakan |
| `response.Internal(err)` di sekitar error bertipe | Mengubah 404/409/412 menjadi 500 | Kembalikan error bertipe |
| Membandingkan string error | Rusak saat pesan berubah | `errors.Is` / error bertipe |
| Logging di service dan handler | Kebisingan ganda, level salah | Catat sekali di batas |
| Merakit body problem manual | Menyimpang dari kontrak | `response.Fail` |
| Mengembalikan version mismatch sebagai `409` | Perilaku client salah | `412` + `version_conflict` |
| Pesan untuk pengguna dalam Bahasa Inggris | Aplikasi untuk pengguna Indonesia | `message` Indonesia, `code` Inggris |
| `panic` untuk validasi atau state | Tanpa kontrak, mematikan request | Error bertipe |
| Kode error baru per pesan | Kontrak tidak stabil | Satu kode per kelas |
| Membungkus `redis.Nil` sebagai `Internal` | Mengubah pengguna yang logout menjadi `500` | `Unauthorized` |
| Fallback ke memori saat Redis mati | Dua sumber kebenaran, kehilangan sesi yang tidak terdeteksi | Gagal dengan `503` |

---

## Referensi Cepat

| Pertanyaan | Jawaban |
|---|---|
| Di mana membuat `NotFound`? | Store, ketika baris tidak ada |
| Di mana membuat `Conflict`? | Store untuk keunikan/FK, service untuk state |
| Di mana membuat `412`? | Di tempat `version` dibandingkan |
| Di mana membuat `422`? | Handler (tag) atau service (lintas field) |
| Di mana membuat `401` untuk sesi kedaluwarsa? | `SessionStore` / auth service, pada `redis.Nil` atau reuse |
| Di mana membuat `503`? | Di tempat error transport Redis muncul, lewat `ServiceUnavailable` |
| Di mana error dipetakan ke JSON? | `response.Fail` |
| Di mana error dicatat? | Sekali, di batas, untuk 5xx |
| Bagaimana membungkus? | `fmt.Errorf("stores.X: %w", err)` |
| Apa yang dilihat client? | `type`, `title`, `status`, `code`, `detail`, `requestId`, `errors[]` |
| Apa yang tidak pernah dilihat client? | SQL, teks driver, stack trace, cause |

# Standar Status Code HTTP

[English](http_status.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Prinsip

Kembalikan kode paling spesifik yang menggambarkan hasilnya. `400` bukan tempat
sampah: status code adalah bagian dari kontrak API dan memberi tahu client *mengapa*
request gagal tanpa harus mem-parsing pesan.

Kode dan pesan punya audiens berbeda: **kode berbahasa Inggris dan stabil**, pesan
berbahasa Indonesia dan boleh diperhalus untuk pengguna.

---

## Referensi Kode

### 2xx — Sukses

| Kode | Nama | Kapan dipakai |
|---|---|---|
| `200` | OK | Read atau update berhasil dan ada body. |
| `201` | Created | Resource dibuat; kembalikan resource (atau minimal ID-nya) di body. |
| `204` | No Content | Sukses tanpa body: update, delete, perubahan state. |

### 4xx — Error Client

| Kode | Nama | Kapan dipakai |
|---|---|---|
| `400` | Bad Request | Tidak valid secara struktur: JSON tidak dapat di-parse, tipe salah, query rusak, nilai mustahil yang tidak dapat diungkapkan validasi. |
| `401` | Unauthorized | Tidak ada access token yang valid: hilang, kedaluwarsa, rusak. |
| `403` | Forbidden | Token valid, tetapi role (`viewer` / `editor` / `owner`) tidak mengizinkan aksi. |
| `404` | Not Found | Resource tidak ada, sudah soft-deleted, atau milik farm lain. |
| `409` | Conflict | Request valid tetapi melanggar aturan **keunikan** yang ada di data: tanggal `daily_records` sudah dipakai, referensi jurnal sudah digunakan. |
| `410` | Gone | Endpoint dihapus permanen, atau unduhan sekali pakai sudah dipakai. |
| `412` | Precondition Failed | Versi `If-Match` tidak cocok dengan baris — orang lain menyimpan lebih dulu. |
| `415` | Unsupported Media Type | Body bukan `application/json`. |
| `422` | Unprocessable Entity | Ter-parse, tetapi validasi field gagal: required, min/max, enum, aturan lintas field (`rejects > eggs`). |
| `429` | Too Many Requests | Rate limit terlampaui. |

### 5xx — Error Server

| Kode | Nama | Kapan dipakai |
|---|---|---|
| `500` | Internal Server Error | Kegagalan tak terduga: bug, nil dereference, kasus tak tertangani. Tidak pernah untuk aturan bisnis yang dapat diprediksi. |
| `503` | Service Unavailable | Dependensi (database, API eksternal) tidak dapat dijangkau atau jenuh — coba lagi nanti. |
| `504` | Gateway Timeout | Panggilan hilir diterima tetapi tidak selesai dalam timeout kita. |

---

## 409 vs 412 vs 422 — Panduan Keputusan

```
Apakah body tidak dapat di-parse atau salah secara struktur?
  └─ ya → 400 Bad Request

Apakah ter-parse tetapi gagal validasi field atau lintas field?
  └─ ya → 422 Unprocessable Entity

Apakah request valid dan data akan menerimanya, kecuali versi barisnya?
  └─ ya → 412 Precondition Failed

Apakah request valid tetapi aturan keunikan sudah berlaku?
  ├─ daily record untuk tanggal itu sudah ada  → 409 Conflict
  ├─ referensi jurnal sudah dipakai            → 409 Conflict
  └─ menghapus baris yang masih direferensikan → 409 Conflict
```

**Patokan:** bila request yang sama bisa berhasil setelah data orang lain berubah
(mereka menyimpan, baris dependen dihapus), itu `409` atau `412`. Bila tidak ada
perubahan state yang bisa membuatnya valid, itu `400` atau `422`.

---

## Bentuk Respons Sukses

```json
// 200 — satu resource
{ "data": { "id": "0192…", "date": "2026-10-31", "qty": 120, "amount": 264000 } }

// 200 — list
{
  "data": [ { "id": "0192…" } ],
  "meta": { "page": 1, "pageSize": 25, "total": 137 }
}

// 201 — created
{ "data": { "id": "0192…", "version": 1 } }
```

- Jangan pernah mengembalikan array telanjang di level atas.
- Jangan mengembalikan `null` untuk list; kembalikan array kosong.
- `meta` wajib pada endpoint list — lihat [List API](list_api.id.md).
- Update yang mengembalikan record juga mengisi `ETag: "<version>"`.

---

## Bentuk Respons Error — RFC 9457

Content type: `application/problem+json`.

```json
{
  "type": "https://rfarm.example/problems/validation",
  "title": "Validation failed",
  "status": 422,
  "code": "validation_failed",
  "requestId": "01J8Z3…",
  "errors": [
    { "field": "rejects", "code": "exceeds_eggs", "message": "Telur afkir tidak boleh melebihi telur terkumpul." }
  ]
}
```

| Field | Aturan |
|---|---|
| `type` | `https://rfarm.example/problems/<code>` — stabil dan terdokumentasi. |
| `title` | Ringkasan singkat Bahasa Inggris, satu per kode. |
| `status` | Harus sama dengan status HTTP. |
| `code` | Kode `snake_case` Bahasa Inggris yang stabil, boleh dipakai bercabang oleh frontend. |
| `detail` | Pesan Bahasa Indonesia untuk error tanpa `errors[]`; dihilangkan bila error field yang membawa pesan. |
| `requestId` | Dari `X-Request-ID`; sertakan saat melaporkan bug. |
| `errors[]` | Error field opsional: `field`, `code`, `message`. `field` sama dengan key JSON. |

`errors[].code` memakai daftar stabil dari spesifikasi: `required`, `min`, `max`,
`invalid_enum`, `duplicate_date`, `exceeds_eggs`, `exceeds_feed_given`,
`journal_unbalanced`, `same_account`, `invalid_account`, `next_before_date`,
`version_conflict`.

Kode berbahasa Inggris; pesan berbahasa Indonesia untuk pengguna. Jangan pernah
menaruh SQL, error driver, stack trace, atau ID farm lain di body.

---

## Pemetaan `pkg/response`

| Konstruktor | Status | `code` |
|---|---|---|
| `response.OK(w, data)` | 200 | — |
| `response.Created(w, data)` | 201 | — |
| `response.NoContent(w)` | 204 | — |
| `response.List(w, data, meta)` | 200 | — |
| `response.BadRequest(msg)` | 400 | `bad_request` |
| `response.Unauthorized(msg)` | 401 | `unauthorized` |
| `response.Forbidden(msg)` | 403 | `forbidden` |
| `response.NotFound(resource)` | 404 | `not_found` |
| `response.Conflict(msg)` | 409 | `conflict` |
| `response.Gone(msg)` | 410 | `gone` |
| `response.PreconditionFailed(msg)` | 412 | `version_conflict` |
| `response.UnsupportedMedia(msg)` | 415 | `unsupported_media_type` |
| `response.Unprocessable(msg, fields)` | 422 | `validation_failed` |
| `response.TooManyRequests(msg)` | 429 | `too_many_requests` |
| `response.Internal(err)` | 500 | `internal` |
| `response.ServiceUnavailable(msg)` | 503 | `service_unavailable` |
| `response.GatewayTimeout(msg)` | 504 | `gateway_timeout` |
| `response.Fail(w, r, err)` | dari error | dari error |

`response.Fail` adalah satu-satunya titik konversi. Handler tidak pernah merakit body
problem secara manual. Detail: [Error Handling](../error_handling.id.md).

---

## Pola Handler

```go
func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req dtos.CreateSaleRequest
    if err := utils.ParseAndValidate(r, &req); err != nil {
        response.Fail(w, r, err)   // 400 (tidak dapat di-parse) atau 422 (field tidak valid)
        return
    }

    sale, err := h.svc.CreateSale(r.Context(), in)
    if err != nil {
        response.Fail(w, r, err)   // 404 / 409 / 412 / 500 / …
        return
    }

    response.Created(w, sale)
}
```

Satu jalan keluar per hasil, tanpa `switch status` di handler, tanpa string error
yang dirakit di handler.

---

## Header Optimistic Locking

| Arah | Header | Aturan |
|---|---|---|
| Response (read/update) | `ETag: "3"` | `version` baris |
| Request (update/delete) | `If-Match: "3"` | wajib; hilang → `428 Precondition Required` |
| Sukses | `200` / `204` dengan `ETag` baru | versi dinaikkan oleh update |
| Mismatch | `412` + `code: version_conflict` | client harus membaca ulang dan mencoba lagi |

```go
w.Header().Set("ETag", fmt.Sprintf(`"%d"`, sale.Version))
```

---

## Header Idempotency

`POST` menerima `Idempotency-Key` opsional agar retry client tidak membuat baris kedua
(`specification.md` §9.1).

| Arah | Header | Aturan |
|---|---|---|
| Request (`POST`) | `Idempotency-Key: <uuid>` | opsional; dibuat sekali per create logis |
| Replay dalam 24 jam | key sama | mengembalikan status dan body asli; tanpa baris kedua |
| Key sama, payload berbeda | — | `409`, `code: conflict` |

- Key ter-scope ke farm dan endpoint, disimpan 24 jam.
- Hanya `POST` yang memakainya: `PUT` idempoten secara definisi dan `DELETE` dijaga
  `If-Match`.
- Header yang hilang bukan error — idempotency bersifat opt-in, bukan prasyarat.

---

## Baseline `responses` OpenAPI

Setiap endpoint mendeklarasikan minimal:

```yaml
responses:
  "401": { $ref: '#/components/responses/Unauthorized' }
  "403": { $ref: '#/components/responses/Forbidden' }
  "500": { $ref: '#/components/responses/Internal' }
```

Tambahkan hanya kode yang benar-benar dapat dicapai jalur itu:

| Kondisi di handler | Tambahkan |
|---|---|
| Mem-parsing dan memvalidasi body | `400`, `422` |
| Membaca resource berdasarkan ID | `404` |
| Menulis field dengan unique constraint | `409` |
| Update atau delete dengan `If-Match` | `412` |
| Menghapus baris yang punya dependen | `409` |
| Pengecekan role di luar autentikasi | `403` |
| Memanggil layanan eksternal | `503`, `504` |

Skema error adalah satu komponen bersama (`Problem`), bukan salinan per endpoint.

---

## Menonaktifkan Endpoint

Dua fase. Jangan pernah melewati Fase 1 — menghapus route tanpa pemberitahuan merusak
frontend secara diam-diam.

### Fase 1 — masih berfungsi, ditandai deprecated

Pertahankan respons 2xx normal dan tambahkan header:

```go
w.Header().Set("Deprecation", "true")
w.Header().Set("Sunset", "Sat, 31 Dec 2026 23:59:59 GMT")
w.Header().Set("Link", `</api/v1/sales>; rel="successor-version"`)
```

Di `api/openapi.yaml`: `deprecated: true` dan deskripsi yang menunjuk penggantinya.

### Fase 2 — dihapus sepenuhnya

Kembalikan `410 Gone`, bukan `404` — itu menandakan penghapusan yang disengaja dan
permanen. Biarkan route tetap terdaftar agar client mendapat jawaban jelas.

```go
r.Get("/old-sales", func(w http.ResponseWriter, r *http.Request) {
    response.Fail(w, r, response.Gone("GET /old-sales was removed; use GET /api/v1/sales"))
})
```

---

## Referensi Cepat

| Situasi | Kode |
|---|---|
| Read / update berhasil dengan body | 200 |
| Dibuat, mengembalikan resource | 201 |
| Update / delete / perubahan state berhasil | 204 |
| Tidak ada token valid | 401 |
| Token valid, role tidak cukup | 403 |
| Baris hilang, soft-deleted, atau milik farm lain | 404 |
| Nilai unik duplikat (tanggal harian, referensi jurnal) | 409 |
| `If-Match` versi mismatch | 412 |
| `If-Match` hilang pada endpoint yang mewajibkannya | 428 |
| Body ter-parse, field tidak valid | 422 |
| Body tidak dapat di-parse / mustahil secara struktur | 400 |
| Endpoint dihapus permanen | 410 |
| Kegagalan tak terduga | 500 |
| Dependensi tidak tersedia | 503 |
| Dependensi timeout | 504 |

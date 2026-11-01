# Penamaan & Nesting API

[English](api_naming.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Aturan Inti

URL menamai **resource**, method HTTP menamai **operasi**, dan struktur path
mencerminkan **kepemilikan** — bukan skema database.

```
GET    /api/v1/sales                list
POST   /api/v1/sales                create
GET    /api/v1/sales/{saleId}      detail
PUT    /api/v1/sales/{saleId}      update
DELETE /api/v1/sales/{saleId}      soft delete
```

---

## 1. Versioning

Setiap endpoint bisnis berada di bawah `/api/v1`.

```
/api/v1/daily-records
/api/v1/sales/{saleId}
```

- Endpoint infrastruktur di luar versi: `/healthz`, `/readyz`, `/metrics`.
  Kontraknya sendiri disajikan di `/api/v1/openapi.yaml`.
- Perubahan breaking berarti `/api/v2` — jangan pernah mengubah kontrak `/v1`
  secara diam-diam.
- Perubahan aditif (field opsional baru, endpoint baru) tetap di `/v1`.

---

## 2. Plural vs Singular

### Plural untuk resource (koleksi)

Resource adalah sesuatu yang bisa dibuat, didaftar, atau dimiliki.

```
GET  /api/v1/sales
GET  /api/v1/sales/{saleId}
GET  /api/v1/daily-records
GET  /api/v1/expenses
GET  /api/v1/journal-entries
```

### Singular untuk namespace modul

Namespace mengelompokkan endpoint terkait di bawah satu domain; namespace bukan
koleksi.

```
/api/v1/stock/movements
/api/v1/accounting/accounts
/imports               ← sebuah proses, bukan koleksi "import"
/backup                ← satu dokumen seluruh farm
```

### Cara membedakannya

Tanyakan: *"Apakah saya bisa membuat atau mendaftar salah satunya?"*

```
/sales       → ya, saya membuat dan mendaftar penjualan  → plural   ✓
/accounting  → tidak, accounting adalah label domain     → singular ✓
```

---

## 3. Singkatan Mengikuti Perannya

Tidak ada kasus khusus untuk singkatan — logika plural/singular yang sama berlaku.

| Istilah | Sebagai resource | Sebagai namespace |
|---|---|---|
| HDP (hen-day production) | `/hdp` (laporan terhitung) | `/reports/hdp` |
| COA (chart of accounts) | `/accounts` | `/accounting/accounts` |

Tulis kata lengkapnya di path kecuali bentuk pendek memang nama yang dipakai tim dan
UI.

---

## 4. CRUD Memakai Method HTTP, Bukan Nama Aksi

| Operasi | Benar | Hindari |
|---|---|---|
| List | `GET /sales` | `GET /get-sales` |
| Create | `POST /sales` | `POST /sales/create` |
| Detail | `GET /sales/{saleId}` | `GET /sales/detail/{saleId}` |
| Replace/update | `PUT /sales/{saleId}` | `PUT /sales/update/{saleId}` |
| Soft delete | `DELETE /sales/{saleId}` | `POST /sales/{saleId}/delete` |

`PUT` mengganti representasi yang dapat diedit. Client mengirim
`If-Match: "<version>"` untuk optimistic locking
([Service & Store](service_repo.id.md)).

---

## 5. Aksi Bisnis Boleh Memakai Verb

Bila operasi bukan CRUD biasa pada resource, verb aksi adalah pilihan yang benar.

```
POST /api/v1/imports                     mulai impor JSON legacy
POST /api/v1/sales/{saleId}/void         batalkan penjualan, baris tetap ada
POST /api/v1/journal-entries/{id}/post    posting jurnal manual
POST /api/v1/exports                      mulai job export asinkron
```

Verb dibenarkan ketika operasi:

- mengubah state dalam sebuah alur kerja (void, post, approve), atau
- memicu efek samping yang bukan resource itu sendiri (import, export, send).

Jangan mengarang verb untuk sesuatu yang sudah diungkapkan HTTP. `create`,
`update`, `delete`, `get` tidak pernah menjadi segmen path.

---

## 6. Nesting — Jaga Tetap Dangkal

### Kapan melakukan nesting

Lakukan nesting ketika parent benar-benar **membatasi atau memiliki** child.

```
/sales/{saleId}/lines                 ← line hanya ada di dalam sale
/accounts/{accountId}/journal-lines   ← line hanya bermakna per akun
```

### Maksimal tiga level resource

```
/sales/{saleId}/lines                          ← 2 level ✓
/journal-entries/{entryId}/lines               ← 2 level ✓
/farms/{farmId}/sales/{saleId}/lines          ← 4 level ✗ (farm ada di token)
```

Farm diidentifikasi oleh token, bukan URL — setiap request terautentikasi sudah
ter-scope. Jangan pernah menambahkan `/farms/{farmId}` ke path bisnis.

### Kapan meratakan

Jika hierarki hanya ada di skema, ratakan route dan gunakan query parameter.

```
❌ /farms/{farmId}/accounts/{accountId}/journal-entries
✓  /journal-entries?account_id=…&month=2026-10
```

---

## 7. Path Mengikuti Resource yang Dikembalikan

Namai path sesuai apa yang dikembalikan endpoint, dan kelompokkan dengan layar atau
alur yang memanggilnya.

```
GET /api/v1/reports/hdp                  ← mengembalikan laporan HDP
GET /api/v1/settings                     ← mengembalikan pengaturan farm
❌ GET /api/v1/farm/settings              ← resource-nya settings, farm adalah konteks
```

---

## 8. Role Tidak Ada di URL

Akses diputuskan oleh role di token (`viewer`, `editor`, `owner`) dan ditegakkan di
middleware — bukan oleh prefix path.

```
✅ DELETE /api/v1/sales/{saleId}        editor dan owner
✅ PUT    /api/v1/settings               hanya owner
❌ /admin/settings                       jangan mengodekan role
```

- Resource yang sama punya satu path untuk setiap role.
- Role tanpa akses mendapat `403` dari middleware, bukan `404`.
- Jangan membuat endpoint kedua hanya karena izinnya berbeda.

---

## 9. Filter dan Paginasi

| Urusan | Di mana | Contoh |
|---|---|---|
| Mengidentifikasi resource | Path | `/sales/{saleId}` |
| Membatasi ke parent | Path | `/sales/{saleId}/lines` |
| Rentang tanggal | Query | `?from=2026-10-01&to=2026-10-31` |
| Shortcut bulan | Query | `?month=2026-10` |
| Pencarian | Query | `?q=bondowoso` |
| Pengurutan | Query | `?sort=-date` (`-` di depan = menurun) |
| Paging | Query | `?page=1&pageSize=25` |
| Perubahan state bisnis | Path + verb | `/journal-entries/{id}/post` |

Jangan pernah menaruh filter di path sebagai pseudo-resource (`/sales/october`), dan
jangan menaruh identifier di query string bila itu mengidentifikasi resource itu
sendiri.

---

## 10. Satu Tujuan per Endpoint

Satu endpoint melakukan satu hal. Jangan mengubah perilaku dengan query flag;
berikan setiap tujuan route-nya sendiri.

```
❌ GET /sales?view=detail            ← dua payload di satu route
✓ GET /sales/{saleId}              ← detail
✓ GET /sales                        ← list

❌ GET /reports?type=hdp&export=true
✓ GET /reports/hdp
✓ POST /exports                     ← job export asinkron
```

---

## 11. Konvensi JSON

| Urusan | Aturan | Contoh |
|---|---|---|
| Nama field | `camelCase` | `pageSize`, `feedBagPrice`, `createdAt` |
| Uang / kuantitas | Number JSON, ≤ 2 desimal, `decimal` di server — bukan string berkutip | `2200.5` |
| Tanggal bisnis | `YYYY-MM-DD`, tanpa timezone | `"2026-10-31"` |
| Timestamp | ISO-8601 UTC | `"2026-10-31T04:12:00Z"` |
| ID | String UUID (UUIDv7) | `"0192f3…"` |
| Enum | `snake_case` Bahasa Inggris yang stabil | `"vaccine_medicine"` |
| List | selalu objek dengan `data` | `{ "data": [] }` |

---

## Referensi Cepat

| Aturan | Benar | Hindari |
|---|---|---|
| Prefix versi | `/api/v1/sales` | `/sales` |
| Koleksi resource | `/sales`, `/expenses` | `/sale`, `/expense` |
| Namespace modul | `/accounting/accounts` | `/accounting/account` |
| CRUD | `DELETE /sales/{id}` | `POST /sales/{id}/delete` |
| Aksi bisnis | `POST /journal-entries/{id}/post` | `POST /journal-entries/post/{id}` |
| Kedalaman nesting | ≤ 3 level | 4+ level mengikuti skema |
| Scope farm | token | `/farms/{farmId}/…` di path |
| Role | middleware + `403` | `/admin/...` |
| Resource yang dikembalikan | `/reports/hdp` | `/reports?type=hdp` |
| Satu tujuan | route terpisah | `?view=`, `?mode=` mengubah perilaku |
| Casing | path kebab-case, JSON camelCase | snake_case di path, PascalCase di JSON |
| Tanggal | `YYYY-MM-DD` | timestamp untuk tanggal bisnis |

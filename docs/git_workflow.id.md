# Git Workflow

[English](git_workflow.md) · **Bahasa Indonesia**

[← Kembali ke Dokumentasi](README.id.md)

---

## Branch

`main` selalu dapat di-deploy dan dilindungi. Semua pekerjaan dilakukan di branch
berumur pendek yang di-merge melalui pull request.

| Prefix | Dipakai untuk | Contoh |
|---|---|---|
| `feat/` | perilaku baru | `feat/sales-export` |
| `fix/` | perbaikan bug | `fix/duplicate-daily-record` |
| `refactor/` | tanpa perubahan perilaku | `refactor/sale-store-interface` |
| `docs/` | hanya dokumentasi | `docs/api-naming` |
| `chore/` | tooling, dependensi, config | `chore/update-go-1.25` |
| `test/` | hanya test | `test/sale-service-branches` |
| `hotfix/` | perbaikan produksi mendesak, dari tag | `hotfix/login-500` |

Aturan:

- Huruf kecil, kebab-case, 2–5 kata: `feat/sales-export`, bukan
  `feature/newFeatureForExport`.
- Satu branch = satu maksud. Bila perlu kata "dan" untuk menjelaskannya, pisahkan.
- Hapus branch setelah merge.
- Rebase ke `main` sebelum membuka PR; jangan merge `main` ke branch fitur untuk
  menyelesaikan konflik.

---

## Commit

[Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <summary>

<body — mengapa, bukan apa>

<footer — refs, breaking changes>
```

| Tipe | Dipakai untuk |
|---|---|
| `feat` | kemampuan baru yang terlihat client |
| `fix` | perbaikan bug |
| `refactor` | perubahan kode tanpa perubahan perilaku |
| `perf` | peningkatan performa |
| `test` | hanya menambah atau memperbaiki test |
| `docs` | hanya dokumentasi |
| `build` | build system, dependensi, Makefile |
| `ci` | konfigurasi CI |
| `chore` | pemeliharaan lain |
| `revert` | membatalkan commit sebelumnya |

`scope` adalah domain atau area: `sales`, `auth`, `stores`, `docs`, `deps`.

```text
feat(sales): add paginated list endpoint

Add GET /api/v1/sales with page/pageSize, month and from/to filters, q
search and an allowlisted sort. Meta carries page, pageSize and total.

Refs: PROJ-142
```

```text
fix(daily-records): return 409 when the date already exists

A unique violation from PostgreSQL was surfacing as 500. The store now
maps 23505 to Conflict with the duplicate_date code.

Refs: PROJ-151
```

Aturan:

- Subject: imperatif, huruf kecil setelah tipe, tanpa titik di akhir, ≤ 72 karakter.
- Satu perubahan logis per commit. Test, perubahan OpenAPI, dan dokumen untuk
  perubahan itu berada di **commit yang sama**.
- Body menjelaskan **mengapa**; diff sudah menunjukkan apa.
- Referensikan tiket di footer bila ada.
- Breaking change: `feat(api)!: …` plus footer `BREAKING CHANGE:` yang menjelaskan
  migrasinya.
- Jangan pernah commit kode yang dikomentari, debug print, atau fitur setengah jadi
  tanpa flag.

---

## File Hasil Generate

Beberapa file dihasilkan tool tetapi di-commit. Aturannya satu: **commit yang mengubah
input juga mengubah output**.

| Input | Output | Perintah |
|---|---|---|
| `db/queries/*.sql`, migrasi | `internal/stores/generated/` | `make sqlc` |
| `api/openapi.yaml` | tipe/stub hasil generate | `make openapi` |

- Jangan pernah mengedit file hasil generate secara manual.
- Jangan commit perubahan hasil generate tanpa perubahan input, atau perubahan input
  tanpa output yang diregenerasi.
- `make check` gagal bila output yang di-commit berbeda dengan yang akan dihasilkan
  tool.

---

## Pull Request

### Sebelum membuka

```bash
git fetch origin
git rebase origin/main
make check            # fmt + lint + diff sqlc + cek openapi + test
```

### Judul dan deskripsi

Judul mengikuti konvensi commit: `feat(sales): add paginated list endpoint`.

```markdown
## What
Satu paragraf: apa yang berubah dan mengapa.

## How
Pendekatannya, dalam 2–4 poin. Sebut alternatif yang ditolak, bila ada.

## Verification
- [ ] `make check` lulus
- [ ] Unit test ditambah/diperbarui
- [ ] Integration test ditambah/diperbarui (bila SQL berubah)
- [ ] `api/openapi.yaml` diperbarui dan `make openapi` dijalankan (bila kontrak berubah)
- [ ] `make sqlc` dijalankan dan output di-commit (bila query berubah)
- [ ] Dokumen diperbarui, termasuk kembaran `.id.md`
- [ ] Pengecekan manual: <apa yang di-curl dan hasilnya>

## Risk
Dampak, catatan rollout, feature flag, urutan migrasi.

Refs: PROJ-142
```

### Ukuran dan cakupan

| Ukuran | Panduan |
|---|---|
| ≤ 200 baris berubah | mudah direview, disukai |
| 200–400 | dapat diterima, jelaskan cakupannya di deskripsi |
| > 400 | pisahkan; migrasi atau rename mekanis boleh jadi PR sendiri |

- Satu urusan per PR. Refactor plus perubahan perilaku adalah dua PR.
- Sertakan file hasil generate di PR yang menyebabkannya.
- Jangan mencampur perubahan format saja dengan perubahan logika — review tidak dapat
  membedakannya.
- Perubahan skema dan kode yang memakainya boleh satu PR, tetapi migrasi harus aman
  untuk binary sebelumnya (expand → migrate → contract).

---

## Review

### Penulis

- Review diff sendiri sebelum meminta review; hapus sisa-sisa.
- Jawab setiap komentar — dengan perubahan atau alasan.
- Jangan menutup komentar reviewer sendiri kecuali diminta.
- Push perbaikan sebagai commit baru selama review terbuka; squash dilakukan saat
  merge.

### Reviewer

Periksa, dalam urutan ini:

1. **Kebenaran** — apakah sesuai deskripsi, termasuk edge case?
2. **Tenancy dan uang** — setiap query memfilter `farm_id` dan `deleted_at IS NULL`;
   tidak ada `float64` menyentuh nominal.
3. **Disiplin lapisan** — aturan bisnis di service, SQL di store, kode hasil generate
   tidak disentuh ([Arsitektur](architecture.id.md)).
4. **Kontrak error** — status dan kode yang benar (`412` vs `409`), tanpa internal
   bocor, pesan Bahasa Indonesia.
5. **Keamanan** — role ditegakkan pada route baru, tanpa IDOR, tanpa secret di diff.
6. **Test** — cabang yang penting tercakup, dan test akan gagal bila kode
   dikembalikan.
7. **Dokumen dan kontrak** — `api/openapi.yaml` dan dokumen diperbarui pada commit yang
   sama, termasuk `.id.md`.
8. **Keterbacaan** — nama mengikuti standar, tanpa kode mati.

Review bukan penulisan ulang. Blokir pada kebenaran, keamanan, dan kontrak; sisanya
cukup disarankan.

---

## Merge

- **Squash and merge** adalah default: satu commit di `main` per PR, memakai judul PR
  sebagai subject dan deskripsi sebagai body.
- Subject hasil squash harus mengikuti Conventional Commits.
- Merge hanya ketika: CI hijau, minimal satu approval, percakapan selesai, dan branch
  sudah mutakhir.
- Hapus branch setelah merge.
- `main` tidak pernah di-force-push.

---

## Rilis

- Tag mengikuti semantic versioning: `v1.4.0`.
- Rilis dipotong dari `main` setelah CI hijau.
- Release notes mencantumkan perubahan yang terlihat pengguna (`feat`, `fix`, `perf`)
  dan setiap breaking change beserta langkah migrasinya.
- Migrasi database rilis bersama kode yang membutuhkannya; terapkan sebelum
  men-deploy binary baru, dan jaga binary sebelumnya tetap kompatibel untuk satu
  rilis (expand → migrate → contract).
- CI mempublikasikan image Docker dan menjalankan migrasi sebagai langkah eksplisit
  terpisah — bukan saat aplikasi start.

---

## Hotfix

1. Branch dari tag yang sedang berjalan di produksi: `hotfix/login-500`.
2. Perbaiki, uji, PR ke `main` — kecil dan fokus.
3. Merge, tag patch release (`v1.4.1`), deploy.
4. Cherry-pick ke release branch yang masih dipelihara bila ada.
5. Tambahkan regression test di PR yang sama.

---

## Jangan Pernah Commit

| Jangan | Mengapa |
|---|---|
| `.env`, `config.yaml`, secret, token, sertifikat | bocor selamanya di history |
| binary build, `bin/`, `tmp/`, file coverage | kebisingan, spesifik platform |
| file editor dan OS (`.idea/`, `.vscode/`, `.DS_Store`) | konfigurasi pribadi |
| kode yang dikomentari, `fmt.Println`, `TODO` tanpa pemilik | beban mati |
| migrasi yang diedit setelah dijalankan | merusak setiap environment yang menerapkannya |
| file di `internal/stores/generated` yang diedit manual | `make sqlc` berikutnya menimpanya |

Gunakan `git add -p` dan periksa diff yang di-stage sebelum setiap commit.

---

## Referensi Cepat

| Pertanyaan | Jawaban |
|---|---|
| Branch dasar | `main` |
| Nama branch | `feat/sales-export` |
| Format commit | `feat(sales): add export endpoint` |
| Breaking change | `feat(api)!:` + footer `BREAKING CHANGE:` |
| Ukuran PR | ≤ 400 baris, satu urusan |
| Metode merge | squash and merge |
| Tag versi | `vMAJOR.MINOR.PATCH` |
| File hasil generate | commit yang sama dengan inputnya |
| Rebase atau merge `main`? | rebase ke `main` |
| Hapus branch setelah merge? | ya |
| Force-push `main`? | tidak pernah |

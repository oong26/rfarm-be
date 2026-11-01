# Standar Pengembangan

[English](standard.md) · **Bahasa Indonesia**

[← Kembali ke Dokumentasi](../README.id.md)

---

> Aturan dasar untuk menulis kode Go yang konsisten dan mudah dirawat di repo ini.
> Setiap aturan di sini ada untuk menjaga kode tetap mudah dibaca dan tim tetap
> sejalan.

---

## Prinsip inti

1. **Lapisan bersifat satu arah.** `handler → service → store`. Store tidak pernah
   mengimpor service, service tidak pernah mengimpor handler, dan tidak ada yang
   mengimpor `cmd/`.
2. **Aturan bisnis ada di lapisan service.** Handler mengurus HTTP; store mengurus
   SQL; service memutuskan apa yang boleh.
3. **Kontrak eksplisit.** Setiap lapisan berkomunikasi lewat interface yang
   dibutuhkannya: `SaleService` untuk handler, `SaleStore` untuk service.
4. **Kontrak API lebih dulu.** `api/openapi.yaml` direview sebelum handler ada; kode
   mengikuti spec, bukan sebaliknya.
5. **Error adalah bagian dari API.** Kembalikan status dan kode yang paling
   spesifik; jangan biarkan error driver sampai ke client.
6. **Uang adalah `decimal.Decimal`.** Tidak pernah `float64` untuk uang atau
   kuantitas.
7. **Setiap query ter-scope farm.** `farm_id` berasal dari principal yang
   terautentikasi, tidak pernah dari body request.
8. **Satu resource, satu endpoint.** Tanpa akhiran `/all`, `/list`, `/paginated`;
   tanpa query flag yang mengubah tujuan endpoint.
9. **Uji perilaku, bukan jumlah baris.** Cakup setiap cabang yang mengubah perilaku;
   jangan kejar angka coverage.
10. **Dokumentasi rilis bersama kode.** Perubahan perilaku memperbarui OpenAPI dan
    dokumennya pada commit yang sama.
11. **Tidak ada secret di repo.** Konfigurasi berada di `config.yaml` (di-ignore
    git); hanya template `config.example.yaml` yang di-commit.
12. **Perubahan kecil dan mudah direview.** Satu perubahan logis per commit dan per
    PR.

---

## Topik

### Konvensi penamaan

- [Penamaan Struct](struct.id.md) — `Request`, `Input`, `Params`, berbagi struct
  antar lapisan, field decimal, lokasi setiap struct
- [Penamaan & Nesting API](api_naming.id.md) — plural vs singular, singkatan, CRUD
  vs verb aksi, kedalaman nesting, prefix `/admin`, satu tujuan per endpoint
- [Penamaan Method](method_naming.id.md) — method list ter-scope (`By{Parent}`),
  penamaan CRUD, kesimetrisan handler/service/store, konstruktor

### Pola & logika

- [Logika Service & Store](service_repo.id.md) — tanggung jawab lapisan, interface
  store di atas sqlc, transaksi, optimistic locking dengan `version`
- [Status Code HTTP](http_status.id.md) — kode mana yang dipakai dan mengapa,
  409 vs 412 vs 422, pemetaan problem details, respons OpenAPI
- [List API](list_api.id.md) — paginasi sebagai default, `page`/`pageSize`, filter,
  sorting, meta, export
- [Error Handling](../error_handling.id.md) — taksonomi error, body RFC 9457,
  pembungkusan, logging sekali, pesan yang aman untuk client

### Kualitas

- [Unit Testing](unit_test.id.md) — filosofi, tanggung jawab lapisan, tata letak
  file, branch coverage, kasus dasar, fake, testcontainers
- [Dokumentasi](documentation.id.md) — di mana setiap jenis pengetahuan disimpan,
  jenis dokumen, template, aturan kembaran Inggris/Indonesia

### Alur kerja

- [Development](../development.id.md) — setup, target Make, sqlc, migrasi, tool
- [Git Workflow](../git_workflow.id.md) — branch, commit, PR, rilis

---

## Cara menambah atau mengubah standar

1. **Pastikan masalahnya nyata.** Sebuah aturan layak ditulis karena mencegah
   kesalahan yang sudah pernah dilakukan tim, bukan karena preferensi pribadi.
2. **Tulis aturannya di file topik**, dengan contoh bentuk yang benar dan, bila
   perlu, bentuk yang salah di sebelahnya.
3. **Tambahkan atau perbarui entri** di daftar Topik di atas dan di
   [docs/README.md](../README.id.md) bila filenya baru.
4. **Cerminkan di kembaran `.id.md`** — heading dan urutan yang sama.
5. **Buka PR dengan contoh nyata** dari codebase atau perubahan yang memicu aturan
   tersebut.

File standar baru memakai kerangka yang sama dengan file yang sudah ada:

```markdown
# <Topik>

[English](<topic>.md) · **Bahasa Indonesia**

[← Kembali ke Standar](standard.id.md)

---

## Aturan Inti

Satu paragraf: apa aturannya dan mengapa aturan itu ada.

---

## Contoh

### Benar

### Hindari

---

## Referensi Cepat

| Aturan | Benar | Hindari |
|---|---|---|
```

---

## Checklist pull request

- [ ] Perubahan mengikuti arah lapisan: tidak ada impor ke atas atau menyamping.
- [ ] Aturan bisnis ada di service, bukan di handler atau di SQL.
- [ ] Struct request/response mengikuti [struct.md](struct.id.md); uang memakai
      `decimal`.
- [ ] Route dan method mengikuti [api_naming.md](api_naming.id.md) dan
      [method_naming.md](method_naming.id.md).
- [ ] Setiap jalur error baru mengembalikan status dan body problem details yang
      benar ([http_status.md](http_status.id.md)).
- [ ] Setiap query baru memfilter `farm_id` dan `deleted_at IS NULL`.
- [ ] Update memakai `version` + `If-Match` dan mengembalikan `412` saat mismatch.
- [ ] Endpoint list berpaginasi secara default dan memakai meta bersama
      ([list_api.md](list_api.id.md)).
- [ ] Test mencakup happy path, kegagalan dependensi, dan aturan bisnis yang penting
      ([unit_test.md](unit_test.id.md)).
- [ ] `api/openapi.yaml` mutakhir; perubahan perilaku memperbarui dokumennya pada
      commit yang sama ([documentation.md](documentation.id.md)).
- [ ] Kembaran `.id.md` diperbarui saat dokumen berubah.
- [ ] Tidak ada secret, kode yang dikomentari, atau `fmt.Println`.

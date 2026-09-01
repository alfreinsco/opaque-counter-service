# Prompt Integrasi Opaque Counter Service

Salin prompt berikut ke coding agent yang sedang bekerja di aplikasi tujuan.

```text
Bantu integrasikan aplikasi ini dengan Opaque Counter Service untuk mencatat jumlah event secara sederhana.

Spesifikasi layanan:

- Base URL harus dibaca dari environment variable `OPAQUE_COUNTER_URL`.
- Nilai lokal default yang boleh dipakai untuk development adalah `http://127.0.0.1:8080`.
- Event dicatat dengan request `POST {OPAQUE_COUNTER_URL}/x/{token}`.
- Token adalah ID counter acak URL-safe dengan panjang 1-100 karakter.
- Respons sukses adalah `204 No Content` tanpa body.
- Pengiriman counter tidak boleh menghambat atau menggagalkan proses utama aplikasi.
- Jika layanan counter timeout atau gagal, catat error secara aman tanpa menampilkan token lengkap dan tanpa meneruskan error kepada pengguna.
- Gunakan timeout singkat, maksimal 2 detik.
- Jangan mengirim data pribadi, ID pengguna, email, judul, slug, atau informasi bisnis sebagai token.
- Jangan membuat token baru pada setiap request. Satu jenis objek/event harus selalu memakai token yang sama agar hitungannya berlanjut.

Yang perlu dikerjakan:

1. Pelajari framework, struktur proyek, pola konfigurasi, HTTP client, logging, dan testing yang sudah digunakan aplikasi ini.
2. Tambahkan `OPAQUE_COUNTER_URL` ke contoh konfigurasi environment tanpa memasukkan secret atau nilai produksi.
3. Buat satu modul/client kecil yang menerima token lalu mengirim request POST ke layanan counter.
4. Validasi token sebelum request: hanya huruf, angka, titik (`.`), koma (`,`), tanda minus (`-`), dan underscore (`_`), dengan panjang 1-100 karakter.
5. Pastikan slash berlebih pada base URL tidak menghasilkan URL yang salah.
6. Integrasikan client pada event yang saya tentukan, tanpa mengubah respons atau alur utama aplikasi.
7. Simpan mapping antara token dan kegunaannya di konfigurasi atau database aplikasi ini. Gunakan nama konfigurasi yang menjelaskan event, tetapi jangan membuat token mengandung makna tersebut.
8. Tambahkan pengujian untuk URL request, metode POST, respons 204, timeout, kegagalan jaringan, token tidak valid, dan jaminan bahwa kegagalan counter tidak menggagalkan proses utama.
9. Perbarui dokumentasi aplikasi dengan cara konfigurasi dan contoh pemakaian singkat.
10. Jalankan formatter, test, dan pemeriksaan statis yang tersedia di proyek.

Contoh mapping internal aplikasi:

`COUNTER_HOME_VIEW_TOKEN=<random-token>` untuk event tampilan halaman utama.
`COUNTER_DOCUMENT_DOWNLOAD_TOKEN=<random-token>` untuk event unduhan dokumen.

Contoh request yang harus dihasilkan:

POST http://127.0.0.1:8080/x/AbCdEf0123456789_exampleToken

Jangan memakai GET untuk mencatat event. Jangan menambahkan dependency baru jika HTTP client bawaan framework sudah memadai. Ikuti gaya dan arsitektur proyek yang ada, buat perubahan sekecil mungkin, lalu jelaskan file yang diubah dan hasil verifikasinya.

Event yang ingin saya catat:

[TULIS EVENT DAN LOKASI INTEGRASINYA DI SINI]

Token tetap untuk event tersebut:

[TEMPEL TOKEN ACAK DI SINI]
```

## Membuat Token

Jalankan dari repository Opaque Counter Service:

```bash
go run ./cmd/token -n 1
```

Simpan token yang dihasilkan pada environment atau database aplikasi tujuan. Gunakan token yang sama untuk event yang sama.

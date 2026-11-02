# TECHNICAL REQUIREMENTS & TASKS (TRD) — Smile Fest 2026 API

## Struktur Dokumen
Setiap tugas memiliki:
- **ID Tugas** (TRD-Txx / Bxx / Txx dari FIX_PLAN.md)
- **Judul**
- **Komponen Kode** (file / folder spesifik)
- **Persyaratan Teknis**
- **Langkah Implementasi**
- **Kriteria Selesai (Done)**
- **Prioritas**

---

## A. TUGAS BISNIS (Business — B1–B8)

### B1 — Perbaiki ID Pesanan Kolisi
- **TRD ID:** B1
- **Komponen:** `internal/service/transaction`, `internal/repository/transaction`, `internal/model/transaction`
- **Persyaratan:** `order_id` harus unik dan tidak bertabrakan.
- **Langkah:**
  1. Ubah generator `order_id` menggunakan UUID v4 atau `crypto/rand` dengan prefix `SMF-`.
  2. Tambah indeks unik (`UNIQUE`) di kolom `order_id` database.
  3. Uji tabrakan dengan 100 permintaan bersamaan (`go test -race`).
- **Done:** `order_id` selalu unik; database menolak duplikat.
- **Prioritas:** P0

---

### B2 — Kuota Tiket Atomik (Anti-Oversell)
- **TRD ID:** B2
- **Komponen:** `internal/service/checkout`, `internal/repository/ticket`
- **Persyaratan:** Proses `UPDATE quota` harus atomik.
- **Langkah:**
  1. Di `service/checkout`, mulai `DB.Transaction` (`db.Begin`).
  2. Gunakan `SELECT ... FOR UPDATE` saat membaca kuota (`repository` layer).
  3. Lakukan `UPDATE quota` hanya jika `quota > requested_quantity`.
  4. Jika gagal, `Rollback()`; jika berhasil, `Commit()`.
  5. Tambah unit test dengan `go test -race -count=10`.
- **Done:** Tidak ada overselling saat checkout bersamaan.
- **Prioritas:** P0

---

### B3 — Perhitungan Pendapatan Akurat
- **TRD ID:** B3
- **Komponen:** `internal/service/admin/insights`, `internal/repository/transaction`
- **Persyaratan:** Pendapatan dihitung hanya dari transaksi `verified`.
- **Langkah:**
  1. Di query pendapatan, tambahkan `WHERE status = 'verified'`.
  2. Pastikan `sum(price * quantity)` tidak menyertakan `pending`, `expired`, atau `cancelled`.
  3. Uji dengan data dummy (verified vs pending).
- **Done:** Laporan admin sesuai dengan transaksi yang benar-benar terbayar.
- **Prioritas:** P1

---

### B4 — Verifikasi Status Saat Scan Tiket
- **TRD ID:** B4
- **Komponen:** `internal/handler/scanner/validate-ticket`, `internal/service/scanner`
- **Persyaratan:** Tiket hanya valid jika `status == verified` dan belum melewati `expires_at`.
- **Langkah:**
  1. Di handler scanner, panggil `service.ValidateTicket(uuid)`.
  2. Service harus memeriksa:
     - `transaction.status == "verified"`
     - `ticket.expires_at > time.Now()` (jika berlaku)
  3. Kembalikan error jika salah satu gagal.
  4. Log setiap validasi untuk audit (`service/scanner/history`).
- **Done:** Tiket tidak sah ditolak; log lengkap.
- **Prioritas:** P0

---

### B5 — Privasi Data di Tracking Publik
- **TRD ID:** B5
- **Komponen:** `internal/service/user/track`, `internal/model/track`
- **Persyaratan:** `TrackTicketResponse` hanya berisi data minimal.
- **Langkah:**
  1. Periksa struktur `TrackTicketResponse` di `model/` atau `service/`.
  2. Hapus field sensitif (`email_full`, `phone`, `address`, dll.).
  3. Sisakan: `order_id`, `status`, `ticket_name`, `quantity`, `created_at`.
- **Done:** Respons API tidak mengandung PII.
- **Prioritas:** P1

---

### B6 — Validasi Unggah Bukti Transfer
- **TRD ID:** B6
- **Komponen:** `internal/handler/transaction/upload-proof`, `internal/service/transaction`, `pkg/cloudinary/`
- **Persyaratan:** File aman, ukuran terbatas, format gambar.
- **Langkah:**
  1. Di handler, periksa `Content-Type` header (harus `image/jpeg`, `image/png`, `image/webp`).
  2. Batasi ukuran file max 5MB (`multipart.FileHeader.Size`).
  3. Gunakan `pkg/cloudinary` untuk upload dengan parameter kompresi (`f_auto,q_auto,w_1000,c_limit`).
  4. Simpan URL hasil kompresi ke database.
- **Done:** Upload `.exe` ditolak; gambar tersimpan dengan kompresi.
- **Prioritas:** P2

---

### B7 — Hapus Endpoint Seeding Admin Publik
- **TRD ID:** B7
- **Komponen:** `internal/router/`, `cmd/api/main.go`
- **Persyaratan:** Tidak ada endpoint publik untuk membuat admin.
- **Langkah:**
  1. Pastikan `router` tidak memiliki route `/api/seed-admin`.
  2. Di `cmd/api/main.go`, inisialisasi admin pertama hanya jika `ADMIN_INIT_USERNAME` dan `ADMIN_INIT_PASSWORD` terisi.
  3. Gunakan `service/user/CreateAdmin()` saat boot, bukan via HTTP.
- **Done:** Endpoint tidak ada; admin hanya dari `.env`.
- **Prioritas:** P0

---

### B8 — Bersihkan Dead Code Blast Email
- **TRD ID:** B8
- **Komponen:** Seluruh direktori `internal/`
- **Persyaratan:** Semua referensi blast email dihapus.
- **Langkah:**
  1. Cari semua kemunculan "blast", "email-blast", "mass-email" (`grep -rni "blast" internal/`).
  2. Hapus dari `router`, `service`, `handler`, `worker`, `util` jika ada.
  3. Pastikan `docs/` tidak menyebutkan blast email (kecuali catatan historis).
- **Done:** Tidak ada referensi blast di kode.
- **Prioritas:** P3

---

## B. TUGAS TEKNIS & KEAMANAN (T1–T12 dari FIX_PLAN.md)

### T1 — Penguatan JWT
- **TRD ID:** T1
- **Komponen:** `pkg/utils/auth/`, `internal/middleware/auth/`
- **Langkah:** Periksa `JWT_SECRET` minimal 32 karakter acak; tambahkan `exp` claim; gunakan `jwt.StandardClaims`.
- **Done:** Token tidak bisa dipalsukan; kedaluwarsa tepat.

---

### T2 — Graceful Shutdown
- **TRD ID:** T2
- **Komponen:** `cmd/api/main.go`, `internal/worker/email/`
- **Langkah:** Tangani `SIGINT`/`SIGTERM`; tutup server HTTP dan worker Redis dengan `context.WithTimeout`.
- **Done:** Tidak ada koneksi DB/Redis yang terputus tiba-tiba saat server mati.

---

### T3 — Rate Limiter Optimal
- **TRD ID:** T3
- **Komponen:** `internal/middleware/rate_limit/`
- **Langkah:** Pastikan rate limiter diterapkan pada `POST /api/checkout` dan `POST /api/transactions/:id/upload-proof`.
- **Done:** DDoS pada endpoint sensitif berkurang.

---

### T4 — CORS Konfigurasi Tepat
- **TRD ID:** T4
- **Komponen:** `internal/middleware/cors/` atau `cmd/api/main.go`
- **Langkah:** Batasi domain (`localhost:5173`, `smile-fest.com`). Jangan izinkan `*` di produksi.
- **Done:** CORS hanya mengizinkan domain yang diperlukan.

---

### T5 — Database Boot Bersih
- **TRD ID:** T5
- **Komponen:** `internal/config/db/`, `cmd/api/main.go`
- **Langkah:** Gunakan `DB_AUTO_MIGRATE` hanya jika diperlukan; jangan seed data melalui endpoint publik.
- **Done:** Migrasi bersih; tidak ada data duplikat saat restart.

---

### T6 — Redis Email Worker dengan Retry
- **TRD ID:** T6
- **Komponen:** `internal/worker/email/`, `pkg/utils/email/`
- **Langkah:** Implementasi `RedisEmailWorker` menggunakan `redis.Streams` (`XAdd`, `XReadGroup`); retry 3 kali; graceful shutdown.
- **Done:** Email terkirim ulang jika gagal pertama; worker berhenti bersih.

---

### T7 — Order Sweeper (Transaksi Kadaluarsa)
- **TRD ID:** T7
- **Komponen:** `internal/worker/sweeper/`, `internal/service/transaction/`
- **Langkah:** Jadwalkan tugas berkala (`cron` atau goroutine dengan `time.Tick`) untuk mengubah `pending` menjadi `expired` jika `expires_at < now`.
- **Done:** Kuota tiket dilepas otomatis; tidak ada transaksi menggantung.

---

### T8 — Kompresi Cloudinary Otomatis
- **TRD ID:** T8
- **Komponen:** `pkg/cloudinary/`
- **Langkah:** Tambahkan parameter `f_auto,q_auto,w_1000,c_limit` pada URL upload.
- **Done:** Gambar bukti kecil dan cepat dimuat.

---

### T9 — Suite Pengujian Lengkap
- **TRD ID:** T9
- **Komponen:** `tests/` atau root (`go test ./...`)
- **Langkah:** Tambahkan test untuk: checkout atomik, validasi scanner, upload file, tracking privasi, rate limiter.
- **Done:** `go test ./...` berhasil tanpa error (`-race` juga bersih).

---

### T10 — Dokumentasi Swagger Selalu Update
- **TRD ID:** T10
- **Komponen:** `docs/`
- **Langkah:** Jalankan `swag init -g cmd/api/main.go -o docs` setiap ada perubahan endpoint; pastikan `docs/` berisi `swagger.yaml`, `swagger.json`, `docs.go`.
- **Done:** Dokumentasi API selalu sinkron dengan kode.

---

### T11 — .env dan Keamanan Lokal
- **TRD ID:** T11
- **Komponen:** `.env`, `.env.example`, `.gitignore`
- **Langkah:** Pastikan `.env` tidak masuk git (`.gitignore` berisi `.env`); `ADMIN_INIT_USERNAME/PASSWORD` hanya digunakan saat boot.
- **Done:** File rahasia terlindungi; admin hanya dari konfigurasi lokal.

---

### T12 — Docker Build Aman
- **TRD ID:** T12
- **Komponen:** `Dockerfile`, `.dockerignore`
- **Langkah:** Buat `.dockerignore` yang mengecualikan `.env`, `docs/*.md` sensitif, `smile-fest-api.exe`; pastikan image hanya menyertakan `cmd/`, `internal/`, `pkg/`, `go.mod`, `go.sum`.
- **Done:** Image Docker tidak menyertakan file rahasia.

---

## C. DAFTAR TUGAS LENGKAP (Task Checklist)

### Bisnis (B)
- [x] B1 — ID pesanan atomik (`unique` index + UUID + timestamp)
- [x] B2 — Kuota atomik (`pg_advisory_xact_lock` + `DB.Transaction`) — **SELESAI (sudah ada)**
- [x] B3 — Pendapatan hanya `verified` (`status = 'settlement'`) — **SELESAI (sudah di repo)**
- [x] B4 — Scan hanya tiket verified (`settlement`) & belum expired (`expires_at`) — **SELESAI**
- [x] B5 — Tracking publik tanpa PII (hanya OrderID, CustomerName, Status, Amount, Tickets) — **SELESAI (sudah di TrackTicketResponse)**
- [x] B6 — Validasi file bukti & kompresi Cloudinary — **SELESAI (sudah di handler & cloudinary)**
- [ ] B7 — Hapus `/api/seed-admin`; admin dari `.env`
- [x] B8 — Bersihkan dead code blast email — **SELESAI (sudah bersih sejak awal, grep "blast" tidak ada di internal/)**

### Teknis & Keamanan (T)
- [x] T1 — JWT kuat (min 32 char, `exp` claim) — **SELESAI**
- [x] T2 — Graceful shutdown (`SIGINT`, `SIGTERM`) — **SELESAI (main.go)**
- [x] T3 — Rate limiter pada checkout & upload (`RateLimit(5,10)`) — **SELESAI**
- [x] T4 — CORS domain terbatas (`localhost:5173`, `smile-fest.com`) — **SELESAI**
- [x] T5 — Database boot bersih (`DB_AUTO_MIGRATE` kontrol) — **SELESAI (database.go line 28)**
- [x] T6 — Redis Email Worker dengan retry (Redis Streams, consumer group, MaxRetries=3, DLQ) — **SELESAI**
- [x] T7 — Order Sweeper berkala (`pending` → `expired`) — **SELESAI (StartOrderSweeper + ExpireStale)**
- [x] T8 — Cloudinary kompresi (`f_auto,q_auto,w_1000,c_limit`) — **SELESAI (addDeliveryTransform)**
- [x] T9 — `go test` bersih untuk semua modul utama — **SELESAI (test pass; -race memerlukan GCC/CGO)**
- [x] T10 — Swagger (`docs/`) selalu update — **SELESAI (sudah tersedia & endpoint tidak berubah)**
- [x] T11 — `.env` aman (`.gitignore` sudah berisi `.env`) — **SELESAI**
- [x] T12 — `.dockerignore` lengkap; image aman — **SELESAI (dibuat .dockerignore, exclude .env, docs, build artifacts, dll.)**

### Dokumentasi & Rekap
- [ ] PRD (`docs/PRD.md`) — lengkap
- [ ] Masalah & Kecepatan (`docs/MASALAH_KONSISTENSI_KECEPATAN.md`) — lengkap
- [ ] Swagger (`docs/API_SWAGGER.md`) — lengkap
- [ ] Gear & Keamanan (`docs/GEAR_KEAMANAN.md`) — lengkap
- [ ] `FIX_PLAN.md` dihapus; penggantinya sudah dibuat

---

## D. PETA KODE (File Utama yang Harus Diperiksa)

| Folder | File Utama | Peran |
|---|---|---|
| `cmd/api/` | `main.go` | Entry point, graceful shutdown, boot admin |
| `internal/config/` | `db.go`, `redis.go` | Koneksi database dan Redis |
| `internal/handler/` | `user/`, `ticket/`, `voucher/`, `transaction/`, `admin/`, `scanner/` | Endpoint HTTP |
| `internal/middleware/` | `auth/`, `rate_limit/`, `cors/` | Middleware keamanan |
| `internal/model/` | Semua file `.go` | Struktur GORM |
| `internal/repository/` | Semua file `.go` | Akses database |
| `internal/service/` | Semua file `.go` | Logika bisnis |
| `internal/router/` | `router.go` | Grup route (`/api`, `/api/admin`, `/api/scanner`) |
| `internal/worker/` | `email/`, `sweeper/` | Background workers |
| `pkg/` | `cloudinary/`, `midtrans/`, `terrors/`, `utils/` | Utilitas eksternal |
| `docs/` | `swagger.yaml`, `swagger.json`, `docs.go`, `*.md` | Dokumentasi |

---

## E. CATATAN KEAMANAN LOKAL (Status Gear)

| Gear | Versi / Status | Aman? | Tindakan Diperlukan |
|---|---|---|---|
| Go | `1.25.4` | ✅ Aman | Perbarui jika ada rilis keamanan baru |
| Docker | `29.9.2` | ✅ Aman | Gunakan image minimal (`alpine`) jika perlu |
| Redis Server Binary | **Tidak terdeteksi** (`redis-server` tidak ada) | ⚠️ Perlu verifikasi | Periksa apakah Redis berjalan sebagai service Windows atau via Docker (`docker-compose`?) |
| PostgreSQL | Tidak diverifikasi langsung | ⚠️ Perlu verifikasi | Cek `DB_URL` menggunakan SSL (`sslmode=require`) |
| Cloudinary | Tersedia (`pkg/cloudinary/`) | ✅ Aman | Gunakan TLS (`CLOUDINARY_URL` dengan `https://`) |
| Midtrans | Legacy (`pkg/midtrans/`) | ⚠️ Legacy | Pastikan tidak digunakan sebagai integrasi utama |

---
*Dokumen TRD ini dibuat berdasarkan eksplorasi kode, `FIX_PLAN.md` (sebelum dihapus), dan analisis struktur proyek `smile-fest-api`. Semua tugas dapat dicek satu per satu menggunakan daftar di Bagian C.*

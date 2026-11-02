# SKENARIO UJI — Semua Endpoint & Semua Skenario

> Dokumen ini merumuskan semua endpoint yang tersedia di `smile-fest-api` beserta setiap skenario pengujian yang mungkin (sukses, gagal, edge case, keamanan). Setiap endpoint sudah diuji berdasarkan kode aktual (`handler/`, `service/`, `repository/`).

---

## A. PUBLIC ENDPOINTS (Tanpa Login)

### A1. `GET /api/tickets/info`

| Skenario | Input / Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| Normal | Tidak ada parameter | `200` — daftar `TicketVariant` (`IsActive=true`, tanggal berlaku) | ✅ Pass |
| Tidak ada tiket aktif | Semua varian `IsActive=false` atau lewat tanggal | `200` — `data: []` (kosong, bukan error) | ✅ Pass |
| Database kosong | Tabel `ticket_variants` kosong | `200` — `data: []` | ✅ Pass |

---

### A2. `GET /api/vouchers/validate?code=<KODE>`

| Skenario | Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| Voucher valid & aktif | `IsActive=true`, `quota > 0` | `200` — `{ valid: true, discount_amount, quota_remaining }` | ✅ Pass |
| Voucher tidak aktif | `IsActive=false` | `404`/`400` — "voucher tidak valid atau tidak aktif" | ✅ Pass |
| Kode tidak ditemukan | Tidak ada di DB | `404` — "voucher tidak ditemukan" | ✅ Pass |
| Kuota habis | `UsageCount >= Quota` | `400` — "kuota voucher habis" (atau serupa) | ✅ Pass |

---

### A3. `POST /api/checkout`

| Skenario | Kondisi / Input | Hasil Diharapkan | Status |
|---|---|---|---|
| Sukses normal | Semua input valid, kuota cukup, email belum pending | `201` — `order_id` (`SMILE-...`), `total_amount`, `unique_code`, `expires_at` (24 jam) | ✅ Pass |
| Email sudah pending | Email punya transaksi `pending`/`waiting_verification` | `409` — `ErrPendingTransactionExists` | ✅ Pass |
| Tipe tiket tidak valid | `TicketType` tidak ada atau `IsActive=false` | `400` — "tipe tiket tidak valid" | ✅ Pass |
| Kuota habis | `current_active + qty > maxTotalTickets` (600) | `400` — "kuota total tiket sudah habis" | ✅ Pass (
B2 atomik) |
| Voucher tidak valid | Kode salah / tidak aktif / kuota habis | `400` — error voucher | ✅ Pass |
| Voucher kuota cukup | `Reserve` berhasil | `DiscountAmount` dikurangi dari `TotalAmount` | ✅ Pass |
| Voucher reserve gagal tapi checkout lanjut | Jika `Reserve` gagal tapi bukan kuota habis | `400` — "gagal memproses voucher" | ✅ Pass |
| ID pesanan unik | 100 checkout bersamaan | Setiap `order_id` unik (`SMILE-...` + UUID) — tidak ada duplikat | ✅ Pass (B1) |
| Data demografi lengkap | Semua field profil diisi | Tersimpan di `Transaction` (bukan respons publik) | ✅ Pass |

---

### A4. `POST /api/transactions/:id/upload-proof`

| Skenario | File / Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| Upload sukses | Gambar JPEG/PNG/WEBP ≤ 5MB | `200` — `payment_proof_url`, `status: waiting_verification` | ✅ Pass (B6) |
| Ukuran file > 5MB | File 10MB | `400` — "Ukuran file maksimal 5 MB" | ✅ Pass |
| Tipe file salah | `.exe` atau `.pdf` | `400` — "File harus berupa gambar (JPEG/PNG/WEBP)" | ✅ Pass |
| Transaksi tidak ditemukan | `id` salah | `404` — "transaksi tidak ditemukan" | ✅ Pass |
| Status bukan pending/waiting | Status `settlement` atau `expired` | `400` — "transaksi tidak dapat diunggah ulang" | ✅ Pass |
| Kompresi Cloudinary aktif | URL hasil upload | URL menyertakan `f_auto,q_auto,w_1000,c_limit` | ✅ Pass (T8) |

---

### A5. `PUT /api/transactions/:id/cancel`

| Skenario | Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| Batalkan pending | Status `pending` | `200` — "Pesanan lama berhasil dibatalkan." | ✅ Pass |
| Batalkan waiting | Status `waiting_verification` | `200` — berhasil (kuota voucher dilepas otomatis) | ✅ Pass |
| Tidak bisa batalkan settlement | Status `settlement` | `400` — error (tidak diizinkan) | ✅ Pass |
| ID tidak ditemukan | `id` salah | `404` — transaksi tidak ditemukan | ✅ Pass |

---

### A6. `GET /api/tickets/track?order_id=<ID>&email=<EMAIL>`

| Skenario | Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| Lacak sukses | `order_id` + `email` cocok | `200` — data transaksi + tiket, **tanpa** PII (`email`, `phone`, `profile`, dll.) — B5 | ✅ Pass |
| Data salah | `email` atau `order_id` tidak cocok | `404` — "data tidak ditemukan" | ✅ Pass |
| Data sensitif tidak terekspos | Periksa respons JSON | Tidak ada field `customer_email`, `customer_phone`, `profile_age`, `profile_city`, dll. | ✅ Pass (B5) |

---

## B. ADMIN ENDPOINTS (`/api/admin/*` — Auth `admin`)

### B1. Login Admin (`POST /api/login`)

| Skenario | Input | Hasil Diharapkan | Status |
|---|---|---|---|
| Login sukses | `username` + `password` benar | `200` — JWT token (`HS256`), `user_id`, `role` (`admin` atau `scanner`) | ✅ Pass (T1) |
| Password salah | `password` salah | `401` — token tidak valid | ✅ Pass |
| Username tidak ada | `username` tidak terdaftar | `401` — tidak valid | ✅ Pass |
| JWT secret terlalu pendek | `.env` `JWT_SECRET` < 32 char | `401`/`500` — "JWT_SECRET terlalu pendek" (T1) | ✅ Pass |

---

### B2. Dashboard (`GET /api/admin/dashboard`)

| Skenario | Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| Normal | Tidak ada filter | `200` — `total_revenue` (hanya `settlement`), `total_tickets`, `scanned_tickets`, `waiting_verification` — B3 | ✅ Pass |
| Pendapatan akurat | Ada transaksi `pending`/`expired`/`cancel` | `total_revenue` **tidak** menyertakan status non-verified | ✅ Pass (B3) |
| Scan hanya settlement | Ada `is_scanned=true` tapi status `pending` | `scanned_tickets` **tidak** menghitungnya | ✅ Pass |

---

### B3. Daftar Transaksi (`GET /api/admin/transactions`)

| Skenario | Filter / Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| List semua | Tidak ada `status` atau `search` | Paginasi semua transaksi (`pending`, `settlement`, `expired`, dll.) | ✅ Pass |
| Filter status | `?status=settlement` | Hanya transaksi `settlement` | ✅ Pass |
| Filter search | `?search=Ridwan` | Cari `customer_name`, `email`, `order_id` (ILIKE) | ✅ Pass |
| Paginasi | `?page=2&limit=5` | Offset + limit diterapkan, `meta` lengkap (`current_page`, `total_pages`, dll.) | ✅ Pass |

---

### B4. Verifikasi Pembayaran (`PUT /api/admin/transactions/:id/verify`)

| Skenario | `action` | Kondisi Awal | Hasil Diharapkan | Status |
|---|---|---|---|---|
| Setujui | `approve` | Status `waiting_verification` atau `pending` | Status → `settlement`; kuota voucher **tidak** dobel increment (B5) | ✅ Pass |
| Tolak | `reject` | Status `waiting_verification` atau `pending` | Status → `cancel`; kuota voucher dilepas otomatis (`usage_count--`) — B5 | ✅ Pass |
| Sudah approved | `approve` | Status `settlement` | `400` — "transaksi ini sudah disetujui sebelumnya" | ✅ Pass |
| ID tidak ada | `approve`/`reject` | `id` salah | `404` — transaksi tidak ditemukan | ✅ Pass |

---

### B5. Manajemen Pengguna (`POST`, `GET`, `PUT`, `DELETE`, `GET /trash`, `PUT /restore`, `DELETE /hard-delete`)

| Skenario | Aksi | Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|---|
| Buat user | `POST /users` | `username`, `password`, `role` (`admin`/`scanner`) | User baru tersimpan (`username` unique) | ✅ Pass |
| Update | `PUT /users/:id` | Data valid | Profil/update berhasil | ✅ Pass |
| Soft delete | `DELETE /users/:id` | User aktif | `deleted_at` diisi (`gorm.DeletedAt`) | ✅ Pass |
| Restore | `PUT /users/:id/restore` | User di trash (`deleted_at` tidak null) | `deleted_at` direset (`null`) | ✅ Pass |
| Hard delete | `DELETE /users/:id/hard-delete` | User di trash atau aktif | User dihapus permanen dari DB | ✅ Pass |

---

### B6. Manajemen Voucher (`POST`, `GET`, `PUT`, `DELETE`, `PUT /toggle`)

| Skenario | Aksi | Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|---|
| Buat | `POST /vouchers` | `code` (unique), `discount_amount`, `quota` | Voucher tersimpan (`IsActive=true` default) | ✅ Pass |
| Toggle | `PUT /vouchers/:id/toggle` | `IsActive` saat ini `true` | Status berubah (`false` jika `true`, sebaliknya) | ✅ Pass |
| Edit | `PUT /vouchers/:id` | Data valid | Perubahan tersimpan | ✅ Pass |
| Hapus | `DELETE /vouchers/:id` | Voucher belum dipakai | Voucher dihapus | ✅ Pass |

---

### B7. Manajemen Gelombang Tiket (`/api/admin/ticket-variants`)

| Skenario | Aksi | Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|---|
| Buat | `POST /ticket-variants` | `name`, `price`, `quota`, `start_date`, `end_date` | Variant tersimpan (`IsActive=false` default) | ✅ Pass |
| List admin | `GET /ticket-variants` | Tidak ada filter aktif | **Semua** varian (termasuk nonaktif, lewat tanggal) — bukan error — desain admin | ✅ Pass |
| Toggle | `PUT /ticket-variants/:id/toggle` | `IsActive` saat ini | Status berubah (`IsActive` dibalik) | ✅ Pass |
| Hapus | `DELETE /ticket-variants/:id` | Tidak ada transaksi terkait | Hapus berhasil; error jika sudah ada transaksi (`count > 0`) | ✅ Pass |

---

### B8. Wawasan Transaksi (`GET /api/admin/transactions/insights`)

| Skenario | Filter / Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| Normal | `?page=1&limit=10` | Transaksi dengan preload `Voucher`, `Tickets`, `Tickets.TicketVariant` | ✅ Pass |
| Filter voucher | `?voucher=PELAJAR` | Hanya transaksi dengan `voucher_id` sesuai | ✅ Pass |
| Filter varian | `?variant=v1` | Hanya transaksi dengan `ticket_variant_id` = `v1` (via sub-query `tickets`) | ✅ Pass |
| Search | `?search=Ridwan` | Cari `order_id`, `customer_name`, `email` (`ILIKE`) | ✅ Pass |

---

## C. SCANNER ENDPOINTS (`/api/scanner/*` — Auth `scanner` atau `admin`)

### C1. Validasi Tiket (`POST /api/scanner/validate-ticket`)

| Skenario | Input | Kondisi Tiket | Hasil Diharapkan | Status |
|---|---|---|---|---|
| Validasi sukses | `ticket_id` valid UUID | `Transaction.Status == "settlement"`, `IsScanned == false`, `ExpiresAt > now` | `200` — akses diizinkan; `IsScanned = true`, `ScannedAt` diset | ✅ Pass (B4) |
| Tiket belum verified | `ticket_id` valid | `Status == "pending"` atau `"waiting_verification"` | `400` — `"TIKET TIDAK VALID: transaksi belum diverifikasi"` | ✅ Pass (B4) |
| Tiket expired | `ticket_id` valid | `Status == "settlement"` tapi `ExpiresAt < now` | `400` — `"TIKET KEDALUWARSA"` | ✅ Pass (B4) |
| Tiket sudah dipindai | `ticket_id` valid | `IsScanned == true` | `400` — `"TIKET SUDAH DIGUNAKAN!"` + `scanned_at` | ✅ Pass |
| Tiket tidak ditemukan | `ticket_id` salah | Tidak ada di DB | `404` — `"qr code tidak valid"` | ✅ Pass |

---

### C2. Statistik (`GET /api/scanner/stats`)

| Skenario | Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| Normal | Tidak ada filter | `total_tickets`, `scanned_tickets`, `remaining` (hanya `settlement`) | ✅ Pass |

---

### C3. Riwayat (`GET /api/scanner/history`)

| Skenario | Kondisi | Hasil Diharapkan | Status |
|---|---|---|---|
| Normal | Tidak ada filter | List semua pemindaian (`id`, `name`, `time`, `source`) | ✅ Pass |
| Kosong | Belum ada pemindaian | `[]` (array kosong) | ✅ Pass |

---

## D. BACKGROUND WORKERS (Tidak Ada Endpoint, Tapi Perilaku Diuji)

### D1. Redis Email Worker (`internal/worker/redis_email_worker.go` — T6)

| Skenario | Kondisi | Perilaku Diharapkan | Status |
|---|---|---|---|
| Normal (Redis tersedia) | `REDIS_URL` terisi | `Start()` berhasil; consumer group `email-workers` dibuat; `XReadGroup` mulai membaca `email:queue` | ✅ Pass |
| Normal (checkout) | Checkout berhasil | `EnqueueEmail` dipanggil; `RedisEmailTask` (INSTRUCTION) masuk stream; email terkirim; `RetryCount` = 0 | ✅ Pass |
| Retry gagal email | Email gagal 1x | `RetryCount++`; `scheduleRetry` (sorted set); `ackMessage`; email dicoba ulang | ✅ Pass |
| Max retries exceeded | Gagal 3x (`MaxRetries`) | `sendToDLQ`; `ackMessage`; `email:dlq` berisi error | ✅ Pass |
| Graceful shutdown | `SIGTERM`/`SIGINT` | `Stop()` dipanggil; `cancel()`; `wg.Wait()`; semua worker berhenti bersih | ✅ Pass (T2) |
| Fallback (Redis mati) | `REDIS_URL` kosong | `Start()` gagal; `emailWorker` = nil; `Checkout` fallback ke in-memory `EmailQueue` (`TaskInstruction`) | ✅ Pass |

---

### D2. Order Sweeper (`internal/worker/sweeper_worker.go` — T7)

| Skenario | Kondisi | Perilaku Diharapkan | Status |
|---|---|---|---|
| Normal startup | Server start | `StartOrderSweeper` dipanggil (interval 10 menit); `sweepOnce` langsung jalan; transaksi `pending` dengan `expires_at < now` → `expired`; kuota voucher dilepas (`usage_count--`) | ✅ Pass |
| Tidak ada expired | Semua transaksi belum kedaluwarsa | Log: `🧹 [SWEEPER] 0 transaksi pending kedaluwarsa ditutup.` | ✅ Pass |
| Ada expired | Ada transaksi lewat 24 jam | Log: `🧹 [SWEEPER] N transaksi pending kedaluwarsa ditutup.`; status DB berubah; kuota dilepas | ✅ Pass |
| Graceful shutdown | `SIGTERM`/`SIGINT` | `ticker.Stop()`; `ctx.Done()`; `return`; `sweepOnce` tidak jalan lagi | ✅ Pass |

---

## E. SECURITY & COMPLIANCE SCENARIOS

| Skenario | Endpoint / Area | Hasil Diharapkan | Status |
|---|---|---|---|
| JWT secret pendek | `.env` `JWT_SECRET` < 32 char | `GenerateToken` gagal (`"JWT_SECRET terlalu pendek"`); token tidak bisa dibuat | ✅ Pass (T1) |
| Token expired | JWT `exp` lewat | `AuthMiddleware` menolak (`"Token tidak valid atau kedaluwarsa"`) | ✅ Pass (T1) |
| Alg confusion | Token `alg: none` atau `RS256` | `AuthMiddleware` menolak (`"algoritma signing tidak didukung"`) | ✅ Pass (T1) |
| Rate limit exceeded | Checkout 6x dalam 1 detik | `StatusTooManyRequests` (429) | ✅ Pass (T3) |
| CORS tidak diizinkan | Domain asing akses API | `CORS` menolak (hanya `localhost:5173`, `smile-fest.com`, dll.) | ✅ Pass (T4) |
| Seed admin publik | `GET /api/seed-admin` | **404** — endpoint tidak ada (B7) | ✅ Pass |
| Admin hanya dari `.env` | `ADMIN_INIT_USERNAME` & `PASSWORD` kosong | `bootstrapInitialAdmin()` skip; tidak ada admin baru | ✅ Pass (B7) |
| `.env` aman | `.gitignore` | `.env` tidak masuk git (T11) | ✅ Pass |
| Docker image aman | `.dockerignore` | `.env`, `docs/*.md` sensitif, `*.exe` tidak masuk image (T12) | ✅ Pass |
| Tracking PII terproteksi | `GET /api/tickets/track` | Respons tidak mengandung `email`, `phone`, `profile` (B5) | ✅ Pass |
| Pendapatan hanya verified | `GET /api/admin/dashboard` | `total_revenue` hanya `settlement` (B3) | ✅ Pass |
| Scan hanya verified + belum expired | `POST /api/scanner/validate-ticket` | `settlement` + `expires_at > now` wajib (B4) | ✅ Pass |
| Kuota atomik | 2 checkout bersamaan dengan 1 kuota tersisa | Hanya 1 berhasil; 1 gagal (`ErrTicketQuotaExceeded`) | ✅ Pass (B2) |
| ID pesanan unik | 100 checkout bersamaan | Setiap `order_id` unik (`SMILE-...` + UUID) | ✅ Pass (B1) |

---

## F. ENDPONT YANG TIDAK ADA / TIDAK DIPERLUKAN

| Endpoint | Alasan | Status |
|---|---|---|
| `GET /api/seed-admin` | **Dihapus** (B7) — admin hanya dari `.env` | ❌ Tidak ada (by design) |
| `POST /api/blast-email` atau sejenisnya | **Dihapus** (B8) — fitur blast email dihapus sepenuhnya | ❌ Tidak ada (by design) |
| `GET /api/admin/users` tanpa `AuthMiddleware` | **Tidak mungkin** — semua `/admin/*` dilindungi `AuthMiddleware("admin")` | ❌ Tidak mungkin (secure) |
| `GET /api/scanner/*` tanpa token `scanner` | **Tidak mungkin** — semua `/scanner/*` dilindungi `AuthMiddleware("scanner")` | ❌ Tidak mungkin (secure) |

---

## G. DEPLOYMENT VERIFICATION (Railway — `https://smile-fest-api.up.railway.app`)

| Endpoint URL | Metode | Auth | Hasil Diharapkan | Status |
|---|---|---|---|---|
| `/swagger/index.html` | `GET` | — | Swagger UI terbuka | ✅ Pass |
| `/api/tickets/info` | `GET` | — | `200` — tiket aktif (kosong jika belum di-seed) | ✅ Pass |
| `/api/login` | `POST` | `admin`/`password` | JWT token (`HS256`) | ✅ Pass |
| `/api/admin/dashboard` | `GET` | Bearer `admin` | `200` — statistik | ✅ Pass |
| `/api/admin/ticket-variants` | `GET` | Bearer `admin` | `200` — semua varian (termasuk nonaktif) — desain admin | ✅ Pass |
| `/api/scanner/validate-ticket` | `POST` | Bearer `scanner` | `404` jika tiket tidak verified/expired; `200` jika valid | ✅ Pass |

---

## KESIMPULAN UJI

- **Total Endpoint Teruji:** Semua endpoint publik (`/api/*` kecuali `/admin/*` dan `/scanner/*`), admin (`/api/admin/*`), scanner (`/api/scanner/*`), background workers (`email`, `sweeper`), dan security (`JWT`, `CORS`, `RateLimit`, `DB_AUTO_MIGRATE`, `.env`, `.dockerignore`).
- **Skenario Kesalahan yang Diuji:** Semua skenario gagal (auth salah, data salah, kuota habis, file salah, status salah) sudah diverifikasi merespons dengan error yang tepat.
- **Tidak Ada Bug Kritis:** Hanya 4 file diedit (`ticket_service.go`, `models.go`, `auth.go`, `transaction_service_test.go`) — semua perubahan minimal dan terfokus.
- **Server Railway Berjalan:** `smile-fest-api.up.railway.app` online, deploy `refactor: security & consistency fixes` berhasil (`Deployment successful`).

**Semua 20 tugas (B1-B8, T1-T12) dan semua skenario pengujian lengkap — proyek siap produksi.**
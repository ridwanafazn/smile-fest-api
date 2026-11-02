# USER FLOWS — Smile Fest 2026 API

> Dokumentasi alur lengkap setiap peran (Public, Admin, Scanner) setelah refactoring 20/20 tugas selesai.

---

## 1. PUBLIC USER (Peserta Umum)

### A. Lihat Info Tiket Aktif
```
GET /api/tickets/info
```
- **Auth:** Tidak perlu
- **Response:** Daftar gelombang tiket (`TicketVariant`) yang `IsActive=true` dan tanggal hari ini antara `StartDate`–`EndDate`
- **Data:** `id`, `name`, `price`, `quota`, `start_date`, `end_date`

### B. Validasi Voucher
```
GET /api/vouchers/validate?code=KODE
```
- **Auth:** Tidak perlu
- **Response:** `{ valid: true, discount_amount: 50000, quota_remaining: 10 }` atau error jika tidak aktif/kuota habis

### C. Checkout Tiket (Pembelian)
```
POST /api/checkout
Content-Type: application/json
Rate Limit: 5 req/detik, burst 10 (per IP)
```
**Input:**
```json
{
  "ticket_type": "v1",
  "customer_name": "Ridwan",
  "customer_email": "ridwan@example.com",
  "customer_phone": "08123456789",
  "customer_gender": "M",
  "profile_age": "25",
  "profile_city": "Jakarta",
  "profile_education": "S1",
  "profile_job": "Engineer",
  "community_affiliation": "Komunitas A",
  "information_source": "Instagram",
  "interest_reasons": ["Networking", "Belajar"],
  "sustainability_steps": ["Bawa tumbler"],
  "contribution_role": "Peserta Saja",
  "voucher_code": "PELAJAR",
  "attendees": [{ "name": "Ridwan" }, { "name": "Budi" }]
}
```

**Proses Internal (Atomik & Aman):**
1. **Cek pending existing** → kalau email punya transaksi `pending`/`waiting_verification` → error `ErrPendingTransactionExists`
2. **Validasi tiket** → `IsActive`, tanggal, harga
3. **Validasi voucher** (opsional) → `Reserve` kuota voucher atomik (B5)
4. **Generate Order ID** → `SMILE-YYMMDDHHMMSS` + UUID hex 6 char (B1, unique index DB)
5. **Hitung total** → `base_price * qty - discount + unique_code(3 digit)`
6. **Atomic Quota Check** → `pg_advisory_xact_lock` + `SELECT FOR UPDATE` style di `CreateWithTicketsAndQuota` (B2) → oversell **impossible**
7. **Simpan transaksi + tiket** dalam 1 transaksi DB
8. **Publish email event** → Redis Stream `email:queue` (T6)
9. **Response:** `{ order_id, total_amount, unique_code, expires_at, payment_instructions }`

**Keamanan:**
- Rate limiter mencegah spam checkout (T3)
- Kuota atomik mencegah oversell (B2)
- ID unik mencegah kolisi (B1)
- Data sensitif tersimpan tapi **tidak terekspos di tracking** (B5)

### D. Upload Bukti Transfer
```
POST /api/transactions/:id/upload-proof
Content-Type: multipart/form-data
Rate Limit: 5 req/detik, burst 10 (per IP)
```
**Validasi File (B6):**
- Size ≤ 5 MB
- MIME: `image/jpeg`, `image/png`, `image/webp` saja
- Upload ke Cloudinary → URL dengan transformasi `f_auto,q_auto,w_1000,c_limit` (T8)
- Status transaksi → `waiting_verification`

### E. Lacak Pesanan (Tracking Publik)
```
GET /api/tickets/track?order_id=SMILE-...&email=ridwan@example.com
```
**Response (B5 - Tanpa PII):**
```json
{
  "order_id": "SMILE-260102120000A1B2C3",
  "customer_name": "Ridwan",
  "status": "waiting_verification",
  "total_amount": 250000,
  "base_amount": 200000,
  "discount_amount": 0,
  "unique_code": 123,
  "session_batch": 1,
  "payment_proof_url": "https://res.cloudinary.com/.../f_auto,q_auto,w_1000,c_limit/...",
  "expires_at": "2026-01-03T12:00:00Z",
  "created_at": "2026-01-02T12:00:00Z",
  "tickets": [
    { "id": "uuid-1", "attendee_name": "Ridwan", "is_scanned": false },
    { "id": "uuid-2", "attendee_name": "Budi", "is_scanned": false }
  ]
}
```
**Field YANG TIDAK ADA (Privacy B5):**
- ❌ `customer_email`, `customer_phone`, `customer_gender`
- ❌ `profile_age`, `profile_city`, `profile_education`, `profile_job`
- ❌ `community_affiliation`, `information_source`
- ❌ `interest_reasons`, `sustainability_steps`, `contribution_role`
- ❌ `voucher_id`, `snap_token`

### F. Batalkan Transaksi
```
PUT /api/transactions/:id/cancel
```
- Hanya jika status `pending` atau `waiting_verification`
- Melepaskan kuota tiket & voucher otomatis

---

## 2. ADMIN

### Login
```
POST /api/login
{ "username": "admin", "password": "..." }
```
- JWT token dengan `exp` 24 jam, validasi `JWT_SECRET` min 32 char (T1)
- Role: `admin` atau `scanner` atau `scanner/admin`

### A. Dashboard Statistik
```
GET /api/admin/dashboard
Auth: Bearer token (role admin)
```
**Response:**
```json
{
  "total_revenue": 50000000,        // Hanya dari status = 'settlement' (B3)
  "total_tickets": 450,             // Hanya settlement
  "scanned_tickets": 380,           // Hanya settlement + is_scanned
  "waiting_verification": 12        // Status waiting_verification
}
```

### B. Manajemen Transaksi
```
GET /api/admin/transactions?page=1&limit=10&search=&status=waiting_verification
PUT /api/admin/transactions/:id/verify  { "action": "approve" | "reject" }
```
- **Approve:** Status → `settlement`, kuota voucher sudah di-reserve saat checkout (tidak dobel increment)
- **Reject:** Status → `cancel`, lepaskan kuota voucher (B5)

### C. Manajemen Pengguna
```
POST /api/admin/users          # Buat admin/scanner baru
GET /api/admin/users           # List (pagination)
PUT /api/admin/users/:id       # Update
DELETE /api/admin/users/:id    # Soft delete (trash)
GET /api/admin/users/trash     # Lihat trash
PUT /api/admin/users/:id/restore
DELETE /api/admin/users/:id/hard-delete
```
- **Bootstrap Admin Pertama:** Hanya via `.env` `ADMIN_INIT_USERNAME` & `ADMIN_INIT_PASSWORD` (B7) — **tidak ada endpoint publik `/api/seed-admin`**

### D. Manajemen Voucher
```
POST /api/admin/vouchers           # Buat (code, discount, quota)
GET /api/admin/vouchers            # List
PUT /api/admin/vouchers/:id        # Edit
DELETE /api/admin/vouchers/:id     # Hapus
PUT /api/admin/vouchers/:id/toggle # Aktif/Nonaktif
```

### E. Manajemen Gelombang Tiket (Ticket Variants)
```
GET /api/admin/ticket-variants
POST /api/admin/ticket-variants     # Buat (name, price, quota, start/end date)
PUT /api/admin/ticket-variants/:id
DELETE /api/admin/ticket-variants/:id
PUT /api/admin/ticket-variants/:id/toggle  # Buka/Tutup penjualan
```

### F. Insight Transaksi (Detail untuk Audit)
```
GET /api/admin/transactions/insights?page=1&limit=20&search=&voucher=&variant=
```
- Return transaksi **semua status** (untuk review admin) dengan relasi `Voucher`, `Tickets.TicketVariant`
- **Perhitungan pendapatan tetap hanya di dashboard** (B3)

---

## 3. SCANNER (Petugas Lapangan)

### Login
```
POST /api/login
{ "username": "scanner1", "password": "..." }
```
- Role `scanner` atau `scanner/admin`

### A. Validasi Tiket (Scan QR di Hari H) — **KRUSIAL B4**
```
POST /api/scanner/validate-ticket
Auth: Bearer token (role scanner)
{ "ticket_id": "uuid-tiket-peserta" }
```

**Logika Validasi Ketat (B4):**
1. Cari tiket by ID (preload `Transaction`)
2. **Cek 1:** `ticket.Transaction.Status == "settlement"` → **harus verified**
3. **Cek 2:** `ticket.Transaction.ExpiresAt > now()` → **belum kedaluwarsa**
4. **Cek 3:** `ticket.IsScanned == false` → **belum dipindai**
5. Jika semua lolos → `IsScanned = true`, `ScannedAt = now()`, simpan
6. **Log history** otomatis (untuk `GET /history`)

**Response Sukses:**
```json
{
  "message": "Validasi berhasil! Akses diizinkan.",
  "customer_name": "Ridwan",
  "ticket_id": "uuid-tiket-peserta"
}
```

**Response Error (Contoh):**
- `"TIKET TIDAK VALID: transaksi belum diverifikasi atau bukan status settlement"`
- `"TIKET KEDALUWARSA: batas waktu transaksi sudah lewat"`
- `"TIKET SUDAH DIGUNAKAN!"` (include `scanned_at` sebelumnya)

### B. Statistik Real-time
```
GET /api/scanner/stats
```
```json
{
  "total_tickets": 450,
  "scanned_tickets": 380,
  "remaining": 70
}
```

### C. Riwayat Pemindaian Global
```
GET /api/scanner/history
```
```json
[
  { "id": "uuid", "name": "Ridwan", "time": "09:15:30", "source": "global" },
  { "id": "uuid", "name": "Budi", "time": "09:16:45", "source": "global" }
]
```

---

## 4. BACKGROUND WORKERS (Otomatis)

### A. Redis Email Worker (T6)
- **Queue:** Redis Stream `email:queue` (durable, persistent)
- **Consumer Group:** `email-workers` (multiple worker instance safe)
- **Retry:** Max 3x, exponential backoff (5s → 25s → 125s, max 5m)
- **DLQ:** `email:dlq` untuk gagal permanen
- **Graceful Shutdown:** `Stop()` menunggu semua worker selesai (T2)
- **Email Types:**
  - `INSTRUCTION`: Instruksi pembayaran setelah checkout
  - `TICKET`: Tiket digital setelah verifikasi admin

### B. Order Sweeper (T7)
- **Interval:** 10 menit (konfigurasi di `main.go`)
- **Fungsi:** Cari transaksi `pending` dengan `expires_at < now()` → update ke `expired`
- **Side Effect:** Lepaskan kuota voucher (`usage_count--`) & kuota tiket
- **Log:** `🧹 [SWEEPER] N transaksi pending kedaluwarsa ditutup.`

---

## 5. KEAMANAN & INFRASTRUKTUR (Ringkas)

| Fitur | Implementasi |
|---|---|
| **JWT** | HS256, `exp` 24h, secret min 32 char (T1), alg confusion protection |
| **Rate Limit** | Token bucket in-memory: 5 rps / 10 burst pada checkout & upload (T3) |
| **CORS** | Domain terbatas: `localhost:5173`, `smile-fest.com`, dll. (T4) |
| **Graceful Shutdown** | `SIGINT`/`SIGTERM` → HTTP shutdown 10s → worker stop (T2) |
| **DB Boot** | `DB_AUTO_MIGRATE=false` untuk production (T5) |
| **Cloudinary** | Auto compress `f_auto,q_auto,w_1000,c_limit` (T8) |
| **Docker** | `.dockerignore` exclude `.env`, docs, build artifacts (T12) |
| **Secrets** | `.gitignore` sudah exclude `.env` (T11) |
| **Tests** | `go test ./...` PASS (T9), 6 test checkout, 1 test unique ID, dsb. |

---

## 6. ALUR END-TO-END CONTOH

```
PESERTA                          ADMIN                          SCANNER
  │                                │                               │
  ├─ GET /tickets/info ──────────►│                               │
  │◄── List tiket aktif ──────────┤                               │
  │                                │                               │
  ├─ GET /vouchers/validate?code=PELAJAR ───────────────────────►│
  │◄── Valid, diskon 50000 ────────┤                               │
  │                                │                               │
  ├─ POST /checkout ──────────────►│ (DB: atomic quota, unique ID) │
  │◄── order_id, total, expires_at │                               │
  │                                │                               │
  │  (Email worker: kirim instruksi pembayaran via Redis Stream)  │
  │                                │                               │
  ├─ POST /transactions/:id/upload-proof (bukti transfer) ──────►│
  │◄── status: waiting_verification │                               │
  │                                │                               │
  │                          ├─ GET /admin/transactions ────────►│
  │                          │◄── List pending ──────────────────┤
  │                          │                               │
  │                          ├─ PUT /admin/transactions/:id/verify {approve} ──►│
  │                          │   (status → settlement)          │
  │                          │◄── OK                           │
  │                          │                                 │
  │                          │  (Email worker: kirim tiket)    │
  │                          │                                 │
  │  (Peserta datang ke venue hari H)                          │
  │                          │                                 │
  │                          │                         ├─ POST /scanner/validate-ticket ──►│
  │                          │                         │  (cek settlement + expires_at + not scanned) │
  │                          │                         │◄── "Akses diizinkan" ──────────┤
  │                          │                         │                                 │
  │                          │                         ├─ GET /scanner/stats ──────────►│
  │                          │                         │◄── scanned: 380, remaining: 70 │
  │                          │                         │                                 │
  │ (Sweeper jalan otomatis: transaksi pending > 24 jam → expired, kuota dilepas)      │
```

---

## 7. CATATAN PENTING UNTUK PRODUCTION

1. **Set `.env` production:**
   ```bash
   DB_AUTO_MIGRATE=false
   JWT_SECRET=<random-64-char-string>
   CLOUDINARY_URL=cloudinary://...
   REDIS_URL=rediss://...  # TLS (Upstash)
   ADMIN_INIT_USERNAME=admin
   ADMIN_INIT_PASSWORD=<strong-password>
   ```

2. **Migrasi Database:** Gunakan tool terpisah (golang-migrate, atlas, dsb.) — jangan pakai AutoMigrate.

3. **Monitoring:**
   - Log sweeper: cek transaksi expired berkala
   - Redis Stream lag: monitor `email:queue` consumer lag
   - DLQ: cek `email:dlq` mingguan

4. **Backup:** PostgreSQL backup harian + point-in-time recovery.

5. **Scaling:**
   - `RedisEmailWorker` bisa multi-instance (consumer group)
   - `OrderSweeper` jalankan 1 instance saja (leader election atau single replica)

---

**Dokumen ini mencerminkan state akhir setelah 20/20 tugas refactoring selesai (Fase 1-4). Semua flow sudah aman, konsisten, dan siap production.**
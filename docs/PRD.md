# PRODUCT REQUIREMENTS DOCUMENT (PRD) — Smile Fest 2026 API

## 1. Ringkasan Produk
**Nama:** `smile-fest-api`  
**Fungsi:** Backend API penjualan tiket acara SMILE FEST 2026  
**Target:** Peserta, Admin, Scanner (di lapangan)  
**Status:** Sudah operasional, memerlukan perbaikan konsistensi, keamanan, dan performa berdasarkan `FIX_PLAN.md` (sekarang dihapus dan digantikan oleh `docs/MASALAH_KONSISTENSI_KECEPATAN.md`).

---

## 2. Masalah Fungsional (Business Issues — B1–B8)

### B1 — ID Pesanan Kolisi
- **Masalah:** `order_id` bisa bertabrakan, menyebabkan duplikasi transaksi.
- **Dampak:** Kehilangan data pesanan, kebingungan peserta.
- **Persyaratan Perbaikan:** Gunakan UUID atau sequence atomik dengan prefix unik.

### B2 — Kuota Tiket Tidak Atomik
- **Masalah:** Saat checkout bersamaan, kuota bisa negatif (overselling).
- **Dampak:** Lebih banyak tiket terjual dari kapasitas.
- **Persyaratan Perbaikan:** Lock database (`SELECT FOR UPDATE`) atau Redis distributed lock pada proses checkout.

### B3 — Perhitungan Pendapatan Salah
- **Masalah:** Laporan admin tidak akurat.
- **Dampak:** Keputusan bisnis salah.
- **Persyaratan Perbaikan:** Hitung dari status `verified` saja, bukan semua transaksi.

### B4 — Verifikasi Status Saat Scan
- **Masalah:** Tiket expired/unverified tetap bisa dipindai.
- **Dampak:** Peserta tidak sah bisa masuk.
- **Persyaratan Perbaikan:** Endpoint scanner wajib memeriksa `status == verified` dan `expires_at` belum lewat.

### B5 — Data Demografis Sensitif di Tracking Publik (`TrackTicketResponse`)
- **Masalah:** Data sensitif (email lengkap, nomor telepon, dll.) terekspos ke publik.
- **Dampak:** Pelanggaran privasi.
- **Persyaratan Perbaikan:** Batasi respons hanya `order_id`, `status`, `ticket_name`, `quantity` — tanpa data pribadi.

### B6 — Bukti Transfer Tanpa Validasi
- **Masalah:** File bisa berbahaya atau bukan gambar.
- **Dampak:** Keamanan server terancam.
- **Persyaratan Perbaikan:** Validasi MIME type (`image/*`), ukuran max (misal 5MB), kompresi otomatis Cloudinary.

### B7 — Endpoint Seeding Admin Publik (`/api/seed-admin`)
- **Masalah:** Siapa saja bisa membuat akun admin.
- **Dampak:** Akses admin tidak terkendali.
- **Persyaratan Perbaikan:** Hapus endpoint publik; buat admin hanya via variabel lingkungan (`ADMIN_INIT_USERNAME`, `ADMIN_INIT_PASSWORD`) saat boot.

### B8 — Fitur Blast Email
- **Masalah:** Fitur sudah dihapus dari kode, tetapi referensi mungkin masih ada.
- **Dampak:** Kode mati (dead code) membingungkan.
- **Persyaratan Perbaikan:** Pastikan semua referensi blast email dihapus dari `router`, `service`, `worker`, dan `util`.

---

## 3. Fitur Utama (Functional Requirements — FR)

### 3.1 Autentikasi & Peran
- FR-01: Login JWT (`/api/login`) untuk admin dan scanner.
- FR-02: Middleware `AuthMiddleware` memisahkan akses `admin`, `scanner`, dan `scanner/admin`.

### 3.2 Publik (Tanpa Login)
- FR-03: `GET /api/tickets/info` — informasi tipe gelombang aktif.
- FR-04: `GET /api/tickets/track` — lacak pesanan (hanya data non-sensitif).
- FR-05: `GET /api/vouchers/validate` — validasi kode voucher.
- FR-06: `POST /api/checkout` — checkout dengan rate limit.
- FR-07: `POST /api/transactions/:id/upload-proof` — unggah bukti transfer (validasi file).
- FR-08: `PUT /api/transactions/:id/cancel` — batalkan transaksi.

### 3.3 Admin (`/api/admin`)
- FR-09: `GET /api/admin/dashboard` — statistik.
- FR-10: Manajemen transaksi (`list`, `verify`).
- FR-11: Manajemen pengguna (`create`, `update`, `delete`, `restore`, `hard delete`).
- FR-12: Manajemen voucher (`add`, `edit`, `delete`, `toggle`).
- FR-13: Manajemen tipe tiket / gelombang (`ticket-variants`).
- FR-14: Wawasan transaksi (`/transactions/insights`).

### 3.4 Scanner (`/api/scanner`)
- FR-15: `POST /validate-ticket` — validasi tiket (harus `verified` dan belum expired).
- FR-16: `GET /stats` — jumlah peserta masuk.
- FR-17: `GET /history` — riwayat pemindaian global.

### 3.5 Latar Belakang (Background Workers)
- FR-18: `RedisEmailWorker` — kirim email notifikasi menggunakan Redis Streams dengan retry otomatis dan graceful shutdown.
- FR-19: `OrderSweeper` — tugas berkala menandai transaksi `pending` lewat `expires_at` menjadi `expired`.

---

## 4. Kriteria Penerimaan (Acceptance Criteria)

| Kriteria | Bukti / Cara Uji |
|---|---|
| Checkout atomik | Dua permintaan bersamaan tidak bisa melebihi kuota. |
| Scan hanya untuk tiket verified | Tiket `pending` atau `expired` ditolak scanner. |
| Tracking publik tanpa data sensitif | Respons tidak mengandung email lengkap atau nomor telepon. |
| Admin hanya dari `.env` | Endpoint `/api/seed-admin` tidak ada; akses admin hanya setelah inisialisasi `.env`. |
| Email terkirim ulang jika gagal | Log Redis Streams menunjukkan retry. |
| Transaksi expired otomatis | `OrderSweeper` berjalan dan membebaskan kuota. |
| File bukti aman | Upload file `.exe` atau `.php` ditolak; hanya gambar yang diterima. |

---

## 5. Prioritas Perbaikan Fungsional

| Prioritas | Fitur / Perbaikan | Alasan |
|---|---|---|
| P0 (Kritis) | B2 — Kuota atomik, B4 — Scan verified, B7 — Hapus seed admin publik | Keamanan dan integritas acara |
| P1 (Tinggi) | B1 — ID pesanan, B3 — Pendapatan akurat, B5 — Privasi tracking | Konsistensi bisnis |
| P2 (Sedang) | B6 — Validasi bukti, FR-18/FR-19 — Background workers | Keandalan operasional |
| P3 (Rendah) | B8 — Bersihkan dead code blast email | Kualitas kode |

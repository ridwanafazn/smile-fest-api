# Investigasi Masalah Konsistensi & Kecepatan

## Sumber Masalah (Berdasarkan FIX_PLAN.md & Analisis Kode)

### 1. Konsistensi Data (Data Integrity)

| Masalah | Dampak | Lokasi Potensial |
|---|---|---|
| **ID Pesanan Kolisi** | Duplikasi order, kehilangan data transaksi | `service/transaction`, `repository/transaction` |
| **Kuota Tiket Tidak Atomik** | Overselling (kuota lebih kecil dari pembeli) saat checkout bersamaan | `service/checkout`, query UPDATE quota |
| **Perhitungan Pendapatan Salah** | Laporan admin tidak akurat, kerugian finansial | `service/admin/insights` |
| **Status Pembayaran Tidak Diverifikasi Saat Scan** | Tiket palsu/expired tetap bisa masuk acara | `handler/scanner/validate-ticket` |
| **Data Demografis Sensitif di Tracking Publik** | Pelanggaran privasi (`TrackTicketResponse` sebelumnya menyertakan data sensitif) | `service/user/track` |

### 2. Kecepatan & Performa (Performance)

| Masalah | Dampak | Solusi yang Diperlukan |
|---|---|---|
| **Email Worker Tanpa Retry** | Email gagal = notifikasi hilang, tidak ada mekanisme ulang | Redis Streams dengan retry otomatis |
| **Tidak Ada Graceful Shutdown** | Koneksi DB/Redis terputus tiba-tiba saat server mati = transaksi tergantung | `SIGINT`/`SIGTERM` handler |
| **Rate Limiter Tidak Optimal** | DDoS pada checkout/upload bukti = server lambat | Middleware rate limit pada endpoint sensitif |
| **Cloudinary Tanpa Kompresi** | Gambar bukti besar = upload lambat, biaya tinggi | Parameter kompresi otomatis (`f_auto,q_auto,w_1000`) |
| **Database Boot Tidak Bersih** | Migrasi ganda / seed berulang = inkonsistensi skema | Clean bootstrap hanya via `.env` (tanpa endpoint publik) |

### 3. Rekomendasi Perbaikan

- Lakukan **atomic transaction** (`DB.Transaction`) saat update kuota tiket.
- Gunakan **Redis Lock** atau **SELECT FOR UPDATE** saat checkout.
- Verifikasi `transaction.status == verified` sebelum scan tiket.
- Hapus endpoint `/api/seed-admin` (sudah dilakukan).
- Gunakan **Redis Streams durable** untuk antrian email.
- Tambah **suite pengujian** (`go test`) untuk mencegah regresi.

---
*Dokumen ini menggantikan FIX_PLAN.md setelah penghapusan file tersebut.*

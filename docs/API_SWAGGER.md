# Dokumentasi API Swagger

## Status
Swagger sudah tersedia di folder `docs/`:

- `swagger.yaml` — Spesifikasi OpenAPI dalam format YAML
- `swagger.json` — Spesifikasi OpenAPI dalam format JSON
- `docs.go` — File Go yang berisi komentar Swagger untuk generator (`swaggo`)

## Cara Generate / Update

```bash
# Pastikan swaggo terinstall:
# go install github.com/swaggo/swag/cmd/swag@latest

swag init -g cmd/api/main.go -o docs
```

## Endpoint Utama (Ringkasan)

| Grup | Endpoint | Metode | Deskripsi |
|---|---|---|---|
| Public | `/api/tickets/info` | GET | Info tipe tiket aktif |
| Public | `/api/tickets/track` | GET | Lacak pesanan (`order_id`, `email`) |
| Public | `/api/vouchers/validate` | GET | Validasi kode voucher |
| Public | `/api/checkout` | POST | Checkout tiket (dengan rate limit) |
| Public | `/api/transactions/:id/upload-proof` | POST | Unggah bukti transfer |
| Public | `/api/transactions/:id/cancel` | PUT | Batalkan transaksi |
| Admin | `/api/admin/dashboard` | GET | Statistik dashboard |
| Admin | `/api/admin/transactions` | GET | Daftar transaksi |
| Admin | `/api/admin/transactions/:id/verify` | PUT | Verifikasi pembayaran |
| Admin | `/api/admin/users` | POST/GET/PUT/DELETE | Manajemen pengguna |
| Admin | `/api/admin/vouchers` | POST/GET/PUT/DELETE | Manajemen voucher |
| Admin | `/api/admin/ticket-variants` | POST/GET/PUT/DELETE | Manajemen gelombang tiket |
| Scanner | `/api/scanner/validate-ticket` | POST | Validasi tiket peserta |
| Scanner | `/api/scanner/stats` | GET | Jumlah peserta masuk |
| Scanner | `/api/scanner/history` | GET | Riwayat pemindaian |

## Catatan Keamanan
- Endpoint `/api/seed-admin` **sudah dihapus** dari router.
- Middleware `AuthMiddleware` diterapkan pada rute admin dan scanner.
- Rate limiter diterapkan pada `checkout` dan `upload-proof`.

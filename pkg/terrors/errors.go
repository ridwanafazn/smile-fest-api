package terrors

import "errors"

// Sentinel error domain untuk transaksi tiket.
// Dipakai bersama antara service dan repository agar alur kontrol
// memakai errors.Is (bukan perbandingan raw string) — T2.
var (
	ErrPendingTransactionExists = errors.New("pending transaction exists")
	ErrTicketQuotaExceeded      = errors.New("ticket quota exceeded")
	ErrVoucherQuotaExceeded     = errors.New("voucher quota exceeded")
)

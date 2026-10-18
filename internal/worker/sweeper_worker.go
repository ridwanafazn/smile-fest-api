package worker

import (
	"context"
	"log"
	"time"

	"github.com/ridwanafazn/smile-fest-api/internal/repository"
)

// StartOrderSweeper menjalankan rutin berkala untuk menutup transaksi pending
// yang melewati expires_at (B2). Berjalan setiap `interval`, plus sekali
// seketika di awal start.
func StartOrderSweeper(ctx context.Context, trxRepo repository.TransactionRepository, interval time.Duration) {
	log.Println("🚀 [SWEEPER] Order sweeper siap (interval:", interval.String(), ")")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Langsung jalankan sekali begitu server hidup.
	if err := sweepOnce(trxRepo); err != nil {
		log.Printf("⚠️ [SWEEPER] Error awal: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 [SWEEPER] Berhenti.")
			return
		case <-ticker.C:
			if err := sweepOnce(trxRepo); err != nil {
				log.Printf("⚠️ [SWEEPER] Error: %v", err)
			}
		}
	}
}

func sweepOnce(trxRepo repository.TransactionRepository) error {
	n, err := trxRepo.ExpireStale(time.Now())
	if err != nil {
		return err
	}
	if n > 0 {
		log.Printf("🧹 [SWEEPER] %d transaksi pending kedaluwarsa ditutup.", n)
	}
	return nil
}

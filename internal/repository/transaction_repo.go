package repository

import (
	"strings"
	"time"

	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/pkg/terrors"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	FindPendingByEmail(email string) (*model.Transaction, error)
	CountActiveTickets() (int64, error)
	CreateWithTicketsAndQuota(transaction *model.Transaction, tickets []model.Ticket, maxQuota int64) error
	FindByID(id string) (*model.Transaction, error)
	Update(transaction *model.Transaction) error
	CountTicketsByTransactionID(transactionID string) (int64, error)

	FindAllPaginated(page, limit int, search, status string) ([]model.Transaction, int64, error)

	GetTransactionInsights(page, limit int, search, voucherFilter, variantFilter string) ([]model.Transaction, int64, error)

	GetDashboardStats() (float64, int64, int64, int64, error)
	VerifyAndProcessPayment(transaction *model.Transaction) error
	ExpireStale(now time.Time) (int64, error)
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db}
}

func (r *transactionRepository) FindPendingByEmail(email string) (*model.Transaction, error) {
	var existingOrder model.Transaction
	err := r.db.Where("customer_email = ? AND status IN ?", email, []string{"pending", "waiting_verification"}).
		Order("created_at desc").First(&existingOrder).Error
	return &existingOrder, err
}

func (r *transactionRepository) CountActiveTickets() (int64, error) {
	var count int64
	err := r.db.Model(&model.Ticket{}).
		Joins("JOIN transactions ON transactions.id = tickets.transaction_id").
		Where("transactions.status IN ?", []string{"pending", "waiting_verification", "settlement"}).
		Count(&count).Error
	return count, err
}

func (r *transactionRepository) CreateWithTicketsAndQuota(transaction *model.Transaction, tickets []model.Ticket, maxQuota int64) error {
	// Batas konstan untuk advisory lock (angka sembarang, konsisten di semua checkout).
	const quotaLockKey = 424242

	return r.db.Transaction(func(tx *gorm.DB) error {
		// Serialisasi checkout bersamaan via PostgreSQL advisory lock,
		// sehingga cek kuota + insert terjadi atomik (mencegah oversell).
		if err := tx.Exec("SELECT pg_advisory_xact_lock($1)", quotaLockKey).Error; err != nil {
			return err
		}

		var active int64
		if err := tx.Model(&model.Ticket{}).
			Joins("JOIN transactions ON transactions.id = tickets.transaction_id").
			Where("transactions.status IN ?", []string{"pending", "waiting_verification", "settlement"}).
			Count(&active).Error; err != nil {
			return err
		}
		if active+int64(len(tickets)) > maxQuota {
			return terrors.ErrTicketQuotaExceeded
		}

		if err := tx.Create(transaction).Error; err != nil {
			return err
		}
		if err := tx.Create(&tickets).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *transactionRepository) CountTicketsByTransactionID(transactionID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.Ticket{}).Where("transaction_id = ?", transactionID).Count(&count).Error
	return count, err
}

func (r *transactionRepository) FindByID(id string) (*model.Transaction, error) {
	var transaction model.Transaction
	err := r.db.Where("id = ?", id).First(&transaction).Error
	return &transaction, err
}

func (r *transactionRepository) Update(transaction *model.Transaction) error {
	return r.db.Save(transaction).Error
}

// Implementasi Server-Side Pagination & Filtering
func (r *transactionRepository) FindAllPaginated(page, limit int, search, status string) ([]model.Transaction, int64, error) {
	var transactions []model.Transaction
	var totalRecords int64

	query := r.db.Model(&model.Transaction{}).Preload("Voucher").Preload("Tickets")

	if search != "" {
		searchParam := "%" + search + "%"
		query = query.Where("customer_name ILIKE ? OR customer_email ILIKE ?", searchParam, searchParam)
	}

	if status != "" && status != "all" {
		query = query.Where("status = ?", strings.ToLower(status))
	}

	// Hitung total data (sebelum limit offset) untuk metadata pagination
	query.Count(&totalRecords)

	offset := (page - 1) * limit
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&transactions).Error

	return transactions, totalRecords, err
}

func (r *transactionRepository) GetDashboardStats() (float64, int64, int64, int64, error) {
	var totalRevenue float64
	var totalTickets, scannedTickets, waitingVerification int64

	// Revenue = harga dasar tiket - diskon (TANPA kode unik transfer).
	// COALESCE menangani baris lama yang base_amount-nya 0.
	r.db.Model(&model.Transaction{}).Where("status = ?", "settlement").
		Select("COALESCE(SUM(base_amount - discount_amount), 0)").Scan(&totalRevenue)

	r.db.Model(&model.Ticket{}).
		Joins("JOIN transactions ON transactions.id = tickets.transaction_id").
		Where("transactions.status = ?", "settlement").Count(&totalTickets)

	// Scanned hanya dari tiket pada transaksi settlement (B7).
	r.db.Model(&model.Ticket{}).
		Joins("JOIN transactions ON transactions.id = tickets.transaction_id").
		Where("is_scanned = ? AND transactions.status = ?", true, "settlement").Count(&scannedTickets)

	r.db.Model(&model.Transaction{}).Where("status = ?", "waiting_verification").Count(&waitingVerification)

	return totalRevenue, totalTickets, scannedTickets, waitingVerification, nil
}

func (r *transactionRepository) VerifyAndProcessPayment(transaction *model.Transaction) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		transaction.Status = "settlement"

		// Kuota voucher sudah di-reserve saat checkout (B5), jadi tidak
		// di-increment lagi di sini agar tidak dobel.

		if err := tx.Save(transaction).Error; err != nil {
			return err
		}
		return nil
	})
}

// ExpireStale menandai transaksi pending yang lewat expires_at menjadi
// "expired" dan melepas reservasi kuota voucher-nya.
func (r *transactionRepository) ExpireStale(now time.Time) (int64, error) {
	var expired int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var txs []model.Transaction
		if err := tx.Preload("Tickets").
			Where("status = ? AND expires_at < ?", "pending", now).
			Find(&txs).Error; err != nil {
			return err
		}

		for _, t := range txs {
			if err := tx.Model(&model.Transaction{}).Where("id = ?", t.ID).
				Update("status", "expired").Error; err != nil {
				return err
			}
			if t.VoucherID != nil && len(t.Tickets) > 0 {
				if err := tx.Model(&model.Voucher{}).Where("id = ?", *t.VoucherID).
					UpdateColumn("usage_count", gorm.Expr("GREATEST(usage_count - ?, 0)", len(t.Tickets))).Error; err != nil {
					return err
				}
			}
		}

		expired = int64(len(txs))
		return nil
	})
	return expired, err
}

func (r *transactionRepository) GetTransactionInsights(page, limit int, search, voucherFilter, variantFilter string) ([]model.Transaction, int64, error) {
	var transactions []model.Transaction
	var totalRecords int64

	// Memulai query dasar dari model Transaction
	query := r.db.Model(&model.Transaction{})

	// 1. FILTER SEARCH: Mencari berdasarkan Order ID, Nama Pembeli, atau Email
	if search != "" {
		query = query.Where("transactions.id LIKE ? OR transactions.customer_name LIKE ? OR transactions.customer_email LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// 2. FILTER VOUCHER: Jika spesifik mencari komunitas tertentu
	if voucherFilter != "" {
		// Menggunakan sub-query mencari ID Voucher berdasarkan kode promo
		query = query.Where("voucher_id IN (SELECT id FROM vouchers WHERE code = ?)", strings.ToUpper(voucherFilter))
	}

	// 3. FILTER VARIAN TIKET: Jika spesifik mencari Presale 1, Presale 2, dll
	if variantFilter != "" {
		// Menggunakan sub-query mencari transaksi yang memiliki varian tiket tertentu
		query = query.Where("transactions.id IN (SELECT transaction_id FROM tickets WHERE ticket_variant_id = ?)", variantFilter)
	}

	// Hitung total record sebelum paginasi dijalankan
	if err := query.Count(&totalRecords).Error; err != nil {
		return nil, 0, err
	}

	// Jalankan Eager Loading dengan Preload bertingkat (Nested Preload untuk Tickets.TicketVariant)
	offset := (page - 1) * limit
	err := query.Preload("Voucher").
		Preload("Tickets").
		Preload("Tickets.TicketVariant"). // Menarik relasi varian di dalam tiket
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error

	return transactions, totalRecords, err
}

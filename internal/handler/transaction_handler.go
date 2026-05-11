package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	midtransPkg "github.com/ridwanafazn/smile-fest-api/pkg/midtrans"
	"gorm.io/gorm"
)

type CheckoutInput struct {
	TicketType    string `json:"ticket_type" binding:"required"` // TICKET-PRESALE-1, dll
	CustomerName  string `json:"customer_name" binding:"required"`
	CustomerEmail string `json:"customer_email" binding:"required,email"`
	CustomerPhone string `json:"customer_phone" binding:"required"`
	VoucherCode   string `json:"voucher_code"` // Opsional
}

// Checkout godoc
// @Summary      Create Transaction Checkout
// @Description  Membuat transaksi dan mendapatkan Snap Token dari Midtrans
// @Tags         public
// @Accept       json
// @Produce      json
// @Param        input  body      CheckoutInput  true  "Data Pembeli dan Tiket"
// @Success      200    {object}  map[string]interface{}
// @Router       /api/checkout [post]
func Checkout(c *gin.Context) {
	var input CheckoutInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Tentukan Harga Dasar dari Database
	var ticketVariant model.TicketVariant
	if err := config.DB.Where("id = ?", input.TicketType).First(&ticketVariant).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipe tiket tidak valid atau tidak ditemukan"})
		return
	}

	if !ticketVariant.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tiket ini sedang tidak aktif/belum dijual"})
		return
	}

	basePrice := ticketVariant.Price

	// 2. Cek Voucher (Jika Ada)
	var voucherID *uint
	discount := float64(0)

	if input.VoucherCode != "" {
		var voucher model.Voucher
		if err := config.DB.Where("code = ?", strings.ToUpper(input.VoucherCode)).First(&voucher).Error; err == nil {
			if voucher.IsActive && voucher.UsageCount < voucher.Quota {
				discount = voucher.DiscountAmount
				voucherID = &voucher.ID
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Voucher tidak valid atau kuota habis"})
				return
			}
		}
	}

	finalPrice := basePrice - discount
	if finalPrice < 0 {
		finalPrice = 0 // Mencegah minus
	}

	// 3. Buat Transaksi di Database
	orderID := fmt.Sprintf("SMILE-%d", time.Now().Unix()) // Generate ID unik: SMILE-1715430000

	transaction := model.Transaction{
		ID:            orderID,
		CustomerName:  input.CustomerName,
		CustomerEmail: input.CustomerEmail,
		CustomerPhone: input.CustomerPhone,
		TotalAmount:   finalPrice,
		Status:        "pending",
		VoucherID:     voucherID,
	}

	if err := config.DB.Create(&transaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat transaksi di database"})
		return
	}

	// 4. Request Snap Token ke Midtrans
	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  orderID,
			GrossAmt: int64(finalPrice),
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: input.CustomerName,
			Email: input.CustomerEmail,
			Phone: input.CustomerPhone,
		},
		Items: &[]midtrans.ItemDetails{
			{
				ID:    input.TicketType,
				Price: int64(basePrice),
				Qty:   1,
				Name:  "Tiket SMILE FEST 2026",
			},
		},
	}

	// Masukkan diskon sebagai item pengurang jika ada voucher
	if discount > 0 {
		items := append(*req.Items, midtrans.ItemDetails{
			ID:    "DISCOUNT",
			Price: -int64(discount),
			Qty:   1,
			Name:  "Diskon Voucher",
		})
		req.Items = &items
	}

	snapResp, err := midtransPkg.SnapClient.CreateTransaction(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghubungi Midtrans"})
		return
	}

	// Update SnapToken di database
	config.DB.Model(&transaction).Update("snap_token", snapResp.Token)

	// Kembalikan token ke Frontend untuk memunculkan popup QRIS
	c.JSON(http.StatusOK, gin.H{
		"message":    "Transaksi berhasil dibuat",
		"order_id":   orderID,
		"snap_token": snapResp.Token,
	})
}

// MidtransWebhook godoc
// @Summary      Midtrans Payment Webhook
// @Description  Menerima notifikasi status pembayaran dari server Midtrans
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /api/webhook/midtrans [post]
func MidtransWebhook(c *gin.Context) {
	var notificationPayload map[string]interface{}

	if err := c.ShouldBindJSON(&notificationPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orderID, exists := notificationPayload["order_id"].(string)
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	transactionStatus, _ := notificationPayload["transaction_status"].(string)

	var transaction model.Transaction
	if err := config.DB.Where("id = ?", orderID).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaksi tidak ditemukan"})
		return
	}

	// Update status transaksi berdasarkan notifikasi Midtrans
	if transactionStatus == "settlement" || transactionStatus == "capture" {
		transaction.Status = "settlement" // LUNAS

		// Jika ada voucher yang dipakai, tambahkan UsageCount
		if transaction.VoucherID != nil {
			config.DB.Model(&model.Voucher{}).Where("id = ?", transaction.VoucherID).UpdateColumn("usage_count", gorm.Expr("usage_count + ?", 1))
		}

		// Terbitkan e-ticket karena sudah lunas
		ticket := model.Ticket{
			TransactionID: transaction.ID,
			IsScanned:     false,
		}
		// Abaikan error create ticket untuk webhook response
		_ = config.DB.Create(&ticket)

	} else if transactionStatus == "cancel" || transactionStatus == "expire" || transactionStatus == "deny" {
		transaction.Status = transactionStatus // GAGAL/EXPIRED
	} else if transactionStatus == "pending" {
		transaction.Status = "pending"
	}

	// Simpan perubahan status ke database
	config.DB.Save(&transaction)

	// Selalu kembalikan 200 OK agar Midtrans berhenti mengirim notifikasi ulang
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- KHUSUS ADMIN ---

// GetDashboardStats godoc
// @Summary      Get Dashboard Statistics
// @Description  Mengambil agregasi data pendapatan dan penjualan tiket (Real-time)
// @Tags         admin
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/dashboard [get]
func GetDashboardStats(c *gin.Context) {
	var totalRevenue float64
	var totalTickets int64
	var scannedTickets int64

	// Hitung total pendapatan (hanya dari transaksi lunas)
	config.DB.Model(&model.Transaction{}).Where("status = ?", "settlement").Select("COALESCE(SUM(total_amount), 0)").Scan(&totalRevenue)

	// Hitung total tiket terjual
	config.DB.Model(&model.Ticket{}).Count(&totalTickets)

	// Hitung total tiket yang sudah di-scan di lapangan
	config.DB.Model(&model.Ticket{}).Where("is_scanned = ?", true).Count(&scannedTickets)

	c.JSON(http.StatusOK, gin.H{
		"message": "Berhasil mengambil statistik dashboard",
		"data": gin.H{
			"total_revenue":   totalRevenue,
			"total_tickets":   totalTickets,
			"scanned_tickets": scannedTickets,
		},
	})
}

// GetTransactions godoc
// @Summary      Get All Transactions
// @Description  Melihat daftar riwayat transaksi peserta. Bisa mencari berdasarkan nama atau email.
// @Tags         admin
// @Produce      json
// @Param        search  query     string  false  "Cari nama / email"
// @Success      200     {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/transactions [get]
func GetTransactions(c *gin.Context) {
	search := c.Query("search")
	var transactions []model.Transaction

	// Buat query dasar, preload data Voucher jika ada biar admin tau dia pakai diskon apa
	query := config.DB.Preload("Voucher")

	if search != "" {
		// Pencarian ILIKE untuk PostgreSQL (tidak case-sensitive)
		searchParam := "%" + search + "%"
		query = query.Where("customer_name ILIKE ? OR customer_email ILIKE ?", searchParam, searchParam)
	}

	// Ambil data, urutkan dari yang paling baru beli
	if err := query.Order("created_at desc").Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data transaksi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Berhasil mengambil data transaksi",
		"data":    transactions,
	})
}

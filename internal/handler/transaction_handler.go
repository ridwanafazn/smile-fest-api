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
	"github.com/ridwanafazn/smile-fest-api/pkg/utils" // Import servis email
	"gorm.io/gorm"
)

// TAHAP 3 (Poin 8): Struktur Input Checkout Grup
type Attendee struct {
	Name string `json:"name" binding:"required"`
}

type CheckoutInput struct {
	TicketType    string     `json:"ticket_type" binding:"required"`
	CustomerName  string     `json:"customer_name" binding:"required"`
	CustomerEmail string     `json:"customer_email" binding:"required,email"`
	CustomerPhone string     `json:"customer_phone" binding:"required"`
	VoucherCode   string     `json:"voucher_code"`
	Attendees     []Attendee `json:"attendees" binding:"required,min=1"` // Menampung nama-nama anggota grup
}

// Checkout godoc
// @Summary      Create Transaction Checkout
// @Description  Membuat transaksi dan mendapatkan Snap Token dari Midtrans
// @Tags         public
// @Accept       json
// @Produce      json
// @Param        input  body      CheckoutInput  true  "Data Pembeli dan Daftar Pemegang Tiket"
// @Success      200    {object}  map[string]interface{}
// @Router       /api/checkout [post]
func Checkout(c *gin.Context) {
	var input CheckoutInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var ticketVariant model.TicketVariant
	if err := config.DB.Where("id = ?", input.TicketType).First(&ticketVariant).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipe tiket tidak valid atau tidak ditemukan"})
		return
	}

	if !ticketVariant.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tiket ini sedang tidak aktif/belum dijual"})
		return
	}

	qty := len(input.Attendees)
	basePrice := ticketVariant.Price * float64(qty) // Total Harga = Harga Tiket x Jumlah Orang

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
		finalPrice = 0
	}

	orderID := fmt.Sprintf("SMILE-%d", time.Now().Unix())

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

	// Buat Tiket-Tiket saat pending, status LUNAS-nya tergantung transaksi induk
	var tickets []model.Ticket
	for _, att := range input.Attendees {
		tickets = append(tickets, model.Ticket{
			TransactionID:   orderID,
			TicketVariantID: input.TicketType,
			AttendeeName:    att.Name,
		})
	}
	config.DB.Create(&tickets)

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
				Price: int64(ticketVariant.Price),
				Qty:   int32(qty),
				Name:  ticketVariant.Name,
			},
		},
	}

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

	config.DB.Model(&transaction).Update("snap_token", snapResp.Token)

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

	statusCode, _ := notificationPayload["status_code"].(string)
	grossAmount, _ := notificationPayload["gross_amount"].(string)
	signatureKey, _ := notificationPayload["signature_key"].(string)
	transactionStatus, _ := notificationPayload["transaction_status"].(string)

	if !midtransPkg.VerifySignatureKey(orderID, statusCode, grossAmount, signatureKey) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid signature key"})
		return
	}

	var transaction model.Transaction
	if err := config.DB.Where("id = ?", orderID).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaksi tidak ditemukan"})
		return
	}

	if transaction.Status == "settlement" {
		c.JSON(http.StatusOK, gin.H{"status": "ok, already processed"})
		return
	}

	if transactionStatus == "settlement" || transactionStatus == "capture" {
		transaction.Status = "settlement"

		if transaction.VoucherID != nil {
			config.DB.Model(&model.Voucher{}).Where("id = ?", transaction.VoucherID).UpdateColumn("usage_count", gorm.Expr("usage_count + ?", 1))
		}

		// TAHAP 2 (Poin 1): Eksekusi Email Otomatis menggunakan Goroutine (agar tidak membebani Webhook)
		go func() {
			emailData := utils.EmailData{
				CustomerName: transaction.CustomerName,
				OrderID:      transaction.ID,
				TicketLink:   fmt.Sprintf("https://smile-fest.com/track-ticket?order_id=%s&email=%s&transaction_status=settlement", transaction.ID, transaction.CustomerEmail),
			}
			// Pastikan sudah mengatur SMTP_HOST dll di .env jika ingin email benar-benar terkirim
			_ = utils.SendTicketEmail(transaction.CustomerEmail, emailData)
		}()

	} else if transactionStatus == "cancel" || transactionStatus == "expire" || transactionStatus == "deny" {
		transaction.Status = transactionStatus
	} else if transactionStatus == "pending" {
		transaction.Status = "pending"
	}

	config.DB.Save(&transaction)
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

	config.DB.Model(&model.Transaction{}).Where("status = ?", "settlement").Select("COALESCE(SUM(total_amount), 0)").Scan(&totalRevenue)

	// Hitung hanya tiket yang nempel ke transaksi lunas
	config.DB.Model(&model.Ticket{}).
		Joins("JOIN transactions ON transactions.id = tickets.transaction_id").
		Where("transactions.status = ?", "settlement").
		Count(&totalTickets)

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

	// TAHAP 1 (Poin 3): Preload Tickets untuk mengetahui jumlah tiket dalam pesanan
	query := config.DB.Preload("Voucher").Preload("Tickets")

	if search != "" {
		searchParam := "%" + search + "%"
		query = query.Where("customer_name ILIKE ? OR customer_email ILIKE ?", searchParam, searchParam)
	}

	if err := query.Order("created_at desc").Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data transaksi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Berhasil mengambil data transaksi",
		"data":    transactions,
	})
}

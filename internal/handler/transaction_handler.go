package handler

import (
	"fmt"
	"log" // PERBAIKAN: Menambahkan modul log untuk mencetak pesan ke terminal/Railway
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	midtransPkg "github.com/ridwanafazn/smile-fest-api/pkg/midtrans"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
	"gorm.io/gorm"
)

type Attendee struct {
	Name string `json:"name" binding:"required"`
}

type CheckoutInput struct {
	TicketType    string `json:"ticket_type" binding:"required"`
	CustomerName  string `json:"customer_name" binding:"required"`
	CustomerEmail string `json:"customer_email" binding:"required,email"`
	CustomerPhone string `json:"customer_phone" binding:"required"`

	SurveyAge        string `json:"survey_age"`
	SurveyCity       string `json:"survey_city"`
	SurveyEducation  string `json:"survey_education"`
	SurveyJob        string `json:"survey_job"`
	SurveyMotivation string `json:"survey_motivation"`
	SurveyAction     string `json:"survey_action"`

	VoucherCode string     `json:"voucher_code"`
	Attendees   []Attendee `json:"attendees" binding:"required,min=1"`
}

// Checkout godoc
// @Summary      Create Transaction Checkout
// @Description  Membuat transaksi dan mendapatkan Snap Token dari Midtrans
// @Tags         public
// @Accept       json
// @Produce      json
// @Param        input  body      CheckoutInput  true  "Data Pembeli, Survei, dan Daftar Pemegang Tiket"
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
	basePrice := ticketVariant.Price * float64(qty)

	var voucherID *uint
	discount := float64(0)
	discountPerItem := float64(0)

	if input.VoucherCode != "" {
		var voucher model.Voucher
		if err := config.DB.Where("code = ?", strings.ToUpper(input.VoucherCode)).First(&voucher).Error; err == nil {
			if !voucher.IsActive {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Voucher sedang tidak aktif"})
				return
			}

			if (voucher.Quota - voucher.UsageCount) < qty {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Sisa kuota voucher tidak cukup untuk %d tiket", qty)})
				return
			}

			// Simpan nilai per item untuk invoice Midtrans nanti
			discountPerItem = voucher.DiscountAmount
			discount = discountPerItem * float64(qty)
			voucherID = &voucher.ID
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Voucher tidak valid"})
			return
		}
	}

	finalPrice := basePrice - discount
	if finalPrice < 0 {
		finalPrice = 0
	}

	orderID := fmt.Sprintf("SMILE-%d", time.Now().Unix())

	transaction := model.Transaction{
		ID:               orderID,
		CustomerName:     input.CustomerName,
		CustomerEmail:    input.CustomerEmail,
		CustomerPhone:    input.CustomerPhone,
		SurveyAge:        input.SurveyAge,
		SurveyCity:       input.SurveyCity,
		SurveyEducation:  input.SurveyEducation,
		SurveyJob:        input.SurveyJob,
		SurveyMotivation: input.SurveyMotivation,
		SurveyAction:     input.SurveyAction,
		TotalAmount:      finalPrice,
		Status:           "pending",
		VoucherID:        voucherID,
	}

	if err := config.DB.Create(&transaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat transaksi di database"})
		return
	}

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
			Price: -int64(discountPerItem),
			Qty:   int32(qty),
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
			var qty int64
			config.DB.Model(&model.Ticket{}).Where("transaction_id = ?", transaction.ID).Count(&qty)
			if qty > 0 {
				config.DB.Model(&model.Voucher{}).Where("id = ?", transaction.VoucherID).UpdateColumn("usage_count", gorm.Expr("usage_count + ?", qty))
			}
		}

		// PERBAIKAN: Menangkap dan mencetak error pengiriman email ke Log
		go func() {
			emailData := utils.EmailData{
				CustomerName: transaction.CustomerName,
				OrderID:      transaction.ID,
				TicketLink:   fmt.Sprintf("https://smile-festival.pages.dev/track-ticket?order_id=%s&email=%s&transaction_status=settlement", transaction.ID, transaction.CustomerEmail),
			}

			err := utils.SendTicketEmail(transaction.CustomerEmail, emailData)
			if err != nil {
				log.Printf("❌ [EMAIL ERROR] Gagal mengirim tiket ke %s. Pesan Error: %v\n", transaction.CustomerEmail, err)
			} else {
				log.Printf("✅ [EMAIL SUCCESS] Tiket berhasil dikirim ke %s\n", transaction.CustomerEmail)
			}
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
// @Description  Melihat daftar riwayat transaksi peserta.
// @Tags         admin
// @Produce      json
// @Param        search  query     string  false  "Cari nama / email"
// @Success      200     {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/transactions [get]
func GetTransactions(c *gin.Context) {
	search := c.Query("search")
	var transactions []model.Transaction

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

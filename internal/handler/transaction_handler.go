package handler

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/pkg/cloudinary"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils" // Import utils untuk memanggil fungsi pengirim email
)

type Attendee struct {
	Name string `json:"name" binding:"required"`
}

type CheckoutInput struct {
	TicketType     string `json:"ticket_type" binding:"required"`
	CustomerName   string `json:"customer_name" binding:"required"`
	CustomerEmail  string `json:"customer_email" binding:"required,email"`
	CustomerPhone  string `json:"customer_phone" binding:"required"`
	CustomerGender string `json:"customer_gender" binding:"required"` // Ikhwan / Akhwat

	// Data Profil
	ProfileAge           string `json:"profile_age"`
	ProfileCity          string `json:"profile_city"`
	ProfileEducation     string `json:"profile_education"`
	ProfileJob           string `json:"profile_job"`
	CommunityAffiliation string `json:"community_affiliation"` // Komunitas/Instansi
	InformationSource    string `json:"information_source"`    // Tahu acara dari mana

	// Kuesioner (Multiple Choice - Array of Strings)
	InterestReasons     []string `json:"interest_reasons"`
	SustainabilitySteps []string `json:"sustainability_steps"`

	ContributionRole string `json:"contribution_role"`

	VoucherCode string     `json:"voucher_code"`
	Attendees   []Attendee `json:"attendees" binding:"required,min=1"`
}

// Checkout godoc
// @Summary      Create Transaction Checkout (Manual Transfer)
// @Description  Membuat transaksi. Mengembalikan 409 Conflict beserta state order_id jika email masih memiliki transaksi menggantung.
// @Tags         public
// @Accept       json
// @Produce      json
// @Param        input  body      CheckoutInput  true  "Data Pembeli dan Daftar Pemegang Tiket"
// @Success      200    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{} "Terdapat transaksi menggantung, kembalikan order_id"
// @Router       /api/checkout [post]
func Checkout(c *gin.Context) {
	var input CheckoutInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// --- 1. LOGIKA ANTI-DUPLIKAT (STATE-BASED RESPONSE) ---
	var existingOrder model.Transaction
	// Perhatikan: "settlement" dihapus! Jadi orang yang sudah lunas bisa beli lagi.
	if err := config.DB.Where("customer_email = ? AND status IN ?", input.CustomerEmail, []string{"pending", "waiting_verification"}).
		Order("created_at desc").First(&existingOrder).Error; err == nil {

		// Lempar HTTP 409 Conflict berserta payload order_id lama agar Frontend bisa mengarahkan user
		c.JSON(http.StatusConflict, gin.H{
			"error":    "Kamu masih memiliki pesanan yang belum diselesaikan.",
			"order_id": existingOrder.ID,
			"status":   existingOrder.Status,
		})
		return
	}
	// ---------------------------------------------------------

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

	// --- LOGIKA AUTO-BATCHING (600 KUOTA TOTAL) ---
	var currentActiveTickets int64
	config.DB.Model(&model.Ticket{}).
		Joins("JOIN transactions ON transactions.id = tickets.transaction_id").
		Where("transactions.status IN ?", []string{"pending", "waiting_verification", "settlement"}).
		Count(&currentActiveTickets)

	if currentActiveTickets+int64(qty) > 600 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mohon maaf, kuota total tiket sudah habis."})
		return
	}

	sessionBatch := 1
	if currentActiveTickets >= 300 {
		sessionBatch = 2
	}

	// Perhitungan Harga & Diskon
	basePrice := ticketVariant.Price * float64(qty)
	var voucherID *uint
	discount := float64(0)

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
			discount = voucher.DiscountAmount * float64(qty)
			voucherID = &voucher.ID
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Voucher tidak valid"})
			return
		}
	}

	// --- LOGIKA KODE UNIK (100 - 999) ---
	uniqueCode := rand.Intn(900) + 100

	finalPrice := basePrice - discount
	if finalPrice < 0 {
		finalPrice = 0
	}
	totalTransfer := finalPrice + float64(uniqueCode)

	orderID := fmt.Sprintf("SMILE-%d", time.Now().Unix())
	expiresAt := time.Now().Add(24 * time.Hour)

	interestStr := strings.Join(input.InterestReasons, ", ")
	stepsStr := strings.Join(input.SustainabilitySteps, ", ")

	transaction := model.Transaction{
		ID:                   orderID,
		CustomerName:         input.CustomerName,
		CustomerEmail:        input.CustomerEmail,
		CustomerPhone:        input.CustomerPhone,
		CustomerGender:       input.CustomerGender,
		ProfileAge:           input.ProfileAge,
		ProfileCity:          input.ProfileCity,
		ProfileEducation:     input.ProfileEducation,
		ProfileJob:           input.ProfileJob,
		CommunityAffiliation: input.CommunityAffiliation,
		InformationSource:    input.InformationSource,
		InterestReasons:      interestStr,
		SustainabilitySteps:  stepsStr,
		ContributionRole:     input.ContributionRole,
		TotalAmount:          totalTransfer,
		UniqueCode:           uniqueCode,
		SessionBatch:         sessionBatch,
		ExpiresAt:            expiresAt,
		Status:               "pending",
		VoucherID:            voucherID,
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

	// --- LOGIKA EMAIL ---
	formattedAmount := fmt.Sprintf("Rp %s", utils.FormatRupiah(totalTransfer))
	trackLink := fmt.Sprintf("https://smile-festival.pages.dev/track-ticket?order_id=%s&email=%s", orderID, input.CustomerEmail)

	go func() {
		err := utils.SendInstructionEmail(input.CustomerEmail, utils.InstructionData{
			CustomerName: input.CustomerName,
			OrderID:      orderID,
			TrackLink:    trackLink,
			TotalAmount:  formattedAmount,
		})
		if err != nil {
			log.Printf("❌ [EMAIL ERROR] Gagal mengirim instruksi ke %s: %v\n", input.CustomerEmail, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message":       "Transaksi berhasil dibuat",
		"order_id":      orderID,
		"total_amount":  totalTransfer,
		"unique_code":   uniqueCode,
		"session_batch": sessionBatch,
		"expires_at":    expiresAt,
	})
}

// CancelTransaction godoc
// @Summary      Cancel Pending Transaction
// @Description  Membatalkan transaksi yang masih pending (fitur user dari Frontend jika ingin ubah pesanan)
// @Tags         public
// @Produce      json
// @Param        id   path      string  true  "Order ID"
// @Success      200  {object}  map[string]interface{}
// @Router       /api/transactions/{id}/cancel [put]
func CancelTransaction(c *gin.Context) {
	orderID := c.Param("id")
	var transaction model.Transaction

	if err := config.DB.Where("id = ?", orderID).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaksi tidak ditemukan"})
		return
	}

	if transaction.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya transaksi yang belum dibayar (pending) yang dapat dibatalkan"})
		return
	}

	transaction.Status = "cancel"
	if err := config.DB.Save(&transaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membatalkan transaksi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Pesanan lama berhasil dibatalkan.",
	})
}

// UploadProof godoc
// ... (Sisa fungsi UploadProof, GetDashboardStats, GetTransactions tetap sama persis seperti kode lamamu)
func UploadProof(c *gin.Context) {
	orderID := c.Param("id")

	var transaction model.Transaction
	if err := config.DB.Where("id = ?", orderID).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaksi tidak ditemukan"})
		return
	}

	if transaction.Status != "pending" && transaction.Status != "waiting_verification" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Transaksi tidak dapat diunggah ulang (status: " + transaction.Status + ")"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File bukti transfer tidak ditemukan dalam request"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca file"})
		return
	}
	defer file.Close()

	// Upload ke Cloudinary
	imageURL, err := cloudinary.UploadImage(c.Request.Context(), file, orderID)
	if err != nil {
		log.Printf("❌ [CLOUDINARY ERROR] %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengunggah gambar ke server"})
		return
	}

	transaction.PaymentProofURL = imageURL
	transaction.Status = "waiting_verification"

	if err := config.DB.Save(&transaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan URL gambar ke database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Bukti pembayaran berhasil diunggah, menunggu verifikasi Admin",
		"payment_proof_url": imageURL,
		"status":            transaction.Status,
	})
}

// GetDashboardStats godoc
func GetDashboardStats(c *gin.Context) {
	var totalRevenue float64
	var totalTickets int64
	var scannedTickets int64
	var waitingVerification int64

	config.DB.Model(&model.Transaction{}).Where("status = ?", "settlement").Select("COALESCE(SUM(total_amount), 0)").Scan(&totalRevenue)

	config.DB.Model(&model.Ticket{}).
		Joins("JOIN transactions ON transactions.id = tickets.transaction_id").
		Where("transactions.status = ?", "settlement").
		Count(&totalTickets)

	config.DB.Model(&model.Ticket{}).Where("is_scanned = ?", true).Count(&scannedTickets)

	// Tambahan untuk memantau beban kerja Admin
	config.DB.Model(&model.Transaction{}).Where("status = ?", "waiting_verification").Count(&waitingVerification)

	c.JSON(http.StatusOK, gin.H{
		"message": "Berhasil mengambil statistik dashboard",
		"data": gin.H{
			"total_revenue":        totalRevenue,
			"total_tickets":        totalTickets,
			"scanned_tickets":      scannedTickets,
			"waiting_verification": waitingVerification,
		},
	})
}

// GetTransactions godoc
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

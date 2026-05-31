package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
)

// --- KHUSUS PUBLIK & ADMIN (MIXED) ---

// GetTicketInfo godoc
// @Summary      Get Ticket Info
// @Description  Mengambil data harga dan tipe tiket. Publik hanya melihat yang aktif, Admin melihat semua.
// @Tags         public
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /api/tickets/info [get]
func GetTicketInfo(c *gin.Context) {
	var ticketVariants []model.TicketVariant

	// Cek apakah request datang dari Admin (memiliki header Authorization)
	authHeader := c.GetHeader("Authorization")
	isAdminRequest := authHeader != "" && strings.HasPrefix(authHeader, "Bearer ")

	if isAdminRequest {
		// Admin: Ambil SEMUA tiket, urutkan dari yang terbaru
		if err := config.DB.Order("created_at desc").Find(&ticketVariants).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data tiket dari database"})
			return
		}
	} else {
		// Publik: Hanya ambil tiket yang aktif DAN masuk dalam rentang tanggal
		now := time.Now()
		if err := config.DB.Where("is_active = ? AND start_date <= ? AND end_date >= ?", true, now, now).Find(&ticketVariants).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data tiket dari database"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Berhasil mengambil data tiket",
		"data":    ticketVariants,
	})
}

// TrackTicket godoc
// @Summary      Track E-Ticket & Payment Status
// @Description  Peserta mencari tiket mereka, mengecek status verifikasi admin, atau melihat instruksi pembayaran manual (jika status masih pending).
// @Tags         public
// @Produce      json
// @Param        order_id  query     string  true  "Order ID (SMILE-xxx)"
// @Param        email     query     string  true  "Email Peserta"
// @Success      200       {object}  map[string]interface{}
// @Router       /api/tickets/track [get]
func TrackTicket(c *gin.Context) {
	orderID := c.Query("order_id")
	email := c.Query("email")

	if orderID == "" || email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID dan Email wajib diisi"})
		return
	}

	var transaction model.Transaction
	// Gunakan Preload("Tickets") agar data array pemegang tiket ikut terbawa
	if err := config.DB.Preload("Tickets").Where("id = ? AND customer_email = ?", orderID, email).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data tidak ditemukan. Pastikan Order ID dan Email benar."})
		return
	}

	// Mengembalikan seluruh objek transaksi ke Frontend.
	// Golang akan otomatis mengubah field struct (TotalAmount, SessionBatch, dll)
	// menjadi JSON keys (total_amount, session_batch, dll) sesuai tag json:"..." di model.
	c.JSON(http.StatusOK, gin.H{
		"message": "Detail transaksi berhasil ditemukan",
		"data":    transaction,
	})
}

// --- KHUSUS ADMIN ---

type TicketVariantInput struct {
	Name      string     `json:"name" binding:"required"`
	Price     float64    `json:"price" binding:"required,min=0"`
	Quota     int        `json:"quota" binding:"required,min=1"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
}

// CreateTicketVariant godoc
// @Summary      Create New Ticket Variant
// @Description  Admin membuat gelombang tiket baru
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        input  body      TicketVariantInput  true  "Data Gelombang Tiket"
// @Success      201    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants [post]
func CreateTicketVariant(c *gin.Context) {
	var input TicketVariantInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Atur default tanggal jika kosong
	now := time.Now()
	startDate := now
	if input.StartDate != nil {
		startDate = *input.StartDate
	}

	// Jika end_date kosong, set 10 tahun dari sekarang (seolah-olah unlimited)
	endDate := now.AddDate(10, 0, 0)
	if input.EndDate != nil {
		endDate = *input.EndDate
	}

	variant := model.TicketVariant{
		ID:        uuid.New().String(),
		Name:      input.Name,
		Price:     input.Price,
		IsActive:  false, // Otomatis nonaktif saat baru dibuat (standar keamanan)
		StartDate: startDate,
		EndDate:   endDate,
		// Asumsi kita menggunakan properti Description sementara untuk menampung Quota sebelum skema DB diperbarui
		// Description: fmt.Sprintf(`{"quota": %d}`, input.Quota),
	}

	if err := config.DB.Create(&variant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan gelombang tiket ke database"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Gelombang tiket berhasil ditambahkan",
		"data":    variant,
	})
}

// UpdateTicketVariant godoc
// @Summary      Update Ticket Variant
// @Description  Admin mengubah harga atau periode gelombang tiket
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id     path      string              true  "Variant ID"
// @Param        input  body      TicketVariantInput  true  "Data Update Gelombang Tiket"
// @Success      200    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants/{id} [put]
func UpdateTicketVariant(c *gin.Context) {
	id := c.Param("id")
	var input TicketVariantInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var variant model.TicketVariant
	if err := config.DB.Where("id = ?", id).First(&variant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tipe tiket tidak ditemukan"})
		return
	}

	variant.Name = input.Name
	variant.Price = input.Price

	if input.StartDate != nil {
		variant.StartDate = *input.StartDate
	}
	if input.EndDate != nil {
		variant.EndDate = *input.EndDate
	}

	if err := config.DB.Save(&variant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui data tiket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Gelombang tiket berhasil diperbarui",
		"data":    variant,
	})
}

// DeleteTicketVariant godoc
// @Summary      Delete Ticket Variant
// @Description  Admin menghapus gelombang tiket (Hanya jika belum ada transaksi terkait)
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "Variant ID"
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants/{id} [delete]
func DeleteTicketVariant(c *gin.Context) {
	id := c.Param("id")

	// Cegah hapus tiket jika sudah ada transaksi (mencegah Foreign Key Error)
	var count int64
	config.DB.Model(&model.Ticket{}).Where("ticket_variant_id = ?", id).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Tidak dapat menghapus tiket karena sudah ada transaksi yang membelinya. Sebaiknya nonaktifkan saja tiket ini."})
		return
	}

	if err := config.DB.Where("id = ?", id).Delete(&model.TicketVariant{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus tiket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Gelombang tiket berhasil dihapus permanen",
	})
}

// ToggleTicketVariant godoc
// @Summary      Toggle Ticket Variant Status
// @Description  Admin membuka atau menutup penjualan fase tiket secara manual (Kill switch)
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "Variant ID"
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants/{id}/toggle [put]
func ToggleTicketVariant(c *gin.Context) {
	id := c.Param("id")
	var variant model.TicketVariant

	if err := config.DB.Where("id = ?", id).First(&variant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tipe tiket tidak ditemukan"})
		return
	}

	variant.IsActive = !variant.IsActive
	config.DB.Save(&variant)

	status := "ditutup"
	if variant.IsActive {
		status = "dibuka"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("Penjualan %s berhasil %s", variant.Name, status),
		"is_active": variant.IsActive,
	})
}

// --- KHUSUS SCANNER ---

// ValidateTicket godoc
// @Summary      Validate Ticket
// @Description  Memvalidasi QR Code dari tiket peserta di hari H
// @Tags         scanner
// @Accept       json
// @Produce      json
// @Param        input  body      map[string]string  true  "Ticket UUID"
// @Success      200    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/scanner/validate-ticket [post]
func ValidateTicket(c *gin.Context) {
	var input struct {
		TicketID string `json:"ticket_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format request tidak valid"})
		return
	}

	var ticket model.Ticket
	if err := config.DB.Preload("Transaction").Where("id = ?", input.TicketID).First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "X QR Code tidak valid atau tiket tidak ditemukan"})
		return
	}

	if ticket.IsScanned {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "TIKET SUDAH DIGUNAKAN!",
			"scanned_at":    ticket.ScannedAt,
			"customer_name": ticket.AttendeeName,
		})
		return
	}

	now := time.Now()
	ticket.IsScanned = true
	ticket.ScannedAt = &now

	if err := config.DB.Save(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status tiket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Validasi berhasil! Akses diizinkan.",
		"customer_name": ticket.AttendeeName,
		"ticket_id":     ticket.ID,
	})
}

// GetScannerStats godoc
// @Summary      Get Scanner Stats
// @Description  Panitia lapangan melihat jumlah peserta yang sudah masuk
// @Tags         scanner
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/scanner/stats [get]
func GetScannerStats(c *gin.Context) {
	var totalTickets int64
	var scannedTickets int64

	config.DB.Model(&model.Ticket{}).
		Joins("JOIN transactions ON transactions.id = tickets.transaction_id").
		Where("transactions.status = ?", "settlement").
		Count(&totalTickets)

	config.DB.Model(&model.Ticket{}).Where("is_scanned = ?", true).Count(&scannedTickets)

	c.JSON(http.StatusOK, gin.H{
		"message": "Statistik antrean",
		"data": gin.H{
			"total_tickets":   totalTickets,
			"scanned_tickets": scannedTickets,
			"remaining":       totalTickets - scannedTickets,
		},
	})
}

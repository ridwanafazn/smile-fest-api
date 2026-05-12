package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
)

// --- KHUSUS PUBLIK ---

// GetTicketInfo godoc
// @Summary      Get Ticket Info
// @Description  Mengambil data harga dan tipe tiket yang sedang aktif berdasarkan periode tanggal hari ini
// @Tags         public
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /api/tickets/info [get]
func GetTicketInfo(c *gin.Context) {
	var ticketVariants []model.TicketVariant

	now := time.Now()
	// TAHAP 3 (Poin 7): Filter ketat berdasarkan waktu. Tiket hanya muncul jika hari ini berada di antara start_date dan end_date.
	if err := config.DB.Where("is_active = ? AND start_date <= ? AND end_date >= ?", true, now, now).Find(&ticketVariants).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data tiket dari database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Berhasil mengambil data tiket",
		"data":    ticketVariants,
	})
}

// TrackTicket godoc
// @Summary      Track E-Ticket
// @Description  Peserta mencari tiket grup mereka jika lupa/tidak dapat email
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
	// Preload "Tickets" karena sekarang 1 transaksi punya BANYAK tiket
	if err := config.DB.Preload("Tickets").Where("id = ? AND customer_email = ?", orderID, email).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data tidak ditemukan. Pastikan Order ID dan Email benar."})
		return
	}

	if transaction.Status != "settlement" {
		c.JSON(http.StatusOK, gin.H{
			"message": "Status transaksi ditemukan",
			"data": gin.H{
				"order_id":      transaction.ID,
				"customer_name": transaction.CustomerName,
				"status":        transaction.Status,
			},
		})
		return
	}

	// TAHAP 3 (Poin 8): Menyesuaikan respon untuk multi-tiket.
	// Menyediakan 'ticket_uuid' pertama (sebagai backward compatibility ke FE yang lama)
	// dan array 'tickets' untuk render banyak QR code di FE yang baru.
	firstTicketUUID := ""
	if len(transaction.Tickets) > 0 {
		firstTicketUUID = transaction.Tickets[0].ID.String()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tiket berhasil ditemukan",
		"data": gin.H{
			"order_id":      transaction.ID,
			"customer_name": transaction.CustomerName,
			"ticket_uuid":   firstTicketUUID,
			"tickets":       transaction.Tickets, // Array lengkap untuk semua peserta grup
			"status":        transaction.Status,
		},
	})
}

// --- KHUSUS ADMIN ---

// ToggleTicketVariant godoc
// @Summary      Toggle Ticket Variant Status
// @Description  Admin membuka atau menutup penjualan fase tiket secara manual (Kill switch)
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "Variant ID"
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants/{id} [put]
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
			"customer_name": ticket.AttendeeName, // Kini menyapa nama spesifik pemegang tiket, bukan si pembeli
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

	// Hanya hitung tiket yang pembayarannya LUNAS
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

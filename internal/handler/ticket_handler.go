package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
)

// GetTicketInfo sekarang mengambil data dinamis langsung dari Database
func GetTicketInfo(c *gin.Context) {
	var ticketVariants []model.TicketVariant

	// Ambil semua data tiket dari database
	if err := config.DB.Find(&ticketVariants).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data tiket dari database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Berhasil mengambil data tiket",
		"data":    ticketVariants,
	})
}

// ValidateTicket diakses oleh aplikasi Scanner di hari H
func ValidateTicket(c *gin.Context) {
	var input struct {
		TicketID string `json:"ticket_id" binding:"required"` // Berisi UUID dari QR Code
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format request tidak valid"})
		return
	}

	var ticket model.Ticket
	// Preload transaksi supaya kita bisa ambil nama pemesannya (biar panitia bisa nyapa orangnya)
	if err := config.DB.Preload("Transaction").Where("id = ?", input.TicketID).First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "X QR Code tidak valid atau tiket tidak ditemukan"})
		return
	}

	// Cek apakah tiket sudah pernah di-scan sebelumnya (Mencegah tiket dobel)
	if ticket.IsScanned {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "TIKET SUDAH DIGUNAKAN!",
			"scanned_at":    ticket.ScannedAt,
			"customer_name": ticket.Transaction.CustomerName,
		})
		return
	}

	// Lolos pengecekan: Update status tiket jadi scanned
	now := time.Now()
	ticket.IsScanned = true
	ticket.ScannedAt = &now

	if err := config.DB.Save(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status tiket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Validasi berhasil! Akses diizinkan.",
		"customer_name": ticket.Transaction.CustomerName,
		"ticket_id":     ticket.ID,
	})
}

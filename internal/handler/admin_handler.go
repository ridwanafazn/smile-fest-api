package handler

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
	"gorm.io/gorm"
)

type VerifyPaymentInput struct {
	Action string `json:"action" binding:"required,oneof=approve reject"`
}

// VerifyPayment godoc
// @Summary      Verify Manual Payment
// @Description  Admin melakukan persetujuan (approve) atau penolakan (reject) terhadap bukti transfer peserta. Approve akan mengirimkan E-Ticket otomatis.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id     path      string              true  "Order ID (Transaction ID)"
// @Param        input  body      VerifyPaymentInput  true  "Aksi Verifikasi (approve/reject)"
// @Success      200    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/transactions/{id}/verify [put]
func VerifyPayment(c *gin.Context) {
	orderID := c.Param("id")
	var input VerifyPaymentInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Aksi tidak valid. Harus 'approve' atau 'reject'"})
		return
	}

	var transaction model.Transaction
	if err := config.DB.Where("id = ?", orderID).First(&transaction).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaksi tidak ditemukan"})
		return
	}

	if transaction.Status == "settlement" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Transaksi ini sudah disetujui sebelumnya"})
		return
	}

	if input.Action == "reject" {
		transaction.Status = "cancel"
		config.DB.Save(&transaction)
		c.JSON(http.StatusOK, gin.H{"message": "Pembayaran ditolak. Status menjadi cancel."})
		return
	}

	// --- JIKA ACTION == "approve" ---
	transaction.Status = "settlement"

	// Potong kuota voucher jika transaksi menggunakan voucher
	if transaction.VoucherID != nil {
		var qty int64
		config.DB.Model(&model.Ticket{}).Where("transaction_id = ?", transaction.ID).Count(&qty)
		if qty > 0 {
			config.DB.Model(&model.Voucher{}).Where("id = ?", transaction.VoucherID).UpdateColumn("usage_count", gorm.Expr("usage_count + ?", qty))
		}
	}

	config.DB.Save(&transaction)

	// Trigger Pengiriman Email Tiket via GAS (Berjalan di background via goroutine)
	go func() {
		emailData := utils.EmailData{
			CustomerName: transaction.CustomerName,
			OrderID:      transaction.ID,
			TicketLink:   fmt.Sprintf("https://smile-festival.pages.dev/track-ticket?order_id=%s&email=%s", transaction.ID, transaction.CustomerEmail),
		}

		err := utils.SendTicketEmail(transaction.CustomerEmail, emailData)
		if err != nil {
			log.Printf("❌ [EMAIL ERROR] Gagal mengirim tiket ke %s. Pesan Error: %v\n", transaction.CustomerEmail, err)
		} else {
			log.Printf("✅ [EMAIL SUCCESS] Tiket berhasil dikirim ke %s\n", transaction.CustomerEmail)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Pembayaran berhasil diverifikasi. E-Ticket sedang dikirim ke email peserta.",
		"status":  "settlement",
	})
}

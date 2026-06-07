package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/service"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

type TransactionHandler struct {
	trxService service.TransactionService
}

func NewTransactionHandler(trxService service.TransactionService) *TransactionHandler {
	return &TransactionHandler{trxService}
}

// Checkout godoc
func (h *TransactionHandler) Checkout(c *gin.Context) {
	var input service.CheckoutInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Data yang dikirim tidak valid", err.Error())
		return
	}

	transaction, err := h.trxService.Checkout(input)
	if err != nil {
		// Menangani Edge Case State-Based 409 Anti-Duplicate
		if err.Error() == "PENDING_TRANSACTION_EXISTS" {
			utils.ErrorResult(c, http.StatusConflict, "Kamu masih memiliki pesanan yang belum diselesaikan.", gin.H{
				"order_id": transaction.ID,
				"status":   transaction.Status,
			})
			return
		}
		utils.ErrorResult(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Transaksi berhasil dibuat", gin.H{
		"order_id":      transaction.ID,
		"total_amount":  transaction.TotalAmount,
		"unique_code":   transaction.UniqueCode,
		"session_batch": transaction.SessionBatch,
		"expires_at":    transaction.ExpiresAt,
	})
}

// CancelTransaction godoc
func (h *TransactionHandler) CancelTransaction(c *gin.Context) {
	orderID := c.Param("id")
	if err := h.trxService.CancelTransaction(orderID); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	utils.SuccessResult(c, http.StatusOK, "Pesanan lama berhasil dibatalkan.", nil)
}

// UploadProof godoc
func (h *TransactionHandler) UploadProof(c *gin.Context) {
	orderID := c.Param("id")

	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "File bukti transfer tidak ditemukan", nil)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, "Gagal membaca file", nil)
		return
	}
	defer file.Close()

	imageURL, status, err := h.trxService.UploadProof(c.Request.Context(), orderID, file)
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Bukti pembayaran berhasil diunggah", gin.H{
		"payment_proof_url": imageURL,
		"status":            status,
	})
}

// GetDashboardStats godoc
func (h *TransactionHandler) GetDashboardStats(c *gin.Context) {
	stats, err := h.trxService.GetDashboardStats()
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	utils.SuccessResult(c, http.StatusOK, "Berhasil mengambil statistik dashboard", stats)
}

// GetTransactions godoc
// @Summary      Get Transactions (Paginated)
// @Description  Admin melihat riwayat transaksi dengan fitur paginasi dan filter
func (h *TransactionHandler) GetTransactions(c *gin.Context) {
	search := c.Query("search")
	status := c.Query("status") // ex: "waiting_verification", "settlement", "pending"

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	transactions, meta, err := h.trxService.GetPaginatedTransactions(page, limit, search, status)
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	utils.PaginatedResult(c, http.StatusOK, "Berhasil mengambil data transaksi", transactions, meta)
}

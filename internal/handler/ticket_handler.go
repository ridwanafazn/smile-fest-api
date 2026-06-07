package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/service"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

type TicketHandler struct {
	ticketService service.TicketService
}

func NewTicketHandler(ticketService service.TicketService) *TicketHandler {
	return &TicketHandler{ticketService}
}

// --- KHUSUS PUBLIK ---

// GetTicketInfo godoc
// @Summary      Get Ticket Info
// @Description  Mengambil data harga dan tipe tiket yang sedang aktif untuk publik.
// @Tags         public
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse
// @Router       /api/tickets/info [get]
func (h *TicketHandler) GetTicketInfo(c *gin.Context) {
	variants, err := h.ticketService.GetActiveTicketVariants()
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, "Gagal mengambil data tiket", err.Error())
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Berhasil mengambil data tiket", variants)
}

// TrackTicket godoc
// @Summary      Track E-Ticket & Payment Status
// @Description  Peserta mencari tiket mereka, mengecek status verifikasi admin, atau melihat instruksi pembayaran manual.
// @Tags         public
// @Produce      json
// @Param        order_id  query     string  true  "Order ID (SMILE-xxx)"
// @Param        email     query     string  true  "Email Peserta"
// @Success      200       {object}  utils.SuccessResponse
// @Router       /api/tickets/track [get]
func (h *TicketHandler) TrackTicket(c *gin.Context) {
	orderID := c.Query("order_id")
	email := c.Query("email")

	if orderID == "" || email == "" {
		utils.ErrorResult(c, http.StatusBadRequest, "Order ID dan Email wajib diisi", nil)
		return
	}

	transaction, err := h.ticketService.TrackTicket(orderID, email)
	if err != nil {
		utils.ErrorResult(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Detail transaksi berhasil ditemukan", transaction)
}

// --- KHUSUS ADMIN ---

// GetAdminTicketVariants godoc
// @Summary      Get All Ticket Variants (Admin)
// @Description  Mengambil seluruh data tipe tiket tanpa peduli status aktif atau periode tanggal.
// @Tags         admin
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants [get]
func (h *TicketHandler) GetAdminTicketVariants(c *gin.Context) {
	variants, err := h.ticketService.GetAllAdminVariants()
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, "Gagal mengambil seluruh data tiket dari database", err.Error())
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Berhasil mengambil seluruh data tiket (Admin)", variants)
}

// CreateTicketVariant godoc
// @Summary      Create New Ticket Variant
// @Description  Admin membuat gelombang tiket baru
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        input  body      service.TicketVariantInput  true  "Data Gelombang Tiket"
// @Success      201    {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants [post]
func (h *TicketHandler) CreateTicketVariant(c *gin.Context) {
	var input service.TicketVariantInput

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Data yang dikirim tidak valid", err.Error())
		return
	}

	variant, err := h.ticketService.CreateVariant(input)
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusCreated, "Gelombang tiket berhasil ditambahkan", variant)
}

// UpdateTicketVariant godoc
// @Summary      Update Ticket Variant
// @Description  Admin mengubah harga atau periode gelombang tiket
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id     path      string                      true  "Variant ID"
// @Param        input  body      service.TicketVariantInput  true  "Data Update Gelombang Tiket"
// @Success      200    {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants/{id} [put]
func (h *TicketHandler) UpdateTicketVariant(c *gin.Context) {
	id := c.Param("id")
	var input service.TicketVariantInput

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Data yang dikirim tidak valid", err.Error())
		return
	}

	variant, err := h.ticketService.UpdateVariant(id, input)
	if err != nil {
		utils.ErrorResult(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Gelombang tiket berhasil diperbarui", variant)
}

// DeleteTicketVariant godoc
// @Summary      Delete Ticket Variant
// @Description  Admin menghapus gelombang tiket (Hanya jika belum ada transaksi terkait)
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "Variant ID"
// @Success      200  {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants/{id} [delete]
func (h *TicketHandler) DeleteTicketVariant(c *gin.Context) {
	id := c.Param("id")

	err := h.ticketService.DeleteVariant(id)
	if err != nil {
		utils.ErrorResult(c, http.StatusConflict, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Gelombang tiket berhasil dihapus permanen", nil)
}

// ToggleTicketVariant godoc
// @Summary      Toggle Ticket Variant Status
// @Description  Admin membuka atau menutup penjualan fase tiket secara manual (Kill switch)
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "Variant ID"
// @Success      200  {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/ticket-variants/{id}/toggle [put]
func (h *TicketHandler) ToggleTicketVariant(c *gin.Context) {
	id := c.Param("id")

	variant, statusStr, err := h.ticketService.ToggleVariantStatus(id)
	if err != nil {
		utils.ErrorResult(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, fmt.Sprintf("Penjualan %s berhasil %s", variant.Name, statusStr), gin.H{
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
// @Success      200    {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/scanner/validate-ticket [post]
func (h *TicketHandler) ValidateTicket(c *gin.Context) {
	var input struct {
		TicketID string `json:"ticket_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Format request tidak valid", err.Error())
		return
	}

	ticket, err := h.ticketService.ValidateTicket(input.TicketID)
	if err != nil {
		if ticket != nil && ticket.IsScanned {
			name := ticket.AttendeeName
			if name == "" {
				name = "Peserta (Data Induk)"
			}

			timeStr := ""
			if ticket.ScannedAt != nil {
				timeStr = ticket.ScannedAt.Format("2006-01-02 15:04:05")
			}

			fullMessage := fmt.Sprintf("Tiket atas nama %s sudah dipindai pada %s", name, timeStr)

			utils.ErrorResult(c, http.StatusBadRequest, fullMessage, gin.H{
				"customer_name": name,
				"scanned_at":    timeStr,
			})
			return
		}

		utils.ErrorResult(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Validasi berhasil! Akses diizinkan.", gin.H{
		"customer_name": ticket.AttendeeName,
		"ticket_id":     ticket.ID,
	})
}

// GetScannerStats godoc
// @Summary      Get Scanner Stats
// @Description  Panitia lapangan melihat jumlah peserta yang sudah masuk
// @Tags         scanner
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/scanner/stats [get]
func (h *TicketHandler) GetScannerStats(c *gin.Context) {
	stats, err := h.ticketService.GetScannerStats()
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Statistik antrean", stats)
}

// GetScannerHistory godoc
// @Summary      Get Global Scanner History
// @Description  Mengambil daftar riwayat pemindaian tiket secara global
// @Tags         scanner
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/scanner/history [get]
func (h *TicketHandler) GetScannerHistory(c *gin.Context) {
	history, err := h.ticketService.GetScannerHistory()
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Berhasil mengambil riwayat pemindaian global", history)
}

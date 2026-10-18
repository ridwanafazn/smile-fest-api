package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/service"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

type AdminHandler struct {
	trxService  service.TransactionService
	userService service.UserService // Tambahan: Injeksi UserService untuk manajemen kotak sampah
}

// Tambahkan userService ke dalam parameter konstruktor
func NewAdminHandler(trxService service.TransactionService, userService service.UserService) *AdminHandler {
	return &AdminHandler{trxService, userService}
}

type VerifyPaymentInput struct {
	Action string `json:"action" binding:"required,oneof=approve reject"`
}

// VerifyPayment godoc
// @Summary      Verify Manual Payment
// @Description  Admin melakukan persetujuan atau penolakan
// @Tags         admin
func (h *AdminHandler) VerifyPayment(c *gin.Context) {
	orderID := c.Param("id")
	var input VerifyPaymentInput

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Aksi tidak valid. Harus 'approve' atau 'reject'", nil)
		return
	}

	if err := h.trxService.VerifyPayment(orderID, input.Action); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	if input.Action == "reject" {
		utils.SuccessResult(c, http.StatusOK, "Pembayaran ditolak. Status menjadi cancel.", nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Pembayaran berhasil diverifikasi. E-Ticket sedang dikirim.", gin.H{
		"status": "settlement",
	})
}

// =====================================================================
// FITUR KOTAK SAMPAH (TRASH BIN) PERSONIL
// =====================================================================

// GetTrashedUsers godoc
// @Summary      Get Trashed Users
// @Description  Melihat daftar personil yang telah dicabut aksesnya (Soft Deleted)
// @Tags         admin
func (h *AdminHandler) GetTrashedUsers(c *gin.Context) {
	users, err := h.userService.GetTrashedUsers()
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, "Gagal mengambil data kotak sampah", err.Error())
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Berhasil mengambil data kotak sampah personil", users)
}

// RestoreUser godoc
// @Summary      Restore Trashed User
// @Description  Memulihkan akses personil dari kotak sampah
// @Tags         admin
func (h *AdminHandler) RestoreUser(c *gin.Context) {
	id := c.Param("id")

	if err := h.userService.RestoreUser(id); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Akun personil berhasil dipulihkan", nil)
}

// HardDeleteUser godoc
// @Summary      Hard Delete User
// @Description  Memusnahkan personil secara fisik dari database beserta relasinya
// @Tags         admin
func (h *AdminHandler) HardDeleteUser(c *gin.Context) {
	id := c.Param("id")

	if err := h.userService.HardDeleteUser(id); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Akun personil berhasil dimusnahkan secara permanen", nil)
}

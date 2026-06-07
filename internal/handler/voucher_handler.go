package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/service"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

type VoucherHandler struct {
	voucherService service.VoucherService
}

func NewVoucherHandler(voucherService service.VoucherService) *VoucherHandler {
	return &VoucherHandler{voucherService}
}

// CreateVoucher godoc
// @Summary      Create New Voucher
// @Description  Admin menambah kode diskon baru (misal: PELAJAR20K)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        input  body      service.CreateVoucherInput  true  "Data Voucher"
// @Success      200    {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/vouchers [post]
func (h *VoucherHandler) CreateVoucher(c *gin.Context) {
	var input service.CreateVoucherInput

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Data input tidak valid", err.Error())
		return
	}

	voucher, err := h.voucherService.CreateVoucher(input)
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Voucher berhasil dibuat", voucher)
}

// GetVouchers godoc
// @Summary      Get All Vouchers
// @Description  Admin melihat daftar voucher dan sisa kuota
// @Tags         admin
// @Produce      json
// @Success      200    {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/vouchers [get]
func (h *VoucherHandler) GetVouchers(c *gin.Context) {
	vouchers, err := h.voucherService.GetAllVouchers()
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, "Gagal mengambil data voucher", err.Error())
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Berhasil mengambil data voucher", vouchers)
}

// ToggleVoucherStatus godoc
// @Summary      Toggle Voucher Status
// @Description  Admin mematikan (Kill Switch) atau menyalakan kembali voucher
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "Voucher ID"
// @Success      200  {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/vouchers/{id}/toggle [put]
func (h *VoucherHandler) ToggleVoucherStatus(c *gin.Context) {
	id := c.Param("id")

	voucher, statusStr, err := h.voucherService.ToggleVoucherStatus(id)
	if err != nil {
		utils.ErrorResult(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, fmt.Sprintf("Voucher berhasil %s", statusStr), gin.H{
		"is_active": voucher.IsActive,
	})
}

// UpdateVoucher godoc
// @Summary      Update Voucher
// @Description  Admin merubah potongan harga atau kuota voucher
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id     path      string                      true  "Voucher ID"
// @Param        input  body      service.UpdateVoucherInput  true  "Data Update Voucher"
// @Success      200    {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/vouchers/{id} [put]
func (h *VoucherHandler) UpdateVoucher(c *gin.Context) {
	id := c.Param("id")
	var input service.UpdateVoucherInput

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Data input tidak valid", err.Error())
		return
	}

	voucher, err := h.voucherService.UpdateVoucher(id, input)
	if err != nil {
		utils.ErrorResult(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Voucher berhasil diperbarui", voucher)
}

// DeleteVoucher godoc
// @Summary      Delete Voucher
// @Description  Admin menghapus voucher secara permanen
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "Voucher ID"
// @Success      200  {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/vouchers/{id} [delete]
func (h *VoucherHandler) DeleteVoucher(c *gin.Context) {
	id := c.Param("id")

	err := h.voucherService.DeleteVoucher(id)
	if err != nil {
		utils.ErrorResult(c, http.StatusNotFound, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Voucher berhasil dihapus", nil)
}

// ValidateVoucher godoc
// @Summary      Validate Voucher Code
// @Description  Peserta mengecek apakah voucher valid dan kuota masih ada sebelum checkout
// @Tags         public
// @Produce      json
// @Param        code   query     string  true  "Kode Voucher"
// @Success      200    {object}  utils.SuccessResponse
// @Router       /api/vouchers/validate [get]
func (h *VoucherHandler) ValidateVoucher(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		utils.ErrorResult(c, http.StatusBadRequest, "Kode voucher harus diisi", nil)
		return
	}

	voucher, err := h.voucherService.ValidateVoucher(code)
	if err != nil {
		// Mengembalikan bad request jika status tidak aktif atau kuota habis, not found jika tidak ada
		statusCode := http.StatusBadRequest
		if err.Error() == "voucher tidak ditemukan" {
			statusCode = http.StatusNotFound
		}

		utils.ErrorResult(c, statusCode, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Voucher valid", gin.H{
		"id":              voucher.ID,
		"code":            voucher.Code,
		"discount_amount": voucher.DiscountAmount,
	})
}

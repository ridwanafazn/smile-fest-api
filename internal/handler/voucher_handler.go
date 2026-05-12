package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
)

type CreateVoucherInput struct {
	Code           string  `json:"code" binding:"required"`
	DiscountAmount float64 `json:"discount_amount" binding:"required"`
	Quota          int     `json:"quota" binding:"required"`
}

type UpdateVoucherInput struct {
	DiscountAmount float64 `json:"discount_amount" binding:"required"`
	Quota          int     `json:"quota" binding:"required"`
}

// CreateVoucher godoc
// @Summary      Create New Voucher
// @Description  Admin menambah kode diskon baru (misal: PELAJAR20K)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        input  body      CreateVoucherInput  true  "Data Voucher"
// @Success      200    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/vouchers [post]
func CreateVoucher(c *gin.Context) {
	var input CreateVoucherInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	voucher := model.Voucher{
		Code:           strings.ToUpper(input.Code),
		DiscountAmount: input.DiscountAmount,
		Quota:          input.Quota,
		IsActive:       true,
	}

	if err := config.DB.Create(&voucher).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat voucher atau kode sudah ada"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Voucher berhasil dibuat", "data": voucher})
}

// GetVouchers godoc
// @Summary      Get All Vouchers
// @Description  Admin melihat daftar voucher dan sisa kuota
// @Tags         admin
// @Produce      json
// @Success      200    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/vouchers [get]
func GetVouchers(c *gin.Context) {
	var vouchers []model.Voucher
	config.DB.Order("created_at desc").Find(&vouchers)

	// Pembungkusan JSON agar seragam (Unboxing ready)
	c.JSON(http.StatusOK, gin.H{
		"message": "Berhasil mengambil data voucher",
		"data":    vouchers,
	})
}

// ToggleVoucherStatus godoc
// @Summary      Toggle Voucher Status
// @Description  Admin mematikan (Kill Switch) atau menyalakan kembali voucher
// @Tags         admin
// @Produce      json
// @Param        id   path      int  true  "Voucher ID"
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/vouchers/{id}/toggle [put]
func ToggleVoucherStatus(c *gin.Context) {
	id := c.Param("id")
	var voucher model.Voucher

	if err := config.DB.First(&voucher, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Voucher tidak ditemukan"})
		return
	}

	voucher.IsActive = !voucher.IsActive
	config.DB.Save(&voucher)

	status := "dinonaktifkan"
	if voucher.IsActive {
		status = "diaktifkan"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Voucher berhasil " + status,
		"is_active": voucher.IsActive,
	})
}

// UpdateVoucher godoc
// @Summary      Update Voucher
// @Description  Admin merubah potongan harga atau kuota voucher
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id     path      int                 true  "Voucher ID"
// @Param        input  body      UpdateVoucherInput  true  "Data Update Voucher"
// @Success      200    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/vouchers/{id} [put]
func UpdateVoucher(c *gin.Context) {
	id := c.Param("id")
	var input UpdateVoucherInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var voucher model.Voucher
	if err := config.DB.First(&voucher, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Voucher tidak ditemukan"})
		return
	}

	voucher.DiscountAmount = input.DiscountAmount
	voucher.Quota = input.Quota

	if err := config.DB.Save(&voucher).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate voucher"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Voucher berhasil diperbarui", "data": voucher})
}

// DeleteVoucher godoc
// @Summary      Delete Voucher
// @Description  Admin menghapus voucher secara permanen
// @Tags         admin
// @Produce      json
// @Param        id   path      int  true  "Voucher ID"
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/vouchers/{id} [delete]
func DeleteVoucher(c *gin.Context) {
	id := c.Param("id")
	var voucher model.Voucher

	if err := config.DB.First(&voucher, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Voucher tidak ditemukan"})
		return
	}

	if err := config.DB.Delete(&voucher).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus voucher"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Voucher berhasil dihapus"})
}

// ValidateVoucher godoc
// @Summary      Validate Voucher Code
// @Description  Peserta mengecek apakah voucher valid dan kuota masih ada sebelum checkout
// @Tags         public
// @Produce      json
// @Param        code   query     string  true  "Kode Voucher"
// @Success      200    {object}  map[string]interface{}
// @Router       /api/vouchers/validate [get]
func ValidateVoucher(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kode voucher harus diisi"})
		return
	}

	var voucher model.Voucher
	if err := config.DB.Where("code = ?", strings.ToUpper(code)).First(&voucher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Voucher tidak ditemukan"})
		return
	}

	if !voucher.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Voucher sedang tidak aktif"})
		return
	}

	if voucher.UsageCount >= voucher.Quota {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kuota voucher sudah habis"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Voucher valid",
		"data": gin.H{
			"id":              voucher.ID,
			"code":            voucher.Code,
			"discount_amount": voucher.DiscountAmount,
		},
	})
}

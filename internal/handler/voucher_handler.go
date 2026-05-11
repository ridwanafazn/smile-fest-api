package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
)

// --- KHUSUS ADMIN ---

// CreateVoucher untuk Admin menambah kode diskon (misal: PELAJAR20K)
func CreateVoucher(c *gin.Context) {
	var input struct {
		Code           string  `json:"code" binding:"required"`
		DiscountAmount float64 `json:"discount_amount" binding:"required"`
		Quota          int     `json:"quota" binding:"required"`
	}

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

// GetVouchers untuk Admin melihat sisa kuota
func GetVouchers(c *gin.Context) {
	var vouchers []model.Voucher
	config.DB.Find(&vouchers)
	c.JSON(http.StatusOK, gin.H{"data": vouchers})
}

// --- KHUSUS PUBLIK / PESERTA ---

// ValidateVoucher untuk Peserta mengecek apakah voucher valid dan kuota masih ada
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Voucher sudah tidak aktif"})
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

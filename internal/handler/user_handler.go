package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

func Login(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := config.DB.Where("username = ?", input.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Username atau password salah"})
		return
	}

	if !utils.CheckPasswordHash(input.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Username atau password salah"})
		return
	}

	// Ubah di baris ini: user.ID dikonversi jadi string
	token, _ := utils.GenerateToken(user.ID.String(), user.Role)
	c.JSON(http.StatusOK, gin.H{"token": token, "role": user.Role})
}

// CreateUser hanya bisa diakses Admin untuk bikin Scanner
func CreateUser(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role" binding:"required"` // admin / scanner
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, _ := utils.HashPassword(input.Password)
	user := model.User{
		Username: input.Username,
		Password: hashedPassword,
		Role:     input.Role,
	}

	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User berhasil dibuat"})
}

// SeedAdmin temporer untuk buat akun pertama
func SeedAdmin(c *gin.Context) {
	hashedPassword, _ := utils.HashPassword("SMILEFEST2026")
	admin := model.User{
		Username: "admin_wang",
		Password: hashedPassword,
		Role:     "admin",
	}

	if err := config.DB.Create(&admin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Admin sudah ada atau gagal buat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Admin berhasil dibuat! Silakan login dengan user: admin_wang"})
}

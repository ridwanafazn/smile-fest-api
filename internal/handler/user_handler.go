package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type CreateUserInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"` // admin / scanner
}

// Login godoc
// @Summary      User Login
// @Description  Mendapatkan JWT token untuk akses API
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        input  body      LoginInput  true  "Kredensial Login"
// @Success      200    {object}  map[string]interface{}
// @Router       /api/login [post]
func Login(c *gin.Context) {
	var input LoginInput

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

	token, _ := utils.GenerateToken(user.ID.String(), user.Role)
	c.JSON(http.StatusOK, gin.H{"token": token, "role": user.Role})
}

// --- KHUSUS ADMIN ---

// CreateUser godoc
// @Summary      Create New User
// @Description  Hanya bisa diakses Admin untuk membuat akun Scanner/Admin baru
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        input  body      CreateUserInput  true  "Data User Baru"
// @Success      200    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/users [post]
func CreateUser(c *gin.Context) {
	var input CreateUserInput

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

// GetUsers godoc
// @Summary      Get All Users
// @Description  Admin melihat daftar seluruh panitia dan admin yang terdaftar
// @Tags         admin
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/users [get]
func GetUsers(c *gin.Context) {
	var users []model.User
	// Sengaja kita Select field tertentu agar password hash tidak ikut terkirim ke frontend
	config.DB.Select("id", "username", "role", "created_at").Find(&users)
	c.JSON(http.StatusOK, gin.H{"data": users})
}

// DeleteUser godoc
// @Summary      Delete User
// @Description  Admin menghapus/mencabut akses akun panitia
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "User ID"
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/admin/users/{id} [delete]
func DeleteUser(c *gin.Context) {
	id := c.Param("id")

	// Ambil ID admin yang sedang melakukan request dari token JWT
	currentUserID, exists := c.Get("user_id")
	if exists && id == currentUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Sabuk pengaman: Anda tidak dapat menghapus akun Anda sendiri!"})
		return
	}

	if err := config.DB.Delete(&model.User{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Akses user berhasil dicabut dan dihapus"})
}

// SeedAdmin godoc
// @Summary      Seed First Admin
// @Description  Temporer untuk membuat akun admin pertama (Matikan rute ini setelah dipakai)
// @Tags         auth
// @Produce      json
// @Success      200    {object}  map[string]interface{}
// @Router       /api/seed-admin [post]
func SeedAdmin(c *gin.Context) {
	hashedPassword, _ := utils.HashPassword("ringkaibinar")
	admin := model.User{
		Username: "admin",
		Password: hashedPassword,
		Role:     "admin",
	}

	if err := config.DB.Create(&admin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Admin sudah ada atau gagal buat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Admin berhasil dibuat! Silakan login dengan user: admin"})
}

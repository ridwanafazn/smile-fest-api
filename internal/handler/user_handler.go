package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/service"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService}
}

// Login godoc
// @Summary      User Login
// @Description  Mendapatkan JWT token untuk akses API
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        input  body      service.LoginInput  true  "Kredensial Login"
// @Success      200    {object}  utils.SuccessResponse
// @Router       /api/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var input service.LoginInput

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Data yang dikirim tidak valid", err.Error())
		return
	}

	token, role, err := h.userService.Login(input)
	if err != nil {
		utils.ErrorResult(c, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Login berhasil", gin.H{
		"token": token,
		"role":  role,
	})
}

// --- KHUSUS ADMIN ---

// CreateUser godoc
// @Summary      Create New User
// @Description  Hanya bisa diakses Admin untuk membuat akun Scanner atau Admin baru
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        input  body      service.CreateUserInput  true  "Data User Baru"
// @Success      200    {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var input service.CreateUserInput

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Data yang dikirim tidak valid", err.Error())
		return
	}

	if err := h.userService.CreateUser(input); err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, "Gagal membuat user", err.Error())
		return
	}

	utils.SuccessResult(c, http.StatusOK, "User berhasil dibuat", nil)
}

// GetUsers godoc
// @Summary      Get All Users
// @Description  Admin melihat daftar seluruh panitia dan admin yang terdaftar
// @Tags         admin
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/users [get]
func (h *UserHandler) GetUsers(c *gin.Context) {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, "Gagal mengambil data user", err.Error())
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Data user berhasil diambil", users)
}

// UpdateUser godoc
// @Summary      Update User
// @Description  Admin memperbarui data personil
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id     path      string  true  "User ID"
// @Param        input  body      service.UpdateUserInput  true  "Data User Baru"
// @Success      200    {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var input service.UpdateUserInput

	if err := c.ShouldBindJSON(&input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, "Data yang dikirim tidak valid", err.Error())
		return
	}

	if err := h.userService.UpdateUser(id, input); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Data personil berhasil diperbarui", nil)
}

// DeleteUser godoc
// @Summary      Delete User
// @Description  Admin menghapus akses akun panitia
// @Tags         admin
// @Produce      json
// @Param        id   path      string  true  "User ID"
// @Success      200  {object}  utils.SuccessResponse
// @Security     BearerAuth
// @Router       /api/admin/users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	targetID := c.Param("id")

	currentUserID, exists := c.Get("user_id")
	if !exists {
		utils.ErrorResult(c, http.StatusUnauthorized, "Sesi tidak valid", nil)
		return
	}

	if err := h.userService.DeleteUser(targetID, currentUserID.(string)); err != nil {
		utils.ErrorResult(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Akses user berhasil dicabut", nil)
}

// SeedAdmin godoc
// @Summary      Seed First Admin
// @Description  Temporer untuk membuat akun admin pertama
// @Tags         auth
// @Produce      json
// @Success      200    {object}  utils.SuccessResponse
// @Router       /api/seed-admin [post]
func (h *UserHandler) SeedAdmin(c *gin.Context) {
	if err := h.userService.SeedAdmin(); err != nil {
		utils.ErrorResult(c, http.StatusInternalServerError, "Admin sudah ada atau gagal dibuat", err.Error())
		return
	}

	utils.SuccessResult(c, http.StatusOK, "Admin berhasil dibuat! Silakan login dengan user: admin", nil)
}

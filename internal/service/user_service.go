package service

import (
	"errors"

	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type CreateUserInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

type UpdateUserInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password"`
	Role     string `json:"role" binding:"required"`
}

type UserService interface {
	Login(input LoginInput) (string, string, error)
	CreateUser(input CreateUserInput) error
	GetAllUsers() ([]model.User, error)
	DeleteUser(targetID string, currentUserID string) error
	UpdateUser(id string, input UpdateUserInput) error
	EnsureAdmin(username, password string) (bool, error)

	GetTrashedUsers() ([]model.User, error)
	RestoreUser(id string) error
	HardDeleteUser(id string) error
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo}
}

func (s *userService) Login(input LoginInput) (string, string, error) {
	user, err := s.userRepo.FindByUsername(input.Username)
	if err != nil {
		return "", "", errors.New("username atau password salah")
	}

	if !utils.CheckPasswordHash(input.Password, user.Password) {
		return "", "", errors.New("username atau password salah")
	}

	token, err := utils.GenerateToken(user.ID.String(), user.Role)
	if err != nil {
		return "", "", errors.New("gagal memproses otentikasi")
	}

	return token, user.Role, nil
}

func (s *userService) CreateUser(input CreateUserInput) error {
	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		return errors.New("gagal memproses kata sandi")
	}

	user := model.User{
		Username: input.Username,
		Password: hashedPassword,
		Role:     input.Role,
	}

	return s.userRepo.Create(&user)
}

func (s *userService) GetAllUsers() ([]model.User, error) {
	return s.userRepo.FindAll()
}

func (s *userService) DeleteUser(targetID string, currentUserID string) error {
	if targetID == currentUserID {
		return errors.New("sabuk pengaman: anda tidak dapat mencabut akses akun anda sendiri")
	}
	return s.userRepo.Delete(targetID)
}

func (s *userService) UpdateUser(id string, input UpdateUserInput) error {
	existingUser, err := s.userRepo.FindByUsername(input.Username)
	if err == nil && existingUser.ID.String() != id {
		return errors.New("username sudah digunakan oleh personil lain")
	}

	updateData := map[string]interface{}{
		"username": input.Username,
		"role":     input.Role,
	}

	if input.Password != "" {
		hashedPassword, err := utils.HashPassword(input.Password)
		if err != nil {
			return errors.New("gagal memproses kata sandi baru")
		}
		updateData["password"] = hashedPassword
	}

	return s.userRepo.Update(id, updateData)
}

func (s *userService) EnsureAdmin(username, password string) (bool, error) {
	if username == "" || password == "" {
		return false, errors.New("ADMIN_INIT_USERNAME dan ADMIN_INIT_PASSWORD wajib diisi")
	}

	// Jika admin (atau username tsb) sudah ada, jangan dibuat ulang.
	if _, err := s.userRepo.FindByUsername(username); err == nil {
		return false, nil
	}

	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return false, errors.New("gagal memproses kata sandi")
	}

	admin := model.User{
		Username: username,
		Password: hashedPassword,
		Role:     "admin",
	}
	if err := s.userRepo.Create(&admin); err != nil {
		return false, errors.New("gagal membuat admin awal")
	}
	return true, nil
}

func (s *userService) GetTrashedUsers() ([]model.User, error) {
	return s.userRepo.FindTrashedUsers()
}

func (s *userService) RestoreUser(id string) error {
	return s.userRepo.Restore(id)
}

func (s *userService) HardDeleteUser(id string) error {
	return s.userRepo.HardDelete(id)
}

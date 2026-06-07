package repository

import (
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"gorm.io/gorm"
)

type UserRepository interface {
	FindByUsername(username string) (*model.User, error)
	Create(user *model.User) error
	FindAll() ([]model.User, error)
	Delete(id string) error
	Update(id string, data map[string]interface{}) error

	FindTrashedUsers() ([]model.User, error)
	Restore(id string) error
	HardDelete(id string) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db}
}

func (r *userRepository) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) FindAll() ([]model.User, error) {
	var users []model.User
	err := r.db.Select("id", "username", "role", "created_at").Find(&users).Error
	return users, err
}

func (r *userRepository) Delete(id string) error {
	return r.db.Delete(&model.User{}, "id = ?", id).Error
}

func (r *userRepository) Update(id string, data map[string]interface{}) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Updates(data).Error
}

func (r *userRepository) FindTrashedUsers() ([]model.User, error) {
	var users []model.User
	err := r.db.Unscoped().Where("deleted_at IS NOT NULL").Select("id", "username", "role", "deleted_at").Find(&users).Error
	return users, err
}

func (r *userRepository) Restore(id string) error {
	return r.db.Unscoped().Model(&model.User{}).Where("id = ?", id).Update("deleted_at", nil).Error
}

func (r *userRepository) HardDelete(id string) error {
	return r.db.Unscoped().Delete(&model.User{}, "id = ?", id).Error
}

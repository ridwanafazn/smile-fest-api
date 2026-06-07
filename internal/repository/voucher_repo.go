package repository

import (
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"gorm.io/gorm"
)

type VoucherRepository interface {
	Create(voucher *model.Voucher) error
	FindAll() ([]model.Voucher, error)
	FindByID(id string) (*model.Voucher, error)
	FindByCode(code string) (*model.Voucher, error)
	Update(voucher *model.Voucher) error
	Delete(voucher *model.Voucher) error
}

type voucherRepository struct {
	db *gorm.DB
}

func NewVoucherRepository(db *gorm.DB) VoucherRepository {
	return &voucherRepository{db}
}

func (r *voucherRepository) Create(voucher *model.Voucher) error {
	return r.db.Create(voucher).Error
}

func (r *voucherRepository) FindAll() ([]model.Voucher, error) {
	var vouchers []model.Voucher
	err := r.db.Order("created_at desc").Find(&vouchers).Error
	return vouchers, err
}

func (r *voucherRepository) FindByID(id string) (*model.Voucher, error) {
	var voucher model.Voucher
	err := r.db.First(&voucher, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &voucher, nil
}

func (r *voucherRepository) FindByCode(code string) (*model.Voucher, error) {
	var voucher model.Voucher
	err := r.db.Where("code = ?", code).First(&voucher).Error
	if err != nil {
		return nil, err
	}
	return &voucher, nil
}

func (r *voucherRepository) Update(voucher *model.Voucher) error {
	return r.db.Save(voucher).Error
}

func (r *voucherRepository) Delete(voucher *model.Voucher) error {
	return r.db.Delete(voucher).Error
}

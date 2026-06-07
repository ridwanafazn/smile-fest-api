package service

import (
	"errors"
	"strings"

	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
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

type VoucherService interface {
	CreateVoucher(input CreateVoucherInput) (*model.Voucher, error)
	GetAllVouchers() ([]model.Voucher, error)
	ToggleVoucherStatus(id string) (*model.Voucher, string, error)
	UpdateVoucher(id string, input UpdateVoucherInput) (*model.Voucher, error)
	DeleteVoucher(id string) error
	ValidateVoucher(code string) (*model.Voucher, error)
}

type voucherService struct {
	voucherRepo repository.VoucherRepository
}

func NewVoucherService(voucherRepo repository.VoucherRepository) VoucherService {
	return &voucherService{voucherRepo}
}

func (s *voucherService) CreateVoucher(input CreateVoucherInput) (*model.Voucher, error) {
	voucher := model.Voucher{
		Code:           strings.ToUpper(input.Code),
		DiscountAmount: input.DiscountAmount,
		Quota:          input.Quota,
		IsActive:       true,
	}

	err := s.voucherRepo.Create(&voucher)
	if err != nil {
		return nil, errors.New("gagal membuat voucher atau kode sudah ada")
	}

	return &voucher, nil
}

func (s *voucherService) GetAllVouchers() ([]model.Voucher, error) {
	return s.voucherRepo.FindAll()
}

func (s *voucherService) ToggleVoucherStatus(id string) (*model.Voucher, string, error) {
	voucher, err := s.voucherRepo.FindByID(id)
	if err != nil {
		return nil, "", errors.New("voucher tidak ditemukan")
	}

	voucher.IsActive = !voucher.IsActive
	err = s.voucherRepo.Update(voucher)
	if err != nil {
		return nil, "", errors.New("gagal merubah status voucher")
	}

	statusStr := "dinonaktifkan"
	if voucher.IsActive {
		statusStr = "diaktifkan"
	}

	return voucher, statusStr, nil
}

func (s *voucherService) UpdateVoucher(id string, input UpdateVoucherInput) (*model.Voucher, error) {
	voucher, err := s.voucherRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("voucher tidak ditemukan")
	}

	voucher.DiscountAmount = input.DiscountAmount
	voucher.Quota = input.Quota

	err = s.voucherRepo.Update(voucher)
	if err != nil {
		return nil, errors.New("gagal mengupdate voucher")
	}

	return voucher, nil
}

func (s *voucherService) DeleteVoucher(id string) error {
	voucher, err := s.voucherRepo.FindByID(id)
	if err != nil {
		return errors.New("voucher tidak ditemukan")
	}

	return s.voucherRepo.Delete(voucher)
}

func (s *voucherService) ValidateVoucher(code string) (*model.Voucher, error) {
	upperCode := strings.ToUpper(code)
	voucher, err := s.voucherRepo.FindByCode(upperCode)
	if err != nil {
		return nil, errors.New("voucher tidak ditemukan")
	}

	if !voucher.IsActive {
		return nil, errors.New("voucher sedang tidak aktif")
	}

	if voucher.UsageCount >= voucher.Quota {
		return nil, errors.New("kuota voucher sudah habis")
	}

	return voucher, nil
}

package service

import (
	"errors"
	"testing"

	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
)

type mockVoucherReadRepo struct {
	repository.VoucherRepository
	fnFindByCode func(code string) (*model.Voucher, error)
}

func (m *mockVoucherReadRepo) FindByCode(code string) (*model.Voucher, error) {
	return m.fnFindByCode(code)
}

func TestValidateVoucher_Valid(t *testing.T) {
	repo := &mockVoucherReadRepo{fnFindByCode: func(string) (*model.Voucher, error) {
		return &model.Voucher{ID: 1, Code: "PELAJAR", DiscountAmount: 50000, Quota: 10, UsageCount: 3, IsActive: true}, nil
	}}
	v, err := NewVoucherService(repo).ValidateVoucher("pelajar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Code != "PELAJAR" {
		t.Errorf("code = %q, want PELAJAR (di-upper)", v.Code)
	}
}

func TestValidateVoucher_Inactive(t *testing.T) {
	repo := &mockVoucherReadRepo{fnFindByCode: func(string) (*model.Voucher, error) {
		return &model.Voucher{ID: 1, Code: "OFF", DiscountAmount: 50000, Quota: 10, UsageCount: 0, IsActive: false}, nil
	}}
	_, err := NewVoucherService(repo).ValidateVoucher("OFF")
	if err == nil {
		t.Fatal("want error for inactive voucher")
	}
}

func TestValidateVoucher_QuotaExhausted(t *testing.T) {
	repo := &mockVoucherReadRepo{fnFindByCode: func(string) (*model.Voucher, error) {
		return &model.Voucher{ID: 1, Code: "FULL", DiscountAmount: 50000, Quota: 5, UsageCount: 5, IsActive: true}, nil
	}}
	_, err := NewVoucherService(repo).ValidateVoucher("FULL")
	if err == nil {
		t.Fatal("want error for exhausted quota")
	}
}

func TestValidateVoucher_NotFound(t *testing.T) {
	repo := &mockVoucherReadRepo{fnFindByCode: func(string) (*model.Voucher, error) {
		return nil, errors.New("not found")
	}}
	_, err := NewVoucherService(repo).ValidateVoucher("NOPE")
	if err == nil {
		t.Fatal("want error for missing voucher")
	}
}

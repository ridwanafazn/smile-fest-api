package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
	"github.com/ridwanafazn/smile-fest-api/pkg/terrors"
)

// --- Mock repositories (embed interface + override hanya metode yang dipakai) ---

type mockTrxRepo struct {
	repository.TransactionRepository
	fnFindPending     func(email string) (*model.Transaction, error)
	fnCountActive     func() (int64, error)
	fnCreateWithQuota func(tx *model.Transaction, tickets []model.Ticket, max int64) error
}

func (m *mockTrxRepo) FindPendingByEmail(e string) (*model.Transaction, error) {
	return m.fnFindPending(e)
}
func (m *mockTrxRepo) CountActiveTickets() (int64, error) { return m.fnCountActive() }
func (m *mockTrxRepo) CreateWithTicketsAndQuota(t *model.Transaction, tk []model.Ticket, mx int64) error {
	return m.fnCreateWithQuota(t, tk, mx)
}

type mockTicketRepo struct {
	repository.TicketRepository
	fnFindVariant func(id string) (*model.TicketVariant, error)
}

func (m *mockTicketRepo) FindVariantByID(id string) (*model.TicketVariant, error) {
	return m.fnFindVariant(id)
}

type mockVoucherRepo struct {
	repository.VoucherRepository
	fnFindByCode func(code string) (*model.Voucher, error)
	fnReserve    func(id uint, qty int) error
	fnRelease    func(id uint, qty int) error
}

func (m *mockVoucherRepo) FindByCode(code string) (*model.Voucher, error) {
	return m.fnFindByCode(code)
}
func (m *mockVoucherRepo) Reserve(id uint, qty int) error { return m.fnReserve(id, qty) }
func (m *mockVoucherRepo) Release(id uint, qty int) error { return m.fnRelease(id, qty) }

// --- Helpers ---

func activeVariant(price float64) *model.TicketVariant {
	return &model.TicketVariant{ID: "v1", Name: "Presale", Price: price, IsActive: true}
}

func baseInput(qty int) CheckoutInput {
	atts := make([]Attendee, 0, qty)
	for i := 0; i < qty; i++ {
		atts = append(atts, Attendee{Name: "Peserta"})
	}
	return CheckoutInput{
		TicketType:     "v1",
		CustomerName:   "Ridwan",
		CustomerEmail:  "a@b.com",
		CustomerPhone:  "0812",
		CustomerGender: "M",
		Attendees:      atts,
	}
}

func noPending() func(string) (*model.Transaction, error) {
	return func(string) (*model.Transaction, error) { return nil, errors.New("not found") }
}

// --- Tests ---

func TestCheckout_Success(t *testing.T) {
	trx := &mockTrxRepo{
		fnFindPending:     noPending(),
		fnCountActive:     func() (int64, error) { return 0, nil },
		fnCreateWithQuota: func(tx *model.Transaction, tk []model.Ticket, max int64) error { return nil },
	}
	ticket := &mockTicketRepo{fnFindVariant: func(string) (*model.TicketVariant, error) { return activeVariant(100000), nil }}
	voucher := &mockVoucherRepo{}

	svc := NewTransactionService(trx, ticket, voucher)
	created, err := svc.Checkout(baseInput(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created.BaseAmount != 200000 {
		t.Errorf("BaseAmount = %v, want 200000", created.BaseAmount)
	}
	if created.DiscountAmount != 0 {
		t.Errorf("DiscountAmount = %v, want 0", created.DiscountAmount)
	}
	if created.UniqueCode < 100 || created.UniqueCode > 999 {
		t.Errorf("UniqueCode = %d, want 100..999", created.UniqueCode)
	}
	wantTotal := 200000 + float64(created.UniqueCode)
	if created.TotalAmount != wantTotal {
		t.Errorf("TotalAmount = %v, want %v", created.TotalAmount, wantTotal)
	}
	if created.Status != "pending" {
		t.Errorf("Status = %q, want pending", created.Status)
	}
	if !strings.HasPrefix(created.ID, "SMILE-") {
		t.Errorf("ID = %q, want prefix SMILE-", created.ID)
	}
	if trx.fnCreateWithQuota == nil {
		t.Fatal("mock misconfigured")
	}
}

func TestCheckout_OrderIDIsUniqueAcrossCalls(t *testing.T) {
	trx := &mockTrxRepo{
		fnFindPending:     noPending(),
		fnCountActive:     func() (int64, error) { return 0, nil },
		fnCreateWithQuota: func(tx *model.Transaction, tk []model.Ticket, max int64) error { return nil },
	}
	ticket := &mockTicketRepo{fnFindVariant: func(string) (*model.TicketVariant, error) { return activeVariant(100000), nil }}
	voucher := &mockVoucherRepo{}
	svc := NewTransactionService(trx, ticket, voucher)

	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		c, err := svc.Checkout(baseInput(1))
		if err != nil {
			t.Fatalf("checkout %d failed: %v", i, err)
		}
		if seen[c.ID] {
			t.Fatalf("duplicate order id: %s", c.ID)
		}
		seen[c.ID] = true
		// ganti email agar tidak terdeteksi pending
	}
}

func TestCheckout_PendingExists(t *testing.T) {
	trx := &mockTrxRepo{
		fnFindPending: func(string) (*model.Transaction, error) { return &model.Transaction{ID: "EXIST"}, nil },
	}
	ticket := &mockTicketRepo{fnFindVariant: func(string) (*model.TicketVariant, error) { return activeVariant(100000), nil }}
	voucher := &mockVoucherRepo{}
	svc := NewTransactionService(trx, ticket, voucher)

	_, err := svc.Checkout(baseInput(1))
	if !errors.Is(err, terrors.ErrPendingTransactionExists) {
		t.Fatalf("want ErrPendingTransactionExists, got %v", err)
	}
}

func TestCheckout_VoucherApplied(t *testing.T) {
	trx := &mockTrxRepo{
		fnFindPending:     noPending(),
		fnCountActive:     func() (int64, error) { return 0, nil },
		fnCreateWithQuota: func(tx *model.Transaction, tk []model.Ticket, max int64) error { return nil },
	}
	ticket := &mockTicketRepo{fnFindVariant: func(string) (*model.TicketVariant, error) { return activeVariant(100000), nil }}
	voucher := &mockVoucherRepo{
		fnFindByCode: func(string) (*model.Voucher, error) {
			return &model.Voucher{ID: 7, Code: "PELAJAR", DiscountAmount: 50000, Quota: 100, UsageCount: 0, IsActive: true}, nil
		},
		fnReserve: func(id uint, qty int) error { return nil },
	}
	svc := NewTransactionService(trx, ticket, voucher)

	input := baseInput(2)
	input.VoucherCode = "PELAJAR"
	created, err := svc.Checkout(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created.BaseAmount != 200000 {
		t.Errorf("BaseAmount = %v, want 200000", created.BaseAmount)
	}
	if created.DiscountAmount != 100000 { // 50000 * 2 tiket
		t.Errorf("DiscountAmount = %v, want 100000", created.DiscountAmount)
	}
	if created.VoucherID == nil || *created.VoucherID != 7 {
		t.Errorf("VoucherID = %v, want 7", created.VoucherID)
	}
	wantTotal := 100000 + float64(created.UniqueCode)
	if created.TotalAmount != wantTotal {
		t.Errorf("TotalAmount = %v, want %v", created.TotalAmount, wantTotal)
	}
}

func TestCheckout_QuotaExceeded(t *testing.T) {
	trx := &mockTrxRepo{
		fnFindPending: noPending(),
		fnCountActive: func() (int64, error) { return 599, nil },
		fnCreateWithQuota: func(tx *model.Transaction, tk []model.Ticket, max int64) error {
			return terrors.ErrTicketQuotaExceeded
		},
	}
	ticket := &mockTicketRepo{fnFindVariant: func(string) (*model.TicketVariant, error) { return activeVariant(100000), nil }}
	voucher := &mockVoucherRepo{}
	svc := NewTransactionService(trx, ticket, voucher)

	_, err := svc.Checkout(baseInput(2))
	if err == nil || !strings.Contains(err.Error(), "kuota") {
		t.Fatalf("want kuota error, got %v", err)
	}
}

func TestCheckout_VoucherQuotaExceeded(t *testing.T) {
	trx := &mockTrxRepo{
		fnFindPending:     noPending(),
		fnCountActive:     func() (int64, error) { return 0, nil },
		fnCreateWithQuota: func(tx *model.Transaction, tk []model.Ticket, max int64) error { return nil },
	}
	ticket := &mockTicketRepo{fnFindVariant: func(string) (*model.TicketVariant, error) { return activeVariant(100000), nil }}
	voucher := &mockVoucherRepo{
		fnFindByCode: func(string) (*model.Voucher, error) {
			return &model.Voucher{ID: 1, Code: "PELAJAR", DiscountAmount: 50000, Quota: 2, UsageCount: 2, IsActive: true}, nil
		},
		fnReserve: func(id uint, qty int) error { return terrors.ErrVoucherQuotaExceeded },
	}
	svc := NewTransactionService(trx, ticket, voucher)

	input := baseInput(2)
	input.VoucherCode = "PELAJAR"
	_, err := svc.Checkout(input)
	if err == nil || !strings.Contains(err.Error(), "voucher") {
		t.Fatalf("want voucher error, got %v", err)
	}
}

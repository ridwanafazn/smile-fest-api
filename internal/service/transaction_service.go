package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"mime/multipart"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
	"github.com/ridwanafazn/smile-fest-api/internal/worker"
	"github.com/ridwanafazn/smile-fest-api/pkg/cloudinary"
	"github.com/ridwanafazn/smile-fest-api/pkg/terrors"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

// Konstanta kuota tiket global.
const (
	maxTotalTickets   = 600
	batchTwoThreshold = 300
)

type Attendee struct {
	Name string `json:"name" binding:"required"`
}

type CheckoutInput struct {
	TicketType           string     `json:"ticket_type" binding:"required"`
	CustomerName         string     `json:"customer_name" binding:"required"`
	CustomerEmail        string     `json:"customer_email" binding:"required,email"`
	CustomerPhone        string     `json:"customer_phone" binding:"required"`
	CustomerGender       string     `json:"customer_gender" binding:"required"`
	ProfileAge           string     `json:"profile_age"`
	ProfileCity          string     `json:"profile_city"`
	ProfileEducation     string     `json:"profile_education"`
	ProfileJob           string     `json:"profile_job"`
	CommunityAffiliation string     `json:"community_affiliation"`
	InformationSource    string     `json:"information_source"`
	InterestReasons      []string   `json:"interest_reasons"`
	SustainabilitySteps  []string   `json:"sustainability_steps"`
	ContributionRole     string     `json:"contribution_role"`
	VoucherCode          string     `json:"voucher_code"`
	Attendees            []Attendee `json:"attendees" binding:"required,min=1"`
}

type TransactionService interface {
	Checkout(input CheckoutInput) (*model.Transaction, error)
	CancelTransaction(orderID string) error
	UploadProof(ctx context.Context, orderID string, file multipart.File) (string, string, error)
	GetPaginatedTransactions(page, limit int, search, status string) ([]model.Transaction, utils.PaginationMeta, error)
	GetDashboardStats() (map[string]interface{}, error)
	VerifyPayment(orderID, action string) error
	GetTransactionInsights(page, limit int, search, voucherFilter, variantFilter string) ([]model.Transaction, utils.PaginationMeta, error)
}

type transactionService struct {
	trxRepo      repository.TransactionRepository
	ticketRepo   repository.TicketRepository
	voucherRepo  repository.VoucherRepository
	emailWorker  worker.EmailWorkerInterface
}

func NewTransactionService(trxRepo repository.TransactionRepository, ticketRepo repository.TicketRepository, voucherRepo repository.VoucherRepository, emailWorker worker.EmailWorkerInterface) TransactionService {
	return &transactionService{trxRepo, ticketRepo, voucherRepo, emailWorker}
}

func (s *transactionService) Checkout(input CheckoutInput) (*model.Transaction, error) {
	if existingOrder, err := s.trxRepo.FindPendingByEmail(input.CustomerEmail); err == nil {
		return existingOrder, terrors.ErrPendingTransactionExists
	}

	ticketVariant, err := s.ticketRepo.FindVariantByID(input.TicketType)
	if err != nil || !ticketVariant.IsActive {
		return nil, errors.New("tipe tiket tidak valid atau sedang tidak aktif")
	}

	qty := len(input.Attendees)

	// T8: tangani error (dipakai utk menentukan session batch saja; cek kuota
	// otoritatif + atomik dilakukan di CreateWithTicketsAndQuota).
	currentActiveTickets, err := s.trxRepo.CountActiveTickets()
	if err != nil {
		currentActiveTickets = 0
	}
	sessionBatch := 1
	if currentActiveTickets >= batchTwoThreshold {
		sessionBatch = 2
	}

	basePrice := ticketVariant.Price * float64(qty)
	var voucherID *uint
	discount := float64(0)

	if input.VoucherCode != "" {
		voucher, err := s.voucherRepo.FindByCode(strings.ToUpper(input.VoucherCode))
		if err != nil || !voucher.IsActive {
			return nil, errors.New("voucher tidak valid atau tidak aktif")
		}
		// B5: reservasi kuota voucher secara atomik dengan guard kuota.
		if err := s.voucherRepo.Reserve(voucher.ID, qty); err != nil {
			if errors.Is(err, terrors.ErrVoucherQuotaExceeded) {
				return nil, fmt.Errorf("sisa kuota voucher tidak cukup untuk %d tiket", qty)
			}
			return nil, errors.New("gagal memproses voucher")
		}
		discount = voucher.DiscountAmount * float64(qty)
		voucherID = &voucher.ID
	}

	uniqueCode := rand.Intn(900) + 100
	finalPrice := math.Max(basePrice-discount, 0)
	totalTransfer := finalPrice + float64(uniqueCode)

	// B3: order ID anti-collision (timestamp + UUID hex).
	orderID := fmt.Sprintf("SMILE-%s%s", time.Now().Format("060102150405"), strings.ToUpper(uuid.New().String()[:6]))

	transaction := model.Transaction{
		ID:                   orderID,
		CustomerName:         input.CustomerName,
		CustomerEmail:        input.CustomerEmail,
		CustomerPhone:        input.CustomerPhone,
		CustomerGender:       input.CustomerGender,
		ProfileAge:           input.ProfileAge,
		ProfileCity:          input.ProfileCity,
		ProfileEducation:     input.ProfileEducation,
		ProfileJob:           input.ProfileJob,
		CommunityAffiliation: input.CommunityAffiliation,
		InformationSource:    input.InformationSource,
		InterestReasons:      strings.Join(input.InterestReasons, ", "),
		SustainabilitySteps:  strings.Join(input.SustainabilitySteps, ", "),
		ContributionRole:     input.ContributionRole,
		TotalAmount:          totalTransfer,
		BaseAmount:           basePrice,
		DiscountAmount:       discount,
		UniqueCode:           uniqueCode,
		SessionBatch:         sessionBatch,
		ExpiresAt:            time.Now().Add(24 * time.Hour),
		Status:               "pending",
		VoucherID:            voucherID,
	}

	var tickets []model.Ticket
	for _, att := range input.Attendees {
		tickets = append(tickets, model.Ticket{
			TransactionID:   orderID,
			TicketVariantID: input.TicketType,
			AttendeeName:    att.Name,
		})
	}

	if err := s.trxRepo.CreateWithTicketsAndQuota(&transaction, tickets, maxTotalTickets); err != nil {
		// B5: bila penciptaan gagal karena kuota tiket habis, lepaskan
		// reservasi voucher yang tadi di-reserve agar tidak bocor.
		if voucherID != nil {
			_ = s.voucherRepo.Release(*voucherID, qty)
		}
		if errors.Is(err, terrors.ErrTicketQuotaExceeded) {
			return nil, errors.New("mohon maaf, kuota total tiket sudah habis")
		}
		return nil, errors.New("gagal membuat transaksi di database")
	}

	// PUBLISH EVENT KE MESSAGE QUEUE (Pola Event-Driven)
	trackLink := fmt.Sprintf("https://smile-festival.pages.dev/track-ticket?order_id=%s&email=%s", orderID, input.CustomerEmail)
	if s.emailWorker != nil {
		_ = s.emailWorker.EnqueueEmail(worker.RedisEmailTask{
			Type:            worker.RedisTaskInstruction,
			CustomerEmail:   input.CustomerEmail,
			InstructionData: &utils.InstructionData{CustomerName: input.CustomerName, OrderID: orderID, TrackLink: trackLink, TotalAmount: fmt.Sprintf("Rp %s", utils.FormatRupiah(totalTransfer))},
		})
	} else {
		// Fallback ke in-memory queue (legacy)
		worker.EmailQueue <- worker.EmailTask{
			Type:            worker.TaskInstruction,
			CustomerEmail:   input.CustomerEmail,
			InstructionData: &utils.InstructionData{CustomerName: input.CustomerName, OrderID: orderID, TrackLink: trackLink, TotalAmount: fmt.Sprintf("Rp %s", utils.FormatRupiah(totalTransfer))},
		}
	}

	return &transaction, nil
}

func (s *transactionService) CancelTransaction(orderID string) error {
	transaction, err := s.trxRepo.FindByID(orderID)
	if err != nil {
		return errors.New("transaksi tidak ditemukan")
	}
	if transaction.Status != "pending" {
		return errors.New("hanya transaksi pending yang dapat dibatalkan")
	}

	transaction.Status = "cancel"
	if err := s.trxRepo.Update(transaction); err != nil {
		return errors.New("gagal membatalkan transaksi")
	}

	// B5: lepaskan reservasi kuota voucher.
	if transaction.VoucherID != nil {
		qty, _ := s.trxRepo.CountTicketsByTransactionID(orderID)
		if qty > 0 {
			_ = s.voucherRepo.Release(*transaction.VoucherID, int(qty))
		}
	}
	return nil
}

func (s *transactionService) UploadProof(ctx context.Context, orderID string, file multipart.File) (string, string, error) {
	transaction, err := s.trxRepo.FindByID(orderID)
	if err != nil {
		return "", "", errors.New("transaksi tidak ditemukan")
	}
	if transaction.Status != "pending" && transaction.Status != "waiting_verification" {
		return "", "", errors.New("transaksi tidak dapat diunggah ulang")
	}

	imageURL, err := cloudinary.UploadImage(ctx, file, orderID)
	if err != nil {
		return "", "", errors.New("gagal mengunggah gambar ke server")
	}

	transaction.PaymentProofURL = imageURL
	transaction.Status = "waiting_verification"

	if err := s.trxRepo.Update(transaction); err != nil {
		return "", "", errors.New("gagal menyimpan data ke database")
	}

	return imageURL, transaction.Status, nil
}

func (s *transactionService) GetPaginatedTransactions(page, limit int, search, status string) ([]model.Transaction, utils.PaginationMeta, error) {
	transactions, totalRecords, err := s.trxRepo.FindAllPaginated(page, limit, search, status)
	if err != nil {
		return nil, utils.PaginationMeta{}, errors.New("gagal mengambil data transaksi")
	}

	totalPages := int(math.Ceil(float64(totalRecords) / float64(limit)))

	meta := utils.PaginationMeta{
		CurrentPage:  page,
		TotalPages:   totalPages,
		TotalRecords: totalRecords,
		Limit:        limit,
	}

	return transactions, meta, nil
}

func (s *transactionService) GetDashboardStats() (map[string]interface{}, error) {
	rev, totTix, scanTix, waitVerif, err := s.trxRepo.GetDashboardStats()
	if err != nil {
		return nil, errors.New("gagal memuat data statistik")
	}

	return map[string]interface{}{
		"total_revenue":        rev,
		"total_tickets":        totTix,
		"scanned_tickets":      scanTix,
		"waiting_verification": waitVerif,
	}, nil
}

func (s *transactionService) VerifyPayment(orderID, action string) error {
	transaction, err := s.trxRepo.FindByID(orderID)
	if err != nil {
		return errors.New("transaksi tidak ditemukan")
	}
	if transaction.Status == "settlement" {
		return errors.New("transaksi ini sudah disetujui sebelumnya")
	}

	if action == "reject" {
		transaction.Status = "cancel"
		if err := s.trxRepo.Update(transaction); err != nil {
			return errors.New("gagal menolak pembayaran")
		}
		// B5: lepaskan reservasi kuota voucher.
		if transaction.VoucherID != nil {
			qty, _ := s.trxRepo.CountTicketsByTransactionID(orderID)
			if qty > 0 {
				_ = s.voucherRepo.Release(*transaction.VoucherID, int(qty))
			}
		}
		return nil
	}

	if err := s.trxRepo.VerifyAndProcessPayment(transaction); err != nil {
		return errors.New("terjadi kesalahan internal saat memproses persetujuan")
	}

	// PUBLISH EVENT KE MESSAGE QUEUE (Pola Event-Driven)
	if s.emailWorker != nil {
		_ = s.emailWorker.EnqueueEmail(worker.RedisEmailTask{
			Type:          worker.RedisTaskTicket,
			CustomerEmail: transaction.CustomerEmail,
			TicketData: &utils.EmailData{
				CustomerName: transaction.CustomerName,
				OrderID:      transaction.ID,
				TicketLink:   fmt.Sprintf("https://smile-festival.pages.dev/track-ticket?order_id=%s&email=%s", transaction.ID, transaction.CustomerEmail),
			},
		})
	} else {
		// Fallback ke in-memory queue (legacy)
		worker.EmailQueue <- worker.EmailTask{
			Type:          worker.TaskTicket,
			CustomerEmail: transaction.CustomerEmail,
			TicketData: &utils.EmailData{
				CustomerName: transaction.CustomerName,
				OrderID:      transaction.ID,
				TicketLink:   fmt.Sprintf("https://smile-festival.pages.dev/track-ticket?order_id=%s&email=%s", transaction.ID, transaction.CustomerEmail),
			},
		}
	}

	return nil
}

func (s *transactionService) GetTransactionInsights(page, limit int, search, voucherFilter, variantFilter string) ([]model.Transaction, utils.PaginationMeta, error) {
	transactions, totalRecords, err := s.trxRepo.GetTransactionInsights(page, limit, search, voucherFilter, variantFilter)
	if err != nil {
		return nil, utils.PaginationMeta{}, errors.New("gagal mengambil data insight transaksi")
	}

	totalPages := int(math.Ceil(float64(totalRecords) / float64(limit)))

	meta := utils.PaginationMeta{
		CurrentPage:  page,
		TotalPages:   totalPages,
		TotalRecords: totalRecords,
		Limit:        limit,
	}

	return transactions, meta, nil
}

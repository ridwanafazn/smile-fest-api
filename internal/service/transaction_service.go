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

	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
	"github.com/ridwanafazn/smile-fest-api/internal/worker"
	"github.com/ridwanafazn/smile-fest-api/pkg/cloudinary"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
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
	SendBlastEmailToSettledTransactions() error
}

type transactionService struct {
	trxRepo     repository.TransactionRepository
	ticketRepo  repository.TicketRepository
	voucherRepo repository.VoucherRepository
}

func NewTransactionService(trxRepo repository.TransactionRepository, ticketRepo repository.TicketRepository, voucherRepo repository.VoucherRepository) TransactionService {
	return &transactionService{trxRepo, ticketRepo, voucherRepo}
}

func (s *transactionService) Checkout(input CheckoutInput) (*model.Transaction, error) {
	if existingOrder, err := s.trxRepo.FindPendingByEmail(input.CustomerEmail); err == nil {
		return existingOrder, errors.New("PENDING_TRANSACTION_EXISTS")
	}

	ticketVariant, err := s.ticketRepo.FindVariantByID(input.TicketType)
	if err != nil || !ticketVariant.IsActive {
		return nil, errors.New("tipe tiket tidak valid atau sedang tidak aktif")
	}

	qty := len(input.Attendees)

	currentActiveTickets, _ := s.trxRepo.CountActiveTickets()
	if currentActiveTickets+int64(qty) > 600 {
		return nil, errors.New("mohon maaf, kuota total tiket sudah habis")
	}
	sessionBatch := 1
	if currentActiveTickets >= 300 {
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
		if (voucher.Quota - voucher.UsageCount) < qty {
			return nil, fmt.Errorf("sisa kuota voucher tidak cukup untuk %d tiket", qty)
		}
		discount = voucher.DiscountAmount * float64(qty)
		voucherID = &voucher.ID
	}

	uniqueCode := rand.Intn(900) + 100
	finalPrice := math.Max(basePrice-discount, 0)
	totalTransfer := finalPrice + float64(uniqueCode)

	orderID := fmt.Sprintf("SMILE-%d", time.Now().Unix())

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

	if err := s.trxRepo.CreateWithTickets(&transaction, tickets); err != nil {
		return nil, errors.New("gagal membuat transaksi di database")
	}

	// PUBLISH EVENT KE MESSAGE QUEUE (Pola Event-Driven)
	trackLink := fmt.Sprintf("https://smile-festival.pages.dev/track-ticket?order_id=%s&email=%s", orderID, input.CustomerEmail)
	worker.EmailQueue <- worker.EmailTask{
		Type:          worker.TaskInstruction,
		CustomerEmail: input.CustomerEmail,
		InstructionData: &utils.InstructionData{
			CustomerName: input.CustomerName,
			OrderID:      orderID,
			TrackLink:    trackLink,
			TotalAmount:  fmt.Sprintf("Rp %s", utils.FormatRupiah(totalTransfer)),
		},
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
	return s.trxRepo.Update(transaction)
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
		return s.trxRepo.Update(transaction)
	}

	if err := s.trxRepo.VerifyAndProcessPayment(transaction); err != nil {
		return errors.New("terjadi kesalahan internal saat memproses persetujuan")
	}

	// PUBLISH EVENT KE MESSAGE QUEUE (Pola Event-Driven)
	worker.EmailQueue <- worker.EmailTask{
		Type:          worker.TaskTicket,
		CustomerEmail: transaction.CustomerEmail,
		TicketData: &utils.EmailData{
			CustomerName: transaction.CustomerName,
			OrderID:      transaction.ID,
			TicketLink:   fmt.Sprintf("https://smile-festival.pages.dev/track-ticket?order_id=%s&email=%s", transaction.ID, transaction.CustomerEmail),
		},
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

// SendBlastEmailToSettledTransactions menarik data lunas dan melemparnya ke worker secara asinkron
func (s *transactionService) SendBlastEmailToSettledTransactions() error {
	// 1. Tarik data email yang sudah dieliminasi gandanya dari repositori
	transactions, err := s.trxRepo.GetSettledEmailsForBlast()
	if err != nil {
		return errors.New("gagal mengambil data transaksi lunas untuk blast email")
	}

	if len(transactions) == 0 {
		return errors.New("tidak ada transaksi berstatus lunas yang ditemukan")
	}

	// 2. Iterasi data dan masukkan ke dalam antrean (channel) worker
	for _, trx := range transactions {

		// =========================================================================
		targetEmail := trx.CustomerEmail
		// =========================================================================
		// targetEmail := "ridwanafzn@gmail.com"

		// Kirim tugas ke dalam in-memory Message Queue tanpa memblokir perulangan
		worker.EmailQueue <- worker.EmailTask{
			Type:          worker.TaskBlast,
			CustomerEmail: targetEmail,
			BlastData: &utils.BlastData{
				CustomerName: trx.CustomerName,
			},
		}
	}

	return nil
}

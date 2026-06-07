package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
)

const ticketCacheKey = "active_ticket_variants"

type TicketVariantInput struct {
	Name      string     `json:"name" binding:"required"`
	Price     float64    `json:"price" binding:"required,min=0"`
	Quota     int        `json:"quota" binding:"required,min=1"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
}

type TicketService interface {
	GetActiveTicketVariants() ([]model.TicketVariant, error)
	TrackTicket(orderID, email string) (*model.Transaction, error)

	GetAllAdminVariants() ([]model.TicketVariant, error)
	CreateVariant(input TicketVariantInput) (*model.TicketVariant, error)
	UpdateVariant(id string, input TicketVariantInput) (*model.TicketVariant, error)
	DeleteVariant(id string) error
	ToggleVariantStatus(id string) (*model.TicketVariant, string, error)

	ValidateTicket(ticketID string) (*model.Ticket, error)
	GetScannerStats() (map[string]int64, error)
	GetScannerHistory() ([]map[string]interface{}, error)
}

type ticketService struct {
	ticketRepo repository.TicketRepository
}

func NewTicketService(ticketRepo repository.TicketRepository) TicketService {
	return &ticketService{ticketRepo}
}

// Helper Internal untuk Invalidasi Cache
func invalidateTicketCache() {
	if config.RedisClient != nil {
		ctx := context.Background()
		config.RedisClient.Del(ctx, ticketCacheKey)
		log.Println("🧹 [REDIS] Cache tiket dihapus (Invalidated)")
	}
}

func (s *ticketService) GetActiveTicketVariants() ([]model.TicketVariant, error) {
	ctx := context.Background()

	// 1. Coba ambil data dari Redis (Cache Hit)
	if config.RedisClient != nil {
		val, err := config.RedisClient.Get(ctx, ticketCacheKey).Result()
		if err == nil {
			var cachedVariants []model.TicketVariant
			if json.Unmarshal([]byte(val), &cachedVariants) == nil {
				return cachedVariants, nil
			}
		}
	}

	// 2. Jika tidak ada di Redis (Cache Miss), ambil dari Database
	variants, err := s.ticketRepo.FindActiveVariants(time.Now())
	if err != nil {
		return nil, err
	}

	// 3. Simpan hasil dari DB ke Redis dengan kedaluwarsa 1 Jam
	if config.RedisClient != nil {
		if cacheData, err := json.Marshal(variants); err == nil {
			config.RedisClient.Set(ctx, ticketCacheKey, cacheData, 1*time.Hour)
		}
	}

	return variants, nil
}

func (s *ticketService) TrackTicket(orderID, email string) (*model.Transaction, error) {
	transaction, err := s.ticketRepo.FindTransactionByOrderAndEmail(orderID, email)
	if err != nil {
		return nil, errors.New("data tidak ditemukan. pastikan order id dan email benar")
	}
	return transaction, nil
}

func (s *ticketService) GetAllAdminVariants() ([]model.TicketVariant, error) {
	return s.ticketRepo.FindAllAdminVariants()
}

func (s *ticketService) CreateVariant(input TicketVariantInput) (*model.TicketVariant, error) {
	now := time.Now()
	startDate := now
	if input.StartDate != nil {
		startDate = *input.StartDate
	}

	endDate := now.AddDate(10, 0, 0)
	if input.EndDate != nil {
		endDate = *input.EndDate
	}

	variant := model.TicketVariant{
		ID:        uuid.New().String(),
		Name:      input.Name,
		Price:     input.Price,
		IsActive:  false,
		StartDate: startDate,
		EndDate:   endDate,
	}

	err := s.ticketRepo.CreateVariant(&variant)
	if err != nil {
		return nil, errors.New("gagal menyimpan gelombang tiket ke database")
	}

	invalidateTicketCache()
	return &variant, nil
}

func (s *ticketService) UpdateVariant(id string, input TicketVariantInput) (*model.TicketVariant, error) {
	variant, err := s.ticketRepo.FindVariantByID(id)
	if err != nil {
		return nil, errors.New("tipe tiket tidak ditemukan")
	}

	variant.Name = input.Name
	variant.Price = input.Price
	if input.StartDate != nil {
		variant.StartDate = *input.StartDate
	}
	if input.EndDate != nil {
		variant.EndDate = *input.EndDate
	}

	err = s.ticketRepo.UpdateVariant(variant)
	if err != nil {
		return nil, errors.New("gagal memperbarui data tiket")
	}

	invalidateTicketCache()
	return variant, nil
}

func (s *ticketService) DeleteVariant(id string) error {
	count, _ := s.ticketRepo.CountTicketsByVariantID(id)
	if count > 0 {
		return errors.New("tidak dapat menghapus tiket karena sudah ada transaksi terkait. sebaiknya nonaktifkan tiket ini")
	}

	err := s.ticketRepo.DeleteVariant(id)
	if err != nil {
		return errors.New("gagal menghapus tiket")
	}

	invalidateTicketCache()
	return nil
}

func (s *ticketService) ToggleVariantStatus(id string) (*model.TicketVariant, string, error) {
	variant, err := s.ticketRepo.FindVariantByID(id)
	if err != nil {
		return nil, "", errors.New("tipe tiket tidak ditemukan")
	}

	variant.IsActive = !variant.IsActive
	s.ticketRepo.UpdateVariant(variant)

	statusStr := "ditutup"
	if variant.IsActive {
		statusStr = "dibuka"
	}

	invalidateTicketCache()
	return variant, statusStr, nil
}

func (s *ticketService) ValidateTicket(ticketID string) (*model.Ticket, error) {
	ticket, err := s.ticketRepo.FindTicketByID(ticketID)
	if err != nil {
		return nil, errors.New("qr code tidak valid atau tiket tidak ditemukan")
	}

	if ticket.IsScanned {
		// Trik: Tetap kembalikan pointer tiket-nya bersamaan dengan pesan error!
		return ticket, errors.New("TIKET SUDAH DIGUNAKAN!")
	}

	now := time.Now()
	ticket.IsScanned = true
	ticket.ScannedAt = &now

	err = s.ticketRepo.UpdateTicket(ticket)
	if err != nil {
		return nil, errors.New("gagal mengupdate status tiket")
	}

	return ticket, nil
}

func (s *ticketService) GetScannerStats() (map[string]int64, error) {
	total, scanned, err := s.ticketRepo.GetScannerStats()
	if err != nil {
		return nil, errors.New("gagal mengambil data statistik antrean")
	}

	return map[string]int64{
		"total_tickets":   total,
		"scanned_tickets": scanned,
		"remaining":       total - scanned,
	}, nil
}

func (s *ticketService) GetScannerHistory() ([]map[string]interface{}, error) {
	tickets, err := s.ticketRepo.GetScannerHistory()
	if err != nil {
		return nil, errors.New("gagal menarik riwayat pemindaian")
	}

	var history []map[string]interface{}
	for _, t := range tickets {
		timeStr := ""
		if t.ScannedAt != nil {
			timeStr = t.ScannedAt.Format("15:04:05")
		}

		name := t.AttendeeName
		if name == "" && t.Transaction.CustomerName != "" {
			name = t.Transaction.CustomerName
		}

		history = append(history, map[string]interface{}{
			"id":     t.ID,
			"name":   name,
			"time":   timeStr,
			"source": "global",
		})
	}

	return history, nil
}

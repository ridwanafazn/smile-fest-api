package repository

import (
	"time"

	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"gorm.io/gorm"
)

type TicketRepository interface {
	FindActiveVariants(now time.Time) ([]model.TicketVariant, error)
	FindAllAdminVariants() ([]model.TicketVariant, error)
	FindVariantByID(id string) (*model.TicketVariant, error)
	CreateVariant(variant *model.TicketVariant) error
	UpdateVariant(variant *model.TicketVariant) error
	DeleteVariant(id string) error
	CountTicketsByVariantID(variantID string) (int64, error)

	FindTransactionByOrderAndEmail(orderID, email string) (*model.Transaction, error)

	FindTicketByID(id string) (*model.Ticket, error)
	UpdateTicket(ticket *model.Ticket) error
	GetScannerStats() (int64, int64, error)

	GetScannerHistory() ([]model.Ticket, error)
}

type ticketRepository struct {
	db *gorm.DB
}

func NewTicketRepository(db *gorm.DB) TicketRepository {
	return &ticketRepository{db}
}

func (r *ticketRepository) FindActiveVariants(now time.Time) ([]model.TicketVariant, error) {
	var variants []model.TicketVariant
	err := r.db.Where("is_active = ? AND start_date <= ? AND end_date >= ?", true, now, now).Find(&variants).Error
	return variants, err
}

func (r *ticketRepository) FindAllAdminVariants() ([]model.TicketVariant, error) {
	var variants []model.TicketVariant
	err := r.db.Order("id desc").Find(&variants).Error
	return variants, err
}

func (r *ticketRepository) FindVariantByID(id string) (*model.TicketVariant, error) {
	var variant model.TicketVariant
	err := r.db.Where("id = ?", id).First(&variant).Error
	if err != nil {
		return nil, err
	}
	return &variant, nil
}

func (r *ticketRepository) CreateVariant(variant *model.TicketVariant) error {
	return r.db.Create(variant).Error
}

func (r *ticketRepository) UpdateVariant(variant *model.TicketVariant) error {
	return r.db.Save(variant).Error
}

func (r *ticketRepository) DeleteVariant(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.TicketVariant{}).Error
}

func (r *ticketRepository) CountTicketsByVariantID(variantID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.Ticket{}).Where("ticket_variant_id = ?", variantID).Count(&count).Error
	return count, err
}

func (r *ticketRepository) FindTransactionByOrderAndEmail(orderID, email string) (*model.Transaction, error) {
	var transaction model.Transaction
	err := r.db.Preload("Tickets").Where("id = ? AND customer_email = ?", orderID, email).First(&transaction).Error
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (r *ticketRepository) FindTicketByID(id string) (*model.Ticket, error) {
	var ticket model.Ticket
	err := r.db.Preload("Transaction").Where("id = ?", id).First(&ticket).Error
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *ticketRepository) UpdateTicket(ticket *model.Ticket) error {
	return r.db.Save(ticket).Error
}

func (r *ticketRepository) GetScannerStats() (int64, int64, error) {
	var totalTickets int64
	var scannedTickets int64

	err := r.db.Model(&model.Ticket{}).
		Joins("JOIN transactions ON transactions.id = tickets.transaction_id").
		Where("transactions.status = ?", "settlement").
		Count(&totalTickets).Error
	if err != nil {
		return 0, 0, err
	}

	err = r.db.Model(&model.Ticket{}).Where("is_scanned = ?", true).Count(&scannedTickets).Error
	if err != nil {
		return 0, 0, err
	}

	return totalTickets, scannedTickets, nil
}

func (r *ticketRepository) GetScannerHistory() ([]model.Ticket, error) {
	var tickets []model.Ticket
	err := r.db.Preload("Transaction").
		Where("is_scanned = ?", true).
		Order("scanned_at desc").
		Limit(100).
		Find(&tickets).Error
	return tickets, err
}

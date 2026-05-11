package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Username  string         `gorm:"unique;not null" json:"username"`
	Password  string         `gorm:"not null" json:"-"`
	Role      string         `gorm:"type:varchar(20);default:'scanner'" json:"role"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Voucher struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Code           string    `gorm:"unique;not null" json:"code"`
	DiscountAmount float64   `gorm:"not null" json:"discount_amount"`
	Quota          int       `gorm:"not null" json:"quota"`
	UsageCount     int       `gorm:"default:0" json:"usage_count"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

type Transaction struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	CustomerName  string    `gorm:"not null" json:"customer_name"`
	CustomerEmail string    `gorm:"not null" json:"customer_email"`
	CustomerPhone string    `gorm:"not null" json:"customer_phone"`
	TotalAmount   float64   `gorm:"not null" json:"total_amount"`
	Status        string    `gorm:"default:'pending'" json:"status"`
	VoucherID     *uint     `json:"voucher_id"`
	Voucher       Voucher   `json:"-"`
	SnapToken     string    `json:"snap_token"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Ticket struct {
	ID            uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	TransactionID string      `json:"transaction_id"`
	Transaction   Transaction `json:"-"`
	IsScanned     bool        `gorm:"default:false" json:"is_scanned"`
	ScannedAt     *time.Time  `json:"scanned_at"`
	CreatedAt     time.Time   `json:"created_at"`
}

type TicketVariant struct {
	ID          string  `gorm:"primaryKey" json:"id"`
	Name        string  `gorm:"not null" json:"name"`
	Price       float64 `gorm:"not null" json:"price"`
	IsActive    bool    `gorm:"default:false" json:"is_active"`
	Description string  `json:"description"`
}

func (t *Ticket) BeforeCreate(tx *gorm.DB) (err error) {
	t.ID = uuid.New()
	return
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID = uuid.New()
	return
}

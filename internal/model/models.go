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
	ID             string `gorm:"primaryKey" json:"id"`
	CustomerName   string `gorm:"not null" json:"customer_name"`
	CustomerEmail  string `gorm:"not null" json:"customer_email"`
	CustomerPhone  string `gorm:"not null" json:"customer_phone"`
	CustomerGender string `gorm:"not null" json:"customer_gender"`

	// --- DATA PROFIL PEMESAN ---
	ProfileAge           string `json:"profile_age"`
	ProfileCity          string `json:"profile_city"`
	ProfileEducation     string `json:"profile_education"`
	ProfileJob           string `json:"profile_job"`
	CommunityAffiliation string `json:"community_affiliation"`
	InformationSource    string `json:"information_source"`

	// --- KUESIONER PILIHAN GANDA (Disimpan sebagai string berbatas koma) ---
	InterestReasons     string `json:"interest_reasons"`
	SustainabilitySteps string `json:"sustainability_steps"`

	// --- UNDANGAN KONTRIBUSI ---
	ContributionRole string `json:"contribution_role"` // Donatur, Relawan, Update Info, Peserta Saja

	// --- MANUAL PAYMENT SYSTEM ---
	TotalAmount     float64   `gorm:"not null" json:"total_amount"`            // Total asli + Kode Unik
	UniqueCode      int       `gorm:"not null" json:"unique_code"`             // 3 Digit Angka Unik
	SessionBatch    int       `gorm:"not null;default:1" json:"session_batch"` // 1 (Pagi) atau 2 (Siang)
	PaymentProofURL string    `json:"payment_proof_url"`                       // URL Gambar dari Cloudinary
	ExpiresAt       time.Time `json:"expires_at"`                              // Batas waktu transfer 24 Jam

	// Status: 'pending', 'waiting_verification', 'settlement', 'cancel', 'expired'
	Status    string   `gorm:"default:'pending'" json:"status"`
	VoucherID *uint    `json:"voucher_id"`
	Voucher   *Voucher `json:"voucher,omitempty"`

	// Legacy Midtrans (Disimpan untuk cadangan/rollback)
	SnapToken string `json:"snap_token"`

	// Relasi One-to-Many: 1 Transaksi bisa memegang BANYAK Tiket
	Tickets   []Ticket  `gorm:"foreignKey:TransactionID" json:"tickets"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Ticket struct {
	ID              uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	TransactionID   string       `gorm:"not null" json:"transaction_id"`
	Transaction     *Transaction `json:"-"`
	TicketVariantID string       `gorm:"not null" json:"ticket_variant_id"` // Menyimpan tipe gelombang (Presale, dll)
	AttendeeName    string       `json:"attendee_name"`                     // Nama individu pemegang tiket (berguna untuk grup)
	IsScanned       bool         `gorm:"default:false" json:"is_scanned"`
	ScannedAt       *time.Time   `json:"scanned_at"`
	CreatedAt       time.Time    `json:"created_at"`

	TicketVariant *TicketVariant `gorm:"foreignKey:TicketVariantID" json:"ticket_variant,omitempty"`
}

type TicketVariant struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Price       float64   `gorm:"not null" json:"price"`
	IsActive    bool      `gorm:"default:false" json:"is_active"`
	Description string    `json:"description"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
}

func (t *Ticket) BeforeCreate(tx *gorm.DB) (err error) {
	t.ID = uuid.New()
	return
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID = uuid.New()
	return
}

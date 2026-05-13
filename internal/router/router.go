package router

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/handler"
	"github.com/ridwanafazn/smile-fest-api/internal/middleware"

	_ "github.com/ridwanafazn/smile-fest-api/docs" // Wajib ada untuk membaca hasil generate swag init
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// --- KONFIGURASI CORS ---
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "https://smile-fest.com", "https://smile-festival.ridwanafzn.workers.dev", "https://smile-festival.pages.dev"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// --- ROUTE SWAGGER ---
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// --- ROUTE PUBLIK (Bisa diakses siapa saja) ---
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// Auth
	r.POST("/api/login", handler.Login)
	r.POST("/api/seed-admin", handler.SeedAdmin)

	// Info & Lacak Tiket
	r.GET("/api/tickets/info", handler.GetTicketInfo)
	r.GET("/api/tickets/track", handler.TrackTicket)

	// Info Voucher
	r.GET("/api/vouchers/validate", handler.ValidateVoucher)

	// Checkout & Webhook (Payment)
	r.POST("/api/checkout", handler.Checkout)
	r.POST("/api/webhook/midtrans", handler.MidtransWebhook)

	// --- ROUTE ADMIN (Hanya token dengan role 'admin') ---
	admin := r.Group("/api/admin")
	admin.Use(middleware.AuthMiddleware("admin"))
	{
		// Observabilitas
		admin.GET("/dashboard", handler.GetDashboardStats)
		admin.GET("/transactions", handler.GetTransactions)

		// Manajemen User/Scanner
		admin.POST("/users", handler.CreateUser)
		admin.GET("/users", handler.GetUsers)
		admin.DELETE("/users/:id", handler.DeleteUser)

		// Manajemen Voucher (CRUD Lengkap)
		admin.POST("/vouchers", handler.CreateVoucher)
		admin.GET("/vouchers", handler.GetVouchers)
		admin.PUT("/vouchers/:id", handler.UpdateVoucher)              // [BARU] Edit Voucher
		admin.DELETE("/vouchers/:id", handler.DeleteVoucher)           // [BARU] Hapus Voucher
		admin.PUT("/vouchers/:id/toggle", handler.ToggleVoucherStatus) // Kill Switch

		// Kontrol Presale
		admin.PUT("/ticket-variants/:id", handler.ToggleTicketVariant)
	}

	// --- ROUTE SCANNER (Bisa diakses Scanner & Admin) ---
	scanner := r.Group("/api/scanner")
	scanner.Use(middleware.AuthMiddleware("scanner"))
	{
		scanner.POST("/validate-ticket", handler.ValidateTicket)
		scanner.GET("/stats", handler.GetScannerStats)
	}

	return r
}

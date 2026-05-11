package router

import (
	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/handler"
	"github.com/ridwanafazn/smile-fest-api/internal/middleware"

	_ "github.com/ridwanafazn/smile-fest-api/docs" // Wajib ada untuk membaca hasil generate swag init
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// --- ROUTE SWAGGER ---
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// --- ROUTE PUBLIK (Bisa diakses siapa saja) ---
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// Auth
	r.POST("/api/login", handler.Login)
	// r.POST("/api/seed-admin", handler.SeedAdmin)

	// Info Tiket & Voucher
	r.GET("/api/tickets/info", handler.GetTicketInfo)
	r.GET("/api/vouchers/validate", handler.ValidateVoucher)

	// Checkout & Pembayaran
	r.POST("/api/checkout", handler.Checkout)

	// Webhook Midtrans (Didaftarkan di Dashboard Midtrans nantinya)
	r.POST("/api/webhook/midtrans", handler.MidtransWebhook)

	// --- ROUTE ADMIN (Hanya token dengan role 'admin') ---
	admin := r.Group("/api/admin")
	admin.Use(middleware.AuthMiddleware("admin"))
	{
		// Manajemen User/Scanner
		admin.POST("/users", handler.CreateUser)
		// admin.GET("/dashboard", handler.GetDashboardStats)

		// Manajemen Voucher
		admin.POST("/vouchers", handler.CreateVoucher)
		admin.GET("/vouchers", handler.GetVouchers)
	}

	// --- ROUTE SCANNER (Bisa diakses Scanner & Admin) ---
	scanner := r.Group("/api/scanner")
	scanner.Use(middleware.AuthMiddleware("scanner"))
	{
		// Rute ini sekarang sudah aktif!
		scanner.POST("/validate-ticket", handler.ValidateTicket)
	}

	return r
}

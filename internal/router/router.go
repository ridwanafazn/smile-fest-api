package router

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/handler"
	"github.com/ridwanafazn/smile-fest-api/internal/middleware"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
	"github.com/ridwanafazn/smile-fest-api/internal/service"

	_ "github.com/ridwanafazn/smile-fest-api/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "https://smile-fest.com", "https://smile-festival.ridwanafzn.workers.dev", "https://smile-festival.pages.dev"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	db := config.DB

	userRepo := repository.NewUserRepository(db)
	ticketRepo := repository.NewTicketRepository(db)
	voucherRepo := repository.NewVoucherRepository(db)
	trxRepo := repository.NewTransactionRepository(db)

	userService := service.NewUserService(userRepo)
	ticketService := service.NewTicketService(ticketRepo)
	voucherService := service.NewVoucherService(voucherRepo)
	trxService := service.NewTransactionService(trxRepo, ticketRepo, voucherRepo)

	userHandler := handler.NewUserHandler(userService)
	ticketHandler := handler.NewTicketHandler(ticketService)
	voucherHandler := handler.NewVoucherHandler(voucherService)
	trxHandler := handler.NewTransactionHandler(trxService)

	adminHandler := handler.NewAdminHandler(trxService, userService)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	r.POST("/api/login", userHandler.Login)
	r.POST("/api/seed-admin", userHandler.SeedAdmin)
	r.GET("/api/tickets/info", ticketHandler.GetTicketInfo)
	r.GET("/api/tickets/track", ticketHandler.TrackTicket)
	r.GET("/api/vouchers/validate", voucherHandler.ValidateVoucher)
	r.POST("/api/checkout", trxHandler.Checkout)
	r.POST("/api/transactions/:id/upload-proof", trxHandler.UploadProof)
	r.PUT("/api/transactions/:id/cancel", trxHandler.CancelTransaction)

	admin := r.Group("/api/admin")
	admin.Use(middleware.AuthMiddleware("admin"))
	{
		admin.GET("/dashboard", trxHandler.GetDashboardStats)

		admin.GET("/transactions", trxHandler.GetTransactions)
		admin.PUT("/transactions/:id/verify", adminHandler.VerifyPayment)

		admin.POST("/users", userHandler.CreateUser)
		admin.GET("/users", userHandler.GetUsers)
		admin.PUT("/users/:id", userHandler.UpdateUser)
		admin.DELETE("/users/:id", userHandler.DeleteUser)

		admin.GET("/users/trash", adminHandler.GetTrashedUsers)
		admin.PUT("/users/:id/restore", adminHandler.RestoreUser)
		admin.DELETE("/users/:id/hard-delete", adminHandler.HardDeleteUser)

		admin.POST("/vouchers", voucherHandler.CreateVoucher)
		admin.GET("/vouchers", voucherHandler.GetVouchers)
		admin.PUT("/vouchers/:id", voucherHandler.UpdateVoucher)
		admin.DELETE("/vouchers/:id", voucherHandler.DeleteVoucher)
		admin.PUT("/vouchers/:id/toggle", voucherHandler.ToggleVoucherStatus)

		admin.GET("/ticket-variants", ticketHandler.GetAdminTicketVariants)
		admin.POST("/ticket-variants", ticketHandler.CreateTicketVariant)
		admin.PUT("/ticket-variants/:id", ticketHandler.UpdateTicketVariant)
		admin.DELETE("/ticket-variants/:id", ticketHandler.DeleteTicketVariant)
		admin.PUT("/ticket-variants/:id/toggle", ticketHandler.ToggleTicketVariant)
	}

	scanner := r.Group("/api/scanner")
	scanner.Use(middleware.AuthMiddleware("scanner"))
	{
		scanner.POST("/validate-ticket", ticketHandler.ValidateTicket)
		scanner.GET("/stats", ticketHandler.GetScannerStats)
		scanner.GET("/history", ticketHandler.GetScannerHistory)
	}

	return r
}

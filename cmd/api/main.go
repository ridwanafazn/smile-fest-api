package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
	"github.com/ridwanafazn/smile-fest-api/internal/router"
	"github.com/ridwanafazn/smile-fest-api/internal/service"
	"github.com/ridwanafazn/smile-fest-api/internal/worker"
	"github.com/ridwanafazn/smile-fest-api/pkg/midtrans"
)

var (
	emailWorker *worker.RedisEmailWorker
)

// @title          SMILE FEST API
// @version        1.0
// @description    API Documentation for SMILE FEST 2026 Ticketing System.
// @termsOfService http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file tidak ditemukan")
	}

	// Inisialisasi Database (PostgreSQL & Redis)
	config.ConnectDatabase()
	config.ConnectRedis()

	// Bootstrap admin awal dari env (AMAN: tanpa endpoint publik & tanpa password hardcoded)
	bootstrapInitialAdmin()

	// Inisialisasi Vendor
	midtrans.InitMidtrans()

	// Menyalakan Redis-based Email Worker dengan durability & retry
	emailWorker = worker.NewRedisEmailWorker(3)
	if err := emailWorker.Start(); err != nil {
		log.Printf("⚠️ [EMAIL WORKER] Redis tidak tersedia, fallback ke in-memory queue: %v", err)
		// Fallback ke in-memory worker lama
		go worker.StartEmailWorker()
	} else {
		// Start delayed retry processor
		emailWorker.StartDelayedRetryProcessor()
		log.Println("✅ [EMAIL WORKER] Redis Streams worker started with durability & retry")
	}

	// Menyalakan Sweeper penutupan transaksi pending yang kedaluwarsa
	trxRepo := repository.NewTransactionRepository(config.DB)
	go worker.StartOrderSweeper(context.Background(), trxRepo, 10*time.Minute)

	// Menyalakan Router HTTP
	r := router.SetupRouter(emailWorker)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{Addr: ":" + port, Handler: r}
	go func() {
		log.Printf("🚀 [HTTP] Listening and serving HTTP on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ [HTTP] Server error: %v", err)
		}
	}()

	// T6: graceful shutdown — menunggu sinyal berhenti, menutup HTTP,
	// lalu membiarkan email worker menyelesaikan task yang sedang diproses.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Menerima sinyal berhenti, menutup server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️ [HTTP] Gagal shutdown dengan bersih: %v", err)
	}

	// Shutdown email worker gracefully
	if emailWorker != nil {
		emailWorker.Stop()
		log.Println("✅ [WORKER] Redis email worker stopped gracefully.")
	}
}

// bootstrapInitialAdmin membuat akun admin pertama bila env
// ADMIN_INIT_USERNAME & ADMIN_INIT_PASSWORD diisi dan username tsb
// belum ada di database.
func bootstrapInitialAdmin() {
	username := os.Getenv("ADMIN_INIT_USERNAME")
	password := os.Getenv("ADMIN_INIT_PASSWORD")
	if username == "" || password == "" {
		return // tidak wajib; lewati bila tidak dikonfigurasi
	}

	userRepo := repository.NewUserRepository(config.DB)
	userService := service.NewUserService(userRepo)

	created, err := userService.EnsureAdmin(username, password)
	if err != nil {
		log.Printf("❌ [BOOTSTRAP] Gagal membuat admin awal: %v", err)
		return
	}
	if created {
		log.Printf("✅ [BOOTSTRAP] Admin awal '%s' berhasil dibuat.", username)
	}
}

package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/router"
	"github.com/ridwanafazn/smile-fest-api/pkg/midtrans" // Import package midtrans kita
)

// @title           SMILE FEST API
// @version         1.0
// @description     API Documentation for SMILE FEST 2026 Ticketing System.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// 1. Load variabel environment dari file .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file tidak ditemukan")
	}

	// 2. Koneksi ke Database Neon Tech & Migrasi
	config.ConnectDatabase()

	// 3. Inisialisasi Midtrans Client (INI YANG BARU)
	midtrans.InitMidtrans()

	// 4. Ambil router yang sudah dipisah
	r := router.SetupRouter()

	// 5. Jalankan server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}

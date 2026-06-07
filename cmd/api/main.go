package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/router"
	"github.com/ridwanafazn/smile-fest-api/internal/worker"
	"github.com/ridwanafazn/smile-fest-api/pkg/midtrans"
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

// @host      localhost:8080
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

	// Inisialisasi Vendor
	midtrans.InitMidtrans()

	// Menyalakan Mesin Consumer Event-Driven di Background
	go worker.StartEmailWorker()

	// Menyalakan Router HTTP
	r := router.SetupRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}

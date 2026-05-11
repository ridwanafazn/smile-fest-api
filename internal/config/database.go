package config

import (
	"fmt"
	"log"
	"os"

	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {
	dsn := os.Getenv("DB_URL")
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatal("Gagal koneksi ke database:", err)
	}

	fmt.Println("Mengeksekusi Drop Table Users...")
	// database.Migrator().DropTable(&model.User{})
	// --------------------------

	err = database.AutoMigrate(&model.User{}, &model.Voucher{}, &model.TicketVariant{}, &model.Transaction{}, &model.Ticket{})
	if err != nil {
		log.Fatal("Gagal migrasi database:", err)
	}

	fmt.Println("Database berhasil terkoneksi dan bermigrasi!")
	DB = database
}

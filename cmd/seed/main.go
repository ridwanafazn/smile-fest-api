package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/internal/repository"
	"github.com/ridwanafazn/smile-fest-api/internal/service"
)

func main() {
	_ = godotenv.Load()
	config.ConnectDatabase()

	userRepo := repository.NewUserRepository(config.DB)
	userService := service.NewUserService(userRepo)

	username := os.Getenv("ADMIN_INIT_USERNAME")
	password := os.Getenv("ADMIN_INIT_PASSWORD")

	if username == "" || password == "" {
		fmt.Println("❌ ADMIN_INIT_USERNAME atau ADMIN_INIT_PASSWORD kosong")
		os.Exit(1)
	}

	created, err := userService.EnsureAdmin(username, password)
	if err != nil {
		fmt.Printf("❌ Gagal seed admin: %v\n", err)
		os.Exit(1)
	}

	if created {
		fmt.Printf("✅ Admin '%s' berhasil dibuat (seed baru)\n", username)
	} else {
		fmt.Printf("✅ Admin '%s' sudah ada (tidak perlu seed ulang)\n", username)
	}
}

package config

import (
	"context"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

func ConnectRedis() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Println("⚠️ [REDIS] REDIS_URL tidak ditemukan di .env, caching dinonaktifkan.")
		return
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("❌ [REDIS] Gagal melakukan parse REDIS_URL: %v", err)
	}

	client := redis.NewClient(opt)

	// Uji koneksi dengan Ping
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ [REDIS] Gagal terhubung ke server Redis: %v", err)
	}

	log.Println("✅ [REDIS] Terhubung ke Redis Upstash!")
	RedisClient = client
}

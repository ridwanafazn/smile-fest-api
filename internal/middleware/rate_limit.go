package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateBucket melacak token yang tersedia untuk satu IP.
type rateBucket struct {
	tokens float64
	last   time.Time
}

// rateLimiter adalah token-bucket sederhana di memori (per-proses).
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
	rate    float64 // token per detik
	burst   float64
}

func newRateLimiter(perSec, burst float64) *rateLimiter {
	return &rateLimiter{buckets: map[string]*rateBucket{}, rate: perSec, burst: burst}
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	now := time.Now()
	if !ok {
		b = &rateBucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RateLimit membatasi jumlah request per IP dengan token bucket
// (mis. RateLimit(5, 10) = rata-rata 5/detik, burst 10). Dipakai untuk
// endpoint publik yang mudah di-spam (checkout & upload bukti).
func RateLimit(perSec, burst float64) gin.HandlerFunc {
	limiter := newRateLimiter(perSec, burst)
	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Terlalu banyak permintaan. Coba beberapa saat lagi."})
			return
		}
		c.Next()
	}
}

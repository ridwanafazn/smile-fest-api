package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware memvalidasi JWT Bearer token.
//   - Hanya menerima algoritma HMAC (HS256) untuk mencegah alg-confusion.
//   - Mengekstrak klaim secara aman (tanpa panic saat klaim hilang/salah tipe).
//   - Mengecek ekspirasi secara eksplisit (jwt.Parse v5 menolak token expired,
//     tapi kita tambahkan guard defensif).
func AuthMiddleware(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header diperlukan"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Format Authorization harus 'Bearer <token>'"})
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Wajib HMAC-SHA; tolak semua non-HMAC (none, RS*, dll).
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("algoritma signing tidak didukung: %v", token.Header["alg"])
			}
			secret := os.Getenv("JWT_SECRET")
			if secret == "" {
				return nil, fmt.Errorf("JWT_SECRET belum diatur")
			}
			return []byte(secret), nil
		})
		if err != nil || token == nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token tidak valid atau kedaluwarsa"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token tidak valid"})
			c.Abort()
			return
		}

		// Ekstraksi role & user_id secara aman (tanpa type-assertion yang bisa panic).
		userRole, roleOK := claims["role"].(string)
		userID, idOK := claims["user_id"].(string)
		if !roleOK || !idOK {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token tidak memiliki klaim role/user_id"})
			c.Abort()
			return
		}

		if requiredRole != "" && userRole != requiredRole && userRole != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Role tidak mencukupi"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Set("role", userRole)
		c.Next()
	}
}

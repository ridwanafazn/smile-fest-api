package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header diperlukan"})
			c.Abort()
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userRole := claims["role"].(string)

			// Cek Role: Jika admin, izinkan akses ke semua endpoint.
			// Jika bukan admin, pastikan role sama persis dengan yang dibutuhkan (misal: scanner).
			if requiredRole != "" && userRole != requiredRole && userRole != "admin" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak: Role tidak mencukupi"})
				c.Abort()
				return
			}

			c.Set("user_id", claims["user_id"])
			c.Set("role", userRole)
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token tidak valid: " + err.Error()})
			c.Abort()
		}
	}
}

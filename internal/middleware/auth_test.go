package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", testSecret)
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func sign(claims jwt.MapClaims) string {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := t.SignedString([]byte(testSecret))
	return s
}

func future() int64 { return time.Now().Add(time.Hour).Unix() }

func setupRoute(requiredRole string) *gin.Engine {
	r := gin.New()
	r.Use(AuthMiddleware(requiredRole))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func doGet(r *gin.Engine, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuth_ValidToken(t *testing.T) {
	r := setupRoute("admin")
	w := doGet(r, sign(jwt.MapClaims{"user_id": "u1", "role": "admin", "exp": future()}))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	r := setupRoute("admin")
	w := doGet(r, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestAuth_NotBearerFormat(t *testing.T) {
	r := setupRoute("admin")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", sign(jwt.MapClaims{"user_id": "u1", "role": "admin", "exp": future()}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	r := setupRoute("admin")
	w := doGet(r, sign(jwt.MapClaims{"user_id": "u1", "role": "admin", "exp": time.Now().Add(-time.Hour).Unix()}))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestAuth_WrongRole(t *testing.T) {
	r := setupRoute("admin")
	w := doGet(r, sign(jwt.MapClaims{"user_id": "u1", "role": "scanner", "exp": future()}))
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", w.Code)
	}
}

func TestAuth_MissingRoleClaim(t *testing.T) {
	r := setupRoute("admin")
	w := doGet(r, sign(jwt.MapClaims{"user_id": "u1", "exp": future()}))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

// Token ditandatangani dengan algoritma "none" harus ditolak (alg-confusion).
func TestAuth_NoneAlgorithm(t *testing.T) {
	r := setupRoute("admin")
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"user_id": "u1", "role": "admin", "exp": future()})
	s, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	w := doGet(r, s)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

// Token dengan secret salah harus ditolak.
func TestAuth_WrongSecret(t *testing.T) {
	r := setupRoute("admin")
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": "u1", "role": "admin", "exp": future()})
	s, _ := tok.SignedString([]byte("wrong-secret"))
	w := doGet(r, s)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

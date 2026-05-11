package midtrans

import (
	"crypto/sha512"
	"encoding/hex"
	"os"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
)

var SnapClient snap.Client

// InitMidtrans dipanggil saat server pertama kali menyala (bisa ditaruh di main.go nanti)
func InitMidtrans() {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	isProd := os.Getenv("MIDTRANS_IS_PROD")

	env := midtrans.Sandbox
	if isProd == "true" {
		env = midtrans.Production
	}

	// Setup Global Midtrans Environment
	midtrans.ServerKey = serverKey
	midtrans.Environment = env

	// Setup Snap Client
	SnapClient.New(serverKey, env)
}

// VerifySignatureKey memverifikasi keaslian webhook dari Midtrans
func VerifySignatureKey(orderID, statusCode, grossAmount, signatureKey string) bool {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	payload := orderID + statusCode + grossAmount + serverKey

	hasher := sha512.New()
	hasher.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(hasher.Sum(nil))

	return expectedSignature == signatureKey
}

package midtrans

import (
	"crypto/sha512"
	"encoding/hex"
	"log"
	"os"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
)

var SnapClient snap.Client

// InitMidtrans dipanggil saat server pertama kali menyala (bisa ditaruh di main.go nanti)
// CATATAN: Pada branch ini (Manual Payment), fungsi Midtrans dipertahankan
// hanya sebagai Legacy/Cadangan jika ingin rollback.
func InitMidtrans() {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	isProd := os.Getenv("MIDTRANS_IS_PROD")

	if serverKey == "" {
		log.Println("⚠️ [MIDTRANS] Server Key kosong. Berjalan dalam mode Manual Payment System.")
		return
	}

	env := midtrans.Sandbox
	if isProd == "true" {
		env = midtrans.Production
	}

	// Setup Global Midtrans Environment
	midtrans.ServerKey = serverKey
	midtrans.Environment = env

	// Setup Snap Client
	SnapClient.New(serverKey, env)
	log.Println("⚠️ [MIDTRANS] Snap Client diinisialisasi (Legacy Mode standby).")
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

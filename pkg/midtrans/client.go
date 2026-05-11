package midtrans

import (
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

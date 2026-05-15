package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// EmailData berisi variabel yang akan dikirim ke Google Apps Script
type EmailData struct {
	CustomerName string
	OrderID      string
	TicketLink   string
}

// SendTicketEmail bertugas mengirimkan instruksi ke Google Apps Script via HTTP POST
func SendTicketEmail(toEmail string, data EmailData) error {
	// Memanggil URL Web App dari Google Apps Script yang sudah di-deploy
	gasURL := os.Getenv("GAS_URL")

	// Bypass jika config kosong (berguna saat mode development lokal)
	if gasURL == "" {
		fmt.Println("⚠️ GAS_URL kosong, bypass pengiriman email ke:", toEmail)
		return nil
	}

	// Bungkus data menjadi JSON sesuai format yang ditangkap oleh fungsi doPost di GAS
	payload := map[string]string{
		"to":           toEmail,
		"subject":      fmt.Sprintf("E-Ticket SMILE FEST 2026 - %s", data.OrderID),
		"customerName": data.CustomerName,
		"orderId":      data.OrderID,
		"ticketLink":   data.TicketLink,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("gagal membungkus payload JSON: %v", err)
	}

	// Tembak URL Google Script via HTTP POST (Jalur Port 443 HTTPS - Dijamin lolos firewall Railway)
	resp, err := http.Post(gasURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("gagal menghubungi Bridge Email (GAS): %v", err)
	}
	defer resp.Body.Close()

	// GAS biasanya merespon dengan 200 OK, tapi kadang 302 Found (Redirect) untuk script. Keduanya aman.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return fmt.Errorf("bridge email merespon dengan status HTTP tidak wajar: %d", resp.StatusCode)
	}

	return nil
}

package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// EmailData berisi variabel dasar untuk email E-Ticket (Lunas)
type EmailData struct {
	CustomerName string
	OrderID      string
	TicketLink   string
}

// InstructionData berisi variabel spesifik untuk email Instruksi Pembayaran (Pending)
type InstructionData struct {
	CustomerName string
	OrderID      string
	TrackLink    string
	TotalAmount  string // Format langsung dari handler, misal: "Rp 59.123"
}

// SendInstructionEmail mengirimkan email instruksi pembayaran setelah checkout berhasil
func SendInstructionEmail(toEmail string, data InstructionData) error {
	gasURL := os.Getenv("GAS_URL")

	if gasURL == "" {
		fmt.Println("⚠️ GAS_URL kosong, bypass pengiriman instruksi email ke:", toEmail)
		return nil
	}

	// Payload spesifik untuk instruksi pembayaran
	payload := map[string]string{
		"action":       "sendInstruction", // Kunci penting untuk membedakan template di GAS
		"to":           toEmail,
		"subject":      fmt.Sprintf("Selesaikan Pembayaran Anda - SMILE FEST 2026 (%s)", data.OrderID),
		"customerName": data.CustomerName,
		"orderId":      data.OrderID,
		"trackLink":    data.TrackLink,
		"totalAmount":  data.TotalAmount,
	}

	return sendToGAS(gasURL, payload)
}

// SendTicketEmail bertugas mengirimkan E-Ticket saat status transaksi menjadi settlement
func SendTicketEmail(toEmail string, data EmailData) error {
	gasURL := os.Getenv("GAS_URL")

	if gasURL == "" {
		fmt.Println("⚠️ GAS_URL kosong, bypass pengiriman tiket email ke:", toEmail)
		return nil
	}

	payload := map[string]string{
		"action":       "sendTicket", // Kunci penting untuk membedakan template di GAS
		"to":           toEmail,
		"subject":      fmt.Sprintf("E-Ticket SMILE FEST 2026 - %s", data.OrderID),
		"customerName": data.CustomerName,
		"orderId":      data.OrderID,
		"ticketLink":   data.TicketLink,
	}

	return sendToGAS(gasURL, payload)
}

// FormatRupiah memformat angka float64 menjadi string dengan pemisah ribuan (titik)
func FormatRupiah(amount float64) string {
	// Konversi ke int64 untuk membuang nilai desimal
	str := fmt.Sprintf("%d", int64(amount))

	var result []byte
	for i, c := range str {
		// Sisipkan titik setiap kelipatan 3 dari belakang (kecuali di awal string)
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, '.')
		}
		result = append(result, byte(c))
	}

	return string(result)
}

// sendToGAS adalah fungsi helper internal agar kita tidak menulis ulang logika HTTP POST
func sendToGAS(gasURL string, payload map[string]string) error {
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

	// GAS biasanya merespon dengan 200 OK, tapi kadang 302 Found (Redirect) untuk eksekusi script. Keduanya aman.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return fmt.Errorf("bridge email merespon dengan status HTTP tidak wajar: %d", resp.StatusCode)
	}

	return nil
}

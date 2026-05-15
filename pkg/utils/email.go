package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
)

// EmailData berisi variabel yang akan dilempar ke template HTML
type EmailData struct {
	CustomerName string
	OrderID      string
	TicketLink   string
}

// SendTicketEmail bertugas mengirimkan E-Ticket ke email pembeli
func SendTicketEmail(toEmail string, data EmailData) error {
	smtpHost := os.Getenv("SMTP_HOST")           // cth: smtp.gmail.com
	smtpPort := os.Getenv("SMTP_PORT")           // cth: 587
	senderEmail := os.Getenv("SMTP_EMAIL")       // Email panitia
	senderPassword := os.Getenv("SMTP_PASSWORD") // App Password

	// Bypass jika config kosong (berguna saat mode development lokal)
	if smtpHost == "" || senderEmail == "" {
		fmt.Println("⚠️ SMTP Config kosong, bypass pengiriman email ke:", toEmail)
		return nil
	}

	auth := smtp.PlainAuth("", senderEmail, senderPassword, smtpHost)

	// Template HTML Email Responsif (Diperbarui copywrtiting-nya)
	tmpl := `
	<!DOCTYPE html>
	<html>
	<body style="font-family: sans-serif; line-height: 1.6; color: #333;">
		<div style="max-w-md; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 10px;">
			<h2 style="color: #292524;">Halo, {{.CustomerName}}!</h2>
			<p>Terima kasih telah mengamankan tiket <strong>SMILE FEST 2026</strong>.</p>
			<p>Pembayaran Anda untuk Order ID <strong>{{.OrderID}}</strong> telah kami terima (LUNAS).</p>
			<p>Klik tombol di bawah ini untuk melihat status pesanan Anda. <strong>PENTING:</strong> Anda harus melengkapi kuesioner singkat di halaman pelacakan sebelum sistem menerbitkan QR Code Anda.</p>
			<div style="text-align: center; margin: 30px 0;">
				<a href="{{.TicketLink}}" style="background-color: #292524; color: #ffffff; padding: 12px 24px; text-decoration: none; border-radius: 8px; font-weight: bold;">Buka E-Ticket Saya</a>
			</div>
			<hr style="border: none; border-top: 1px solid #eee;" />
			<p style="font-size: 12px; color: #888;">Sampai jumpa di venue!<br/>Salam hangat,<br/>Tim SMILE FEST</p>
		</div>
	</body>
	</html>
	`

	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	body.Write([]byte(fmt.Sprintf("To: %s\r\n", toEmail)))
	body.Write([]byte(fmt.Sprintf("Subject: E-Ticket SMILE FEST 2026 - %s\r\n", data.OrderID)))
	body.Write([]byte("MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"))

	err = t.Execute(&body, data)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	err = smtp.SendMail(addr, auth, senderEmail, []string{toEmail}, body.Bytes())
	if err != nil {
		return err
	}

	fmt.Println("✅ Email tiket berhasil dikirim ke:", toEmail)
	return nil
}

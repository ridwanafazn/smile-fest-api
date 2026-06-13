package worker

import (
	"log"
	"time"

	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

// Tipe tugas yang dipahami oleh Worker
type EmailTaskType string

const (
	TaskInstruction EmailTaskType = "INSTRUCTION"
	TaskTicket      EmailTaskType = "TICKET"
	TaskBlast       EmailTaskType = "BLAST"
)

// Struktur Payload Pesan Event
type EmailTask struct {
	Type            EmailTaskType
	CustomerEmail   string
	InstructionData *utils.InstructionData
	TicketData      *utils.EmailData
	BlastData       *utils.BlastData
}

// Channel bertindak sebagai in-memory Message Queue / Broker
var EmailQueue = make(chan EmailTask, 200)

// StartEmailWorker dijalankan di dalam Goroutine pada main.go
func StartEmailWorker() {
	log.Println("🚀 [WORKER] Email Queue Consumer siap mendengarkan event...")

	for task := range EmailQueue {
		switch task.Type {
		case TaskInstruction:
			if task.InstructionData != nil {
				err := utils.SendInstructionEmail(task.CustomerEmail, *task.InstructionData)
				if err != nil {
					log.Printf("❌ [WORKER ERROR] Gagal mengirim instruksi pembayaran ke %s: %v\n", task.CustomerEmail, err)
				} else {
					log.Printf("✅ [WORKER SUCCESS] Instruksi pembayaran asinkron dikirim ke %s\n", task.CustomerEmail)
				}
			}

		case TaskTicket:
			if task.TicketData != nil {
				err := utils.SendTicketEmail(task.CustomerEmail, *task.TicketData)
				if err != nil {
					log.Printf("❌ [WORKER ERROR] Gagal mengirim e-ticket ke %s: %v\n", task.CustomerEmail, err)
				} else {
					log.Printf("✅ [WORKER SUCCESS] E-Ticket asinkron dikirim ke %s\n", task.CustomerEmail)
				}
			}

		case TaskBlast:
			if task.BlastData != nil {
				err := utils.SendBlastEmail(task.CustomerEmail, *task.BlastData)
				if err != nil {
					log.Printf("❌ [WORKER ERROR] Gagal mengirim email blast ke %s: %v\n", task.CustomerEmail, err)
				} else {
					log.Printf("✅ [WORKER SUCCESS] Email blast asinkron dikirim ke %s\n", task.CustomerEmail)
				}
				time.Sleep(250 * time.Millisecond)
			}
		}
	}
}

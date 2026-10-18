package worker

import (
	"log"
	"sync"

	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

type EmailTaskType string

const (
	TaskInstruction EmailTaskType = "INSTRUCTION"
	TaskTicket      EmailTaskType = "TICKET"
)

// Struktur Payload Pesan Event
type EmailTask struct {
	Type            EmailTaskType
	CustomerEmail   string
	InstructionData *utils.InstructionData
	TicketData      *utils.EmailData
}

const (
	emailQueueBuffer = 1000
	emailWorkerCount = 3
)

// Channel bertindak sebagai in-memory Message Queue / Broker
var EmailQueue = make(chan EmailTask, emailQueueBuffer)

// StartEmailWorker menjalankan pool konsumen dari antrian (T5):
// beberapa goroutine menarik task sekaligus sehingga antrian tidak menumpuk
// dan request HTTP yang meng-enqueue tidak mudah terblokir.
func StartEmailWorker() {
	var wg sync.WaitGroup
	for i := 0; i < emailWorkerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			log.Printf("🚀 [WORKER-%d] Email Consumer siap mendengarkan event...", id)
			for task := range EmailQueue {
				processTask(task)
			}
			log.Printf("🛑 [WORKER-%d] Antrian ditutup, berhenti.", id)
		}(i)
	}
	// Blokir sampai semua konsumen selesai (terjadi saat EmailQueue ditutup
	// pada graceful shutdown di main.go).
	wg.Wait()
}

func processTask(task EmailTask) {
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
	}
}

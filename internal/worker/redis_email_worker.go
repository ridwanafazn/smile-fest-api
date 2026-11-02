package worker

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/ridwanafazn/smile-fest-api/internal/config"
	"github.com/ridwanafazn/smile-fest-api/pkg/utils"
)

const (
	// Redis Stream keys
	EmailStreamKey      = "email:queue"
	EmailDLQKey         = "email:dlq"
	EmailConsumerGroup  = "email-workers"

	// Retry configuration
	MaxRetries      = 3
	BaseRetryDelay  = 5 * time.Second
	MaxRetryDelay   = 5 * time.Minute
	ClaimIdleTime   = 30 * time.Second // Claim stuck messages after this time

	// Worker configuration
	DefaultWorkerCount = 3
	BlockDuration      = 5 * time.Second // XREADGROUP block time
)

// RedisEmailTaskType represents the type of email task
type RedisEmailTaskType string

const (
	RedisTaskInstruction RedisEmailTaskType = "INSTRUCTION"
	RedisTaskTicket      RedisEmailTaskType = "TICKET"
)

// RedisEmailTask represents a serializable email task for Redis Streams
type RedisEmailTask struct {
	ID              string                 `json:"id"`               // Redis Stream entry ID
	Type            RedisEmailTaskType     `json:"type"`
	CustomerEmail   string                 `json:"customer_email"`
	InstructionData *utils.InstructionData `json:"instruction_data,omitempty"`
	TicketData      *utils.EmailData       `json:"ticket_data,omitempty"`
	RetryCount      int                    `json:"retry_count"`
	CreatedAt       time.Time              `json:"created_at"`
	LastAttemptAt   *time.Time             `json:"last_attempt_at,omitempty"`
}

// RedisEmailWorker manages the Redis Streams based email worker pool
type RedisEmailWorker struct {
	client       *redis.Client
	workerCount  int
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	consumerName string
}

// NewRedisEmailWorker creates a new Redis-based email worker
func NewRedisEmailWorker(workerCount int) *RedisEmailWorker {
	if workerCount <= 0 {
		workerCount = DefaultWorkerCount
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisEmailWorker{
		client:       config.RedisClient,
		workerCount:  workerCount,
		ctx:          ctx,
		cancel:       cancel,
		consumerName: "worker-" + strconv.Itoa(int(time.Now().UnixNano()%10000)),
	}
}

// Start begins the worker pool and consumer group setup
func (w *RedisEmailWorker) Start() error {
	if w.client == nil {
		log.Println("⚠️ [REDIS EMAIL WORKER] Redis client not available, falling back to in-memory queue")
		return ErrRedisNotAvailable
	}

	// Ensure consumer group exists
	if err := w.ensureConsumerGroup(); err != nil {
		return err
	}

	log.Printf("🚀 [REDIS EMAIL WORKER] Starting %d workers (consumer: %s)", w.workerCount, w.consumerName)

	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.runConsumer(i)
	}

	// Start background tasks
	w.wg.Add(1)
	go w.reclaimStalledMessages()

	w.wg.Add(1)
	go w.cleanupDLQ()

	return nil
}

// Stop gracefully shuts down all workers
func (w *RedisEmailWorker) Stop() {
	log.Println("🛑 [REDIS EMAIL WORKER] Shutting down...")
	w.cancel()
	w.wg.Wait()
	log.Println("✅ [REDIS EMAIL WORKER] All workers stopped")
}

// ensureConsumerGroup creates the consumer group if it doesn't exist
func (w *RedisEmailWorker) ensureConsumerGroup() error {
	err := w.client.XGroupCreateMkStream(w.ctx, EmailStreamKey, EmailConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// runConsumer runs a single consumer loop
func (w *RedisEmailWorker) runConsumer(workerID int) {
	defer w.wg.Done()
	consumerName := w.consumerName + "-" + strconv.Itoa(workerID)
	log.Printf("🚀 [REDIS EMAIL WORKER-%d] Consumer %s started", workerID, consumerName)

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("🛑 [REDIS EMAIL WORKER-%d] Consumer %s stopping", workerID, consumerName)
			return
		default:
			// Read from stream using consumer group
			streams, err := w.client.XReadGroup(w.ctx, &redis.XReadGroupArgs{
				Group:    EmailConsumerGroup,
				Consumer: consumerName,
				Streams:  []string{EmailStreamKey, ">"},
				Count:    10,
				Block:    BlockDuration,
			}).Result()

			if err != nil {
				if err == redis.Nil || err == context.Canceled {
					continue
				}
				log.Printf("❌ [REDIS EMAIL WORKER-%d] XReadGroup error: %v", workerID, err)
				time.Sleep(time.Second)
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					w.processMessage(workerID, consumerName, msg)
				}
			}
		}
	}
}

// processMessage handles a single message from the stream
func (w *RedisEmailWorker) processMessage(workerID int, consumerName string, msg redis.XMessage) {
	var task RedisEmailTask
	data, ok := msg.Values["data"].(string)
	if !ok {
		log.Printf("❌ [REDIS EMAIL WORKER-%d] Invalid message format, sending to DLQ", workerID)
		w.sendToDLQ(msg, "invalid_message_format")
		w.ackMessage(msg.ID)
		return
	}

	if err := json.Unmarshal([]byte(data), &task); err != nil {
		log.Printf("❌ [REDIS EMAIL WORKER-%d] Failed to unmarshal task: %v", workerID, err)
		w.sendToDLQ(msg, "unmarshal_failed: "+err.Error())
		w.ackMessage(msg.ID)
		return
	}

	// Update last attempt time
	now := time.Now()
	task.LastAttemptAt = &now
	task.RetryCount++

	var err error
	switch task.Type {
	case RedisTaskInstruction:
		if task.InstructionData != nil {
			err = utils.SendInstructionEmail(task.CustomerEmail, *task.InstructionData)
		}
	case RedisTaskTicket:
		if task.TicketData != nil {
			err = utils.SendTicketEmail(task.CustomerEmail, *task.TicketData)
		}
	default:
		err = ErrUnknownTaskType
	}

	if err != nil {
		log.Printf("❌ [REDIS EMAIL WORKER-%d] Failed to send email to %s (attempt %d/%d): %v",
			workerID, task.CustomerEmail, task.RetryCount, MaxRetries, err)

		if task.RetryCount >= MaxRetries {
			log.Printf("💀 [REDIS EMAIL WORKER-%d] Max retries exceeded for %s, sending to DLQ",
				workerID, task.CustomerEmail)
			w.sendToDLQ(msg, "max_retries_exceeded: "+err.Error())
			w.ackMessage(msg.ID)
			return
		}

		// Re-queue with delay using Redis sorted set for delayed retry
		w.scheduleRetry(task)
		w.ackMessage(msg.ID)
		return
	}

	log.Printf("✅ [REDIS EMAIL WORKER-%d] Email sent successfully to %s (type: %s)",
		workerID, task.CustomerEmail, task.Type)
	w.ackMessage(msg.ID)
}

// scheduleRetry re-queues the task with exponential backoff
func (w *RedisEmailWorker) scheduleRetry(task RedisEmailTask) {
	delay := calculateBackoff(task.RetryCount)
	executeAt := time.Now().Add(delay).UnixMilli()

	taskJSON, _ := json.Marshal(task)
	// Use a sorted set for delayed execution: key = email:delayed, score = executeAt timestamp
	w.client.ZAdd(w.ctx, "email:delayed", redis.Z{
		Score:  float64(executeAt),
		Member: string(taskJSON),
	})
}

// reclaimStalledMessages periodically claims messages that were delivered but not acknowledged
func (w *RedisEmailWorker) reclaimStalledMessages() {
	defer w.wg.Done()
	ticker := time.NewTicker(ClaimIdleTime)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.claimStalled()
		}
	}
}

// claimStalled uses XPENDING and XCLAIM to recover unacknowledged messages
func (w *RedisEmailWorker) claimStalled() {
	pending, err := w.client.XPendingExt(w.ctx, &redis.XPendingExtArgs{
		Stream: EmailStreamKey,
		Group:  EmailConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
		Idle:   ClaimIdleTime,
	}).Result()

	if err != nil {
		log.Printf("⚠️ [REDIS EMAIL WORKER] XPendingExt error: %v", err)
		return
	}

	if len(pending) == 0 {
		return
	}

	messageIDs := make([]string, len(pending))
	for i, p := range pending {
		messageIDs[i] = p.ID
	}

	claimed, err := w.client.XClaim(w.ctx, &redis.XClaimArgs{
		Stream:   EmailStreamKey,
		Group:    EmailConsumerGroup,
		Consumer: w.consumerName + "-reclaimer",
		MinIdle:  ClaimIdleTime,
		Messages: messageIDs,
	}).Result()

	if err != nil {
		log.Printf("⚠️ [REDIS EMAIL WORKER] XClaim error: %v", err)
		return
	}

	for _, msg := range claimed {
		log.Printf("🔄 [REDIS EMAIL WORKER] Reclaimed stalled message %s", msg.ID)
		w.processMessage(-1, "reclaimer", msg)
	}
}

// processDelayedRetries checks for tasks ready to be retried
func (w *RedisEmailWorker) processDelayedRetries() {
	now := time.Now().UnixMilli()
	tasks, err := w.client.ZRangeByScore(w.ctx, "email:delayed", &redis.ZRangeBy{
		Min:   "0",
		Max:   strconv.FormatInt(now, 10),
		Count: 50,
	}).Result()

	if err != nil {
		return
	}

	if len(tasks) == 0 {
		return
	}

	// Remove from delayed queue
	w.client.ZRem(w.ctx, "email:delayed", tasks)

	// Re-add to stream
	for _, taskJSON := range tasks {
		w.client.XAdd(w.ctx, &redis.XAddArgs{
			Stream: EmailStreamKey,
			Values: map[string]interface{}{"data": taskJSON},
		})
	}
}

// cleanupDLQ periodically cleans old DLQ entries (older than 7 days)
func (w *RedisEmailWorker) cleanupDLQ() {
	defer w.wg.Done()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-7 * 24 * time.Hour).UnixMilli()
			w.client.ZRemRangeByScore(w.ctx, EmailDLQKey, "0", strconv.FormatInt(cutoff, 10))
		}
	}
}

// sendToDLQ moves a failed message to the Dead Letter Queue
func (w *RedisEmailWorker) sendToDLQ(msg redis.XMessage, reason string) {
	dlqEntry := map[string]interface{}{
		"original_id":    msg.ID,
		"original_data":  msg.Values["data"],
		"failure_reason": reason,
		"failed_at":      time.Now().Format(time.RFC3339),
		"consumer":       w.consumerName,
	}
	dlqJSON, _ := json.Marshal(dlqEntry)
	w.client.ZAdd(w.ctx, EmailDLQKey, redis.Z{
		Score:  float64(time.Now().UnixMilli()),
		Member: string(dlqJSON),
	})
}

// ackMessage acknowledges a message in the consumer group
func (w *RedisEmailWorker) ackMessage(msgID string) {
	w.client.XAck(w.ctx, EmailStreamKey, EmailConsumerGroup, msgID)
}

// EnqueueEmail adds an email task to the Redis Stream
func (w *RedisEmailWorker) EnqueueEmail(task RedisEmailTask) error {
	if w.client == nil {
		return ErrRedisNotAvailable
	}

	task.CreatedAt = time.Now()
	task.RetryCount = 0

	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	return w.client.XAdd(w.ctx, &redis.XAddArgs{
		Stream: EmailStreamKey,
		Values: map[string]interface{}{"data": string(data)},
	}).Err()
}

// GetDLQEntries retrieves dead letter queue entries for inspection
func (w *RedisEmailWorker) GetDLQEntries(limit int) ([]map[string]interface{}, error) {
	entries, err := w.client.ZRevRangeWithScores(w.ctx, EmailDLQKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		var dlqEntry map[string]interface{}
		if err := json.Unmarshal([]byte(entry.Member.(string)), &dlqEntry); err == nil {
			dlqEntry["failed_at_timestamp"] = entry.Score
			result = append(result, dlqEntry)
		}
	}
	return result, nil
}

// RetryDLQEntry re-queues a DLQ entry back to the main stream
func (w *RedisEmailWorker) RetryDLQEntry(originalData string) error {
	var task RedisEmailTask
	if err := json.Unmarshal([]byte(originalData), &task); err != nil {
		return err
	}
	task.RetryCount = 0 // Reset retry count for manual retry
	return w.EnqueueEmail(task)
}

// EmailWorkerInterface defines the contract for email queue implementations
type EmailWorkerInterface interface {
	EnqueueEmail(task RedisEmailTask) error
	Start() error
	Stop()
}

// Error definitions
var (
	ErrRedisNotAvailable = &WorkerError{Code: "REDIS_UNAVAILABLE", Message: "Redis client not configured"}
	ErrUnknownTaskType   = &WorkerError{Code: "UNKNOWN_TASK_TYPE", Message: "Unknown email task type"}
)

// Compile-time check that RedisEmailWorker implements EmailWorkerInterface
var _ EmailWorkerInterface = (*RedisEmailWorker)(nil)

type WorkerError struct {
	Code    string
	Message string
}

func (e *WorkerError) Error() string {
	return e.Code + ": " + e.Message
}

// calculateBackoff computes exponential backoff with jitter
func calculateBackoff(retryCount int) time.Duration {
	delay := BaseRetryDelay * time.Duration(1<<uint(retryCount-1)) // 5s, 10s, 20s...
	if delay > MaxRetryDelay {
		delay = MaxRetryDelay
	}
	// Add jitter (±10%)
	jitter := time.Duration(float64(delay) * 0.1 * (2*float64(time.Now().UnixNano()%100)/100 - 1))
	return delay + jitter
}

// StartDelayedRetryProcessor starts a background goroutine to process delayed retries
// Call this once after worker.Start()
func (w *RedisEmailWorker) StartDelayedRetryProcessor() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				w.processDelayedRetries()
			}
		}
	}()
}
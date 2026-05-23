package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// MessageQueue provides store-and-forward messaging for offline resilience.
// When IPC sends fail (no clients connected), messages are stored in SQLite
// and drained when clients reconnect.
type MessageQueue struct {
	db            *Database
	ipcServer     *IPCServer
	drainInterval time.Duration
	maxRetries    int
	stopCh        chan struct{}
	running       bool
	mu            sync.Mutex
}

// NewMessageQueue creates a new message queue
func NewMessageQueue(db *Database, ipcServer *IPCServer) *MessageQueue {
	return &MessageQueue{
		db:            db,
		ipcServer:     ipcServer,
		drainInterval: time.Duration(GetQueueDrainIntervalSecs()) * time.Second,
		maxRetries:    GetQueueMaxRetries(),
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background drain goroutine
func (mq *MessageQueue) Start() {
	mq.mu.Lock()
	if mq.running {
		mq.mu.Unlock()
		return
	}
	mq.running = true
	mq.mu.Unlock()

	log.Printf("Message queue started (drain interval: %s, max retries: %d)",
		mq.drainInterval, mq.maxRetries)

	go func() {
		ticker := time.NewTicker(mq.drainInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mq.DrainOnce()
			case <-mq.stopCh:
				log.Println("Message queue stopped")
				return
			}
		}
	}()
}

// Stop stops the message queue drain goroutine
func (mq *MessageQueue) Stop() {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.running {
		close(mq.stopCh)
		mq.running = false
	}
}

// SendOrQueue attempts to send a message via IPC. If sending fails (no clients),
// the message is queued in SQLite for later delivery.
func (mq *MessageQueue) SendOrQueue(msg IPCMessage) error {
	// Try direct send first
	err := mq.ipcServer.SendMessage(msg)
	if err == nil {
		return nil
	}

	// If error is not ErrNoClients, it's a different problem
	if err != ErrNoClients {
		log.Printf("Queue: IPC send error (non-recoverable): %v", err)
		// Still enqueue for retry
	}

	// Enqueue for later delivery
	return mq.Enqueue(msg)
}

// Enqueue stores a message in the SQLite queue for later delivery
func (mq *MessageQueue) Enqueue(msg IPCMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message for queue: %w", err)
	}

	record := QueuedMessage{
		MessageType: string(msg.Type),
		MessageID:   msg.ID,
		Payload:     string(payload),
		Status:      "pending",
		MaxRetries:  mq.maxRetries,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	id, err := mq.db.EnqueueMessage(record)
	if err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	log.Printf("Queue: message enqueued (id=%d, type=%s)", id, msg.Type)
	return nil
}

// DrainOnce attempts to send all pending messages in the queue.
// Called periodically by the background goroutine.
func (mq *MessageQueue) DrainOnce() {
	// Only try to drain if there are clients
	if !mq.ipcServer.HasClients() {
		return
	}

	messages, err := mq.db.GetPendingMessages(50)
	if err != nil {
		log.Printf("Queue: failed to get pending messages: %v", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	log.Printf("Queue: draining %d pending messages", len(messages))

	sent := 0
	failed := 0
	for _, qm := range messages {
		var msg IPCMessage
		if err := json.Unmarshal([]byte(qm.Payload), &msg); err != nil {
			log.Printf("Queue: failed to unmarshal queued message %d: %v", qm.ID, err)
			mq.db.MarkMessageFailed(qm.ID, fmt.Sprintf("unmarshal error: %v", err))
			failed++
			continue
		}

		if err := mq.ipcServer.SendMessage(msg); err != nil {
			mq.db.MarkMessageFailed(qm.ID, err.Error())
			failed++
			// If clients disconnected during drain, stop
			if err == ErrNoClients {
				log.Printf("Queue: clients disconnected during drain, stopping")
				break
			}
		} else {
			mq.db.MarkMessageSent(qm.ID)
			sent++
		}
	}

	if sent > 0 || failed > 0 {
		log.Printf("Queue: drain complete — %d sent, %d failed", sent, failed)
	}

	// Clean old sent messages
	retentionDays := GetQueueRetentionDays()
	if err := mq.db.CleanOldMessages(retentionDays); err != nil {
		log.Printf("Queue: failed to clean old messages: %v", err)
	}
}

// GetStats returns queue statistics
func (mq *MessageQueue) GetStats() (pending, failed, sent int, err error) {
	return mq.db.GetMessageQueueStats()
}

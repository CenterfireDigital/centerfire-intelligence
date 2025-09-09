package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type ClickHouseConsumer struct {
	redisClient   *redis.Client
	ctx           context.Context
	consumerGroup string
	consumerName  string
	httpClient    *http.Client
	clickhouseURL string
	batchSize     int
	batchTimeout  time.Duration
	isClickHouseUp bool
}

type ConversationData struct {
	SessionID  string `json:"session_id"`
	AgentID    string `json:"agent_id"`
	Timestamp  string `json:"timestamp"`
	User       string `json:"user"`
	Assistant  string `json:"assistant"`
	TurnCount  int    `json:"turn_count"`
}

func NewClickHouseConsumer() *ClickHouseConsumer {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &ClickHouseConsumer{
		redisClient:   rdb,
		ctx:          ctx,
		consumerGroup: "clickhouse-consumers",
		consumerName:  fmt.Sprintf("ch-consumer-%d", time.Now().UnixNano()),
		httpClient:    httpClient,
		clickhouseURL: "http://centerfire:@localhost:8123",
		batchSize:     100,          // Process 100 conversations at once
		batchTimeout:  30 * time.Second, // Max wait time before processing batch
		isClickHouseUp: false,
	}
}

func (cc *ClickHouseConsumer) createConsumerGroup() error {
	err := cc.redisClient.XGroupCreateMkStream(cc.ctx, "centerfire:semantic:conversations", cc.consumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %v", err)
	}
	return nil
}

func (cc *ClickHouseConsumer) checkClickHouse() bool {
	resp, err := cc.httpClient.Get(cc.clickhouseURL + "/?query=SELECT+1")
	if err != nil {
		log.Printf("ClickHouse not available: %v", err)
		cc.isClickHouseUp = false
		return false
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 200 {
		if !cc.isClickHouseUp {
			log.Println("🟢 ClickHouse is now available")
			cc.isClickHouseUp = true
		}
		return true
	}
	
	cc.isClickHouseUp = false
	return false
}

func (cc *ClickHouseConsumer) ensureClickHouseTable() error {
	if !cc.checkClickHouse() {
		return fmt.Errorf("ClickHouse not available")
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS conversations (
		session_id String,
		agent_id String,
		timestamp DateTime64(3),
		user_message String,
		assistant_message String,
		turn_count UInt32,
		created_at DateTime DEFAULT now()
	) ENGINE = MergeTree()
	ORDER BY (timestamp, session_id)
	PARTITION BY toYYYYMM(timestamp)
	`

	resp, err := cc.httpClient.Post(
		cc.clickhouseURL,
		"text/plain",
		strings.NewReader(createTableSQL),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to create table: status %d", resp.StatusCode)
	}

	log.Println("🗄️  ClickHouse conversations table ready")
	return nil
}

func (cc *ClickHouseConsumer) insertBatch(conversations []ConversationData) error {
	if !cc.checkClickHouse() {
		return fmt.Errorf("ClickHouse not available for batch insert")
	}

	if len(conversations) == 0 {
		return nil
	}

	// Build batch insert SQL
	var values []string
	for _, conv := range conversations {
		timestamp := conv.Timestamp
		if timestamp == "" {
			timestamp = time.Now().Format("2006-01-02 15:04:05.000")
		} else {
			// Convert ISO format to ClickHouse format
			if strings.Contains(timestamp, "T") {
				timestamp = strings.ReplaceAll(timestamp, "T", " ")
				timestamp = strings.ReplaceAll(timestamp, "Z", "")
				if len(timestamp) == 19 { // No milliseconds
					timestamp += ".000"
				}
			}
		}

		// Escape single quotes in strings
		userMsg := strings.ReplaceAll(conv.User, "'", "''")
		assistantMsg := strings.ReplaceAll(conv.Assistant, "'", "''")

		value := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', %d)",
			conv.SessionID, conv.AgentID, timestamp, userMsg, assistantMsg, conv.TurnCount)
		values = append(values, value)
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO conversations (session_id, agent_id, timestamp, user_message, assistant_message, turn_count)
		VALUES %s
	`, strings.Join(values, ","))

	log.Printf("🔍 Executing SQL: %s", insertSQL)

	resp, err := cc.httpClient.Post(
		cc.clickhouseURL,
		"text/plain",
		strings.NewReader(insertSQL),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to insert batch: status %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("🧊 ClickHouse: Stored %d conversations in cold storage", len(conversations))
	return nil
}

func (cc *ClickHouseConsumer) startConsuming() {
	log.Printf("🧊 Starting ClickHouse consumer: %s", cc.consumerName)
	log.Println("📦 Strategy: Ephemeral container-aware batch processing")

	batch := make([]ConversationData, 0, cc.batchSize)
	batchTimer := time.NewTimer(cc.batchTimeout)
	defer batchTimer.Stop()

	for {
		select {
		case <-batchTimer.C:
			// Timeout reached - process whatever we have
			if len(batch) > 0 {
				cc.processBatch(batch)
				batch = batch[:0] // Clear batch
			}
			batchTimer.Reset(cc.batchTimeout)

		default:
			// Try to read messages
			log.Printf("🔍 Reading from stream...")
			streams, err := cc.redisClient.XReadGroup(cc.ctx, &redis.XReadGroupArgs{
				Group:    cc.consumerGroup,
				Consumer: cc.consumerName,
				Streams:  []string{"centerfire:semantic:conversations", ">"},
				Count:    10,
				Block:    1 * time.Second,
			}).Result()

			if err != nil {
				if err != redis.Nil {
					log.Printf("❌ Error reading from stream: %v", err)
					time.Sleep(5 * time.Second)
				} else {
					log.Printf("⏰ No messages available (redis.Nil)")
				}
				continue
			}

			log.Printf("📥 Got %d streams", len(streams))

			// Process messages into batch
			for _, stream := range streams {
				log.Printf("📥 Stream has %d messages", len(stream.Messages))
				for _, message := range stream.Messages {
					log.Printf("🔍 Parsing message: %s", message.ID)
					if conv := cc.parseMessage(message); conv != nil {
						log.Printf("✅ Successfully parsed message for %s", conv.SessionID)
						batch = append(batch, *conv)

						// Acknowledge message immediately
						cc.redisClient.XAck(cc.ctx, "centerfire:semantic:conversations", cc.consumerGroup, message.ID)

						// Process batch if it's full
						if len(batch) >= cc.batchSize {
							cc.processBatch(batch)
							batch = batch[:0] // Clear batch
							batchTimer.Reset(cc.batchTimeout)
						}
					}
				}
			}
			
			// Prevent busy loop when no messages are available
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (cc *ClickHouseConsumer) parseMessage(message redis.XMessage) *ConversationData {
	dataStr, ok := message.Values["data"].(string)
	if !ok {
		log.Printf("Invalid message format: %v", message.Values)
		return nil
	}

	var convData ConversationData
	if err := json.Unmarshal([]byte(dataStr), &convData); err != nil {
		// Try to recover malformed JSON
		cleanedData := strings.ReplaceAll(dataStr, `\!`, `!`)
		cleanedData = strings.ReplaceAll(cleanedData, `\"`, `"`)

		if err := json.Unmarshal([]byte(cleanedData), &convData); err != nil {
			log.Printf("❌ Failed to parse conversation data: %v", err)
			return nil
		}
	}

	return &convData
}

func (cc *ClickHouseConsumer) processBatch(batch []ConversationData) {
	if len(batch) == 0 {
		return
	}

	log.Printf("📦 Processing batch of %d conversations", len(batch))

	// Check if ClickHouse is available
	if !cc.checkClickHouse() {
		log.Printf("⏸️  ClickHouse unavailable - requesting container startup")
		cc.requestClickHouseStartup()
		
		// Wait a bit and retry once
		time.Sleep(10 * time.Second)
		if !cc.checkClickHouse() {
			log.Printf("❌ ClickHouse still unavailable - will retry batch later")
			// TODO: Could implement retry queue here
			return
		}
	}

	// Ensure table exists
	if err := cc.ensureClickHouseTable(); err != nil {
		log.Printf("❌ Failed to ensure ClickHouse table: %v", err)
		return
	}

	// Insert batch
	if err := cc.insertBatch(batch); err != nil {
		log.Printf("❌ Failed to insert batch: %v", err)
		return
	}

	log.Printf("✅ Batch of %d conversations stored successfully", len(batch))
}

func (cc *ClickHouseConsumer) requestClickHouseStartup() {
	log.Println("🚀 Requesting AGT-STACK-1 to start ClickHouse container")
	
	// Send request to AGT-STACK-1 for analytics profile startup
	stackRequest := map[string]interface{}{
		"type":      "stack_request",
		"client_id": "clickhouse_consumer",
		"operation": "start_profile",
		"profile":   "analytics",
	}
	
	requestJSON, err := json.Marshal(stackRequest)
	if err != nil {
		log.Printf("❌ Failed to marshal stack request: %v", err)
		return
	}
	
	err = cc.redisClient.Publish(cc.ctx, "agent.stack.request", string(requestJSON)).Err()
	if err != nil {
		log.Printf("❌ Failed to publish stack request: %v", err)
		return
	}
	
	log.Println("📡 Stack request sent to AGT-STACK-1")
}

func main() {
	consumer := NewClickHouseConsumer()

	// Create consumer group
	if err := consumer.createConsumerGroup(); err != nil {
		log.Fatalf("Failed to create consumer group: %v", err)
	}

	log.Println("🧊 ClickHouse consumer ready - monitoring centerfire:semantic:conversations")
	log.Println("📦 Batch mode: Collect conversations and process when ClickHouse is available")

	// Start consuming (blocks)
	consumer.startConsuming()
}
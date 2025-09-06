package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	
	"github.com/go-redis/redis/v8"
)

type ConversationConsumer struct {
	redisClient   *redis.Client
	ctx           context.Context
	consumerGroup string
	consumerName  string
}

type ConversationData struct {
	SessionID string `json:"session_id"`
	ClientID  string `json:"client_id"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
	Type      string `json:"type"`
	Source    string `json:"source"`
}

func NewConversationConsumer() *ConversationConsumer {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	return &ConversationConsumer{
		redisClient:   rdb,
		ctx:          ctx,
		consumerGroup: "wn-conversation-consumers",
		consumerName:  fmt.Sprintf("consumer-%d", time.Now().UnixNano()),
	}
}

func (cc *ConversationConsumer) createConsumerGroup() error {
	// Create consumer group if it doesn't exist
	err := cc.redisClient.XGroupCreateMkStream(cc.ctx, "centerfire:conversations", cc.consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %v", err)
	}
	return nil
}

func (cc *ConversationConsumer) startConsuming() {
	log.Printf("🔥 Starting conversation consumer: %s", cc.consumerName)
	
	for {
		// Read from Redis stream
		streams, err := cc.redisClient.XReadGroup(cc.ctx, &redis.XReadGroupArgs{
			Group:    cc.consumerGroup,
			Consumer: cc.consumerName,
			Streams:  []string{"centerfire:conversations", ">"},
			Count:    10,
			Block:    5 * time.Second,
		}).Result()
		
		if err != nil {
			if err == redis.Nil {
				// No messages, continue
				continue
			}
			log.Printf("Error reading from stream: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		
		// Process messages
		for _, stream := range streams {
			for _, message := range stream.Messages {
				cc.processConversationMessage(message)
				
				// Acknowledge message
				cc.redisClient.XAck(cc.ctx, "centerfire:conversations", cc.consumerGroup, message.ID)
			}
		}
	}
}

func (cc *ConversationConsumer) processConversationMessage(message redis.XMessage) {
	dataStr, ok := message.Values["data"].(string)
	if !ok {
		log.Printf("Invalid message format: %v", message.Values)
		return
	}
	
	var convData ConversationData
	if err := json.Unmarshal([]byte(dataStr), &convData); err != nil {
		log.Printf("Error unmarshaling conversation data: %v", err)
		return
	}
	
	log.Printf("📝 Processing conversation from %s (%s): %d chars", 
		convData.SessionID, convData.Source, len(convData.Content))
	
	// Here we would normally send to Weaviate/Neo4j
	// For now, just log and simulate storage
	cc.simulateWeaviateStorage(convData)
	cc.simulateNeo4jStorage(convData)
}

func (cc *ConversationConsumer) simulateWeaviateStorage(data ConversationData) {
	// Simulate Weaviate vector storage
	log.Printf("🔍 [SIMULATED] Weaviate: Stored conversation vector for %s", data.SessionID)
	
	// In real implementation:
	// - Create embedding from content
	// - Store in Weaviate with metadata
	// - Associate with semantic namespace
}

func (cc *ConversationConsumer) simulateNeo4jStorage(data ConversationData) {
	// Simulate Neo4j relationship storage
	log.Printf("🔗 [SIMULATED] Neo4j: Created conversation node for %s → %s", data.ClientID, data.SessionID)
	
	// In real implementation:
	// - Create conversation node
	// - Link to session/client nodes
	// - Extract entities and create relationships
	// - Connect to semantic graph
}

func main() {
	consumer := NewConversationConsumer()
	
	// Create consumer group
	if err := consumer.createConsumerGroup(); err != nil {
		log.Fatalf("Failed to create consumer group: %v", err)
	}
	
	log.Println("🎯 Conversation consumer ready - monitoring centerfire:conversations stream")
	
	// Start consuming (blocks)
	consumer.startConsuming()
}
package main

import (
	"bytes"
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

type ConversationConsumer struct {
	redisClient   *redis.Client
	ctx           context.Context
	consumerGroup string
	consumerName  string
	httpClient    *http.Client
	weaviateURL   string
	schemaCreated bool
}

type ConversationData struct {
	SessionID  string `json:"session_id"`
	AgentID    string `json:"agent_id"`
	Timestamp  string `json:"timestamp"`
	User       string `json:"user"`
	Assistant  string `json:"assistant"`
	TurnCount  int    `json:"turn_count"`
}

func NewConversationConsumer() *ConversationConsumer {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	// Create HTTP client for Weaviate
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	return &ConversationConsumer{
		redisClient:   rdb,
		ctx:          ctx,
		consumerGroup: "wn-conversation-consumers",
		consumerName:  fmt.Sprintf("consumer-%d", time.Now().UnixNano()),
		httpClient:    httpClient,
		weaviateURL:   "http://localhost:8080",
		schemaCreated: false,
	}
}

func (cc *ConversationConsumer) createConsumerGroup() error {
	// Create consumer group if it doesn't exist
	err := cc.redisClient.XGroupCreateMkStream(cc.ctx, "centerfire:semantic:conversations", cc.consumerGroup, "0").Err()
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
			Streams:  []string{"centerfire:semantic:conversations", ">"}, 
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
				cc.redisClient.XAck(cc.ctx, "centerfire:semantic:conversations", cc.consumerGroup, message.ID)
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
		log.Printf("❌ Error unmarshaling conversation data: %v", err)
		log.Printf("📄 Malformed data: %s", dataStr)
		
		// Try to recover by cleaning the JSON
		cleanedData := strings.ReplaceAll(dataStr, `\!`, `!`)
		cleanedData = strings.ReplaceAll(cleanedData, `\"`, `"`)
		
		if err := json.Unmarshal([]byte(cleanedData), &convData); err != nil {
			log.Printf("❌ Recovery failed, skipping message: %v", err)
			return
		}
		log.Printf("✅ Successfully recovered malformed JSON")
	}
	
	log.Printf("📝 Processing conversation from %s (agent:%s): user=%d chars, assistant=%d chars", 
		convData.SessionID, convData.AgentID, len(convData.User), len(convData.Assistant))
	
	// Store in Weaviate and Neo4j
	cc.storeInWeaviate(convData)
	cc.createNeo4jRelationships(convData)
}

func (cc *ConversationConsumer) storeInWeaviate(data ConversationData) {
	// Ensure schema exists
	if !cc.schemaCreated {
		if err := cc.createConversationSchema(); err != nil {
			log.Printf("❌ Failed to create Weaviate schema: %v", err)
			return
		}
		cc.schemaCreated = true
	}
	
	// Create conversation object in Weaviate
	convObject := map[string]interface{}{
		"class": "ConversationHistory",
		"properties": map[string]interface{}{
			"content":    data.User + " | " + data.Assistant,
			"session_id": data.SessionID,
			"timestamp":  data.Timestamp,
			"agent_id":   data.AgentID,
			"user":       data.User,
			"assistant":  data.Assistant,
			"turn_count": data.TurnCount,
		},
	}
	
	if err := cc.createWeaviateObject(convObject); err != nil {
		log.Printf("❌ Failed to store conversation in Weaviate: %v", err)
		return
	}
	
	log.Printf("🔍 Weaviate: Stored conversation for %s (turn %d)", data.SessionID, data.TurnCount)
}

func (cc *ConversationConsumer) createNeo4jRelationships(data ConversationData) {
	// Use Neo4j HTTP API (v5.x compatible)
	cypher := `
	MERGE (session:Session {id: $session_id})
	MERGE (agent:Agent {id: $agent_id})
	CREATE (conversation:Conversation {
		id: $conv_id,
		timestamp: $timestamp,
		turn_count: $turn_count,
		user_message: $user_message,
		assistant_message: $assistant_message
	})
	MERGE (session)-[:HAS_CONVERSATION]->(conversation)
	MERGE (agent)-[:PARTICIPATED_IN]->(conversation)
	MERGE (agent)-[:INTERACTED_WITH]->(session)
	`
	
	payload := map[string]interface{}{
		"statements": []map[string]interface{}{
			{
				"statement": cypher,
				"parameters": map[string]interface{}{
					"session_id":         data.SessionID,
					"agent_id":           data.AgentID, 
					"conv_id":           fmt.Sprintf("%s-%d", data.SessionID, data.TurnCount),
					"timestamp":         data.Timestamp,
					"turn_count":        data.TurnCount,
					"user_message":      data.User,
					"assistant_message": data.Assistant,
				},
			},
		},
	}

	jsonPayload, _ := json.Marshal(payload)
	
	// Create HTTP request with authentication
	req, err := http.NewRequest("POST", "http://localhost:7474/db/neo4j/tx/commit", strings.NewReader(string(jsonPayload)))
	if err != nil {
		log.Printf("🔗 ❌ Neo4j: Request creation failed for %s → %s: %v", data.AgentID, data.SessionID, err)
		return
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic bmVvNGo6bXlfc2VjdXJlX3Bhc3N3b3JkMTIz") // neo4j:my_secure_password123 in base64
	
	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	
	if err != nil {
		log.Printf("🔗 ❌ Neo4j: Connection failed for %s → %s: %v", data.AgentID, data.SessionID, err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("🔗 ❌ Neo4j: Failed to create relationships for %s → %s (status %d): %s", 
			data.AgentID, data.SessionID, resp.StatusCode, string(body))
	} else {
		log.Printf("🔗 ✅ Neo4j: Created conversation relationships for %s → %s", data.AgentID, data.SessionID)
	}
}

// createConversationSchema creates the ConversationHistory schema in Weaviate
func (cc *ConversationConsumer) createConversationSchema() error {
	schema := map[string]interface{}{
		"class": "ConversationHistory",
		"description": "AI Agent conversation history with semantic search capabilities",
		"properties": []map[string]interface{}{
			{
				"name": "content",
				"dataType": []string{"text"},
				"description": "Full conversation content",
			},
			{
				"name": "session_id",
				"dataType": []string{"string"},
				"description": "Session identifier",
			},
			{
				"name": "timestamp",
				"dataType": []string{"string"},
				"description": "ISO timestamp",
			},
			{
				"name": "agent_id",
				"dataType": []string{"string"},
				"description": "Agent identifier",
			},
			{
				"name": "user",
				"dataType": []string{"text"},
				"description": "User message",
			},
			{
				"name": "assistant",
				"dataType": []string{"text"},
				"description": "Assistant response",
			},
			{
				"name": "turn_count",
				"dataType": []string{"int"},
				"description": "Conversation turn number",
			},
		},
		"vectorizer": "text2vec-transformers",
	}
	
	jsonData, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	
	resp, err := cc.httpClient.Post(
		cc.weaviateURL+"/v1/schema",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 && resp.StatusCode != 422 { // 422 = already exists
		return fmt.Errorf("failed to create schema: status %d", resp.StatusCode)
	}
	
	log.Println("🎆 ConversationHistory schema created in Weaviate")
	return nil
}

// createWeaviateObject stores a conversation object in Weaviate
func (cc *ConversationConsumer) createWeaviateObject(obj map[string]interface{}) error {
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	
	resp, err := cc.httpClient.Post(
		cc.weaviateURL+"/v1/objects",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to create object: status %d", resp.StatusCode)
	}
	
	return nil
}

func main() {
	consumer := NewConversationConsumer()
	
	// Create consumer group
	if err := consumer.createConsumerGroup(); err != nil {
		log.Fatalf("Failed to create consumer group: %v", err)
	}
	
	log.Println("🎯 Conversation consumer ready - monitoring centerfire:semantic:conversations stream")
	
	// Start consuming (blocks)
	consumer.startConsuming()
}
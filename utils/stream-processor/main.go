package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

type StreamProcessor struct {
	redisClient *redis.Client
	ctx         context.Context
}

type SemanticNameEvent struct {
	Slug      string `json:"slug"`
	CID       string `json:"cid"`
	Directory string `json:"directory"`
	Domain    string `json:"domain"`
	Purpose   string `json:"purpose"`
	Sequence  int64  `json:"sequence"`
	Allocated string `json:"allocated"`
	EventType string `json:"event_type"`
}

type SemanticConceptEvent struct {
	EventType   string `json:"event_type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Domain      string `json:"domain"`
	CID         string `json:"cid"`
	Metadata    string `json:"metadata"`
	Project     string `json:"project"`
	Environment string `json:"environment"`
	ClassName   string `json:"className"`
	Namespace   string `json:"namespace"`
}

func NewStreamProcessor() *StreamProcessor {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	return &StreamProcessor{
		redisClient: rdb,
		ctx:         context.Background(),
	}
}

func (sp *StreamProcessor) Start() {
	fmt.Println("Starting Stream Processor for W/N consumers...")

	// Test Redis connection
	_, err := sp.redisClient.Ping(sp.ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis successfully")

	// Create consumer group for semantic names if it doesn't exist
	streamName := "centerfire:semantic:names"
	consumerGroup := "wn-consumers"
	
	// Try to create consumer group (ignore error if it already exists)
	sp.redisClient.XGroupCreate(sp.ctx, streamName, consumerGroup, "0").Err()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming from multiple streams
	go sp.consumeSemanticNames(streamName, consumerGroup)
	go sp.consumeSemanticConcepts("centerfire:semantic:concepts", consumerGroup)

	fmt.Println("Stream Processor ready - listening for semantic name events")
	<-sigChan
	fmt.Println("\nStream Processor shutting down...")
}

func (sp *StreamProcessor) consumeSemanticNames(streamName, consumerGroup string) {
	consumerName := "wn-consumer-1"

	for {
		// Read from stream with consumer group
		streams, err := sp.redisClient.XReadGroup(sp.ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{streamName, ">"},
			Count:    10,
			Block:    time.Second * 5,
		}).Result()

		if err != nil {
			if err != redis.Nil {
				log.Printf("Error reading from stream: %v", err)
			}
			continue
		}

		// Process each stream
		for _, stream := range streams {
			for _, message := range stream.Messages {
				sp.processSemanticNameEvent(message)
				
				// Acknowledge message
				sp.redisClient.XAck(sp.ctx, streamName, consumerGroup, message.ID)
			}
		}
	}
}

func (sp *StreamProcessor) processSemanticNameEvent(message redis.XMessage) {
	fmt.Printf("Processing semantic name event: %s\n", message.ID)
	
	// Extract event data
	var event SemanticNameEvent
	if data, ok := message.Values["data"].(string); ok {
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			log.Printf("Error unmarshaling event data: %v", err)
			return
		}
	}

	fmt.Printf("Event: %+v\n", event)

	// Process to Weaviate
	sp.sendToWeaviate(event)
	
	// Process to Neo4j
	sp.sendToNeo4j(event)
}

func (sp *StreamProcessor) sendToWeaviate(event SemanticNameEvent) {
	fmt.Printf("Sending to Weaviate: %s (CID: %s)\n", event.Slug, event.CID)
	
	// TODO: Implement actual Weaviate client
	// For now, just log the operation
	fmt.Printf("Weaviate: Created semantic object for %s\n", event.Slug)
}

func (sp *StreamProcessor) sendToNeo4j(event SemanticNameEvent) {
	fmt.Printf("Sending to Neo4j: %s (Domain: %s)\n", event.Slug, event.Domain)
	
	// TODO: Implement actual Neo4j client  
	// For now, just log the operation
	fmt.Printf("Neo4j: Created semantic node for %s\n", event.Slug)
}

func (sp *StreamProcessor) consumeSemanticConcepts(streamName, consumerGroup string) {
	consumerName := "concept-consumer-1"

	for {
		// Read from stream with consumer group
		streams, err := sp.redisClient.XReadGroup(sp.ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{streamName, ">"},
			Count:    10,
			Block:    time.Second * 5,
		}).Result()

		if err != nil {
			if err != redis.Nil {
				log.Printf("Error reading from concept stream: %v", err)
			}
			continue
		}

		// Process each stream
		for _, stream := range streams {
			for _, message := range stream.Messages {
				sp.processSemanticConceptEvent(message)
				
				// Acknowledge message
				sp.redisClient.XAck(sp.ctx, streamName, consumerGroup, message.ID)
			}
		}
	}
}

func (sp *StreamProcessor) processSemanticConceptEvent(message redis.XMessage) {
	fmt.Printf("Processing semantic concept event: %s\n", message.ID)
	
	// Extract event data
	var event SemanticConceptEvent
	if data, ok := message.Values["data"].(string); ok {
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			log.Printf("Error unmarshaling concept event data: %v", err)
			return
		}
	}

	fmt.Printf("Concept Event: %+v\n", event)

	// Process to Weaviate
	sp.sendConceptToWeaviate(event)
	
	// Process to Neo4j
	sp.sendConceptToNeo4j(event)
}

func (sp *StreamProcessor) sendConceptToWeaviate(event SemanticConceptEvent) {
	fmt.Printf("Sending concept to Weaviate: %s in class %s (CID: %s)\n", event.Name, event.ClassName, event.CID)
	
	// TODO: Implement actual Weaviate client for concept storage
	// For now, just log the operation
	fmt.Printf("Weaviate: Created concept object for %s in namespace %s\n", event.Name, event.Namespace)
}

func (sp *StreamProcessor) sendConceptToNeo4j(event SemanticConceptEvent) {
	fmt.Printf("Sending concept to Neo4j: %s (Domain: %s)\n", event.Name, event.Domain)
	
	// TODO: Implement actual Neo4j client for concept storage
	// For now, just log the operation
	fmt.Printf("Neo4j: Created concept node for %s in namespace %s\n", event.Name, event.Namespace)
}

// PublishSemanticNameEvent - Utility to publish semantic name events to the stream
func (sp *StreamProcessor) PublishSemanticNameEvent(event SemanticNameEvent) error {
	streamName := "centerfire:semantic:names"
	
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error marshaling event: %v", err)
	}
	
	_, err = sp.redisClient.XAdd(sp.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data":      string(eventData),
			"timestamp": time.Now().Unix(),
			"source":    "naming-agent",
		},
	}).Result()
	
	if err != nil {
		return fmt.Errorf("error publishing to stream: %v", err)
	}
	
	fmt.Printf("Published semantic name event to stream: %s\n", event.Slug)
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "publish-test" {
		// Test publishing an event
		sp := NewStreamProcessor()
		event := SemanticNameEvent{
			Slug:      "CAP-TEST-2",
			CID:       "cid:centerfire:capability:test123",
			Directory: "CAP-TEST-2__test123",
			Domain:    "TEST",
			Purpose:   "Test stream processing",
			Sequence:  2,
			Allocated: time.Now().Format(time.RFC3339),
			EventType: "capability_allocated",
		}
		
		if err := sp.PublishSemanticNameEvent(event); err != nil {
			log.Fatalf("Error publishing test event: %v", err)
		}
		fmt.Println("Test event published successfully")
		return
	}

	processor := NewStreamProcessor()
	processor.Start()
}
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type BackfillUtility struct {
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

func NewBackfillUtility() *BackfillUtility {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	return &BackfillUtility{
		redisClient: rdb,
		ctx:         context.Background(),
	}
}

func (b *BackfillUtility) BackfillSemanticNames() error {
	fmt.Println("Starting backfill of semantic names from Redis to streams...")

	// Test Redis connection
	_, err := b.redisClient.Ping(b.ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis successfully")

	// Find all semantic name keys
	keys, err := b.redisClient.Keys(b.ctx, "centerfire.dev.names:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get semantic name keys: %v", err)
	}

	fmt.Printf("Found %d semantic name entries to backfill\n", len(keys))

	backfilledCount := 0
	for _, key := range keys {
		// Get the stored semantic name data
		data, err := b.redisClient.Get(b.ctx, key).Result()
		if err != nil {
			log.Printf("Error getting data for key %s: %v", key, err)
			continue
		}

		// Parse the stored JSON
		var nameData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &nameData); err != nil {
			log.Printf("Error parsing JSON for key %s: %v", key, err)
			continue
		}

		// Create stream event from stored data
		event := SemanticNameEvent{
			Slug:      getStringValue(nameData, "slug"),
			CID:       getStringValue(nameData, "cid"),
			Directory: getStringValue(nameData, "directory"),
			Domain:    getStringValue(nameData, "domain"),
			Purpose:   getStringValue(nameData, "purpose"),
			Sequence:  getIntValue(nameData, "sequence"),
			Allocated: getStringValue(nameData, "allocated"),
			EventType: "backfill_capability_allocated",
		}

		// Publish to stream
		if err := b.publishSemanticNameEvent(event); err != nil {
			log.Printf("Error publishing backfill event for %s: %v", event.Slug, err)
			continue
		}

		backfilledCount++
		fmt.Printf("Backfilled: %s (CID: %s)\n", event.Slug, event.CID)
	}

	fmt.Printf("\nBackfill complete: %d/%d semantic names processed\n", backfilledCount, len(keys))
	return nil
}

func (b *BackfillUtility) publishSemanticNameEvent(event SemanticNameEvent) error {
	streamName := "centerfire:semantic:names"

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error marshaling event: %v", err)
	}

	_, err = b.redisClient.XAdd(b.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data":      string(eventData),
			"timestamp": time.Now().Unix(),
			"source":    "backfill-utility",
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("error publishing to stream: %v", err)
	}

	return nil
}

// Helper functions to safely extract values from map[string]interface{}
func getStringValue(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

func getIntValue(data map[string]interface{}, key string) int64 {
	if val, ok := data[key].(float64); ok {
		return int64(val)
	}
	if val, ok := data[key].(int64); ok {
		return val
	}
	if val, ok := data[key].(int); ok {
		return int64(val)
	}
	return 0
}

func main() {
	utility := NewBackfillUtility()
	if err := utility.BackfillSemanticNames(); err != nil {
		log.Fatalf("Backfill failed: %v", err)
	}
}
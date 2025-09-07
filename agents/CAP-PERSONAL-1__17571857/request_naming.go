package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

func requestSemanticNameAllocation() error {
	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	defer rdb.Close()

	ctx := context.Background()

	// Test Redis connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	// Create request for AGT-NAMING-1
	request := map[string]interface{}{
		"from":   "CAP-PERSONAL-1",
		"action": "allocate_capability",
		"params": map[string]interface{}{
			"domain":      "CONTEXT",
			"description": "Fast Weaviate GraphQL context retrieval agent for conversation history and semantic search",
			"type":        "Agent",
			"project":     "centerfire",
			"environment": "dev",
		},
		"request_id": fmt.Sprintf("context_agent_request_%d", time.Now().UnixNano()),
	}

	// Subscribe to response channel before sending request
	pubsub := rdb.Subscribe(ctx, "agent.naming.response")
	defer pubsub.Close()

	// Send request to AGT-NAMING-1
	requestData, _ := json.Marshal(request)
	err = rdb.Publish(ctx, "agent.naming.request", requestData).Err()
	if err != nil {
		return fmt.Errorf("error sending request to AGT-NAMING-1: %v", err)
	}

	fmt.Println("📤 Sent allocation request to AGT-NAMING-1")
	fmt.Printf("   Domain: CONTEXT\n")
	fmt.Printf("   Description: Fast Weaviate GraphQL context retrieval agent\n")
	fmt.Printf("   Type: Agent (persistent)\n")
	fmt.Printf("   Project: centerfire\n")
	fmt.Printf("   Environment: dev\n\n")

	// Wait for response with timeout
	ch := pubsub.Channel()
	timeout := time.After(10 * time.Second)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for response from AGT-NAMING-1")
		case msg := <-ch:
			var response map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &response); err != nil {
				fmt.Printf("Error parsing response: %v\n", err)
				continue
			}

			// Check if this response is for our request
			if reqID, ok := response["request_id"].(string); ok && reqID == request["request_id"] {
				if errorMsg, hasError := response["error"].(string); hasError {
					return fmt.Errorf("AGT-NAMING-1 error: %s", errorMsg)
				}

				// Extract the allocated semantic name
				if semanticName, ok := response["semantic_name"].(string); ok {
					fmt.Println("✅ SUCCESS: Semantic name allocated by AGT-NAMING-1")
					fmt.Printf("🏷️  Allocated Name: %s\n", semanticName)
					
					if cid, ok := response["cid"].(string); ok {
						fmt.Printf("🔑 CID: %s\n", cid)
					}
					if slug, ok := response["slug"].(string); ok {
						fmt.Printf("📂 Slug: %s\n", slug)
					}
					
					fmt.Printf("\n🚀 Next steps:\n")
					fmt.Printf("   1. Create agent directory: agents/%s/\n", semanticName)
					fmt.Printf("   2. Implement Weaviate GraphQL context retrieval\n")
					fmt.Printf("   3. Add conversation history search capabilities\n")
					fmt.Printf("   4. Register with AGT-MANAGER-1\n")
					
					return nil
				}
			}
		}
	}
}

func main() {
	fmt.Println("🤖 Requesting semantic name allocation from AGT-NAMING-1...")
	fmt.Println("═══════════════════════════════════════════════════════════")
	
	if err := requestSemanticNameAllocation(); err != nil {
		log.Fatalf("❌ Request failed: %v", err)
	}
}
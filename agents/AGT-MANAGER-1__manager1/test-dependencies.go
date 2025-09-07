package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"context"
)

// test-dependencies.go - Test service dependency tracking features
func main() {
	fmt.Println("🔬 Testing AGT-MANAGER-1 Service Dependency Tracking")
	
	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	ctx := context.Background()

	// Test 1: Check dependencies for AGT-SEMANTIC-1
	fmt.Println("\n📋 Test 1: Checking dependencies for AGT-SEMANTIC-1")
	request1 := AgentRequest{
		RequestType: "check_dependencies",
		AgentName:   "AGT-SEMANTIC-1",
	}
	
	requestJSON, _ := json.Marshal(request1)
	if err := rdb.Publish(ctx, "centerfire:agent:manager", string(requestJSON)).Err(); err != nil {
		log.Printf("Failed to publish request: %v", err)
		return
	}
	
	time.Sleep(time.Second * 2)
	
	// Test 2: Validate Redis service health
	fmt.Println("\n🏥 Test 2: Validating Redis service health")
	request2 := AgentRequest{
		RequestType: "validate_service_health",
		AgentName:   "redis", // using agent_name field for service name
	}
	
	requestJSON, _ = json.Marshal(request2)
	if err := rdb.Publish(ctx, "centerfire:agent:manager", string(requestJSON)).Err(); err != nil {
		log.Printf("Failed to publish request: %v", err)
		return
	}
	
	time.Sleep(time.Second * 2)

	// Test 3: Attempt dependency-aware restart of AGT-STACK-1 
	fmt.Println("\n🔄 Test 3: Dependency-aware restart of AGT-STACK-1")
	request3 := AgentRequest{
		RequestType: "restart_with_dependencies",
		AgentName:   "AGT-STACK-1",
		DependencyCheck: true,
		ForceRestart:    false,
	}
	
	requestJSON, _ = json.Marshal(request3)
	if err := rdb.Publish(ctx, "centerfire:agent:manager", string(requestJSON)).Err(); err != nil {
		log.Printf("Failed to publish request: %v", err)
		return
	}
	
	time.Sleep(time.Second * 3)
	
	fmt.Println("\n✅ Dependency tracking tests completed")
	fmt.Println("Check AGT-MANAGER-1 logs for detailed dependency validation results")
	
	rdb.Close()
}
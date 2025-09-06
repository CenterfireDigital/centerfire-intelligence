package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

// CleanupAgent - Agent for cleaning pre-semantic data from W/N/R
type CleanupAgent struct {
	AgentID         string
	CID            string
	RequestChannel  string
	ResponseChannel string
	RedisClient    *redis.Client
	WeaviateURL    string
	Neo4jURL       string
	ctx            context.Context
}

// NewAgent - Create new CLEANUP agent
func NewAgent() *CleanupAgent {
	// Connect to Redis container on port 6380
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	return &CleanupAgent{
		AgentID:         "AGT-CLEANUP-1",
		CID:            "cid:centerfire:capability:17571335",
		RequestChannel:  "agent.cleanup.request",
		ResponseChannel: "agent.cleanup.response",
		RedisClient:    rdb,
		WeaviateURL:    "http://localhost:8080",
		Neo4jURL:       "bolt://localhost:7687",
		ctx:            context.Background(),
	}
}

// Start - Start listening for requests
func (a *CleanupAgent) Start() {
	fmt.Printf("%s starting...\n", a.AgentID)
	fmt.Printf("Listening on: %s\n", a.RequestChannel)
	
	// Test Redis connection
	_, err := a.RedisClient.Ping(a.ctx).Result()
	if err != nil {
		fmt.Printf("Failed to connect to Redis: %v\n", err)
		return
	}
	fmt.Printf("Connected to Redis successfully\n")
	
	// Subscribe to request channel
	pubsub := a.RedisClient.Subscribe(a.ctx, a.RequestChannel)
	defer pubsub.Close()
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Listen for messages
	ch := pubsub.Channel()
	
	fmt.Printf("%s ready - listening for cleanup requests\n", a.AgentID)
	
	for {
		select {
		case <-sigChan:
			fmt.Printf("\n%s shutting down...\n", a.AgentID)
			return
		case msg := <-ch:
			a.processMessage(msg.Payload)
		}
	}
}

// processMessage - Process incoming Redis message
func (a *CleanupAgent) processMessage(payload string) {
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		fmt.Printf("Error parsing request: %v\n", err)
		return
	}
	
	fmt.Printf("%s received request: %s\n", a.AgentID, request["action"])
	
	// Handle the request
	response := a.HandleRequest(request)
	
	// Send response back
	responseData, _ := json.Marshal(response)
	a.RedisClient.Publish(a.ctx, a.ResponseChannel, responseData)
}

// HandleRequest - Handle incoming request
func (a *CleanupAgent) HandleRequest(request map[string]interface{}) map[string]interface{} {
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
		}
	}
	
	// Route to appropriate handler
	switch action {
	case "cleanup_weaviate_classes":
		return a.handleCleanupWeaviateClasses(request)
	case "cleanup_neo4j_nodes":
		return a.handleCleanupNeo4jNodes(request)
	case "cleanup_redis_keys":
		return a.handleCleanupRedisKeys(request)
	case "cleanup_pre_semantic":
		return a.handleCleanupPreSemantic(request)
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown action: %s", action),
		}
	}
}

// handleCleanupPreSemantic - Main handler for cleaning pre-semantic data
func (a *CleanupAgent) handleCleanupPreSemantic(request map[string]interface{}) map[string]interface{} {
	fmt.Printf("%s: Starting pre-semantic data cleanup\n", a.AgentID)
	
	results := map[string]interface{}{
		"cleanup_started": time.Now().Format(time.RFC3339),
		"weaviate_results": nil,
		"neo4j_results": nil,
		"redis_results": nil,
	}
	
	// Clean Weaviate pre-semantic classes
	weaviateResult := a.cleanupWeaviatePreSemanticClasses()
	results["weaviate_results"] = weaviateResult
	
	// Note: Neo4j and Redis cleanup would be implemented here if needed
	fmt.Printf("%s: Pre-semantic cleanup completed\n", a.AgentID)
	
	return results
}

// handleCleanupWeaviateClasses - Clean specific Weaviate classes
func (a *CleanupAgent) handleCleanupWeaviateClasses(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	classes, ok := params["classes"].([]interface{})
	if !ok {
		return map[string]interface{}{"error": "No classes specified"}
	}
	
	var classNames []string
	for _, class := range classes {
		if className, ok := class.(string); ok {
			classNames = append(classNames, className)
		}
	}
	
	return a.deleteWeaviateClasses(classNames)
}

// cleanupWeaviatePreSemanticClasses - Remove known pre-semantic classes
func (a *CleanupAgent) cleanupWeaviatePreSemanticClasses() map[string]interface{} {
	preSemanticClasses := []string{
		"Centerfire_Dev_Concept",
		"Centerfire_Test_Concept", 
		"Centerfire_Prod_Concept",
	}
	
	fmt.Printf("%s: Cleaning up pre-semantic Weaviate classes: %v\n", a.AgentID, preSemanticClasses)
	
	return a.deleteWeaviateClasses(preSemanticClasses)
}

// deleteWeaviateClasses - Delete classes from Weaviate
func (a *CleanupAgent) deleteWeaviateClasses(classNames []string) map[string]interface{} {
	results := map[string]interface{}{
		"deleted_classes": []string{},
		"failed_classes": []string{},
		"errors": []string{},
	}
	
	var deletedClasses []string
	var failedClasses []string
	var errors []string
	
	for _, className := range classNames {
		url := fmt.Sprintf("%s/v1/schema/%s", a.WeaviateURL, className)
		
		req, err := http.NewRequestWithContext(a.ctx, "DELETE", url, nil)
		if err != nil {
			errorMsg := fmt.Sprintf("Error creating request for %s: %v", className, err)
			errors = append(errors, errorMsg)
			failedClasses = append(failedClasses, className)
			continue
		}
		
		req.Header.Set("Content-Type", "application/json")
		
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			errorMsg := fmt.Sprintf("Error deleting %s: %v", className, err)
			errors = append(errors, errorMsg)
			failedClasses = append(failedClasses, className)
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == 200 {
			deletedClasses = append(deletedClasses, className)
			fmt.Printf("%s: Successfully deleted Weaviate class: %s\n", a.AgentID, className)
		} else {
			body, _ := io.ReadAll(resp.Body)
			errorMsg := fmt.Sprintf("Failed to delete %s: %s (status: %d)", className, string(body), resp.StatusCode)
			errors = append(errors, errorMsg)
			failedClasses = append(failedClasses, className)
		}
	}
	
	results["deleted_classes"] = deletedClasses
	results["failed_classes"] = failedClasses
	results["errors"] = errors
	
	fmt.Printf("%s: Weaviate cleanup complete. Deleted: %d, Failed: %d\n", 
		a.AgentID, len(deletedClasses), len(failedClasses))
	
	return results
}

// handleCleanupNeo4jNodes - Placeholder for Neo4j cleanup
func (a *CleanupAgent) handleCleanupNeo4jNodes(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"status": "not_implemented",
		"message": "Neo4j cleanup not implemented yet",
	}
}

// handleCleanupRedisKeys - Placeholder for Redis cleanup
func (a *CleanupAgent) handleCleanupRedisKeys(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"status": "not_implemented", 
		"message": "Redis cleanup not implemented yet",
	}
}

// DirectCleanup - Direct cleanup method for immediate use
func (a *CleanupAgent) DirectCleanup() error {
	fmt.Printf("%s: Starting direct pre-semantic cleanup\n", a.AgentID)
	
	result := a.cleanupWeaviatePreSemanticClasses()
	
	if errors, ok := result["errors"].([]string); ok && len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	
	return nil
}

func main() {
	// Check for direct cleanup mode
	if len(os.Args) > 1 && os.Args[1] == "cleanup" {
		agent := NewAgent()
		if err := agent.DirectCleanup(); err != nil {
			log.Fatalf("Direct cleanup failed: %v", err)
		}
		return
	}
	
	agent := NewAgent()
	agent.Start()
}
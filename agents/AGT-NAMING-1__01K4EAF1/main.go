package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"github.com/redis/go-redis/v9"
)

// NamingAgent - Generated agent for NAMING domain
type NamingAgent struct {
	AgentID         string
	CID            string
	RequestChannel  string
	ResponseChannel string
	RedisClient    *redis.Client
	ctx            context.Context
}

// NewAgent - Create new NAMING agent
func NewAgent() *NamingAgent {
	// Connect to Redis container on port 6380
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	return &NamingAgent{
		AgentID:         "AGT-NAMING-1",
		CID:            "cid:centerfire:agent:01K4EAF14SC75RSJG1G9WV2APV",
		RequestChannel:  "agent.naming.request",
		ResponseChannel: "agent.naming.response",
		RedisClient:    rdb,
		ctx:            context.Background(),
	}
}

// Start - Start listening for requests
func (a *NamingAgent) Start() {
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
	
	fmt.Printf("%s ready - listening for requests\n", a.AgentID)
	
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
func (a *NamingAgent) processMessage(payload string) {
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
func (a *NamingAgent) HandleRequest(request map[string]interface{}) map[string]interface{} {
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
		}
	}
	
	// Route to appropriate handler
	switch action {
	case "allocate_capability":
		return a.handleAllocateCapability(request)
	case "allocate_module":
		return a.handleAllocateModule(request)
	case "allocate_function":
		return a.handleAllocateFunction(request)
	case "allocate_session":
		return a.handleAllocateSession(request)
	case "validate_name":
		return a.handleValidateName(request)
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown action: %s", action),
		}
	}
}

// handleAllocateCapability - Allocate new capability name and delegate structure creation
func (a *NamingAgent) handleAllocateCapability(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	domain, ok := params["domain"].(string)
	if !ok {
		return map[string]interface{}{"error": "Domain required"}
	}
	
	purpose, ok := params["purpose"].(string)
	if !ok {
		purpose = "Generated capability"
	}
	
	// Generate name (this is what naming agent does)
	capability := a.generateCapabilityName(domain, purpose)
	
	// Delegate structure creation to AGT-STRUCT-1
	structRequest := map[string]interface{}{
		"from":   a.AgentID,
		"action": "create_structure",
		"params": map[string]interface{}{
			"name":     capability["slug"],
			"type":     "capability", 
			"template": "default_capability",
			"cid":      capability["cid"],
		},
	}
	
	fmt.Printf("%s: Allocated %s, delegating structure creation to AGT-STRUCT-1\n", 
		a.AgentID, capability["slug"])
	
	// Send delegation request via Redis
	requestData, _ := json.Marshal(structRequest)
	err := a.RedisClient.Publish(a.ctx, "agent.struct.request", requestData).Err()
	if err != nil {
		fmt.Printf("%s: Error delegating to AGT-STRUCT-1: %v\n", a.AgentID, err)
	} else {
		fmt.Printf("%s: Delegated to AGT-STRUCT-1 via Redis\n", a.AgentID)
	}
	
	return capability
}

// generateCapabilityName - Generate new capability name with sequence
func (a *NamingAgent) generateCapabilityName(domain, purpose string) map[string]interface{} {
	// In real implementation, would check Redis for sequence
	// For now, simulate sequence increment
	sequence := 1 // TODO: Get from Redis
	
	// Generate ULID for uniqueness
	now := time.Now()
	ulid := fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
	
	slug := fmt.Sprintf("CAP-%s-%d", domain, sequence)
	cid := fmt.Sprintf("cid:centerfire:capability:%s", ulid)
	directory := fmt.Sprintf("%s__%s", slug, ulid)
	
	return map[string]interface{}{
		"slug":      slug,
		"cid":       cid, 
		"directory": directory,
		"domain":    domain,
		"purpose":   purpose,
		"sequence":  sequence,
	}
}

// handleAllocateSession - Allocate new session ID for session management
func (a *NamingAgent) handleAllocateSession(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}

	sessionType, ok := params["type"].(string)
	if !ok {
		sessionType = "claude_coding"
	}

	context, ok := params["context"].(string)
	if !ok {
		context = "session"
	}

	requestID, _ := params["request_id"].(string)

	// Generate session ID (this is what naming agent does)
	sessionID := a.generateSessionID(sessionType, context)

	response := map[string]interface{}{
		"session_id": sessionID,
		"type":       sessionType,
		"context":    context,
	}

	// Include request_id in response if provided
	if requestID != "" {
		response["request_id"] = requestID
	}

	fmt.Printf("%s: Allocated session ID: %s for type: %s\n", 
		a.AgentID, sessionID, sessionType)

	return response
}

// generateSessionID - Generate new session ID with sequence
func (a *NamingAgent) generateSessionID(sessionType, context string) string {
	// In real implementation, would check Redis for sequence
	// For now, simulate sequence increment
	sequence := 1 // TODO: Get from Redis

	// Generate ULID for uniqueness
	now := time.Now()
	ulid := fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]

	// Map session types to prefixes
	prefix := "SES-CLAUDE"
	switch sessionType {
	case "claude_coding":
		prefix = "SES-CLAUDE"
	case "agent_session":
		prefix = "SES-AGENT"
	default:
		prefix = "SES-GENERIC"
	}

	return fmt.Sprintf("%s-%d-%s", prefix, sequence, ulid)
}

// Placeholder handlers
func (a *NamingAgent) handleAllocateModule(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"error": "Module allocation not implemented yet"}
}

func (a *NamingAgent) handleAllocateFunction(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"error": "Function allocation not implemented yet"}
}

func (a *NamingAgent) handleValidateName(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"error": "Name validation not implemented yet"}
}

func main() {
	// Check if we're in test mode
	if len(os.Args) > 1 && os.Args[1] == "test" {
		testNaming()
		return
	}
	
	agent := NewAgent()
	agent.Start()
}

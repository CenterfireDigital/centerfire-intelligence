package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
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
	case "allocate_namespace":
		return a.handleAllocateNamespace(request)
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
	// Get sequence from Redis and increment atomically
	sequenceKey := fmt.Sprintf("centerfire.dev.sequence:CAP-%s", domain)
	sequence, err := a.RedisClient.Incr(a.ctx, sequenceKey).Result()
	if err != nil {
		fmt.Printf("Error getting sequence from Redis: %v, using fallback\n", err)
		sequence = 1
	}
	
	// Generate ULID for uniqueness
	now := time.Now()
	ulid := fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
	
	slug := fmt.Sprintf("CAP-%s-%d", domain, sequence)
	cid := fmt.Sprintf("cid:centerfire:capability:%s", ulid)
	directory := fmt.Sprintf("%s__%s", slug, ulid)
	
	// Store the allocated name in Redis for tracking
	nameKey := fmt.Sprintf("centerfire.dev.names:capability:%s", slug)
	allocated := time.Now().Format(time.RFC3339)
	nameData := map[string]interface{}{
		"slug":      slug,
		"cid":       cid,
		"directory": directory,
		"domain":    domain,
		"purpose":   purpose,
		"sequence":  sequence,
		"allocated": allocated,
	}
	nameJSON, _ := json.Marshal(nameData)
	a.RedisClient.Set(a.ctx, nameKey, nameJSON, 0) // No expiration
	
	// Publish semantic name event to stream for W/N consumers
	a.publishSemanticNameEvent(slug, cid, directory, domain, purpose, sequence, allocated)
	
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
	// Get sequence from Redis and increment atomically
	sequenceKey := fmt.Sprintf("centerfire.dev.sequence:SES-%s", strings.ToUpper(sessionType))
	sequence, err := a.RedisClient.Incr(a.ctx, sequenceKey).Result()
	if err != nil {
		fmt.Printf("Error getting session sequence from Redis: %v, using fallback\n", err)
		sequence = 1
	}

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

// publishSemanticNameEvent - Publish semantic name events to Redis streams for W/N consumers
func (a *NamingAgent) publishSemanticNameEvent(slug, cid, directory, domain, purpose string, sequence int64, allocated string) {
	streamName := "centerfire:semantic:names"
	
	eventData := map[string]interface{}{
		"slug":       slug,
		"cid":        cid,
		"directory":  directory,
		"domain":     domain,
		"purpose":    purpose,
		"sequence":   sequence,
		"allocated":  allocated,
		"event_type": "capability_allocated",
	}
	
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		fmt.Printf("%s: Error marshaling event data: %v\n", a.AgentID, err)
		return
	}
	
	_, err = a.RedisClient.XAdd(a.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data":      string(eventJSON),
			"timestamp": time.Now().Unix(),
			"source":    a.AgentID,
		},
	}).Result()
	
	if err != nil {
		fmt.Printf("%s: Error publishing to stream: %v\n", a.AgentID, err)
	} else {
		fmt.Printf("%s: Published semantic name event to stream: %s\n", a.AgentID, slug)
	}
}

// handleAllocateNamespace - Allocate semantic namespace names instead of string concatenation
func (a *NamingAgent) handleAllocateNamespace(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	project, ok := params["project"].(string)
	if !ok {
		return map[string]interface{}{"error": "Project required"}
	}
	
	environment, ok := params["environment"].(string)
	if !ok {
		return map[string]interface{}{"error": "Environment required"}
	}
	
	// Optional class type for className generation
	classType, _ := params["class_type"].(string)
	
	// Generate semantic namespace name 
	namespace := a.generateNamespaceID(project, environment)
	
	response := map[string]interface{}{
		"namespace": namespace["namespace"],
		"cid":       namespace["cid"],
		"project":   project,
		"environment": environment,
	}
	
	// Generate className if requested
	if classType != "" {
		className := a.generateClassName(namespace["cid"].(string), classType)
		response["className"] = className
	}
	
	fmt.Printf("%s: Allocated semantic namespace: %s (CID: %s)\n", 
		a.AgentID, namespace["namespace"], namespace["cid"])
	
	return response
}

// generateNamespaceID - Generate semantic namespace with CID instead of string concatenation
func (a *NamingAgent) generateNamespaceID(project, environment string) map[string]interface{} {
	// Get sequence from Redis for namespace allocation
	sequenceKey := fmt.Sprintf("centerfire.%s.sequence:NS-%s", environment, strings.ToUpper(project))
	sequence, err := a.RedisClient.Incr(a.ctx, sequenceKey).Result()
	if err != nil {
		fmt.Printf("Error getting namespace sequence from Redis: %v, using fallback\n", err)
		sequence = 1
	}
	
	// Generate ULID for uniqueness
	now := time.Now()
	ulid := fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
	
	// Create semantic namespace CID
	cid := fmt.Sprintf("cid:%s:%s:namespace:%s", project, environment, ulid)
	
	// Use CID-based namespace instead of simple concatenation
	namespace := fmt.Sprintf("%s.%s.ns%d", project, environment, sequence)
	
	// Store namespace allocation in Redis for tracking
	nameKey := fmt.Sprintf("centerfire.%s.namespaces:%s", environment, namespace)
	allocated := time.Now().Format(time.RFC3339)
	namespaceData := map[string]interface{}{
		"namespace":   namespace,
		"cid":         cid,
		"project":     project,
		"environment": environment,
		"sequence":    sequence,
		"allocated":   allocated,
	}
	nameJSON, _ := json.Marshal(namespaceData)
	a.RedisClient.Set(a.ctx, nameKey, nameJSON, 0) // No expiration
	
	// Publish semantic namespace event to stream for W/N consumers
	a.publishSemanticNamespaceEvent(namespace, cid, project, environment, sequence, allocated)
	
	return map[string]interface{}{
		"namespace": namespace,
		"cid":       cid,
		"project":   project,
		"environment": environment,
		"sequence":  sequence,
	}
}

// generateClassName - Generate Weaviate className using semantic namespace CID
func (a *NamingAgent) generateClassName(namespaceCID, classType string) string {
	// Extract project and environment from CID for consistent naming
	// Format: cid:project:environment:namespace:ulid
	parts := strings.Split(namespaceCID, ":")
	if len(parts) >= 3 {
		project := strings.Title(strings.ToLower(parts[1]))
		env := strings.Title(strings.ToLower(parts[2]))
		class := strings.Title(strings.ToLower(classType))
		return fmt.Sprintf("%s_%s_%s", project, env, class)
	}
	
	// Fallback if CID parsing fails
	class := strings.Title(strings.ToLower(classType))
	return fmt.Sprintf("Semantic_%s", class)
}

// publishSemanticNamespaceEvent - Publish namespace allocation events to Redis streams
func (a *NamingAgent) publishSemanticNamespaceEvent(namespace, cid, project, environment string, sequence int64, allocated string) {
	streamName := "centerfire:semantic:namespaces"
	
	eventData := map[string]interface{}{
		"namespace":   namespace,
		"cid":         cid,
		"project":     project,
		"environment": environment,
		"sequence":    sequence,
		"allocated":   allocated,
		"event_type":  "namespace_allocated",
	}
	
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		fmt.Printf("%s: Error marshaling namespace event data: %v\n", a.AgentID, err)
		return
	}
	
	_, err = a.RedisClient.XAdd(a.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data":      string(eventJSON),
			"timestamp": time.Now().Unix(),
			"source":    a.AgentID,
		},
	}).Result()
	
	if err != nil {
		fmt.Printf("%s: Error publishing to namespace stream: %v\n", a.AgentID, err)
	} else {
		fmt.Printf("%s: Published semantic namespace event to stream: %s\n", a.AgentID, namespace)
	}
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
	agent := NewAgent()
	agent.Start()
}

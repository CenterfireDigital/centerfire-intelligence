package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	
	"github.com/redis/go-redis/v9"
)

// ContextAgent - Fast Weaviate GraphQL context retrieval agent
type ContextAgent struct {
	AgentID         string
	CID            string
	RequestChannel  string
	ResponseChannel string
	RedisClient    *redis.Client
	WeaviateClient *http.Client
	WeaviateURL    string
	Project        string  // Project name (e.g., "centerfire")
	Environment    string  // Environment (dev/test/prod)
	ctx            context.Context
	queryCache     map[string]*CacheEntry
	cacheTimeout   time.Duration
}

type CacheEntry struct {
	Data      interface{}
	Timestamp time.Time
}

type GraphQLQuery struct {
	Query string `json:"query"`
}

type GraphQLResponse struct {
	Data   interface{} `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message string `json:"message"`
	Path    []interface{} `json:"path,omitempty"`
}

// NewAgent - Create new CONTEXT agent
func NewAgent() *ContextAgent {
	// Connect to Redis container on port 6380
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	// Create HTTP client with persistent connections for Weaviate
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}
	
	return &ContextAgent{
		AgentID:         "AGT-CONTEXT-1",
		CID:            "cid:centerfire.dev:agent:17572052",
		RequestChannel:  "agent.context.request",
		ResponseChannel: "agent.context.response",
		RedisClient:    rdb,
		WeaviateClient: httpClient,
		WeaviateURL:    "http://localhost:8080",
		Project:        "centerfire",  // Default project
		Environment:    "dev",         // Default environment
		ctx:            context.Background(),
		queryCache:     make(map[string]*CacheEntry),
		cacheTimeout:   5 * time.Minute,
	}
}

// Start - Start listening for requests
func (a *ContextAgent) Start() {
	fmt.Printf("%s starting...\n", a.AgentID)
	fmt.Printf("Listening on: %s\n", a.RequestChannel)
	
	// Test Redis connection
	_, err := a.RedisClient.Ping(a.ctx).Result()
	if err != nil {
		fmt.Printf("Failed to connect to Redis: %v\n", err)
		return
	}
	fmt.Printf("Connected to Redis successfully\n")
	
	// Test Weaviate connection
	err = a.testWeaviateConnection()
	if err != nil {
		fmt.Printf("Failed to connect to Weaviate: %v\n", err)
		return
	}
	fmt.Printf("Connected to Weaviate successfully\n")
	
	// Register with AGT-MANAGER-1 for singleton enforcement
	err = a.registerWithManager()
	if err != nil {
		fmt.Printf("Failed to register with manager: %v\n", err)
		return
	}
	fmt.Printf("Registered with AGT-MANAGER-1 successfully\n")
	
	// Subscribe to request channel
	pubsub := a.RedisClient.Subscribe(a.ctx, a.RequestChannel)
	defer pubsub.Close()
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start heartbeat
	go a.startHeartbeat()
	
	// Start cache cleanup
	go a.startCacheCleanup()
	
	fmt.Printf("%s ready for context retrieval requests\n", a.AgentID)
	
	// Listen for messages
	for {
		select {
		case msg := <-pubsub.Channel():
			go a.handleRequest(msg.Payload)
		case <-sigChan:
			fmt.Printf("\n%s shutting down gracefully...\n", a.AgentID)
			a.unregisterWithManager()
			return
		}
	}
}

// testWeaviateConnection - Test connection to Weaviate
func (a *ContextAgent) testWeaviateConnection() error {
	resp, err := a.WeaviateClient.Get(a.WeaviateURL + "/v1/meta")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("Weaviate returned status: %d", resp.StatusCode)
	}
	
	return nil
}

// handleRequest - Process incoming requests
func (a *ContextAgent) handleRequest(payload string) {
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		fmt.Printf("Failed to parse request: %v\n", err)
		return
	}
	
	action, ok := request["action"].(string)
	if !ok {
		a.publishError("Missing action field", request)
		return
	}
	
	requestID, _ := request["request_id"].(string)
	
	fmt.Printf("Processing request: %s (ID: %s)\n", action, requestID)
	
	switch action {
	case "search_conversations":
		a.handleSearchConversations(request)
	case "get_context":
		a.handleGetContext(request)
	case "search_semantic":
		a.handleSearchSemantic(request)
	case "get_session_history":
		a.handleGetSessionHistory(request)
	default:
		a.publishError(fmt.Sprintf("Unknown action: %s", action), request)
	}
}

// handleSearchConversations - Search conversation history
func (a *ContextAgent) handleSearchConversations(request map[string]interface{}) {
	query, ok := request["query"].(string)
	if !ok {
		a.publishError("Missing query parameter", request)
		return
	}
	
	limit := 10
	if l, ok := request["limit"].(float64); ok {
		limit = int(l)
	}
	
	// Check cache first
	cacheKey := fmt.Sprintf("conv_%s_%d", query, limit)
	if cached := a.getFromCache(cacheKey); cached != nil {
		a.publishResponse(map[string]interface{}{
			"success":    true,
			"data":       cached,
			"request_id": request["request_id"],
			"cached":     true,
		})
		return
	}
	
	// Build GraphQL query for conversation search
	graphqlQuery := fmt.Sprintf(`{
		Get {
			ConversationHistory(
				nearText: {concepts: ["%s"]}
				limit: %d
			) {
				content
				session_id
				timestamp
				agent_id
				user
				assistant
			}
		}
	}`, strings.ReplaceAll(query, `"`, `\"`), limit)
	
	result, err := a.executeGraphQLQuery(graphqlQuery)
	if err != nil {
		a.publishError(fmt.Sprintf("GraphQL query failed: %v", err), request)
		return
	}
	
	// Cache the result
	a.setCache(cacheKey, result)
	
	a.publishResponse(map[string]interface{}{
		"success":    true,
		"data":       result,
		"request_id": request["request_id"],
		"cached":     false,
	})
}

// handleGetContext - Get context for a specific session or topic
func (a *ContextAgent) handleGetContext(request map[string]interface{}) {
	sessionID, hasSession := request["session_id"].(string)
	topic, hasTopic := request["topic"].(string)
	
	if !hasSession && !hasTopic {
		a.publishError("Missing session_id or topic parameter", request)
		return
	}
	
	limit := 5
	if l, ok := request["limit"].(float64); ok {
		limit = int(l)
	}
	
	var graphqlQuery string
	var cacheKey string
	
	if hasSession {
		cacheKey = fmt.Sprintf("context_session_%s_%d", sessionID, limit)
		graphqlQuery = fmt.Sprintf(`{
			Get {
				ConversationHistory(
					where: {
						path: ["session_id"]
						operator: Equal
						valueString: "%s"
					}
					limit: %d
				) {
					content
					timestamp
					user
					assistant
				}
			}
		}`, sessionID, limit)
	} else {
		cacheKey = fmt.Sprintf("context_topic_%s_%d", topic, limit)
		graphqlQuery = fmt.Sprintf(`{
			Get {
				ConversationHistory(
					nearText: {concepts: ["%s"]}
					limit: %d
				) {
					content
					session_id
					timestamp
					user
					assistant
				}
			}
		}`, strings.ReplaceAll(topic, `"`, `\"`), limit)
	}
	
	// Check cache
	if cached := a.getFromCache(cacheKey); cached != nil {
		a.publishResponse(map[string]interface{}{
			"success":    true,
			"data":       cached,
			"request_id": request["request_id"],
			"cached":     true,
		})
		return
	}
	
	result, err := a.executeGraphQLQuery(graphqlQuery)
	if err != nil {
		a.publishError(fmt.Sprintf("GraphQL query failed: %v", err), request)
		return
	}
	
	a.setCache(cacheKey, result)
	
	a.publishResponse(map[string]interface{}{
		"success":    true,
		"data":       result,
		"request_id": request["request_id"],
		"cached":     false,
	})
}

// handleSearchSemantic - Generic semantic search
func (a *ContextAgent) handleSearchSemantic(request map[string]interface{}) {
	concepts, ok := request["concepts"].([]interface{})
	if !ok {
		a.publishError("Missing concepts parameter", request)
		return
	}
	
	conceptStrings := make([]string, len(concepts))
	for i, c := range concepts {
		if s, ok := c.(string); ok {
			conceptStrings[i] = fmt.Sprintf(`"%s"`, strings.ReplaceAll(s, `"`, `\"`))
		}
	}
	
	limit := 10
	if l, ok := request["limit"].(float64); ok {
		limit = int(l)
	}
	
	cacheKey := fmt.Sprintf("semantic_%s_%d", strings.Join(conceptStrings, "_"), limit)
	
	if cached := a.getFromCache(cacheKey); cached != nil {
		a.publishResponse(map[string]interface{}{
			"success":    true,
			"data":       cached,
			"request_id": request["request_id"],
			"cached":     true,
		})
		return
	}
	
	graphqlQuery := fmt.Sprintf(`{
		Get {
			ConversationHistory(
				nearText: {concepts: [%s]}
				limit: %d
			) {
				content
				session_id
				timestamp
				agent_id
				user
				assistant
			}
		}
	}`, strings.Join(conceptStrings, ", "), limit)
	
	result, err := a.executeGraphQLQuery(graphqlQuery)
	if err != nil {
		a.publishError(fmt.Sprintf("GraphQL query failed: %v", err), request)
		return
	}
	
	a.setCache(cacheKey, result)
	
	a.publishResponse(map[string]interface{}{
		"success":    true,
		"data":       result,
		"request_id": request["request_id"],
		"cached":     false,
	})
}

// handleGetSessionHistory - Get complete session history
func (a *ContextAgent) handleGetSessionHistory(request map[string]interface{}) {
	sessionID, ok := request["session_id"].(string)
	if !ok {
		a.publishError("Missing session_id parameter", request)
		return
	}
	
	cacheKey := fmt.Sprintf("session_history_%s", sessionID)
	
	if cached := a.getFromCache(cacheKey); cached != nil {
		a.publishResponse(map[string]interface{}{
			"success":    true,
			"data":       cached,
			"request_id": request["request_id"],
			"cached":     true,
		})
		return
	}
	
	graphqlQuery := fmt.Sprintf(`{
		Get {
			ConversationHistory(
				where: {
					path: ["session_id"]
					operator: Equal
					valueString: "%s"
				}
			) {
				content
				timestamp
				user
				assistant
				turn_count
			}
		}
	}`, sessionID)
	
	result, err := a.executeGraphQLQuery(graphqlQuery)
	if err != nil {
		a.publishError(fmt.Sprintf("GraphQL query failed: %v", err), request)
		return
	}
	
	a.setCache(cacheKey, result)
	
	a.publishResponse(map[string]interface{}{
		"success":    true,
		"data":       result,
		"request_id": request["request_id"],
		"cached":     false,
	})
}

// executeGraphQLQuery - Execute GraphQL query against Weaviate
func (a *ContextAgent) executeGraphQLQuery(query string) (interface{}, error) {
	queryBody := GraphQLQuery{Query: query}
	jsonBody, err := json.Marshal(queryBody)
	if err != nil {
		return nil, err
	}
	
	resp, err := a.WeaviateClient.Post(
		a.WeaviateURL+"/v1/graphql",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GraphQL query failed with status: %d", resp.StatusCode)
	}
	
	var response GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	
	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", response.Errors)
	}
	
	return response.Data, nil
}

// Cache management
func (a *ContextAgent) getFromCache(key string) interface{} {
	entry, exists := a.queryCache[key]
	if !exists {
		return nil
	}
	
	if time.Since(entry.Timestamp) > a.cacheTimeout {
		delete(a.queryCache, key)
		return nil
	}
	
	return entry.Data
}

func (a *ContextAgent) setCache(key string, data interface{}) {
	a.queryCache[key] = &CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
	}
}

func (a *ContextAgent) startCacheCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			for key, entry := range a.queryCache {
				if now.Sub(entry.Timestamp) > a.cacheTimeout {
					delete(a.queryCache, key)
				}
			}
		case <-a.ctx.Done():
			return
		}
	}
}

// Agent lifecycle management
func (a *ContextAgent) registerWithManager() error {
	registrationData := map[string]interface{}{
		"action":      "register_running",
		"agent_name":  a.AgentID,
		"session_id":  fmt.Sprintf("%s_%d", a.AgentID, time.Now().Unix()),
		"pid":         os.Getpid(),
		"agent_type":  "persistent",
		"capabilities": []string{"search_conversations", "get_context", "search_semantic", "get_session_history"},
		"channels":    []string{a.RequestChannel},
	}
	
	data, _ := json.Marshal(registrationData)
	return a.RedisClient.Publish(a.ctx, "agent.manager.request", string(data)).Err()
}

func (a *ContextAgent) unregisterWithManager() error {
	unregisterData := map[string]interface{}{
		"action":     "unregister_running",
		"agent_name": a.AgentID,
	}
	
	data, _ := json.Marshal(unregisterData)
	return a.RedisClient.Publish(a.ctx, "agent.manager.request", string(data)).Err()
}

func (a *ContextAgent) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			heartbeatData := map[string]interface{}{
				"action":     "heartbeat",
				"agent_name": a.AgentID,
				"timestamp":  time.Now().Unix(),
				"status":     "healthy",
				"cache_size": len(a.queryCache),
			}
			
			data, _ := json.Marshal(heartbeatData)
			a.RedisClient.Publish(a.ctx, "agent.manager.request", string(data))
		case <-a.ctx.Done():
			return
		}
	}
}

// Response publishing
func (a *ContextAgent) publishResponse(response map[string]interface{}) {
	data, _ := json.Marshal(response)
	a.RedisClient.Publish(a.ctx, a.ResponseChannel, string(data))
}

func (a *ContextAgent) publishError(errorMsg string, request map[string]interface{}) {
	response := map[string]interface{}{
		"success": false,
		"error":   errorMsg,
	}
	
	if requestID, ok := request["request_id"]; ok {
		response["request_id"] = requestID
	}
	
	a.publishResponse(response)
}

func main() {
	agent := NewAgent()
	if agent == nil {
		fmt.Println("Failed to create CONTEXT agent")
		return
	}
	
	agent.Start()
}
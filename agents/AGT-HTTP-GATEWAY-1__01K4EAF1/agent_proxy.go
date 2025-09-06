package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	
	"github.com/redis/go-redis/v9"
)

// AgentProxy manages Redis-based communication with agents
type AgentProxy struct {
	mu            sync.RWMutex
	redisClient   *redis.Client
	requestTimeout time.Duration
	ctx           context.Context
	verbose       bool // For diagnostic logging
}

// AgentResponse represents a response from an agent
type AgentResponse struct {
	Success   bool                   `json:"success"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                `json:"error,omitempty"`
	RequestID string                `json:"request_id,omitempty"`
	Timestamp time.Time             `json:"timestamp"`
}

// AgentRequest represents a request to an agent
type AgentRequest struct {
	Action    string                 `json:"action"`
	Data      map[string]interface{} `json:"data,omitempty"`
	ClientID  string                `json:"client_id"`
	RequestID string                `json:"request_id"`
}

// AgentStatus represents the status of an agent
type AgentStatus struct {
	Name         string    `json:"name"`
	Online       bool      `json:"online"`
	LastCheck    time.Time `json:"last_check"`
	SocketPath   string    `json:"socket_path"`
	ResponseTime *int64    `json:"response_time_ms,omitempty"` // nil if offline
}

// NewAgentProxy creates a new agent proxy
func NewAgentProxy(ctx context.Context) *AgentProxy {
	// Connect to Redis container on port 6380
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	return &AgentProxy{
		redisClient:    rdb,
		requestTimeout: 30 * time.Second,
		ctx:           ctx,
		verbose:       true, // Enable verbose logging for diagnostics
	}
}

// ForwardToAgent forwards a request to the specified agent via Redis pub/sub
func (ap *AgentProxy) ForwardToAgent(agent, action string, data map[string]interface{}, clientID, requestID string) (*AgentResponse, error) {
	startTime := time.Now()
	
	// Create agent request in the format agents expect
	agentReq := map[string]interface{}{
		"action":     action,
		"params":     data,  // AGT-NAMING-1 expects "params" not "data"
		"client_id":  clientID,
		"request_id": requestID,
	}
	
	// Determine request and response channels based on agent
	var requestChannel, responseChannel string
	switch agent {
	case "naming":
		requestChannel = "agent.naming.request"
		responseChannel = "agent.naming.response"
	case "struct":
		requestChannel = "agent.struct.request"
		responseChannel = "agent.struct.response"
	case "semantic":
		requestChannel = "agent.semantic.request"
		responseChannel = "agent.semantic.response"
	case "manager":
		requestChannel = "agent.manager.request"
		responseChannel = "agent.manager.response"
	default:
		return nil, fmt.Errorf("unknown agent: %s", agent)
	}
	
	// Subscribe to response channel before sending request
	pubsub := ap.redisClient.Subscribe(ap.ctx, responseChannel)
	defer pubsub.Close()
	
	// Send request
	requestData, err := json.Marshal(agentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	if ap.verbose {
		fmt.Printf("🔍 Publishing to %s: %s\n", requestChannel, string(requestData))
	}
	
	err = ap.redisClient.Publish(ap.ctx, requestChannel, requestData).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to publish request to agent %s: %v", agent, err)
	}
	
	// Wait for response with timeout, filtering by request ID
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			if ap.verbose {
				fmt.Printf("🔍 Received on %s: %s\n", responseChannel, msg.Payload)
			}
			
			var response map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &response); err != nil {
				if ap.verbose {
					fmt.Printf("❌ Failed to parse message: %v\n", err)
				}
				continue // Skip malformed messages
			}
			
			// Check if this is our response by request ID
			if responseReqID, ok := response["request_id"].(string); ok && responseReqID == requestID {
				if ap.verbose {
					fmt.Printf("✅ Found matching response for request %s\n", requestID)
				}
				
				// Determine success - if there's an error field, it's a failure, otherwise success
				success := true
				errorMsg := ""
				if err, ok := response["error"].(string); ok && err != "" {
					success = false
					errorMsg = err
				} else if successField, ok := response["success"].(bool); ok {
					success = successField
				}
				
				// Convert to AgentResponse format
				agentResp := AgentResponse{
					Success:   success,
					Error:     errorMsg,
					RequestID: requestID,
					Timestamp: time.Now(),
				}
				
				// If successful, the entire response is the data
				if success {
					agentResp.Data = response
				}
				
				responseTime := time.Since(startTime).Milliseconds()
				fmt.Printf("📨 Agent %s responded in %dms\n", agent, responseTime)
				
				return &agentResp, nil
			} else {
				if ap.verbose {
					fmt.Printf("⏳ Skipping response with ID %v (waiting for %s)\n", response["request_id"], requestID)
				}
			}
			
		case <-time.After(ap.requestTimeout):
			return nil, fmt.Errorf("timeout waiting for response from agent %s", agent)
		case <-ap.ctx.Done():
			return nil, fmt.Errorf("request cancelled")
		}
	}
}

// pingAgent sends a ping request to check if agent is responsive
func (ap *AgentProxy) pingAgent(agent string) (*AgentResponse, error) {
	requestID := fmt.Sprintf("ping_%d", time.Now().UnixNano())
	return ap.ForwardToAgent(agent, "ping", nil, "gateway", requestID)
}

// HealthCheckAgent performs a health check on an agent via Redis ping
func (ap *AgentProxy) HealthCheckAgent(agent string) *AgentStatus {
	startTime := time.Now()
	
	status := &AgentStatus{
		Name:       agent,
		Online:     false,
		LastCheck:  startTime,
		SocketPath: fmt.Sprintf("redis:%s", agent), // Change to indicate Redis-based
	}
	
	// Try to ping the agent via Redis
	response, err := ap.pingAgent(agent)
	if err != nil {
		// Agent is offline or not responding
		return status
	}
	
	responseTime := time.Since(startTime).Milliseconds()
	status.Online = response.Success
	status.ResponseTime = &responseTime
	
	return status
}

// GetAvailableAgents returns the status of all known agents
func (ap *AgentProxy) GetAvailableAgents() map[string]*AgentStatus {
	// List of known agents (could be loaded from config)
	knownAgents := []string{"naming", "struct", "semantic", "manager"}
	
	results := make(map[string]*AgentStatus)
	
	// Check each agent concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for _, agent := range knownAgents {
		wg.Add(1)
		go func(agentName string) {
			defer wg.Done()
			status := ap.HealthCheckAgent(agentName)
			mu.Lock()
			results[agentName] = status
			mu.Unlock()
		}(agent)
	}
	
	wg.Wait()
	return results
}

// CloseAllConnections closes the Redis connection
func (ap *AgentProxy) CloseAllConnections() {
	if ap.redisClient != nil {
		ap.redisClient.Close()
		fmt.Printf("🔌 Closed Redis connection\n")
	}
}
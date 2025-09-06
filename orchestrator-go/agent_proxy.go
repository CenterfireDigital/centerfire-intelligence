package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// AgentProxy manages connections to socket-based agents and forwards requests
type AgentProxy struct {
	mu                sync.RWMutex
	socketConnections map[string]*net.Conn
	connectionTimeout time.Duration
	requestTimeout    time.Duration
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
func NewAgentProxy() *AgentProxy {
	return &AgentProxy{
		socketConnections: make(map[string]*net.Conn),
		connectionTimeout: 10 * time.Second,
		requestTimeout:    30 * time.Second,
	}
}

// ForwardToAgent forwards a request to the specified agent via socket
func (ap *AgentProxy) ForwardToAgent(agent, action string, data map[string]interface{}, clientID, requestID string) (*AgentResponse, error) {
	startTime := time.Now()
	
	// Create agent request
	agentReq := AgentRequest{
		Action:    action,
		Data:      data,
		ClientID:  clientID,
		RequestID: requestID,
	}
	
	// Get or create socket connection
	conn, err := ap.getSocketConnection(agent)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent %s: %v", agent, err)
	}
	
	// Send request
	requestData, err := json.Marshal(agentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	// Set write timeout
	(*conn).SetWriteDeadline(time.Now().Add(ap.requestTimeout))
	_, err = (*conn).Write(append(requestData, '\n'))
	if err != nil {
		// Connection failed, remove it and try once more
		ap.removeConnection(agent)
		return nil, fmt.Errorf("failed to send request to agent %s: %v", agent, err)
	}
	
	// Read response
	(*conn).SetReadDeadline(time.Now().Add(ap.requestTimeout))
	buffer := make([]byte, 4096)
	n, err := (*conn).Read(buffer)
	if err != nil {
		ap.removeConnection(agent)
		return nil, fmt.Errorf("failed to read response from agent %s: %v", agent, err)
	}
	
	// Parse response
	var agentResp AgentResponse
	if err := json.Unmarshal(buffer[:n], &agentResp); err != nil {
		return nil, fmt.Errorf("failed to parse response from agent %s: %v", agent, err)
	}
	
	// Add response metadata
	agentResp.Timestamp = time.Now()
	if agentResp.RequestID == "" {
		agentResp.RequestID = requestID
	}
	
	responseTime := time.Since(startTime).Milliseconds()
	fmt.Printf("📨 Agent %s responded in %dms\n", agent, responseTime)
	
	return &agentResp, nil
}

// getSocketConnection gets or creates a socket connection to an agent
func (ap *AgentProxy) getSocketConnection(agent string) (*net.Conn, error) {
	ap.mu.RLock()
	if conn, exists := ap.socketConnections[agent]; exists {
		ap.mu.RUnlock()
		// Test if connection is still alive
		if ap.testConnection(conn) {
			return conn, nil
		}
		// Connection is dead, remove it
		ap.removeConnection(agent)
	} else {
		ap.mu.RUnlock()
	}
	
	// Create new connection
	socketPath := fmt.Sprintf("/tmp/orchestrator-%s.sock", agent)
	conn, err := net.DialTimeout("unix", socketPath, ap.connectionTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to socket %s: %v", socketPath, err)
	}
	
	// Store connection
	ap.mu.Lock()
	ap.socketConnections[agent] = &conn
	ap.mu.Unlock()
	
	fmt.Printf("🔌 Connected to agent %s via socket\n", agent)
	return &conn, nil
}

// testConnection tests if a connection is still alive
func (ap *AgentProxy) testConnection(conn *net.Conn) bool {
	if conn == nil || *conn == nil {
		return false
	}
	
	// Set a short deadline for testing
	(*conn).SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	
	// Try to read without blocking
	buffer := make([]byte, 1)
	_, err := (*conn).Read(buffer)
	
	// Reset deadline
	(*conn).SetReadDeadline(time.Time{})
	
	// If we get EOF or timeout, connection might still be good for writing
	// If we get other errors, connection is likely bad
	if err != nil && !isTimeoutError(err) && err.Error() != "EOF" {
		return false
	}
	
	return true
}

// isTimeoutError checks if an error is a timeout error
func isTimeoutError(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}

// removeConnection removes a connection from the pool
func (ap *AgentProxy) removeConnection(agent string) {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	
	if conn, exists := ap.socketConnections[agent]; exists {
		if conn != nil && *conn != nil {
			(*conn).Close()
		}
		delete(ap.socketConnections, agent)
		fmt.Printf("🔌 Disconnected from agent %s\n", agent)
	}
}

// HealthCheckAgent performs a health check on an agent
func (ap *AgentProxy) HealthCheckAgent(agent string) *AgentStatus {
	startTime := time.Now()
	socketPath := fmt.Sprintf("/tmp/orchestrator-%s.sock", agent)
	
	status := &AgentStatus{
		Name:       agent,
		Online:     false,
		LastCheck:  startTime,
		SocketPath: socketPath,
	}
	
	// Try to connect
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return status
	}
	defer conn.Close()
	
	// Send ping request
	pingReq := AgentRequest{
		Action:    "ping",
		ClientID:  "orchestrator",
		RequestID: fmt.Sprintf("health_%d", time.Now().UnixNano()),
	}
	
	requestData, _ := json.Marshal(pingReq)
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(append(requestData, '\n'))
	if err != nil {
		return status
	}
	
	// Read response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		return status
	}
	
	// Parse response
	var response AgentResponse
	if err := json.Unmarshal(buffer[:n], &response); err != nil {
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

// CloseAllConnections closes all socket connections
func (ap *AgentProxy) CloseAllConnections() {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	
	for agent, conn := range ap.socketConnections {
		if conn != nil && *conn != nil {
			(*conn).Close()
		}
		fmt.Printf("🔌 Closed connection to agent %s\n", agent)
	}
	
	// Clear the map
	ap.socketConnections = make(map[string]*net.Conn)
}
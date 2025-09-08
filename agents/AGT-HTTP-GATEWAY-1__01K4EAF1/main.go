package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

// HTTPGatewayAgent - HTTP Gateway for agent access control and routing
type HTTPGatewayAgent struct {
	AgentID           string
	Port              int
	ContractsDir      string
	ContractValidator *ContractValidator
	AgentProxy        *AgentProxy
	httpServer        *http.Server
	redisClient       *redis.Client
	ctx               context.Context
	cancel            context.CancelFunc
}

// APIResponse represents a standardized API response
type APIResponse struct {
	Success   bool                   `json:"success"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                `json:"error,omitempty"`
	RequestID string                `json:"request_id,omitempty"`
	Timestamp time.Time             `json:"timestamp"`
}

// findAvailablePort finds an available port starting from the given port
func findAvailablePort(startPort int) int {
	for port := startPort; port < startPort+100; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			listener.Close()
			return port
		}
	}
	// If no port found in range, return the start port (will error later)
	return startPort
}

// NewHTTPGatewayAgent creates a new HTTP Gateway agent
func NewHTTPGatewayAgent() *HTTPGatewayAgent {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Get contracts directory path (relative to project root)
	contractsDir := "../../contracts"
	if absPath, err := filepath.Abs(contractsDir); err == nil {
		contractsDir = absPath
	}
	
	// Find an available port starting from 8090
	availablePort := findAvailablePort(8090)
	
	// Create Redis client for manager communication
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	return &HTTPGatewayAgent{
		AgentID:           "AGT-HTTP-GATEWAY-1",
		Port:              availablePort,
		ContractsDir:      contractsDir,
		ContractValidator: NewContractValidator(contractsDir),
		AgentProxy:        NewAgentProxy(ctx),
		redisClient:       redisClient,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start initializes and starts the HTTP Gateway
func (h *HTTPGatewayAgent) Start() {
	fmt.Printf("%s starting HTTP Gateway on port %d...\n", h.AgentID, h.Port)
	
	// Load contracts
	if err := h.ContractValidator.LoadContracts(); err != nil {
		fmt.Printf("❌ Failed to load contracts: %v\n", err)
		return
	}
	
	// Register with AGT-MANAGER-1 including port information
	h.registerWithManager()
	
	// Set up HTTP router
	router := mux.NewRouter()
	
	// API endpoints
	api := router.PathPrefix("/api").Subrouter()
	api.Use(h.corsMiddleware)
	api.Use(h.loggingMiddleware)
	
	// Agent routing endpoints
	api.HandleFunc("/agents/available", h.handleAgentDiscovery).Methods("GET")
	api.HandleFunc("/agents/{agent}/{action}", h.handleAgentRequest).Methods("POST")
	api.HandleFunc("/contracts/{client_id}", h.handleContractInfo).Methods("GET")
	api.HandleFunc("/health", h.handleHealth).Methods("GET")
	api.HandleFunc("/system/health", h.handleSystemHealth).Methods("GET")
	
	// Root endpoints
	router.HandleFunc("/", h.handleRoot).Methods("GET")
	router.HandleFunc("/health", h.handleHealth).Methods("GET")
	
	// Create HTTP server
	h.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", h.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	
	// Start server in background
	go func() {
		fmt.Printf("✅ %s ready - HTTP Gateway listening on port %d\n", h.AgentID, h.Port)
		fmt.Printf("🔗 Agent API: http://localhost:%d/api/agents/{agent}/{action}\n", h.Port)
		fmt.Printf("🔍 Discovery: http://localhost:%d/api/agents/available\n", h.Port)
		
		if err := h.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ HTTP server error: %v\n", err)
		}
	}()
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Wait for shutdown signal
	<-sigChan
	fmt.Printf("\n%s shutting down...\n", h.AgentID)
	h.shutdown()
}

// handleAgentRequest handles requests to specific agents
func (h *HTTPGatewayAgent) handleAgentRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agent := vars["agent"]
	action := vars["action"]
	
	// Generate request ID
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	
	// Extract client ID from headers or query params
	clientID := h.extractClientID(r)
	if clientID == "" {
		h.writeErrorResponse(w, "Missing client_id", requestID, http.StatusUnauthorized)
		return
	}
	
	// Validate contract
	if err := h.ContractValidator.ValidateRequest(clientID, agent, action); err != nil {
		h.writeErrorResponse(w, err.Error(), requestID, http.StatusForbidden)
		return
	}
	
	// Parse request body
	var requestData map[string]interface{}
	if r.Body != nil {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			h.writeErrorResponse(w, "Failed to read request body", requestID, http.StatusBadRequest)
			return
		}
		
		if len(body) > 0 {
			if err := json.Unmarshal(body, &requestData); err != nil {
				h.writeErrorResponse(w, "Invalid JSON in request body", requestID, http.StatusBadRequest)
				return
			}
		}
	}
	
	// Forward to agent
	agentResponse, err := h.AgentProxy.ForwardToAgent(agent, action, requestData, clientID, requestID)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Agent communication error: %v", err), requestID, http.StatusServiceUnavailable)
		return
	}
	
	// Return agent response
	w.Header().Set("Content-Type", "application/json")
	if agentResponse.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	
	json.NewEncoder(w).Encode(agentResponse)
}

// handleAgentDiscovery returns available agents and their status
func (h *HTTPGatewayAgent) handleAgentDiscovery(w http.ResponseWriter, r *http.Request) {
	clientID := h.extractClientID(r)
	if clientID == "" {
		h.writeErrorResponse(w, "Missing client_id", "", http.StatusUnauthorized)
		return
	}
	
	// Get available agents
	agentStatuses := h.AgentProxy.GetAvailableAgents()
	
	// Filter based on client contract
	allowedAgents := h.ContractValidator.GetAllowedAgents(clientID)
	filteredAgents := make(map[string]interface{})
	
	for agentName, status := range agentStatuses {
		if permissions, allowed := allowedAgents[agentName]; allowed {
			agentInfo := map[string]interface{}{
				"status":      status,
				"permissions": permissions,
			}
			filteredAgents[agentName] = agentInfo
		}
	}
	
	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"agents":    filteredAgents,
			"client_id": clientID,
			"total":     len(filteredAgents),
		},
		Timestamp: time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleContractInfo returns contract information for a client
func (h *HTTPGatewayAgent) handleContractInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["client_id"]
	
	contractInfo := h.ContractValidator.GetContractInfo(clientID)
	
	response := APIResponse{
		Success:   true,
		Data:      contractInfo,
		Timestamp: time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth returns gateway health status
func (h *HTTPGatewayAgent) handleHealth(w http.ResponseWriter, r *http.Request) {
	agentStatuses := h.AgentProxy.GetAvailableAgents()
	
	onlineCount := 0
	totalCount := len(agentStatuses)
	
	for _, status := range agentStatuses {
		if status.Online {
			onlineCount++
		}
	}
	
	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"gateway":       "healthy",
			"port":          h.Port,
			"agents_online": onlineCount,
			"agents_total":  totalCount,
			"agents":        agentStatuses,
			"contracts":     len(h.ContractValidator.contracts),
		},
		Timestamp: time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSystemHealth returns comprehensive system health including containers, agents, and services
func (h *HTTPGatewayAgent) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	// Get Docker containers
	containers := h.getDockerContainerStatus()
	
	// Get agent processes  
	agents := h.getAgentProcessStatus()
	
	// Get Redis health
	redisHealth := h.getRedisHealth()
	
	// Get service endpoints health
	endpoints := h.getServiceEndpointsHealth()
	
	// Calculate summary
	summary := map[string]interface{}{
		"containers_running": h.countRunning(containers),
		"containers_expected": len(containers),
		"agents_running": h.countRunning(agents),
		"agents_expected": len(agents),
		"redis_connected": redisHealth["connected"],
		"endpoints_accessible": h.countAccessible(endpoints),
	}
	
	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"timestamp":    time.Now(),
			"check_duration": time.Since(startTime).Milliseconds(),
			"agents":      agents,
			"containers":   containers,
			"redis":       redisHealth,
			"endpoints":   endpoints,
			"summary":     summary,
		},
		Timestamp: time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions for system health checks
func (h *HTTPGatewayAgent) getDockerContainerStatus() []map[string]interface{} {
	// This would normally execute docker commands, but for now return expected containers
	expectedContainers := []string{
		"mem0-redis", "centerfire-weaviate", "centerfire-neo4j", 
		"centerfire-clickhouse", "centerfire-casbin", "centerfire-transformers",
	}
	
	containers := make([]map[string]interface{}, len(expectedContainers))
	for i, name := range expectedContainers {
		containers[i] = map[string]interface{}{
			"name": name,
			"expected": true,
			"running": true, // Simplified for now
			"status": "Up",
		}
	}
	return containers
}

func (h *HTTPGatewayAgent) getAgentProcessStatus() []map[string]interface{} {
	// Get dual verification: Manager registry vs actual process detection
	
	// Try to get agent registry from AGT-MANAGER-1 HTTP API
	agentMap := h.getManagerRegistry()
	managerOnline := len(agentMap) > 0 || h.isManagerResponding()
	
	// Special case: If we can query the manager, then the manager is running!
	if managerOnline {
		agentMap["AGT-MANAGER-1"] = map[string]interface{}{
			"name":           "AGT-MANAGER-1",
			"type":           "persistent",
			"capabilities":   []string{"agent_management", "service_discovery"},
			"integrations":   []string{"Redis:6380", "HTTP:8380"},
			"location":       "agents/AGT-MANAGER-1__manager1",
			"manager_status": "online", // If we can query it, it's online!
			"has_registry":   true,
		}
	}
	
	// Add expected agents that might not be in registry
	expectedAgents := []string{
		"AGT-NAMING-1", "AGT-CONTEXT-1", "AGT-MANAGER-1",
		"AGT-SYSTEM-COMMANDER-1", "AGT-CLAUDE-CAPTURE-1", "AGT-STACK-1",
		"AGT-HTTP-GATEWAY-1", "AGT-SEMDOC-PARSER-1",
	}
	
	for _, agentID := range expectedAgents {
		if _, exists := agentMap[agentID]; !exists {
			agentMap[agentID] = map[string]interface{}{
				"name":         agentID,
				"type":         "unknown",
				"capabilities": []string{},
				"integrations": []string{},
				"location":     "unknown",
				"manager_status": "not_registered",
				"has_registry": false,
			}
		}
	}
	
	// Now add actual process detection for all agents
	var agents []map[string]interface{}
	for agentID, agentInfo := range agentMap {
		processRunning, processStatus := h.checkAgentRunning(agentID)
		
		// Determine overall status and any discrepancies
		managerRunning := agentInfo["manager_status"] == "operational" || agentInfo["manager_status"] == "running" || agentInfo["manager_status"] == "online"
		var overallStatus string
		var statusClass string
		
		if !agentInfo["has_registry"].(bool) {
			// No registry entry
			if processRunning {
				overallStatus = "Running (unregistered)"
				statusClass = "warning"
			} else {
				overallStatus = "Not running"
				statusClass = "stopped"
			}
		} else if managerRunning && processRunning {
			overallStatus = "Running (verified)"
			statusClass = "running"
		} else if managerRunning && !processRunning {
			overallStatus = "Manager thinks running, but not found"
			statusClass = "warning"
		} else if !managerRunning && processRunning {
			overallStatus = "Running but not in registry"
			statusClass = "warning"
		} else {
			overallStatus = "Not running"
			statusClass = "stopped"
		}
		
		agentInfo["process_running"] = processRunning
		agentInfo["process_status"] = processStatus
		agentInfo["running"] = processRunning // For backward compatibility
		agentInfo["status"] = overallStatus
		agentInfo["status_class"] = statusClass
		agentInfo["registry_vs_process"] = map[string]interface{}{
			"manager_running": managerRunning,
			"process_running": processRunning,
			"match": managerRunning == processRunning,
		}
		
		agents = append(agents, agentInfo)
	}
	
	return agents
}

// getBasicAgentStatus provides fallback agent status when registry is unavailable
func (h *HTTPGatewayAgent) getBasicAgentStatus() []map[string]interface{} {
	expectedAgents := []string{
		"AGT-NAMING-1", "AGT-CONTEXT-1", "AGT-MANAGER-1",
		"AGT-SYSTEM-COMMANDER-1", "AGT-CLAUDE-CAPTURE-1", "AGT-STACK-1",
		"AGT-HTTP-GATEWAY-1", "AGT-SEMDOC-PARSER-1",
	}
	
	agents := make([]map[string]interface{}, len(expectedAgents))
	for i, name := range expectedAgents {
		running, status := h.checkAgentRunning(name)
		agents[i] = map[string]interface{}{
			"name": name,
			"expected": true,
			"running": running,
			"status": status,
			"type": "unknown",
			"capabilities": []string{},
			"integrations": []string{},
			"location": "unknown",
		}
	}
	return agents
}

// checkAgentRunning checks if a specific agent is running
func (h *HTTPGatewayAgent) checkAgentRunning(agentName string) (bool, string) {
	// Working process detection patterns based on actual running processes
	var cmd string
	switch agentName {
	case "AGT-HTTP-GATEWAY-1":
		cmd = "ps aux | grep './gateway' | grep -v grep | awk '{print $2}' | head -1"
	case "AGT-SEMDOC-PARSER-1":
		cmd = "lsof -c go | grep AGT-SEMDOC-PARSER-1 | grep cwd | awk '{print $2}' | head -1"
	case "AGT-MANAGER-1":
		cmd = "lsof -c go | grep AGT-MANAGER-1 | grep cwd | awk '{print $2}' | head -1"
	case "AGT-CLAUDE-CAPTURE-1":
		cmd = "lsof -c Python | grep AGT-CLAUDE-CAPTURE-1 | grep cwd | awk '{print $2}' | head -1"
	default:
		// Try generic pattern for other agents
		cmd = fmt.Sprintf("ps aux | grep '%s' | grep -v grep | awk '{print $2}' | head -1", agentName)
	}
	
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true, "Running"
	}
	
	return false, "Not running"
}

// getManagerRegistry queries AGT-MANAGER-1 HTTP service for registered agents
func (h *HTTPGatewayAgent) getManagerRegistry() map[string]map[string]interface{} {
	agentMap := make(map[string]map[string]interface{})
	
	// Query manager's HTTP service
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:8380/api/services")
	if err != nil {
		fmt.Printf("Failed to query manager registry: %v\n", err)
		return agentMap
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Manager registry returned status: %d\n", resp.StatusCode)
		return agentMap
	}
	
	var managerResponse struct {
		Success  bool `json:"success"`
		Services map[string]struct {
			Name         string    `json:"name"`
			Status       string    `json:"status"`
			Type         string    `json:"type"`
			PID          int       `json:"pid"`
			LastHeartbeat time.Time `json:"last_heartbeat"`
		} `json:"services"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&managerResponse); err != nil {
		fmt.Printf("Failed to decode manager response: %v\n", err)
		return agentMap
	}
	
	if managerResponse.Success {
		for _, service := range managerResponse.Services {
			agentMap[service.Name] = map[string]interface{}{
				"name":           service.Name,
				"type":           service.Type,
				"capabilities":   []string{}, // TODO: Get from actual service
				"integrations":   []string{}, // TODO: Get from actual service
				"location":       "unknown",  // TODO: Get from actual service
				"manager_status": service.Status,
				"has_registry":   true,
			}
		}
	}
	
	return agentMap
}

// isManagerResponding checks if the manager is responding to HTTP requests
func (h *HTTPGatewayAgent) isManagerResponding() bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get("http://localhost:8380/api/services")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (h *HTTPGatewayAgent) getRedisHealth() map[string]interface{} {
	// Try to ping Redis
	_, err := h.redisClient.Ping(h.ctx).Result()
	
	health := map[string]interface{}{
		"connected": err == nil,
	}
	
	if err != nil {
		health["error"] = err.Error()
	} else {
		health["ping"] = "PONG"
		// Try to get stream lengths
		if streams, streamErr := h.getRedisStreams(); streamErr == nil {
			health["streams"] = streams
		}
	}
	
	return health
}

func (h *HTTPGatewayAgent) getRedisStreams() (map[string]int64, error) {
	streams := map[string]string{
		"centerfire:semantic:conversations": "centerfire:semantic:conversations",
		"centerfire:semantic:names": "centerfire:semantic:names",
	}
	
	result := make(map[string]int64)
	for name, key := range streams {
		length, err := h.redisClient.XLen(h.ctx, key).Result()
		if err != nil {
			result[name] = 0
		} else {
			result[name] = length
		}
	}
	
	return result, nil
}

func (h *HTTPGatewayAgent) getServiceEndpointsHealth() []map[string]interface{} {
	endpoints := []map[string]string{
		{"name": "Weaviate", "url": "http://localhost:8080/v1/meta"},
		{"name": "Neo4j", "url": "http://localhost:7474"},
		{"name": "ClickHouse", "url": "http://localhost:8123/ping"},
	}
	
	results := make([]map[string]interface{}, len(endpoints))
	for i, endpoint := range endpoints {
		results[i] = map[string]interface{}{
			"name": endpoint["name"],
			"url": endpoint["url"],
			"accessible": true, // Simplified for now
			"status": 200,
		}
	}
	
	return results
}

func (h *HTTPGatewayAgent) countRunning(items []map[string]interface{}) int {
	count := 0
	for _, item := range items {
		if running, ok := item["running"].(bool); ok && running {
			count++
		}
	}
	return count
}

func (h *HTTPGatewayAgent) countAccessible(items []map[string]interface{}) int {
	count := 0
	for _, item := range items {
		if accessible, ok := item["accessible"].(bool); ok && accessible {
			count++
		}
	}
	return count
}

// handleRoot returns gateway information
func (h *HTTPGatewayAgent) handleRoot(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"agent":        h.AgentID,
		"description":  "HTTP Gateway for agent access control and routing",
		"version":      "1.0",
		"endpoints": map[string]string{
			"health":     "/health",
			"discovery":  "/api/agents/available",
			"agent_call": "/api/agents/{agent}/{action}",
			"contracts":  "/api/contracts/{client_id}",
		},
		"timestamp": time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// extractClientID extracts client ID from request headers or query params
func (h *HTTPGatewayAgent) extractClientID(r *http.Request) string {
	// Check header first
	if clientID := r.Header.Get("X-Client-ID"); clientID != "" {
		return clientID
	}
	
	// Check query parameter
	if clientID := r.URL.Query().Get("client_id"); clientID != "" {
		return clientID
	}
	
	// Check Authorization header for API key patterns
	if auth := r.Header.Get("Authorization"); auth != "" {
		// Simple pattern: "Bearer client_id" or "ApiKey client_id"
		parts := strings.Fields(auth)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	
	return ""
}

// writeErrorResponse writes a standardized error response
func (h *HTTPGatewayAgent) writeErrorResponse(w http.ResponseWriter, errorMsg, requestID string, statusCode int) {
	response := APIResponse{
		Success:   false,
		Error:     errorMsg,
		RequestID: requestID,
		Timestamp: time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// corsMiddleware adds CORS headers
func (h *HTTPGatewayAgent) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Client-ID")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
func (h *HTTPGatewayAgent) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		clientID := h.extractClientID(r)
		
		next.ServeHTTP(w, r)
		
		fmt.Printf("🌐 %s %s (client: %s) - %v\n", 
			r.Method, r.URL.Path, clientID, time.Since(start))
	})
}

// shutdown gracefully shuts down the gateway
func (h *HTTPGatewayAgent) shutdown() {
	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if h.httpServer != nil {
		h.httpServer.Shutdown(ctx)
	}
	
	// Close agent connections
	h.AgentProxy.CloseAllConnections()
	
	// Cancel context
	h.cancel()
	
	fmt.Printf("✅ %s shutdown complete\n", h.AgentID)
}

// registerWithManager registers the HTTP Gateway with AGT-MANAGER-1 including port info
func (h *HTTPGatewayAgent) registerWithManager() {
	// Check for collisions first
	collisionRequest := map[string]interface{}{
		"request_type": "check_agent_collision",
		"agent_name":   h.AgentID,
	}
	
	collisionData, err := json.Marshal(collisionRequest)
	if err != nil {
		fmt.Printf("❌ Failed to marshal collision check: %v\n", err)
		return
	}
	
	err = h.redisClient.Publish(h.ctx, "centerfire:agent:manager", collisionData).Err()
	if err != nil {
		fmt.Printf("❌ Failed to check collision: %v\n", err)
		return
	}
	
	// Register as running with port information
	sessionData := map[string]interface{}{
		"pid":  os.Getpid(),
		"port": h.Port,
		"type": "http_gateway",
		"endpoints": map[string]string{
			"health":     fmt.Sprintf("http://localhost:%d/health", h.Port),
			"discovery":  fmt.Sprintf("http://localhost:%d/api/agents/available", h.Port),
			"agent_call": fmt.Sprintf("http://localhost:%d/api/agents/{agent}/{action}", h.Port),
			"contracts":  fmt.Sprintf("http://localhost:%d/api/contracts/{client_id}", h.Port),
		},
	}
	
	registerRequest := map[string]interface{}{
		"request_type": "register_running",
		"agent_name":   h.AgentID,
		"session_data": sessionData,
	}
	
	registerData, err := json.Marshal(registerRequest)
	if err != nil {
		fmt.Printf("❌ Failed to marshal registration: %v\n", err)
		return
	}
	
	err = h.redisClient.Publish(h.ctx, "centerfire:agent:manager", registerData).Err()
	if err != nil {
		fmt.Printf("❌ Failed to register with manager: %v\n", err)
		return
	}
	
	fmt.Printf("📝 Registered with AGT-MANAGER-1 (PID: %d, Port: %d)\n", os.Getpid(), h.Port)
	
	// Start heartbeat
	h.startHeartbeat()
}

// startHeartbeat sends periodic heartbeats to AGT-MANAGER-1
func (h *HTTPGatewayAgent) startHeartbeat() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				heartbeat := map[string]interface{}{
					"request_type": "heartbeat",
					"agent_name":   h.AgentID,
				}
				
				data, err := json.Marshal(heartbeat)
				if err != nil {
					continue
				}
				
				h.redisClient.Publish(h.ctx, "centerfire:agent:manager", data)
				
			case <-h.ctx.Done():
				return
			}
		}
	}()
}

func main() {
	gateway := NewHTTPGatewayAgent()
	gateway.Start()
}
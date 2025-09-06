// Socket-Based Multi-Interface Orchestrator
// Decouples agents from Claude Code, enables multi-interface support

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// AgentPool manages connections to socket-based agents
type AgentPool struct {
	mu      sync.RWMutex
	agents  map[string]*AgentConnection
}

// AgentConnection represents a connection to a socket-based agent
type AgentConnection struct {
	Name   string
	Socket net.Conn
	Active bool
}

// LLMProvider represents a language model provider
type LLMProvider struct {
	Name           string    `json:"name"`
	CostPer1M      float64   `json:"cost_per_1m"`      // Cost per 1M tokens
	MaxContext     int       `json:"max_context"`      // Maximum context window
	LatencyMS      int       `json:"latency_ms"`       // Average response latency
	Quality        float64   `json:"quality"`          // Quality score 0-1
	Available      bool      `json:"available"`        // Provider availability
	Capabilities   []string  `json:"capabilities"`     // ["coding", "reasoning", "creative", etc]
	LastHealth     time.Time `json:"last_health"`      // Last health check
}

// LLMRouter handles intelligent routing to optimal LLM providers
type LLMRouter struct {
	providers    map[string]*LLMProvider
	dailyBudget  float64
	currentSpend float64
	mu           sync.RWMutex
}

// RoutingRequest contains context for LLM routing decisions
type RoutingRequest struct {
	TokenCount   int      `json:"token_count"`
	TaskType     string   `json:"task_type"`     // "coding", "reasoning", "creative", etc
	Priority     string   `json:"priority"`      // "high", "medium", "low"
	MaxLatency   int      `json:"max_latency"`   // Maximum acceptable latency (ms)
	Interface    string   `json:"interface"`     // Request source context
}

// RoutingDecision contains the selected provider and reasoning
type RoutingDecision struct {
	Provider   string  `json:"provider"`
	Reasoning  string  `json:"reasoning"`
	Cost       float64 `json:"estimated_cost"`
	Confidence float64 `json:"confidence"`
}

// Orchestrator coordinates between interfaces and agents
type Orchestrator struct {
	agentPool    *AgentPool
	httpServer   *http.Server
	wsUpgrader   websocket.Upgrader
	llmRouter    *LLMRouter
	ctx          context.Context
	cancel       context.CancelFunc
}

// Request represents a request from any interface
type Request struct {
	ID        string                 `json:"id"`
	Interface string                 `json:"interface"` // "claude-code", "web", "api"
	Agent     string                 `json:"agent"`
	Action    string                 `json:"action"`
	Data      map[string]interface{} `json:"data"`
}

// Response represents a response to any interface
type Response struct {
	ID        string                 `json:"id"`
	Success   bool                   `json:"success"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

func main() {
	orchestrator := NewOrchestrator()
	
	// Start orchestrator services
	if err := orchestrator.Start(); err != nil {
		log.Fatalf("Failed to start orchestrator: %v", err)
	}
	
	// Wait for shutdown signal
	orchestrator.WaitForShutdown()
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator() *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Orchestrator{
		agentPool: &AgentPool{
			agents: make(map[string]*AgentConnection),
		},
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
		llmRouter: newLLMRouter(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start initializes all orchestrator services
func (o *Orchestrator) Start() error {
	log.Println("🚀 Starting Socket-Based Multi-Interface Orchestrator")
	
	// Start agent socket listeners
	go o.startAgentSocketListeners()
	
	// Start HTTP/WebSocket server for interfaces
	go o.startHTTPServer()
	
	// Start intelligent LLM router
	go o.startLLMRouter()
	
	log.Println("✅ Orchestrator started successfully")
	log.Println("📡 HTTP Server: http://localhost:8090")
	log.Println("🔌 Agent Sockets: /tmp/orchestrator-*.sock")
	
	return nil
}

// startAgentSocketListeners creates Unix domain socket listeners for agents
func (o *Orchestrator) startAgentSocketListeners() {
	agents := []string{"naming", "struct", "semantic", "manager"}
	
	for _, agent := range agents {
		go func(agentName string) {
			socketPath := fmt.Sprintf("/tmp/orchestrator-%s.sock", agentName)
			
			// Remove existing socket file
			os.Remove(socketPath)
			
			listener, err := net.Listen("unix", socketPath)
			if err != nil {
				log.Printf("❌ Failed to create socket for %s: %v", agentName, err)
				return
			}
			defer listener.Close()
			
			log.Printf("🔌 Agent %s socket listening: %s", agentName, socketPath)
			
			for {
				conn, err := listener.Accept()
				if err != nil {
					if o.ctx.Err() != nil {
						return // Context cancelled
					}
					log.Printf("❌ Socket accept error for %s: %v", agentName, err)
					continue
				}
				
				// Register agent connection
				o.agentPool.mu.Lock()
				o.agentPool.agents[agentName] = &AgentConnection{
					Name:   agentName,
					Socket: conn,
					Active: true,
				}
				o.agentPool.mu.Unlock()
				
				log.Printf("✅ Agent %s connected via socket", agentName)
				
				// Handle agent communication in goroutine
				go o.handleAgentConnection(agentName, conn)
			}
		}(agent)
	}
}

// startHTTPServer starts HTTP server for web interfaces and API endpoints
func (o *Orchestrator) startHTTPServer() {
	mux := http.NewServeMux()
	
	// WebSocket endpoint for real-time web interface
	mux.HandleFunc("/ws", o.handleWebSocket)
	
	// REST API endpoint for Claude Code and other clients
	mux.HandleFunc("/api/request", o.handleAPIRequest)
	
	// Health check endpoint
	mux.HandleFunc("/health", o.handleHealth)
	
	// LLM routing endpoint
	mux.HandleFunc("/api/route-llm", o.handleLLMRoute)
	
	// Serve static web interface (future)
	mux.Handle("/", http.FileServer(http.Dir("./web/")))
	
	o.httpServer = &http.Server{
		Addr:    ":8090",
		Handler: mux,
	}
	
	log.Println("🌐 Starting HTTP/WebSocket server on :8090")
	if err := o.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("❌ HTTP server error: %v", err)
	}
}

// startLLMRouter implements intelligent LLM routing
func (o *Orchestrator) startLLMRouter() {
	log.Println("🧠 Starting intelligent LLM router")
	
	// Initialize provider health checking
	go o.llmRouter.startHealthMonitoring()
	
	log.Printf("📊 LLM Router initialized with %d providers", len(o.llmRouter.providers))
	log.Printf("💰 Daily budget: $%.2f", o.llmRouter.dailyBudget)
}

// newLLMRouter creates a new LLM router with default providers
func newLLMRouter() *LLMRouter {
	return &LLMRouter{
		providers: map[string]*LLMProvider{
			"claude-sonnet-4": {
				Name:         "Claude Sonnet 4",
				CostPer1M:    15.0,
				MaxContext:   200000,
				LatencyMS:    2000,
				Quality:      0.95,
				Available:    true,
				Capabilities: []string{"coding", "reasoning", "creative", "analysis"},
				LastHealth:   time.Now(),
			},
			"gpt-4-turbo": {
				Name:         "GPT-4 Turbo",
				CostPer1M:    10.0,
				MaxContext:   128000,
				LatencyMS:    1500,
				Quality:      0.90,
				Available:    true,
				Capabilities: []string{"coding", "reasoning", "creative"},
				LastHealth:   time.Now(),
			},
			"gemini-pro": {
				Name:         "Gemini Pro",
				CostPer1M:    7.0,
				MaxContext:   32000,
				LatencyMS:    1200,
				Quality:      0.85,
				Available:    true,
				Capabilities: []string{"reasoning", "creative", "analysis"},
				LastHealth:   time.Now(),
			},
			"local-llm": {
				Name:         "Local Llama",
				CostPer1M:    0.0,
				MaxContext:   8000,
				LatencyMS:    800,
				Quality:      0.75,
				Available:    true,
				Capabilities: []string{"coding", "reasoning"},
				LastHealth:   time.Now(),
			},
		},
		dailyBudget:  100.0, // $100/day budget
		currentSpend: 0.0,
	}
}

// RouteRequest intelligently selects the best LLM provider for a request
func (r *LLMRouter) RouteRequest(req RoutingRequest) RoutingDecision {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidates []*LLMProvider
	var scores []float64

	// Filter available providers that can handle the request
	for _, provider := range r.providers {
		if !provider.Available {
			continue
		}
		if req.TokenCount > provider.MaxContext {
			continue
		}
		
		// Check capability match
		if !r.hasCapability(provider, req.TaskType) {
			continue
		}

		score := r.calculateScore(provider, req)
		candidates = append(candidates, provider)
		scores = append(scores, score)
	}

	if len(candidates) == 0 {
		return RoutingDecision{
			Provider:   "claude-sonnet-4", // Fallback to most capable
			Reasoning:  "No suitable providers found, using fallback",
			Cost:       float64(req.TokenCount) * 15.0 / 1000000,
			Confidence: 0.5,
		}
	}

	// Find best candidate
	bestIdx := 0
	bestScore := scores[0]
	for i, score := range scores {
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	selectedProvider := candidates[bestIdx]
	estimatedCost := float64(req.TokenCount) * selectedProvider.CostPer1M / 1000000

	return RoutingDecision{
		Provider:   selectedProvider.Name,
		Reasoning:  r.generateReasoning(selectedProvider, req, bestScore),
		Cost:       estimatedCost,
		Confidence: bestScore,
	}
}

// calculateScore computes a multi-factor score for provider selection
func (r *LLMRouter) calculateScore(provider *LLMProvider, req RoutingRequest) float64 {
	// Base quality score (0-1)
	score := provider.Quality

	// Cost factor - prefer cheaper options but with diminishing returns
	costFactor := 1.0
	if provider.CostPer1M > 0 {
		remainingBudget := r.dailyBudget - r.currentSpend
		requestCost := float64(req.TokenCount) * provider.CostPer1M / 1000000
		
		if requestCost > remainingBudget {
			costFactor = 0.1 // Heavily penalize budget-exceeding options
		} else {
			// Prefer cost-effective options: higher cost = lower factor
			costFactor = 1.0 - (provider.CostPer1M / 20.0) // Normalize against $20/1M max
		}
	} else {
		costFactor = 1.2 // Bonus for free providers
	}

	// Latency factor - prefer faster responses
	latencyFactor := 1.0
	if req.MaxLatency > 0 && provider.LatencyMS > req.MaxLatency {
		latencyFactor = 0.3 // Heavy penalty for exceeding latency requirements
	} else {
		latencyFactor = 1.0 - (float64(provider.LatencyMS) / 5000.0) // Normalize against 5s max
	}

	// Priority factor - adjust based on request priority
	priorityFactor := 1.0
	switch req.Priority {
	case "high":
		// High priority: prefer quality over cost
		score *= 1.2
		costFactor *= 0.8
	case "low":
		// Low priority: prefer cost over quality
		score *= 0.9
		costFactor *= 1.3
	}

	// Context efficiency - bonus for providers that can handle large contexts
	contextFactor := 1.0
	if req.TokenCount > provider.MaxContext/2 {
		contextFactor = 0.8 // Slight penalty for near-capacity usage
	}

	// Combine all factors
	finalScore := score * costFactor * latencyFactor * priorityFactor * contextFactor

	// Ensure score is between 0 and 1
	if finalScore > 1.0 {
		finalScore = 1.0
	}
	if finalScore < 0 {
		finalScore = 0
	}

	return finalScore
}

// hasCapability checks if provider supports the required task type
func (r *LLMRouter) hasCapability(provider *LLMProvider, taskType string) bool {
	if taskType == "" {
		return true // No specific requirement
	}
	
	for _, capability := range provider.Capabilities {
		if capability == taskType {
			return true
		}
	}
	return false
}

// generateReasoning creates human-readable explanation for routing decision
func (r *LLMRouter) generateReasoning(provider *LLMProvider, req RoutingRequest, score float64) string {
	factors := []string{}
	
	if score > 0.8 {
		factors = append(factors, "high quality match")
	}
	
	remainingBudget := r.dailyBudget - r.currentSpend
	requestCost := float64(req.TokenCount) * provider.CostPer1M / 1000000
	if requestCost < remainingBudget*0.1 {
		factors = append(factors, "cost-effective")
	}
	
	if req.MaxLatency > 0 && provider.LatencyMS < req.MaxLatency {
		factors = append(factors, "meets latency requirements")
	}
	
	if r.hasCapability(provider, req.TaskType) {
		factors = append(factors, fmt.Sprintf("supports %s tasks", req.TaskType))
	}
	
	if len(factors) == 0 {
		return "best available option"
	}
	
	return fmt.Sprintf("Selected for: %s", strings.Join(factors, ", "))
}

// startHealthMonitoring periodically checks provider health
func (r *LLMRouter) startHealthMonitoring() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		r.checkProviderHealth()
	}
}

// checkProviderHealth updates provider availability status
func (r *LLMRouter) checkProviderHealth() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for name, provider := range r.providers {
		// Simple health check - in production, this would make actual API calls
		provider.Available = time.Since(provider.LastHealth) < 10*time.Minute
		provider.LastHealth = time.Now()
		
		if !provider.Available {
			log.Printf("⚠️ Provider %s marked as unavailable", name)
		}
	}
}

// TrackSpending updates current daily spending
func (r *LLMRouter) TrackSpending(cost float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.currentSpend += cost
	
	if r.currentSpend > r.dailyBudget*0.8 {
		log.Printf("⚠️ LLM spending at %.1f%% of daily budget ($%.2f/$%.2f)", 
			(r.currentSpend/r.dailyBudget)*100, r.currentSpend, r.dailyBudget)
	}
}

// GetSpendingStatus returns current spending information
func (r *LLMRouter) GetSpendingStatus() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	return map[string]interface{}{
		"daily_budget":    r.dailyBudget,
		"current_spend":   r.currentSpend,
		"remaining":       r.dailyBudget - r.currentSpend,
		"utilization_pct": (r.currentSpend / r.dailyBudget) * 100,
	}
}

// handleWebSocket manages WebSocket connections for web interface
func (o *Orchestrator) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := o.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()
	
	log.Println("🌐 WebSocket client connected")
	
	for {
		var req Request
		if err := conn.ReadJSON(&req); err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("🌐 WebSocket client disconnected")
				return
			}
			log.Printf("❌ WebSocket read error: %v", err)
			return
		}
		
		req.Interface = "web"
		response := o.processRequest(req)
		
		if err := conn.WriteJSON(response); err != nil {
			log.Printf("❌ WebSocket write error: %v", err)
			return
		}
	}
}

// handleAPIRequest processes HTTP API requests (for Claude Code, etc.)
func (o *Orchestrator) handleAPIRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	req.Interface = "api"
	response := o.processRequest(req)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth provides health check endpoint
func (o *Orchestrator) handleHealth(w http.ResponseWriter, r *http.Request) {
	o.agentPool.mu.RLock()
	activeAgents := len(o.agentPool.agents)
	o.agentPool.mu.RUnlock()
	
	status := map[string]interface{}{
		"status":       "healthy",
		"agents":       activeAgents,
		"timestamp":    time.Now(),
		"interfaces":   []string{"websocket", "http", "unix-sockets"},
		"llm_router":   o.llmRouter.GetSpendingStatus(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleLLMRoute processes LLM routing requests
func (o *Orchestrator) handleLLMRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req RoutingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// Apply defaults if not specified
	if req.Priority == "" {
		req.Priority = "medium"
	}
	if req.TaskType == "" {
		req.TaskType = "reasoning"
	}
	if req.Interface == "" {
		req.Interface = "api"
	}
	
	// Get routing decision
	decision := o.llmRouter.RouteRequest(req)
	
	// Track spending
	o.llmRouter.TrackSpending(decision.Cost)
	
	log.Printf("🎯 LLM Route: %s -> %s (cost: $%.4f, confidence: %.2f)", 
		req.TaskType, decision.Provider, decision.Cost, decision.Confidence)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decision)
}

// processRequest routes requests to appropriate agents
func (o *Orchestrator) processRequest(req Request) Response {
	log.Printf("📥 Processing request: %s -> %s.%s", req.Interface, req.Agent, req.Action)
	
	// Find target agent
	o.agentPool.mu.RLock()
	agent, exists := o.agentPool.agents[req.Agent]
	o.agentPool.mu.RUnlock()
	
	if !exists || !agent.Active {
		return Response{
			ID:        req.ID,
			Success:   false,
			Error:     fmt.Sprintf("Agent %s not available", req.Agent),
			Timestamp: time.Now(),
		}
	}
	
	// Forward request to agent via socket
	requestData, _ := json.Marshal(req)
	_, err := agent.Socket.Write(requestData)
	if err != nil {
		return Response{
			ID:        req.ID,
			Success:   false,
			Error:     fmt.Sprintf("Failed to communicate with agent %s: %v", req.Agent, err),
			Timestamp: time.Now(),
		}
	}
	
	// TODO: Read response from agent socket
	// For now, return success
	return Response{
		ID:      req.ID,
		Success: true,
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Request forwarded to %s", req.Agent),
			"agent":   req.Agent,
			"action":  req.Action,
		},
		Timestamp: time.Now(),
	}
}

// handleAgentConnection manages communication with a connected agent
func (o *Orchestrator) handleAgentConnection(agentName string, conn net.Conn) {
	defer conn.Close()
	
	// Handle agent responses and keep connection alive
	buffer := make([]byte, 4096)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("❌ Agent %s disconnected: %v", agentName, err)
			break
		}
		
		// Process agent response
		log.Printf("📤 Agent %s response: %s", agentName, string(buffer[:n]))
	}
	
	// Mark agent as inactive
	o.agentPool.mu.Lock()
	if agent, exists := o.agentPool.agents[agentName]; exists {
		agent.Active = false
	}
	o.agentPool.mu.Unlock()
}

// WaitForShutdown waits for shutdown signal and gracefully stops services
func (o *Orchestrator) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	<-sigChan
	log.Println("🛑 Shutdown signal received")
	
	// Cancel context
	o.cancel()
	
	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if o.httpServer != nil {
		o.httpServer.Shutdown(ctx)
	}
	
	// Close agent connections
	o.agentPool.mu.Lock()
	for _, agent := range o.agentPool.agents {
		if agent.Socket != nil {
			agent.Socket.Close()
		}
	}
	o.agentPool.mu.Unlock()
	
	// Clean up socket files
	agents := []string{"naming", "struct", "semantic", "manager"}
	for _, agent := range agents {
		os.Remove(fmt.Sprintf("/tmp/orchestrator-%s.sock", agent))
	}
	
	log.Println("✅ Orchestrator shutdown complete")
}
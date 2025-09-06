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

// Orchestrator coordinates between interfaces and agents
type Orchestrator struct {
	agentPool    *AgentPool
	httpServer   *http.Server
	wsUpgrader   websocket.Upgrader
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
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start initializes all orchestrator services
func (o *Orchestrator) Start() error {
	log.Println("🚀 Starting Socket-Based Multi-Interface Orchestrator")
	
	// Start agent socket listeners
	go o.startAgentSocketListeners()
	
	// Start HTTP/WebSocket server for interfaces
	go o.startHTTPServer()
	
	// Start cost-aware LLM router
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

// startLLMRouter implements cost-aware LLM routing
func (o *Orchestrator) startLLMRouter() {
	log.Println("🧠 Starting cost-aware LLM router")
	
	// TODO: Implement cost matrix and routing logic
	// - Claude API: $15/1M tokens (high quality)
	// - GPT-4: $10/1M tokens (balanced)
	// - Gemini: $7/1M tokens (cost-effective)
	// - Local LLM: $0/1M tokens (free, lower quality)
	
	// This will be expanded with actual routing logic
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
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
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
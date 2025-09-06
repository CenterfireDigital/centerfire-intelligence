package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

// SessionData represents the structure of session data
type SessionData struct {
	Namespace    string            `json:"namespace"`
	SessionType  string            `json:"session_type"`
	StartTime    string            `json:"start_time"`
	User         string            `json:"user"`
	Context      string            `json:"context"`
	Progress     []string          `json:"progress"`
	AgentsActive []string          `json:"agents_active"`
	Metadata     map[string]interface{} `json:"metadata"`
	SessionID    string            `json:"session_id,omitempty"`
}

// NamingRequest represents a request to AGT-NAMING-1 for session ID allocation
type NamingRequest struct {
	From   string                 `json:"from"`
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params"`
}

// SessionManager manages Redis-based sessions
type SessionManager struct {
	redisClient   *redis.Client
	ctx           context.Context
	httpServer    *http.Server
	requestID     int
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	// Connect to Redis mem0-redis:6380
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380", // mem0-redis container port
		Password: "",
		DB:       0,
	})

	return &SessionManager{
		redisClient: rdb,
		ctx:         context.Background(),
		requestID:   0,
	}
}

// Start initializes the session manager
func (sm *SessionManager) Start() error {
	// Test Redis connection
	_, err := sm.redisClient.Ping(sm.ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}
	fmt.Println("Connected to Redis successfully")

	return nil
}

// requestSessionID requests a session ID from AGT-NAMING-1
func (sm *SessionManager) requestSessionID(sessionType, context string) (string, error) {
	sm.requestID++
	
	// Prepare request for AGT-NAMING-1
	request := NamingRequest{
		From:   "session-manager",
		Action: "allocate_session",
		Params: map[string]interface{}{
			"domain":  "SESSION",
			"type":    sessionType,
			"context": context,
			"request_id": fmt.Sprintf("req_%d", sm.requestID),
		},
	}

	// Subscribe to response channel first
	pubsub := sm.redisClient.Subscribe(sm.ctx, "agent.naming.response")
	defer pubsub.Close()

	// Send request to AGT-NAMING-1
	requestData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	err = sm.redisClient.Publish(sm.ctx, "agent.naming.request", requestData).Err()
	if err != nil {
		return "", fmt.Errorf("failed to send request to AGT-NAMING-1: %v", err)
	}

	fmt.Printf("Sent session ID request to AGT-NAMING-1 for type: %s\n", sessionType)

	// Wait for response (with timeout)
	ch := pubsub.Channel()
	timeout := time.After(10 * time.Second)
	
	for {
		select {
		case msg := <-ch:
			var response map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &response); err != nil {
				continue
			}
			
			// Check if this response is for our request
			if reqID, ok := response["request_id"].(string); ok && 
			   reqID == fmt.Sprintf("req_%d", sm.requestID) {
				if sessionID, ok := response["session_id"].(string); ok {
					fmt.Printf("Received session ID from AGT-NAMING-1: %s\n", sessionID)
					return sessionID, nil
				}
			}
		case <-timeout:
			// If AGT-NAMING-1 is not responding, generate a fallback ID
			fallbackID := fmt.Sprintf("SES-CLAUDE-%d-%d", time.Now().Unix(), sm.requestID)
			fmt.Printf("AGT-NAMING-1 timeout, using fallback session ID: %s\n", fallbackID)
			return fallbackID, nil
		}
	}
}

// createSession creates a new session with name allocation from AGT-NAMING-1
func (sm *SessionManager) createSession(sessionType, context string, metadata map[string]interface{}) (*SessionData, error) {
	// Request session ID from AGT-NAMING-1
	sessionID, err := sm.requestSessionID(sessionType, context)
	if err != nil {
		return nil, fmt.Errorf("failed to get session ID: %v", err)
	}

	// Create session data
	session := &SessionData{
		Namespace:    "centerfire.dev",
		SessionType:  sessionType,
		StartTime:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		User:         "larrydiffey", // Could be made configurable
		Context:      context,
		Progress:     []string{},
		AgentsActive: []string{"AGT-NAMING-1"},
		Metadata:     metadata,
		SessionID:    sessionID,
	}

	// Store in Redis with semantic namespace
	redisKey := fmt.Sprintf("centerfire.dev.session:claude:%s", sessionID)
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session: %v", err)
	}

	err = sm.redisClient.Set(sm.ctx, redisKey, sessionJSON, 24*time.Hour).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to store session in Redis: %v", err)
	}

	fmt.Printf("Created session: %s\n", sessionID)
	return session, nil
}

// updateSession updates an existing session
func (sm *SessionManager) updateSession(sessionID string, progress []string, state map[string]interface{}) error {
	redisKey := fmt.Sprintf("centerfire.dev.session:claude:%s", sessionID)
	
	// Get existing session
	sessionJSON, err := sm.redisClient.Get(sm.ctx, redisKey).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("failed to get session: %v", err)
	}

	// Parse existing session
	var session SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return fmt.Errorf("failed to parse session data: %v", err)
	}

	// Update progress
	if progress != nil {
		session.Progress = append(session.Progress, progress...)
	}

	// Update metadata with state
	if state != nil {
		for k, v := range state {
			session.Metadata[k] = v
		}
	}

	// Store updated session
	updatedJSON, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal updated session: %v", err)
	}

	err = sm.redisClient.Set(sm.ctx, redisKey, updatedJSON, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to update session in Redis: %v", err)
	}

	fmt.Printf("Updated session: %s\n", sessionID)
	return nil
}

// getSession retrieves a session by ID
func (sm *SessionManager) getSession(sessionID string) (*SessionData, error) {
	redisKey := fmt.Sprintf("centerfire.dev.session:claude:%s", sessionID)
	
	sessionJSON, err := sm.redisClient.Get(sm.ctx, redisKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get session: %v", err)
	}

	var session SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %v", err)
	}

	return &session, nil
}

// expireSession sets TTL for session cleanup
func (sm *SessionManager) expireSession(sessionID string, ttl time.Duration) error {
	redisKey := fmt.Sprintf("centerfire.dev.session:claude:%s", sessionID)
	
	err := sm.redisClient.Expire(sm.ctx, redisKey, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration for session %s: %v", sessionID, err)
	}

	fmt.Printf("Set expiration for session %s: %v\n", sessionID, ttl)
	return nil
}

// listActiveSessions gets all active sessions
func (sm *SessionManager) listActiveSessions() ([]*SessionData, error) {
	pattern := "centerfire.dev.session:claude:*"
	keys, err := sm.redisClient.Keys(sm.ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session keys: %v", err)
	}

	sessions := make([]*SessionData, 0, len(keys))
	for _, key := range keys {
		sessionJSON, err := sm.redisClient.Get(sm.ctx, key).Result()
		if err != nil {
			continue // Skip keys that might have expired
		}

		var session SessionData
		if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
			continue // Skip invalid session data
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// HTTP API Handlers

func (sm *SessionManager) createSessionHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionType string                 `json:"session_type"`
		Context     string                 `json:"context"`
		Metadata    map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.SessionType == "" {
		req.SessionType = "claude_coding"
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}

	session, err := sm.createSession(req.SessionType, req.Context, req.Metadata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (sm *SessionManager) getSessionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	session, err := sm.getSession(sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (sm *SessionManager) updateSessionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	var req struct {
		Progress []string               `json:"progress"`
		State    map[string]interface{} `json:"state"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := sm.updateSession(sessionID, req.Progress, req.State)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (sm *SessionManager) expireSessionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	ttlStr := r.URL.Query().Get("ttl")
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		ttl = 1 * time.Hour // Default TTL
	}

	err = sm.expireSession(sessionID, ttl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "expired"})
}

func (sm *SessionManager) listSessionsHandler(w http.ResponseWriter, r *http.Request) {
	sessions, err := sm.listActiveSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func (sm *SessionManager) healthHandler(w http.ResponseWriter, r *http.Request) {
	// Check Redis connection
	_, err := sm.redisClient.Ping(sm.ctx).Result()
	if err != nil {
		http.Error(w, "Redis connection failed", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"redis":  "connected",
	})
}

// startHTTPServer starts the HTTP API server
func (sm *SessionManager) startHTTPServer() {
	router := mux.NewRouter()

	// API routes
	router.HandleFunc("/health", sm.healthHandler).Methods("GET")
	router.HandleFunc("/session", sm.createSessionHandler).Methods("POST")
	router.HandleFunc("/session/{id}", sm.getSessionHandler).Methods("GET")
	router.HandleFunc("/session/{id}", sm.updateSessionHandler).Methods("PUT")
	router.HandleFunc("/session/{id}", sm.expireSessionHandler).Methods("DELETE")
	router.HandleFunc("/sessions", sm.listSessionsHandler).Methods("GET")

	sm.httpServer = &http.Server{
		Addr:    ":8083",
		Handler: router,
	}

	fmt.Println("HTTP API server starting on :8083")
	if err := sm.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

// CLI Interface Functions

func (sm *SessionManager) handleCLI() {
	if len(os.Args) < 2 {
		sm.printUsage()
		return
	}

	command := os.Args[1]
	switch command {
	case "create":
		sm.handleCreateCLI()
	case "update":
		sm.handleUpdateCLI()
	case "get":
		sm.handleGetCLI()
	case "list":
		sm.handleListCLI()
	case "expire":
		sm.handleExpireCLI()
	case "server":
		sm.startHTTPServer()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		sm.printUsage()
	}
}

func (sm *SessionManager) printUsage() {
	fmt.Println("Session Manager CLI Usage:")
	fmt.Println("  create --type=<type> --context=<context>    Create new session")
	fmt.Println("  update --id=<id> --progress=<task>          Update session progress")
	fmt.Println("  get --id=<id>                               Get session data")
	fmt.Println("  list                                        List active sessions")
	fmt.Println("  expire --id=<id> --ttl=<duration>          Set session expiration")
	fmt.Println("  server                                      Start HTTP API server")
}

func (sm *SessionManager) handleCreateCLI() {
	sessionType := "claude_coding"
	context := "CLI session"

	// Parse CLI flags
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--type=") {
			sessionType = strings.TrimPrefix(arg, "--type=")
		} else if strings.HasPrefix(arg, "--context=") {
			context = strings.TrimPrefix(arg, "--context=")
		}
	}

	session, err := sm.createSession(sessionType, context, map[string]interface{}{
		"created_via": "cli",
	})
	if err != nil {
		fmt.Printf("Error creating session: %v\n", err)
		return
	}

	fmt.Printf("Created session: %s\n", session.SessionID)
	sessionJSON, _ := json.MarshalIndent(session, "", "  ")
	fmt.Println(string(sessionJSON))
}

func (sm *SessionManager) handleUpdateCLI() {
	var sessionID, progressItem string

	// Parse CLI flags
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--id=") {
			sessionID = strings.TrimPrefix(arg, "--id=")
		} else if strings.HasPrefix(arg, "--progress=") {
			progressItem = strings.TrimPrefix(arg, "--progress=")
		}
	}

	if sessionID == "" {
		fmt.Println("Error: --id is required")
		return
	}

	progress := []string{}
	if progressItem != "" {
		progress = []string{progressItem}
	}

	err := sm.updateSession(sessionID, progress, map[string]interface{}{
		"updated_via": "cli",
		"updated_at":  time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	})
	if err != nil {
		fmt.Printf("Error updating session: %v\n", err)
		return
	}

	fmt.Printf("Updated session: %s\n", sessionID)
}

func (sm *SessionManager) handleGetCLI() {
	var sessionID string

	// Parse CLI flags
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--id=") {
			sessionID = strings.TrimPrefix(arg, "--id=")
		}
	}

	if sessionID == "" {
		fmt.Println("Error: --id is required")
		return
	}

	session, err := sm.getSession(sessionID)
	if err != nil {
		fmt.Printf("Error getting session: %v\n", err)
		return
	}

	sessionJSON, _ := json.MarshalIndent(session, "", "  ")
	fmt.Println(string(sessionJSON))
}

func (sm *SessionManager) handleListCLI() {
	sessions, err := sm.listActiveSessions()
	if err != nil {
		fmt.Printf("Error listing sessions: %v\n", err)
		return
	}

	fmt.Printf("Found %d active sessions:\n", len(sessions))
	for _, session := range sessions {
		fmt.Printf("  %s (%s) - %s\n", session.SessionID, session.SessionType, session.Context)
	}
}

func (sm *SessionManager) handleExpireCLI() {
	var sessionID, ttlStr string

	// Parse CLI flags
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--id=") {
			sessionID = strings.TrimPrefix(arg, "--id=")
		} else if strings.HasPrefix(arg, "--ttl=") {
			ttlStr = strings.TrimPrefix(arg, "--ttl=")
		}
	}

	if sessionID == "" {
		fmt.Println("Error: --id is required")
		return
	}

	ttl := 1 * time.Hour
	if ttlStr != "" {
		var err error
		ttl, err = time.ParseDuration(ttlStr)
		if err != nil {
			fmt.Printf("Error parsing TTL: %v\n", err)
			return
		}
	}

	err := sm.expireSession(sessionID, ttl)
	if err != nil {
		fmt.Printf("Error setting session expiration: %v\n", err)
		return
	}

	fmt.Printf("Set expiration for session %s: %v\n", sessionID, ttl)
}

func main() {
	sm := NewSessionManager()
	
	if err := sm.Start(); err != nil {
		log.Fatalf("Failed to start session manager: %v", err)
	}

	// Handle CLI commands or start server
	if len(os.Args) > 1 {
		sm.handleCLI()
	} else {
		// Start HTTP server by default
		// Set up graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go sm.startHTTPServer()

		// Wait for shutdown signal
		<-sigChan
		fmt.Println("\nShutting down session manager...")
		
		if sm.httpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sm.httpServer.Shutdown(ctx)
		}
	}
}
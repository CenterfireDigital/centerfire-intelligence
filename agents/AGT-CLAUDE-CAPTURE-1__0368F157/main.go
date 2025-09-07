package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

// ClaudeCaptureAgent - Agent for capturing Claude Code sessions and streaming to Redis
type ClaudeCaptureAgent struct {
	AgentID         string
	CID            string
	RequestChannel  string
	ResponseChannel string
	RedisClient    *redis.Client
	ctx            context.Context
	cancel         context.CancelFunc
}

// SessionData - Structure for Claude Code session data
type SessionData struct {
	SessionID   string                 `json:"session_id"`
	Timestamp   time.Time              `json:"timestamp"`
	MessageType string                 `json:"message_type"` // "user_input", "assistant_response", "system_info"
	Content     string                 `json:"content"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ConversationEvent - Structure for conversation events
type ConversationEvent struct {
	EventID     string                 `json:"event_id"`
	SessionID   string                 `json:"session_id"`
	Timestamp   time.Time              `json:"timestamp"`
	EventType   string                 `json:"event_type"` // "session_start", "user_message", "assistant_message", "session_end"
	Data        map[string]interface{} `json:"data"`
	AgentSource string                 `json:"agent_source"`
}

// NewAgent - Create new Claude Code capture agent
func NewAgent() *ClaudeCaptureAgent {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Connect to Redis container on port 6380
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	return &ClaudeCaptureAgent{
		AgentID:         "AGT-CLAUDE-CAPTURE-1",
		CID:            "cid:centerfire:agent:0368F157",
		RequestChannel:  "claude.sessions.request",
		ResponseChannel: "claude.sessions.response",
		RedisClient:    rdb,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start - Start the Claude Code capture agent
func (a *ClaudeCaptureAgent) Start() {
	fmt.Printf("%s starting Claude Code session capture...\n", a.AgentID)
	fmt.Printf("CID: %s\n", a.CID)
	fmt.Printf("Monitoring Claude Code sessions and streaming to Redis\n")
	
	// Test Redis connection
	_, err := a.RedisClient.Ping(a.ctx).Result()
	if err != nil {
		fmt.Printf("Failed to connect to Redis: %v\n", err)
		return
	}
	fmt.Printf("Connected to Redis successfully\n")
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start session capture in goroutine
	go a.startSessionCapture()
	
	// Start Redis request listener in goroutine
	go a.startRedisListener()
	
	fmt.Printf("%s ready - capturing Claude Code sessions\n", a.AgentID)
	
	// Wait for shutdown signal
	<-sigChan
	fmt.Printf("\n%s shutting down...\n", a.AgentID)
	a.cancel()
}

// startSessionCapture - Monitor and capture Claude Code sessions
func (a *ClaudeCaptureAgent) startSessionCapture() {
	fmt.Printf("%s: Starting session capture monitoring\n", a.AgentID)
	
	// Create a session for this capture instance
	sessionID := a.generateSessionID()
	
	// Publish session start event
	a.publishConversationEvent(sessionID, "session_start", map[string]interface{}{
		"agent_id": a.AgentID,
		"cid":      a.CID,
		"started_at": time.Now(),
	})
	
	// Monitor standard input for Claude Code interactions
	go a.monitorStdInput(sessionID)
	
	// Keep the capture running
	for {
		select {
		case <-a.ctx.Done():
			// Publish session end event
			a.publishConversationEvent(sessionID, "session_end", map[string]interface{}{
				"agent_id": a.AgentID,
				"ended_at": time.Now(),
			})
			return
		case <-time.After(30 * time.Second):
			// Periodic heartbeat
			a.publishSessionHeartbeat(sessionID)
		}
	}
}

// monitorStdInput - Monitor standard input for Claude Code interactions
func (a *ClaudeCaptureAgent) monitorStdInput(sessionID string) {
	reader := bufio.NewReader(os.Stdin)
	
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			// Check if there's input available (non-blocking)
			if a.hasInputAvailable() {
				line, err := reader.ReadString('\n')
				if err != nil && err != io.EOF {
					continue
				}
				
				if strings.TrimSpace(line) != "" {
					// Capture user input
					sessionData := SessionData{
						SessionID:   sessionID,
						Timestamp:   time.Now(),
						MessageType: "user_input",
						Content:     strings.TrimSpace(line),
						Metadata: map[string]interface{}{
							"source":    "claude_code",
							"agent_id":  a.AgentID,
							"capture_method": "stdin_monitor",
						},
					}
					
					a.streamSessionData(sessionData)
					
					// Publish conversation event
					a.publishConversationEvent(sessionID, "user_message", map[string]interface{}{
						"content": strings.TrimSpace(line),
						"length":  len(strings.TrimSpace(line)),
					})
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// hasInputAvailable - Check if input is available (simplified version)
func (a *ClaudeCaptureAgent) hasInputAvailable() bool {
	// This is a simplified check - in a real implementation,
	// you might use more sophisticated methods to detect Claude Code sessions
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == 0
}

// startRedisListener - Listen for Redis requests
func (a *ClaudeCaptureAgent) startRedisListener() {
	// Subscribe to request channel
	pubsub := a.RedisClient.Subscribe(a.ctx, a.RequestChannel)
	defer pubsub.Close()
	
	// Listen for messages
	ch := pubsub.Channel()
	
	fmt.Printf("%s: Redis listener started\n", a.AgentID)
	
	for {
		select {
		case <-a.ctx.Done():
			fmt.Printf("%s: Redis listener stopping\n", a.AgentID)
			return
		case msg := <-ch:
			a.processRedisMessage(msg.Payload)
		}
	}
}

// processRedisMessage - Process incoming Redis message
func (a *ClaudeCaptureAgent) processRedisMessage(payload string) {
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		fmt.Printf("%s: Error parsing request: %v\n", a.AgentID, err)
		return
	}
	
	fmt.Printf("%s received Redis request: %v\n", a.AgentID, request["action"])
	
	// Handle the request
	response := a.HandleRequest(request)
	
	// Send response back via Redis
	responseData, _ := json.Marshal(response)
	a.RedisClient.Publish(a.ctx, a.ResponseChannel, responseData)
}

// HandleRequest - Handle incoming requests
func (a *ClaudeCaptureAgent) HandleRequest(request map[string]interface{}) map[string]interface{} {
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
		}
	}
	
	// Route to appropriate handler
	switch action {
	case "capture_session":
		return a.handleCaptureSession(request)
	case "stream_conversation":
		return a.handleStreamConversation(request)
	case "get_session_status":
		return a.handleGetSessionStatus(request)
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown action: %s", action),
		}
	}
}

// handleCaptureSession - Handle session capture requests
func (a *ClaudeCaptureAgent) handleCaptureSession(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	sessionID := a.generateSessionID()
	if id, ok := params["session_id"].(string); ok {
		sessionID = id
	}
	
	content, ok := params["content"].(string)
	if !ok {
		return map[string]interface{}{"error": "Content required"}
	}
	
	messageType, ok := params["type"].(string)
	if !ok {
		messageType = "captured_session"
	}
	
	// Create session data
	sessionData := SessionData{
		SessionID:   sessionID,
		Timestamp:   time.Now(),
		MessageType: messageType,
		Content:     content,
		Metadata: map[string]interface{}{
			"source":      "api_request",
			"agent_id":    a.AgentID,
			"request_id":  request["request_id"],
		},
	}
	
	// Stream to Redis
	a.streamSessionData(sessionData)
	
	return map[string]interface{}{
		"success":    true,
		"session_id": sessionID,
		"message":    "Session data captured and streamed",
	}
}

// handleStreamConversation - Handle conversation streaming requests
func (a *ClaudeCaptureAgent) handleStreamConversation(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	sessionID, ok := params["session_id"].(string)
	if !ok {
		return map[string]interface{}{"error": "Session ID required"}
	}
	
	eventType, ok := params["event_type"].(string)
	if !ok {
		eventType = "conversation_event"
	}
	
	data, ok := params["data"].(map[string]interface{})
	if !ok {
		data = make(map[string]interface{})
	}
	
	// Publish conversation event
	a.publishConversationEvent(sessionID, eventType, data)
	
	return map[string]interface{}{
		"success":   true,
		"session_id": sessionID,
		"message":   "Conversation event streamed",
	}
}

// handleGetSessionStatus - Handle session status requests
func (a *ClaudeCaptureAgent) handleGetSessionStatus(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"agent_id":     a.AgentID,
		"cid":          a.CID,
		"status":       "running",
		"uptime":       time.Since(time.Now()).String(),
		"capabilities": []string{"session_capture", "redis_streaming", "conversation_tracking"},
	}
}

// streamSessionData - Stream session data to Redis
func (a *ClaudeCaptureAgent) streamSessionData(data SessionData) {
	streamName := "claude:sessions:stream"
	
	dataJSON, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("%s: Error marshaling session data: %v\n", a.AgentID, err)
		return
	}
	
	_, err = a.RedisClient.XAdd(a.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"session_id":   data.SessionID,
			"message_type": data.MessageType,
			"content":      data.Content,
			"data":         string(dataJSON),
			"timestamp":    data.Timestamp.Unix(),
			"source":       a.AgentID,
		},
	}).Result()
	
	if err != nil {
		fmt.Printf("%s: Error streaming session data: %v\n", a.AgentID, err)
	} else {
		fmt.Printf("%s: Streamed session data for session %s\n", a.AgentID, data.SessionID)
	}
}

// publishConversationEvent - Publish conversation events to Redis
func (a *ClaudeCaptureAgent) publishConversationEvent(sessionID, eventType string, data map[string]interface{}) {
	streamName := "claude:conversations:stream"
	
	event := ConversationEvent{
		EventID:     fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		SessionID:   sessionID,
		Timestamp:   time.Now(),
		EventType:   eventType,
		Data:        data,
		AgentSource: a.AgentID,
	}
	
	eventJSON, err := json.Marshal(event)
	if err != nil {
		fmt.Printf("%s: Error marshaling conversation event: %v\n", a.AgentID, err)
		return
	}
	
	_, err = a.RedisClient.XAdd(a.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"event_id":    event.EventID,
			"session_id":  event.SessionID,
			"event_type":  event.EventType,
			"data":        string(eventJSON),
			"timestamp":   event.Timestamp.Unix(),
			"source":      a.AgentID,
		},
	}).Result()
	
	if err != nil {
		fmt.Printf("%s: Error publishing conversation event: %v\n", a.AgentID, err)
	} else {
		fmt.Printf("%s: Published conversation event: %s for session %s\n", a.AgentID, eventType, sessionID)
	}
}

// publishSessionHeartbeat - Publish periodic heartbeat for active sessions
func (a *ClaudeCaptureAgent) publishSessionHeartbeat(sessionID string) {
	a.publishConversationEvent(sessionID, "session_heartbeat", map[string]interface{}{
		"agent_id": a.AgentID,
		"status":   "active",
		"heartbeat_time": time.Now(),
	})
}

// generateSessionID - Generate unique session ID
func (a *ClaudeCaptureAgent) generateSessionID() string {
	return fmt.Sprintf("claude_session_%d", time.Now().UnixNano())
}

func main() {
	agent := NewAgent()
	agent.Start()
}
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v2"
)

type SystemCommander struct {
	RedisClient   *redis.Client
	ctx           context.Context
	agentID       string
	sessions      map[string]*TmuxSession
	sessionsMutex sync.RWMutex
}

type TmuxSession struct {
	SessionName string
	ClientID    string
	Created     time.Time
	LastUsed    time.Time
}

type CommandRequest struct {
	Command   string `json:"command"`
	ClientID  string `json:"client_id"`
	RequestID string `json:"request_id"`
	Session   string `json:"session,omitempty"`
	TTY       bool   `json:"tty,omitempty"`
}

type CommandResponse struct {
	Success     bool   `json:"success"`
	Output      string `json:"output"`
	Error       string `json:"error,omitempty"`
	RequestID   string `json:"request_id"`
	SessionName string `json:"session_name,omitempty"`
	ExitCode    int    `json:"exit_code"`
}

type Contract struct {
	Clients map[string]ClientPermissions `yaml:"clients"`
}

type ClientPermissions struct {
	Commands []string `yaml:"commands"`
	TTY      bool     `yaml:"tty"`
	Sessions bool     `yaml:"sessions"`
}

func NewSystemCommander() *SystemCommander {
	ctx := context.Background()
	
	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	// Test connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	return &SystemCommander{
		RedisClient: rdb,
		ctx:         ctx,
		agentID:     "AGT-SYSTEM-COMMANDER-1",
		sessions:    make(map[string]*TmuxSession),
	}
}

func (sc *SystemCommander) loadContract() (*Contract, error) {
	// Try to load contract, default to allow-all if not found
	contractFile := "./contract.yaml"
	
	if _, err := os.Stat(contractFile); os.IsNotExist(err) {
		// Create default allow-all contract
		defaultContract := &Contract{
			Clients: map[string]ClientPermissions{
				"claude_code": {
					Commands: []string{"*"}, // Allow all commands
					TTY:      true,
					Sessions: true,
				},
			},
		}
		
		// Save default contract
		data, _ := yaml.Marshal(defaultContract)
		os.WriteFile(contractFile, data, 0644)
		return defaultContract, nil
	}

	// Load existing contract
	data, err := os.ReadFile(contractFile)
	if err != nil {
		return nil, err
	}

	var contract Contract
	err = yaml.Unmarshal(data, &contract)
	return &contract, err
}

func (sc *SystemCommander) isCommandAuthorized(clientID, command string) bool {
	contract, err := sc.loadContract()
	if err != nil {
		log.Printf("Failed to load contract: %v", err)
		return false
	}

	permissions, exists := contract.Clients[clientID]
	if !exists {
		return false
	}

	// Check if command is authorized
	for _, allowedCmd := range permissions.Commands {
		if allowedCmd == "*" {
			return true // Allow all
		}
		if strings.HasPrefix(command, allowedCmd) {
			return true
		}
	}
	
	return false
}

func (sc *SystemCommander) getOrCreateSession(clientID string) string {
	sc.sessionsMutex.Lock()
	defer sc.sessionsMutex.Unlock()
	
	sessionName := fmt.Sprintf("syscmd_%s_%d", clientID, time.Now().Unix())
	
	// Create new tmux session
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	err := cmd.Run()
	if err != nil {
		log.Printf("Failed to create tmux session: %v", err)
		return ""
	}
	
	sc.sessions[sessionName] = &TmuxSession{
		SessionName: sessionName,
		ClientID:    clientID,
		Created:     time.Now(),
		LastUsed:    time.Now(),
	}
	
	return sessionName
}

func (sc *SystemCommander) executeCommand(req CommandRequest) CommandResponse {
	// Check authorization
	if !sc.isCommandAuthorized(req.ClientID, req.Command) {
		return CommandResponse{
			Success:   false,
			Error:     fmt.Sprintf("Command not authorized for client: %s", req.ClientID),
			RequestID: req.RequestID,
			ExitCode:  -1,
		}
	}

	// Execute command based on TTY requirement
	if req.TTY {
		return sc.executeInTmux(req)
	} else {
		return sc.executeDirectly(req)
	}
}

func (sc *SystemCommander) executeDirectly(req CommandRequest) CommandResponse {
	// Execute command directly using exec.Command
	parts := strings.Fields(req.Command)
	if len(parts) == 0 {
		return CommandResponse{
			Success:   false,
			Error:     "Empty command",
			RequestID: req.RequestID,
			ExitCode:  -1,
		}
	}
	
	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return CommandResponse{
		Success:   err == nil,
		Output:    string(output),
		Error:     func() string { if err != nil { return err.Error() } else { return "" } }(),
		RequestID: req.RequestID,
		ExitCode:  exitCode,
	}
}

func (sc *SystemCommander) executeInTmux(req CommandRequest) CommandResponse {
	sessionName := req.Session
	if sessionName == "" {
		sessionName = sc.getOrCreateSession(req.ClientID)
		if sessionName == "" {
			return CommandResponse{
				Success:   false,
				Error:     "Failed to create tmux session",
				RequestID: req.RequestID,
				ExitCode:  -1,
			}
		}
	}

	// Send command to tmux session
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName, req.Command, "Enter")
	err := cmd.Run()
	if err != nil {
		return CommandResponse{
			Success:   false,
			Error:     fmt.Sprintf("Failed to send command to tmux: %v", err),
			RequestID: req.RequestID,
			ExitCode:  -1,
		}
	}

	// Wait a moment for command to execute
	time.Sleep(500 * time.Millisecond)

	// Capture pane content (last screen)
	captureCmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	output, err := captureCmd.Output()
	if err != nil {
		return CommandResponse{
			Success:     true, // Command was sent successfully
			Output:      "(failed to capture output)",
			SessionName: sessionName,
			RequestID:   req.RequestID,
			ExitCode:    0,
		}
	}

	// Update session last used time
	sc.sessionsMutex.Lock()
	if session, exists := sc.sessions[sessionName]; exists {
		session.LastUsed = time.Now()
	}
	sc.sessionsMutex.Unlock()

	return CommandResponse{
		Success:     true,
		Output:      string(output),
		SessionName: sessionName,
		RequestID:   req.RequestID,
		ExitCode:    0, // tmux doesn't provide exit code easily
	}
}

func (sc *SystemCommander) cleanupOldSessions() {
	sc.sessionsMutex.Lock()
	defer sc.sessionsMutex.Unlock()
	
	cutoff := time.Now().Add(-30 * time.Minute) // 30 minutes idle timeout
	
	for sessionName, session := range sc.sessions {
		if session.LastUsed.Before(cutoff) {
			// Kill tmux session
			exec.Command("tmux", "kill-session", "-t", sessionName).Run()
			delete(sc.sessions, sessionName)
			log.Printf("Cleaned up idle session: %s", sessionName)
		}
	}
}

func (sc *SystemCommander) startCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			sc.cleanupOldSessions()
		}
	}()
}

func (sc *SystemCommander) handleRequest(payload string) {
	var req CommandRequest
	err := json.Unmarshal([]byte(payload), &req)
	if err != nil {
		log.Printf("Failed to parse request: %v", err)
		return
	}

	log.Printf("Executing command for %s: %s", req.ClientID, req.Command)

	// Execute command
	response := sc.executeCommand(req)

	// Send response
	responseData, _ := json.Marshal(response)
	sc.RedisClient.Publish(sc.ctx, "agent.system.response", string(responseData))
}

func (sc *SystemCommander) startListener() {
	pubsub := sc.RedisClient.Subscribe(sc.ctx, "agent.system.request")
	defer pubsub.Close()

	log.Println("System Commander listening for requests...")

	for msg := range pubsub.Channel() {
		sc.handleRequest(msg.Payload)
	}
}

func (sc *SystemCommander) registerWithManager() {
	// Register this agent with the manager
	registrationData := map[string]interface{}{
		"agent_name":   sc.agentID,
		"agent_type":   "system_commander",
		"pid":          os.Getpid(),
		"capabilities": []string{"execute_command", "tmux_sessions", "direct_execution"},
		"channels":     []string{"agent.system.request"},
		"contracts":    []string{"claude_code"},
	}

	data, _ := json.Marshal(registrationData)
	sc.RedisClient.Publish(sc.ctx, "agent.manager.register", string(data))
	log.Printf("Registered %s with manager", sc.agentID)
}

func main() {
	log.Println("Starting AGT-SYSTEM-COMMANDER-1...")

	sc := NewSystemCommander()
	
	// Register with manager
	sc.registerWithManager()
	
	// Start session cleanup routine
	sc.startCleanupRoutine()
	
	// Start listening for commands
	log.Println("System Commander ready for secure command execution")
	sc.startListener()
}
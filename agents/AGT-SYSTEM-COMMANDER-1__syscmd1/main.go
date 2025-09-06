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
	shellPool     *ShellPool
	sessions      map[string]*TmuxSession  // Legacy single sessions
	sessionsMutex sync.RWMutex
}

type TmuxSession struct {
	SessionName string
	ClientID    string
	Created     time.Time
	LastUsed    time.Time
}

// ShellPool manages multiple concurrent shells for orchestration
type ShellPool struct {
	shells    map[string]*Shell
	mutex     sync.RWMutex
	maxShells int
	cleanupInterval time.Duration
}

// Shell represents a managed shell session
type Shell struct {
	ID          string
	SessionName string
	ClientID    string
	Purpose     string    // "general", "build", "monitor", "conversation_logging", etc.
	Created     time.Time
	LastUsed    time.Time
	State       ShellState
	LastLoggedLine int      // Track last conversation line sent to streams
	MonitoringEnabled bool  // Whether to monitor for conversation logging
	Busy        bool      // Currently executing a command
	mutex       sync.RWMutex
}

type ShellState string

const (
	ShellStateReady   ShellState = "ready"
	ShellStateBusy    ShellState = "busy"
	ShellStateWaiting ShellState = "waiting"
	ShellStateFailed  ShellState = "failed"
)

type CommandRequest struct {
	Command     string            `json:"command"`
	ClientID    string            `json:"client_id"`
	RequestID   string            `json:"request_id"`
	Session     string            `json:"session,omitempty"`
	TTY         bool              `json:"tty,omitempty"`
	// Orchestration fields
	Mode        string            `json:"mode,omitempty"`        // "direct", "tmux", "parallel", "sequence"
	ShellID     string            `json:"shell_id,omitempty"`    // Specific shell to use
	Purpose     string            `json:"purpose,omitempty"`     // "build", "test", "monitor", etc.
	Parallel    []ParallelCommand `json:"parallel,omitempty"`    // Multiple commands to run in parallel
	DependsOn   []string          `json:"depends_on,omitempty"` // Shell IDs this command depends on
	Timeout     int               `json:"timeout,omitempty"`    // Command timeout in seconds
}

type ParallelCommand struct {
	Command string `json:"command"`
	ShellID string `json:"shell_id,omitempty"`
	Purpose string `json:"purpose,omitempty"`
}

type CommandResponse struct {
	Success     bool                     `json:"success"`
	Output      string                   `json:"output"`
	Error       string                   `json:"error,omitempty"`
	RequestID   string                   `json:"request_id"`
	SessionName string                   `json:"session_name,omitempty"`
	ExitCode    int                      `json:"exit_code"`
	// Orchestration fields
	ShellID     string                   `json:"shell_id,omitempty"`
	Mode        string                   `json:"mode,omitempty"`
	Results     []ParallelCommandResult  `json:"results,omitempty"` // For parallel command responses
	ActiveShells int                     `json:"active_shells,omitempty"`
}

type ParallelCommandResult struct {
	Command   string `json:"command"`
	ShellID   string `json:"shell_id"`
	Success   bool   `json:"success"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	ExitCode  int    `json:"exit_code"`
	Duration  int64  `json:"duration_ms"`
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
		shellPool:   NewShellPool(10, 5*time.Minute), // Max 10 shells, cleanup every 5 minutes
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

	// Determine execution mode
	mode := req.Mode
	if mode == "" {
		if req.TTY {
			mode = "tmux"
		} else {
			mode = "direct"
		}
	}

	// Execute based on mode
	switch mode {
	case "direct":
		return sc.executeDirectly(req)
	case "tmux":
		return sc.executeInTmux(req)
	case "orchestration":
		return sc.executeOrchestration(req)
	case "parallel":
		return sc.executeParallel(req)
	default:
		// Backward compatibility - use TTY flag
		if req.TTY {
			return sc.executeInTmux(req)
		} else {
			return sc.executeDirectly(req)
		}
	}
}

func (sc *SystemCommander) executeDirectly(req CommandRequest) CommandResponse {
	// Execute command directly using exec.Command
	log.Printf("DEBUG executeDirectly: Command: '%s', Length: %d", req.Command, len(req.Command))
	parts := strings.Fields(req.Command)
	log.Printf("DEBUG executeDirectly: Parts length: %d, Parts: %v", len(parts), parts)
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

	response := CommandResponse{
		Success:   err == nil,
		Output:    string(output),
		Error:     func() string { if err != nil { return err.Error() } else { return "" } }(),
		RequestID: req.RequestID,
		ExitCode:  exitCode,
	}
	
	log.Printf("DEBUG executeDirectly: Response Success: %t, Output length: %d, Error: '%s'", response.Success, len(response.Output), response.Error)
	return response
}

func (sc *SystemCommander) executeInTmux(req CommandRequest) CommandResponse {
	// Use shell pool for orchestrated execution
	if req.ShellID != "" {
		if shell, exists := sc.shellPool.GetShell(req.ShellID); exists {
			output, err := sc.shellPool.ExecuteInShell(shell, req.Command)
			return CommandResponse{
				Success:     err == nil,
				Output:      output,
				Error:       func() string { if err != nil { return err.Error() } else { return "" } }(),
				RequestID:   req.RequestID,
				ShellID:     req.ShellID,
				Mode:        "tmux",
				ExitCode:    func() int { if err != nil { return -1 } else { return 0 } }(),
			}
		} else {
			return CommandResponse{
				Success:   false,
				Error:     fmt.Sprintf("Shell %s not found", req.ShellID),
				RequestID: req.RequestID,
				ExitCode:  -1,
			}
		}
	}

	// Get or create shell from pool
	purpose := req.Purpose
	if purpose == "" {
		purpose = "general"
	}
	
	shell, err := sc.shellPool.GetOrCreateShell(req.ClientID, purpose)
	if err != nil {
		return CommandResponse{
			Success:   false,
			Error:     fmt.Sprintf("Failed to get shell: %v", err),
			RequestID: req.RequestID,
			ExitCode:  -1,
		}
	}

	// Execute command in shell
	output, err := sc.shellPool.ExecuteInShell(shell, req.Command)
	return CommandResponse{
		Success:     err == nil,
		Output:      output,
		Error:       func() string { if err != nil { return err.Error() } else { return "" } }(),
		RequestID:   req.RequestID,
		ShellID:     shell.ID,
		SessionName: shell.SessionName,
		Mode:        "tmux",
		ExitCode:    func() int { if err != nil { return -1 } else { return 0 } }(),
	}
}

// ShellPool methods
func NewShellPool(maxShells int, cleanupInterval time.Duration) *ShellPool {
	return &ShellPool{
		shells:          make(map[string]*Shell),
		maxShells:       maxShells,
		cleanupInterval: cleanupInterval,
	}
}

func (sp *ShellPool) GetOrCreateShell(clientID, purpose string) (*Shell, error) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	
	// Look for existing available shell with same purpose
	for _, shell := range sp.shells {
		if shell.ClientID == clientID && shell.Purpose == purpose && shell.State == ShellStateReady && !shell.Busy {
			shell.LastUsed = time.Now()
			return shell, nil
		}
	}
	
	// Check if we can create a new shell
	if len(sp.shells) >= sp.maxShells {
		return nil, fmt.Errorf("maximum shells (%d) reached", sp.maxShells)
	}
	
	// Create new shell
	shellID := fmt.Sprintf("%s_%s_%d", clientID, purpose, time.Now().UnixNano())
	sessionName := fmt.Sprintf("sc_%s", shellID)
	
	// Create tmux session
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to create tmux session: %v", err)
	}
	
	shell := &Shell{
		ID:          shellID,
		SessionName: sessionName,
		ClientID:    clientID,
		Purpose:     purpose,
		Created:     time.Now(),
		LastUsed:    time.Now(),
		State:       ShellStateReady,
		Busy:        false,
	}
	
	sp.shells[shellID] = shell
	log.Printf("Created new shell %s for client %s (purpose: %s)", shellID, clientID, purpose)
	return shell, nil
}

func (sp *ShellPool) GetShell(shellID string) (*Shell, bool) {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()
	shell, exists := sp.shells[shellID]
	return shell, exists
}

func (sp *ShellPool) ExecuteInShell(shell *Shell, command string) (string, error) {
	shell.mutex.Lock()
	defer shell.mutex.Unlock()
	
	if shell.Busy {
		return "", fmt.Errorf("shell %s is busy", shell.ID)
	}
	
	shell.Busy = true
	shell.State = ShellStateBusy
	defer func() {
		shell.Busy = false
		shell.State = ShellStateReady
		shell.LastUsed = time.Now()
	}()
	
	// Send command to tmux session
	cmd := exec.Command("tmux", "send-keys", "-t", shell.SessionName, command, "Enter")
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to send command to tmux: %v", err)
	}
	
	// Wait for command to execute
	time.Sleep(500 * time.Millisecond)
	
	// Capture output
	captureCmd := exec.Command("tmux", "capture-pane", "-t", shell.SessionName, "-p")
	output, err := captureCmd.Output()
	if err != nil {
		return "(failed to capture output)", nil
	}
	
	return string(output), nil
}

func (sp *ShellPool) ListShells() []*Shell {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()
	
	shells := make([]*Shell, 0, len(sp.shells))
	for _, shell := range sp.shells {
		shells = append(shells, shell)
	}
	return shells
}

func (sp *ShellPool) CleanupIdleShells(idleTimeout time.Duration) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	
	cutoff := time.Now().Add(-idleTimeout)
	
	for shellID, shell := range sp.shells {
		if shell.LastUsed.Before(cutoff) && !shell.Busy {
			// Kill tmux session
			exec.Command("tmux", "kill-session", "-t", shell.SessionName).Run()
			delete(sp.shells, shellID)
			log.Printf("Cleaned up idle shell: %s", shellID)
		}
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
			sc.cleanupOldSessions()  // Legacy sessions
			sc.shellPool.CleanupIdleShells(30 * time.Minute)  // Shell pool cleanup
		}
	}()
}

func (sc *SystemCommander) handleRequest(payload string) {
	// First try to parse as gateway format (with params wrapper)
	var gatewayReq struct {
		Action    string                 `json:"action"`
		Params    CommandRequest         `json:"params"`
		ClientID  string                `json:"client_id"`
		RequestID string                `json:"request_id"`
	}
	
	var req CommandRequest
	
	// Try gateway format first
	if err := json.Unmarshal([]byte(payload), &gatewayReq); err == nil && gatewayReq.Action == "execute_command" {
		req = gatewayReq.Params
		req.ClientID = gatewayReq.ClientID
		req.RequestID = gatewayReq.RequestID
	} else {
		// Fall back to direct format
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			log.Printf("Failed to parse request: %v", err)
			return
		}
	}

	// Handle status command specially
	if req.Command == "__status__" {
		log.Printf("Status request from %s", req.ClientID)
		response := sc.getShellStatus()
		response.RequestID = req.RequestID
		responseData, _ := json.Marshal(response)
		sc.RedisClient.Publish(sc.ctx, "agent.system.response", string(responseData))
		return
	}
	
	// Handle conversation monitoring commands
	if strings.HasPrefix(req.Command, "__monitor_conversations__") {
		log.Printf("Conversation monitoring request from %s", req.ClientID)
		parts := strings.Fields(req.Command)
		var response CommandResponse
		if len(parts) > 1 {
			sessionName := parts[1]
			sc.enableConversationMonitoring(sessionName)
			response = CommandResponse{
				Success:   true,
				Output:    fmt.Sprintf("Conversation monitoring enabled for session: %s", sessionName),
				RequestID: req.RequestID,
			}
		} else {
			response = CommandResponse{
				Success:   false,
				Error:     "Session name required: __monitor_conversations__ <session_name>",
				RequestID: req.RequestID,
			}
		}
		responseData, _ := json.Marshal(response)
		sc.RedisClient.Publish(sc.ctx, "agent.system.response", string(responseData))
		return
	}

	log.Printf("Executing command for %s: %s (mode: %s)", req.ClientID, req.Command, req.Mode)
	log.Printf("DEBUG: Command length: %d, Command: '%s'", len(req.Command), req.Command)

	// Execute command
	response := sc.executeCommand(req)

	log.Printf("DEBUG handleRequest: Final Response Success: %t, Error: '%s'", response.Success, response.Error)

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

// Orchestration methods
func (sc *SystemCommander) executeParallel(req CommandRequest) CommandResponse {
	if len(req.Parallel) == 0 {
		return CommandResponse{
			Success:   false,
			Error:     "No parallel commands specified",
			RequestID: req.RequestID,
			ExitCode:  -1,
		}
	}

	// Execute all commands in parallel
	var wg sync.WaitGroup
	results := make([]ParallelCommandResult, len(req.Parallel))
	
	for i, parallelCmd := range req.Parallel {
		wg.Add(1)
		go func(index int, cmd ParallelCommand) {
			defer wg.Done()
			
			startTime := time.Now()
			
			// Get or create shell for this command
			purpose := cmd.Purpose
			if purpose == "" {
				purpose = fmt.Sprintf("parallel_%d", index)
			}
			
			shell, err := sc.shellPool.GetOrCreateShell(req.ClientID, purpose)
			if err != nil {
				results[index] = ParallelCommandResult{
					Command:  cmd.Command,
					ShellID:  cmd.ShellID,
					Success:  false,
					Error:    fmt.Sprintf("Failed to get shell: %v", err),
					ExitCode: -1,
					Duration: time.Since(startTime).Milliseconds(),
				}
				return
			}
			
			// Execute command
			output, err := sc.shellPool.ExecuteInShell(shell, cmd.Command)
			
			results[index] = ParallelCommandResult{
				Command:  cmd.Command,
				ShellID:  shell.ID,
				Success:  err == nil,
				Output:   output,
				Error:    func() string { if err != nil { return err.Error() } else { return "" } }(),
				ExitCode: func() int { if err != nil { return -1 } else { return 0 } }(),
				Duration: time.Since(startTime).Milliseconds(),
			}
		}(i, parallelCmd)
	}
	
	// Wait for all commands to complete
	wg.Wait()
	
	// Check overall success
	overallSuccess := true
	var combinedOutput strings.Builder
	for i, result := range results {
		if !result.Success {
			overallSuccess = false
		}
		combinedOutput.WriteString(fmt.Sprintf("=== Command %d: %s (Shell: %s) ===\n", i+1, result.Command, result.ShellID))
		if result.Success {
			combinedOutput.WriteString(result.Output)
		} else {
			combinedOutput.WriteString(fmt.Sprintf("ERROR: %s\n", result.Error))
		}
		combinedOutput.WriteString(fmt.Sprintf("Duration: %dms\n\n", result.Duration))
	}
	
	return CommandResponse{
		Success:      overallSuccess,
		Output:       combinedOutput.String(),
		RequestID:    req.RequestID,
		Mode:         "parallel",
		Results:      results,
		ActiveShells: len(sc.shellPool.ListShells()),
		ExitCode:     func() int { if overallSuccess { return 0 } else { return 1 } }(),
	}
}

func (sc *SystemCommander) executeOrchestration(req CommandRequest) CommandResponse {
	// For now, orchestration mode just delegates to parallel
	// In Phase 3, this will handle dependencies and signaling
	log.Printf("Orchestration mode requested - delegating to parallel execution for now")
	req.Mode = "parallel"
	return sc.executeParallel(req)
}

// Status and management methods
func (sc *SystemCommander) getShellStatus() CommandResponse {
	shells := sc.shellPool.ListShells()
	
	var statusOutput strings.Builder
	statusOutput.WriteString(fmt.Sprintf("=== System Commander Status ===\n"))
	statusOutput.WriteString(fmt.Sprintf("Active Shells: %d\n\n", len(shells)))
	
	for _, shell := range shells {
		statusOutput.WriteString(fmt.Sprintf("Shell ID: %s\n", shell.ID))
		statusOutput.WriteString(fmt.Sprintf("  Client: %s\n", shell.ClientID))
		statusOutput.WriteString(fmt.Sprintf("  Purpose: %s\n", shell.Purpose))
		statusOutput.WriteString(fmt.Sprintf("  State: %s\n", shell.State))
		statusOutput.WriteString(fmt.Sprintf("  Busy: %t\n", shell.Busy))
		statusOutput.WriteString(fmt.Sprintf("  Created: %s\n", shell.Created.Format(time.RFC3339)))
		statusOutput.WriteString(fmt.Sprintf("  Last Used: %s\n", shell.LastUsed.Format(time.RFC3339)))
		statusOutput.WriteString("\n")
	}
	
	return CommandResponse{
		Success:      true,
		Output:       statusOutput.String(),
		Mode:         "status",
		ActiveShells: len(shells),
		ExitCode:     0,
	}
}

func (sc *SystemCommander) registerWithManager() {
	// Register this agent with the manager
	registrationData := map[string]interface{}{
		"agent_name":   sc.agentID,
		"agent_type":   "system_commander",
		"pid":          os.Getpid(),
		"capabilities": []string{"execute_command", "tmux_sessions", "direct_execution", "shell_pool", "parallel_execution", "orchestration"},
		"channels":     []string{"agent.system.request"},
		"contracts":    []string{"claude_code"},
	}

	data, _ := json.Marshal(registrationData)
	sc.RedisClient.Publish(sc.ctx, "agent.manager.register", string(data))
	log.Printf("Registered %s with manager", sc.agentID)
}

// Conversation monitoring functionality
func (sc *SystemCommander) startConversationMonitor() {
	log.Println("Starting conversation monitor for W/N streaming...")
	go func() {
		ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				sc.monitorAllSessions()
			case <-sc.ctx.Done():
				return
			}
		}
	}()
}

func (sc *SystemCommander) monitorAllSessions() {
	sc.shellPool.mutex.RLock()
	defer sc.shellPool.mutex.RUnlock()
	
	for _, shell := range sc.shellPool.shells {
		if shell.MonitoringEnabled {
			sc.extractConversationFromSession(shell)
		}
	}
}

func (sc *SystemCommander) extractConversationFromSession(shell *Shell) {
	// Capture tmux session content
	cmd := exec.Command("tmux", "capture-pane", "-t", shell.SessionName, "-p")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error capturing session %s: %v", shell.SessionName, err)
		return
	}
	
	lines := strings.Split(string(output), "\n")
	
	// Only process new lines since last check
	if len(lines) > shell.LastLoggedLine {
		newLines := lines[shell.LastLoggedLine:]
		
		// Filter out empty lines and system prompts
		conversationChunk := ""
		for _, line := range newLines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "larrydiffey@") && !strings.HasPrefix(line, ">_") {
				conversationChunk += line + "\n"
			}
		}
		
		if conversationChunk != "" {
			sc.streamConversationChunk(shell.ClientID, shell.SessionName, conversationChunk)
			shell.LastLoggedLine = len(lines)
		}
	}
}

func (sc *SystemCommander) streamConversationChunk(clientID, sessionName, chunk string) {
	// Create conversation stream entry
	conversationData := map[string]interface{}{
		"session_id": sessionName,
		"client_id": clientID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"content": chunk,
		"type": "conversation_chunk",
		"source": "tmux_monitor",
	}
	
	dataBytes, err := json.Marshal(conversationData)
	if err != nil {
		log.Printf("Error marshaling conversation data: %v", err)
		return
	}
	
	// Send to Redis stream for W/N consumers
	streamKey := "centerfire:conversations"
	_, err = sc.RedisClient.XAdd(sc.ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"data": string(dataBytes),
		},
	}).Result()
	
	if err != nil {
		log.Printf("Error adding to conversation stream: %v", err)
	} else {
		log.Printf("📝 Streamed conversation chunk from %s (%d chars)", sessionName, len(chunk))
	}
}

// Enable conversation monitoring for specific sessions
func (sc *SystemCommander) enableConversationMonitoring(sessionName string) {
	sc.shellPool.mutex.Lock()
	defer sc.shellPool.mutex.Unlock()
	
	for _, shell := range sc.shellPool.shells {
		if shell.SessionName == sessionName {
			shell.MonitoringEnabled = true
			log.Printf("📊 Enabled conversation monitoring for session: %s", sessionName)
			break
		}
	}
}

func main() {
	log.Println("Starting AGT-SYSTEM-COMMANDER-1...")

	sc := NewSystemCommander()
	
	// Register with manager
	sc.registerWithManager()
	
	// Start session cleanup routine
	sc.startCleanupRoutine()
	
	// Start conversation monitoring for W/N streaming
	sc.startConversationMonitor()
	
	// Start listening for commands
	log.Println("System Commander ready for secure command execution and conversation logging")
	sc.startListener()
}
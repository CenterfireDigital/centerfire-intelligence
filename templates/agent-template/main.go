package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// AgentConfig represents the minimal agent configuration
type AgentConfig struct {
	AgentID      string   `yaml:"agent_id"`
	CID          string   `yaml:"cid"`
	FriendlyName string   `yaml:"friendly_name"`
	Namespace    string   `yaml:"namespace"`
	Language     string   `yaml:"language"`
	Capabilities []string `yaml:"capabilities"`
}

// Agent represents the minimal agent structure
type Agent struct {
	config     AgentConfig
	ctx        context.Context
	cancel     context.CancelFunc
	pidFile    string
	healthFile string
}

// NewAgent creates a new minimal agent
func NewAgent(config AgentConfig) *Agent {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Agent{
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
		pidFile:    fmt.Sprintf("/tmp/%s.pid", config.AgentID),
		healthFile: fmt.Sprintf("/tmp/%s.health", config.AgentID),
	}
}

// Start initializes and runs the agent
func (a *Agent) Start() error {
	log.Printf("🚀 Starting %s (%s)", a.config.AgentID, a.config.FriendlyName)
	
	// 1. Write PID file
	if err := a.writePIDFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	
	// 2. Setup signal handling for graceful shutdown
	a.setupSignalHandling()
	
	// 3. Register with monitor (simplified - file-based for now)
	if err := a.registerWithMonitor(); err != nil {
		log.Printf("⚠️ Failed to register with monitor: %v", err)
	}
	
	// 4. Start health reporting
	go a.healthReporter()
	
	// 5. Run main agent logic
	return a.run()
}

// run contains the main agent logic (to be implemented by specific agents)
func (a *Agent) run() error {
	log.Printf("✅ %s ready", a.config.FriendlyName)
	
	// Main agent loop - replace with actual agent logic
	for {
		select {
		case <-a.ctx.Done():
			log.Printf("🛑 %s shutting down", a.config.AgentID)
			return nil
		case <-time.After(30 * time.Second):
			// Agent-specific work goes here
			a.logToCapture("Agent heartbeat", map[string]interface{}{
				"status": "running",
				"uptime": time.Now().Unix(),
			})
		}
	}
}

// writePIDFile writes the process ID to a file for monitoring
func (a *Agent) writePIDFile() error {
	pid := os.Getpid()
	return os.WriteFile(a.pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// setupSignalHandling configures graceful shutdown
func (a *Agent) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-sigChan
		log.Printf("🛑 Received %s, initiating graceful shutdown", sig)
		a.shutdown()
	}()
}

// registerWithMonitor notifies the monitor of agent startup
func (a *Agent) registerWithMonitor() error {
	registrationData := map[string]interface{}{
		"agent_id":      a.config.AgentID,
		"cid":           a.config.CID,
		"friendly_name": a.config.FriendlyName,
		"namespace":     a.config.Namespace,
		"pid":           os.Getpid(),
		"capabilities":  a.config.Capabilities,
		"status":        "starting",
		"timestamp":     time.Now().Unix(),
	}
	
	// For now, write to a file that monitor can read
	// In production, this would send to monitor via appropriate channel
	regFile := fmt.Sprintf("/tmp/agent-registry-%s.json", a.config.AgentID)
	data, _ := json.MarshalIndent(registrationData, "", "  ")
	
	return os.WriteFile(regFile, data, 0644)
}

// healthReporter maintains health status file
func (a *Agent) healthReporter() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			health := map[string]interface{}{
				"agent_id":   a.config.AgentID,
				"status":     "healthy",
				"timestamp":  time.Now().Unix(),
				"pid":        os.Getpid(),
				"namespace":  a.config.Namespace,
			}
			
			data, _ := json.Marshal(health)
			os.WriteFile(a.healthFile, data, 0644)
		}
	}
}

// logToCapture sends logs to Claude Capture agent with namespace
func (a *Agent) logToCapture(message string, data map[string]interface{}) {
	logEntry := map[string]interface{}{
		"agent_id":  a.config.AgentID,
		"namespace": a.config.Namespace,
		"timestamp": time.Now().Unix(),
		"message":   message,
		"data":      data,
	}
	
	// For now, log to stdout with structured format
	// In production, this would send to Claude Capture via appropriate channel
	jsonData, _ := json.Marshal(logEntry)
	log.Printf("CAPTURE: %s", string(jsonData))
}

// shutdown performs graceful shutdown
func (a *Agent) shutdown() {
	log.Printf("🔄 %s performing graceful shutdown", a.config.AgentID)
	
	// Notify monitor of shutdown
	a.logToCapture("Agent shutting down", map[string]interface{}{
		"reason": "graceful_shutdown",
		"uptime": time.Now().Unix(),
	})
	
	// Clean up PID and health files
	os.Remove(a.pidFile)
	os.Remove(a.healthFile)
	
	// Cancel context to stop all goroutines
	a.cancel()
	
	log.Printf("✅ %s shutdown complete", a.config.AgentID)
	os.Exit(0)
}

func main() {
	// Example configuration - in practice this would be loaded from agent.yaml
	config := AgentConfig{
		AgentID:      "AGT-TEMPLATE-1",
		CID:          "cid:centerfire:agent:template001",
		FriendlyName: "Template Agent",
		Namespace:    "centerfire.agents.template",
		Language:     "go",
		Capabilities: []string{"template", "example"},
	}
	
	agent := NewAgent(config)
	
	if err := agent.Start(); err != nil {
		log.Fatalf("❌ Agent failed to start: %v", err)
	}
}
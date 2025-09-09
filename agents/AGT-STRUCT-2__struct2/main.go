package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v2"
)

// AgentConfig represents the agent configuration from YAML
type AgentConfig struct {
	AgentID      string                 `yaml:"agent_id"`
	CID          string                 `yaml:"cid"`
	FriendlyName string                 `yaml:"friendly_name"`
	Namespace    string                 `yaml:"namespace"`
	Language     string                 `yaml:"language"`
	AgentType    string                 `yaml:"agent_type"`
	Capabilities []string               `yaml:"capabilities"`
	Communication map[string]interface{} `yaml:"communication"`
	Monitoring   map[string]interface{}  `yaml:"monitoring"`
	Logging      map[string]interface{}  `yaml:"logging"`
}

// StructAgent represents the template-based structure management agent
type StructAgent struct {
	config      AgentConfig
	ctx         context.Context
	cancel      context.CancelFunc
	pidFile     string
	healthFile  string
	redisClient *redis.Client
	socketPath  string
}

// NewAgent creates a new structure agent from configuration
func NewAgent(configPath string) (*StructAgent, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Load configuration
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config AgentConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	// Extract socket path from communication config
	socketPath := "/tmp/agt-struct-2.sock"
	if comm, ok := config.Communication["unix_socket"].(string); ok {
		socketPath = comm
	}

	agent := &StructAgent{
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		pidFile:     fmt.Sprintf("/tmp/%s.pid", config.AgentID),
		healthFile:  fmt.Sprintf("/tmp/%s.health", config.AgentID),
		redisClient: redisClient,
		socketPath:  socketPath,
	}

	return agent, nil
}

// Start begins the agent lifecycle
func (a *StructAgent) Start() error {
	// Write PID file
	if err := a.writePIDFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %v", err)
	}

	// Write initial health file
	if err := a.updateHealthFile("starting"); err != nil {
		return fmt.Errorf("failed to write health file: %v", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Test Redis connection
	if err := a.testRedisConnection(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	// Register with AGT-MANAGER-1 if configured
	if err := a.registerWithManager(); err != nil {
		log.Printf("Warning: Failed to register with manager: %v", err)
	}

	// Start Unix socket listener
	go a.startUnixSocketListener()

	// Start Redis pub/sub listener
	go a.startRedisListener()

	// Send to Claude Capture if configured
	a.sendToClaude("startup", map[string]interface{}{
		"agent_id": a.config.AgentID,
		"status":   "operational",
		"pid":      os.Getpid(),
	})

	// Update health to healthy
	if err := a.updateHealthFile("healthy"); err != nil {
		log.Printf("Warning: Failed to update health file: %v", err)
	}

	fmt.Printf("🏗️  %s (%s) started successfully\n", a.config.AgentID, a.config.FriendlyName)
	fmt.Printf("📡 Redis: agent.struct.request\n")
	fmt.Printf("🔌 Unix Socket: %s\n", a.socketPath)
	fmt.Printf("📊 Health: %s\n", a.healthFile)
	fmt.Printf("🆔 PID: %s\n", a.pidFile)

	// Wait for shutdown signal
	<-sigChan
	fmt.Printf("\n🛑 %s shutting down gracefully...\n", a.config.AgentID)

	// Cleanup
	a.cleanup()
	return nil
}

// testRedisConnection verifies Redis connectivity
func (a *StructAgent) testRedisConnection() error {
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	pong, err := a.redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}

	if pong != "PONG" {
		return fmt.Errorf("unexpected ping response: %s", pong)
	}

	fmt.Printf("✅ Connected to Redis successfully\n")
	return nil
}

// registerWithManager registers this agent with AGT-MANAGER-1
func (a *StructAgent) registerWithManager() error {
	registration := map[string]interface{}{
		"agent_name":    a.config.AgentID,
		"request_type":  "register",
		"pid":           os.Getpid(),
		"capabilities":  a.config.Capabilities,
		"redis_channel": "agent.struct.request",
		"unix_socket":   a.socketPath,
		"health_file":   a.healthFile,
	}

	data, err := json.Marshal(registration)
	if err != nil {
		return err
	}

	return a.redisClient.Publish(a.ctx, "centerfire:agent:manager", data).Err()
}

// startUnixSocketListener starts the Unix socket server
func (a *StructAgent) startUnixSocketListener() {
	// Remove existing socket if it exists
	os.Remove(a.socketPath)

	listener, err := net.Listen("unix", a.socketPath)
	if err != nil {
		log.Printf("Failed to start Unix socket listener: %v", err)
		return
	}
	defer listener.Close()
	defer os.Remove(a.socketPath)

	fmt.Printf("🔌 Unix socket listening on %s\n", a.socketPath)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-a.ctx.Done():
				return
			default:
				log.Printf("Unix socket accept error: %v", err)
				continue
			}
		}

		go a.handleUnixConnection(conn)
	}
}

// startRedisListener starts the Redis pub/sub listener
func (a *StructAgent) startRedisListener() {
	pubsub := a.redisClient.Subscribe(a.ctx, "agent.struct.request")
	defer pubsub.Close()

	fmt.Printf("📡 Redis pub/sub listening on agent.struct.request\n")

	ch := pubsub.Channel()
	for {
		select {
		case <-a.ctx.Done():
			return
		case msg := <-ch:
			if msg != nil {
				go a.processRedisMessage(msg.Payload)
			}
		}
	}
}

// processRedisMessage processes incoming Redis pub/sub messages
func (a *StructAgent) processRedisMessage(payload string) {
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		log.Printf("Error parsing Redis message: %v", err)
		return
	}

	response := a.handleRequest(request)
	
	// Send response
	responseData, _ := json.Marshal(response)
	a.redisClient.Publish(a.ctx, "agent.struct.response", responseData)
}

// handleUnixConnection handles Unix socket connections
func (a *StructAgent) handleUnixConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var request map[string]interface{}
	if err := decoder.Decode(&request); err != nil {
		log.Printf("Unix socket decode error: %v", err)
		return
	}

	response := a.handleRequest(request)
	
	if err := encoder.Encode(response); err != nil {
		log.Printf("Unix socket encode error: %v", err)
	}
}

// handleRequest processes requests from any source
func (a *StructAgent) handleRequest(request map[string]interface{}) map[string]interface{} {
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
			"agent": a.config.AgentID,
		}
	}

	fmt.Printf("🏗️  %s received request: %s\n", a.config.AgentID, action)

	switch action {
	case "create_structure":
		return a.handleCreateStructure(request)
	case "health":
		return map[string]interface{}{
			"status":     "healthy",
			"agent":      a.config.AgentID,
			"pid":        os.Getpid(),
			"uptime":     time.Since(time.Now()).String(),
			"timestamp":  time.Now().UTC(),
		}
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown action: %s", action),
			"agent": a.config.AgentID,
		}
	}
}

// handleCreateStructure handles create_structure requests
func (a *StructAgent) handleCreateStructure(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{
			"error": "No params provided",
			"agent": a.config.AgentID,
		}
	}

	name, ok := params["name"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "Name required",
			"agent": a.config.AgentID,
		}
	}

	structType, ok := params["type"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "Type required", 
			"agent": a.config.AgentID,
		}
	}

	template, ok := params["template"].(string)
	if !ok {
		template = "default"
	}

	cid, ok := params["cid"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "CID required",
			"agent": a.config.AgentID,
		}
	}

	fmt.Printf("🏗️  %s: Creating %s structure for %s\n", a.config.AgentID, structType, name)

	// Create directory structure based on type
	var err error
	switch structType {
	case "capability":
		err = a.createCapabilityStructure(name, template, cid)
	case "agent":
		err = a.createAgentStructure(name, template, cid)
	case "module":
		err = a.createModuleStructure(name, template, cid)
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown structure type: %s", structType),
			"agent": a.config.AgentID,
		}
	}

	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Failed to create structure: %v", err),
			"agent": a.config.AgentID,
		}
	}

	// Delegate documentation creation to AGT-SEMDOC-1
	a.delegateDocumentation(name, structType, cid)

	// Send to Claude Capture
	a.sendToClaude("structure_created", map[string]interface{}{
		"name":      name,
		"type":      structType,
		"template":  template,
		"cid":       cid,
		"agent":     a.config.AgentID,
	})

	return map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Created %s structure for %s", structType, name),
		"name":    name,
		"type":    structType,
		"cid":     cid,
		"agent":   a.config.AgentID,
	}
}

// createCapabilityStructure creates directory structure for capability
func (a *StructAgent) createCapabilityStructure(name, template, cid string) error {
	// Create base directory
	capDir := filepath.Join("capabilities", name)
	if err := os.MkdirAll(capDir, 0755); err != nil {
		return fmt.Errorf("failed to create capability directory: %v", err)
	}

	// Create spec.yaml
	specPath := filepath.Join(capDir, "spec.yaml")
	specContent := fmt.Sprintf(`name: %s
cid: %s
type: capability
template: %s
created: %s
description: "Auto-generated capability specification"

# Capability specification
spec:
  domain: ""
  purpose: ""
  dependencies: []
  interfaces: []
`, name, cid, template, time.Now().UTC().Format(time.RFC3339))

	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		return fmt.Errorf("failed to create spec.yaml: %v", err)
	}

	// Create basic main.go based on template
	mainPath := filepath.Join(capDir, "main.go")
	mainContent := fmt.Sprintf(`package main

import "fmt"

// %s - Auto-generated capability
func main() {
	fmt.Printf("%s capability initialized\\n")
	
	// TODO: Implement capability logic
}
`, name, name)

	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		return fmt.Errorf("failed to create main.go: %v", err)
	}

	fmt.Printf("✅ Created capability structure: %s\n", capDir)
	return nil
}

// createAgentStructure creates directory structure for agent
func (a *StructAgent) createAgentStructure(name, template, cid string) error {
	// Create base directory using the CID as directory suffix
	agentDir := filepath.Join("agents", name)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("failed to create agent directory: %v", err)
	}

	// Create agent.yaml configuration
	yamlPath := filepath.Join(agentDir, "agent.yaml")
	yamlContent := fmt.Sprintf(`# Template-based Agent Configuration
agent_id: "%s"
cid: "%s"
friendly_name: "Auto-generated Agent"
namespace: "centerfire.agents"
language: "go"
agent_type: "persistent"
capabilities: []

communication:
  redis_channels: 
    - "agent.%s.request"
    - "agent.%s.response"
  unix_socket: "/tmp/agt-%s.sock"

monitoring:
  register_with_monitor: true
  health_check_method: "file"
  health_check_path: "/tmp/%s.health"

logging:
  send_to_capture: true
  level: "info"
  include_namespace: true
`, name, cid, strings.ToLower(name), strings.ToLower(name), strings.ToLower(name), name)

	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("failed to create agent.yaml: %v", err)
	}

	// Create main.go with template architecture
	mainPath := filepath.Join(agentDir, "main.go")
	mainContent := fmt.Sprintf(`package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// %s - Template-based agent
type Agent struct {
	agentID string
	ctx     context.Context
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: ./agent <config-path>")
	}

	agent, err := NewAgent(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to create agent: %%v", err)
	}

	if err := agent.Start(); err != nil {
		log.Fatalf("Agent failed: %%v", err)
	}
}

func NewAgent(configPath string) (*Agent, error) {
	return &Agent{
		agentID: "%s",
		ctx:     context.Background(),
	}, nil
}

func (a *Agent) Start() error {
	fmt.Printf("🤖 %%s starting...\\n", a.agentID)
	
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// TODO: Implement agent logic
	
	// Wait for shutdown
	<-sigChan
	fmt.Printf("\\n🛑 %%s shutting down...\\n", a.agentID)
	return nil
}
`, name, name)

	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		return fmt.Errorf("failed to create main.go: %v", err)
	}

	fmt.Printf("✅ Created agent structure: %s\n", agentDir)
	return nil
}

// createModuleStructure creates directory structure for module
func (a *StructAgent) createModuleStructure(name, template, cid string) error {
	// Create base directory
	modDir := filepath.Join("modules", name)
	if err := os.MkdirAll(modDir, 0755); err != nil {
		return fmt.Errorf("failed to create module directory: %v", err)
	}

	// Create module.yaml
	yamlPath := filepath.Join(modDir, "module.yaml")
	yamlContent := fmt.Sprintf(`name: %s
cid: %s
type: module
template: %s
created: %s
description: "Auto-generated module specification"

# Module specification
spec:
  purpose: ""
  dependencies: []
  exports: []
`, name, cid, template, time.Now().UTC().Format(time.RFC3339))

	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("failed to create module.yaml: %v", err)
	}

	// Create basic implementation file
	implPath := filepath.Join(modDir, fmt.Sprintf("%s.go", name))
	implContent := fmt.Sprintf(`package %s

// %s - Auto-generated module
type %s struct {
	// TODO: Add module fields
}

// New%s creates a new instance
func New%s() *%s {
	return &%s{
		// TODO: Initialize module
	}
}
`, name, name, name, name, name, name, name)

	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		return fmt.Errorf("failed to create implementation file: %v", err)
	}

	fmt.Printf("✅ Created module structure: %s\n", modDir)
	return nil
}

// delegateDocumentation sends documentation request to AGT-SEMDOC-1
func (a *StructAgent) delegateDocumentation(name, structType, cid string) {
	request := map[string]interface{}{
		"action": "create_documentation",
		"params": map[string]interface{}{
			"name":   name,
			"type":   structType,
			"cid":    cid,
			"source": a.config.AgentID,
		},
	}

	data, err := json.Marshal(request)
	if err != nil {
		log.Printf("Failed to marshal documentation request: %v", err)
		return
	}

	// Send to AGT-SEMDOC-1
	if err := a.redisClient.Publish(a.ctx, "agent.semdoc.request", data).Err(); err != nil {
		log.Printf("Failed to delegate documentation: %v", err)
	} else {
		fmt.Printf("📚 Delegated documentation creation to AGT-SEMDOC-1 for %s\n", name)
	}
}

// sendToClaude sends structured data to AGT-CLAUDE-CAPTURE-1
func (a *StructAgent) sendToClaude(eventType string, data map[string]interface{}) {
	if logging, ok := a.config.Logging["send_to_capture"].(bool); !ok || !logging {
		return
	}

	event := map[string]interface{}{
		"agent_id":   a.config.AgentID,
		"event_type": eventType,
		"timestamp":  time.Now().UTC(),
		"data":       data,
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal Claude event: %v", err)
		return
	}

	// Send to Claude Capture stream
	if err := a.redisClient.XAdd(a.ctx, &redis.XAddArgs{
		Stream: "centerfire:semantic:agent_events",
		Values: map[string]interface{}{
			"data": string(eventData),
		},
	}).Err(); err != nil {
		log.Printf("Failed to send to Claude Capture: %v", err)
	}
}

// writePIDFile writes the current process ID to a file
func (a *StructAgent) writePIDFile() error {
	return os.WriteFile(a.pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

// updateHealthFile updates the health status file
func (a *StructAgent) updateHealthFile(status string) error {
	healthData := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC(),
		"pid":       os.Getpid(),
		"agent":     a.config.AgentID,
	}

	data, err := json.Marshal(healthData)
	if err != nil {
		return err
	}

	return os.WriteFile(a.healthFile, data, 0644)
}

// cleanup performs graceful shutdown cleanup
func (a *StructAgent) cleanup() {
	a.cancel()

	// Update health to shutting down
	a.updateHealthFile("shutting_down")

	// Unregister from manager
	unregister := map[string]interface{}{
		"agent_name":   a.config.AgentID,
		"request_type": "unregister",
		"pid":          os.Getpid(),
	}

	if data, err := json.Marshal(unregister); err == nil {
		a.redisClient.Publish(a.ctx, "centerfire:agent:manager", data)
	}

	// Send shutdown event to Claude
	a.sendToClaude("shutdown", map[string]interface{}{
		"agent_id": a.config.AgentID,
		"status":   "shutdown_complete",
		"pid":      os.Getpid(),
	})

	// Close Redis connection
	a.redisClient.Close()

	// Remove Unix socket
	os.Remove(a.socketPath)

	// Remove PID file
	os.Remove(a.pidFile)

	// Update health to stopped
	a.updateHealthFile("stopped")

	fmt.Printf("✅ %s cleanup completed\n", a.config.AgentID)
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: ./agt-struct-2 <config-path>")
	}

	agent, err := NewAgent(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	if err := agent.Start(); err != nil {
		log.Fatalf("Agent failed: %v", err)
	}
}
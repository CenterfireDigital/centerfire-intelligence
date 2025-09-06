package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	
	"github.com/redis/go-redis/v9"
)

// StructAgent - Generated agent for STRUCT domain
type StructAgent struct {
	AgentID         string
	CID            string
	RequestChannel  string
	ResponseChannel string
	RedisClient    *redis.Client
	ctx            context.Context
}

// NewAgent - Create new STRUCT agent
func NewAgent() *StructAgent {
	// Connect to Redis container on port 6380
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	return &StructAgent{
		AgentID:         "AGT-STRUCT-1",
		CID:            "cid:centerfire:agent:01K4EAF14T65H4PHVHGFMPR500",
		RequestChannel:  "agent.struct.request",
		ResponseChannel: "agent.struct.response",
		RedisClient:    rdb,
		ctx:            context.Background(),
	}
}

// Start - Start listening for requests
func (a *StructAgent) Start() {
	fmt.Printf("%s starting...\n", a.AgentID)
	fmt.Printf("Listening on: %s\n", a.RequestChannel)
	
	// Test Redis connection
	_, err := a.RedisClient.Ping(a.ctx).Result()
	if err != nil {
		fmt.Printf("Failed to connect to Redis: %v\n", err)
		return
	}
	fmt.Printf("Connected to Redis successfully\n")
	
	// Subscribe to request channel
	pubsub := a.RedisClient.Subscribe(a.ctx, a.RequestChannel)
	defer pubsub.Close()
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Listen for messages
	ch := pubsub.Channel()
	
	fmt.Printf("%s ready - listening for requests\n", a.AgentID)
	
	for {
		select {
		case <-sigChan:
			fmt.Printf("\n%s shutting down...\n", a.AgentID)
			return
		case msg := <-ch:
			a.processMessage(msg.Payload)
		}
	}
}

// processMessage - Process incoming Redis message
func (a *StructAgent) processMessage(payload string) {
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		fmt.Printf("Error parsing request: %v\n", err)
		return
	}
	
	fmt.Printf("%s received request: %s\n", a.AgentID, request["action"])
	
	// Handle the request
	response := a.HandleRequest(request)
	
	// Send response back
	responseData, _ := json.Marshal(response)
	a.RedisClient.Publish(a.ctx, a.ResponseChannel, responseData)
}

// HandleRequest - Handle incoming request
func (a *StructAgent) HandleRequest(request map[string]interface{}) map[string]interface{} {
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
		}
	}
	
	// Route to appropriate handler
	switch action {
	case "create_structure":
		return a.handleCreateStructure(request)
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown action: %s", action),
		}
	}
}

// handleCreateStructure - Handle create_structure requests
func (a *StructAgent) handleCreateStructure(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	name, ok := params["name"].(string)
	if !ok {
		return map[string]interface{}{"error": "Name required"}
	}
	
	structType, ok := params["type"].(string)
	if !ok {
		return map[string]interface{}{"error": "Type required"}
	}
	
	template, ok := params["template"].(string)
	if !ok {
		template = "default"
	}
	
	cid, ok := params["cid"].(string)
	if !ok {
		return map[string]interface{}{"error": "CID required"}
	}
	
	fmt.Printf("%s: Creating %s structure for %s\n", a.AgentID, structType, name)
	
	// Create directory structure based on type
	var err error
	switch structType {
	case "capability":
		err = a.createCapabilityStructure(name, template, cid)
	default:
		return map[string]interface{}{"error": fmt.Sprintf("Unknown structure type: %s", structType)}
	}
	
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Failed to create structure: %v", err),
		}
	}
	
	// Delegate documentation creation to AGT-SEMDOC-1
	a.delegateDocumentation(name, structType, cid)
	
	return map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Created %s structure for %s", structType, name),
		"name":    name,
		"type":    structType,
		"cid":     cid,
	}
}

// createCapabilityStructure - Create directory structure for capability
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
  
# Implementation details  
implementation:
  language: "go"
  entry_point: "main.go"
  
# Metadata
metadata:
  version: "1.0.0"
  author: "AGT-STRUCT-1"
  tags: []
`, name, cid, template, time.Now().Format(time.RFC3339))
	
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		return fmt.Errorf("failed to create spec.yaml: %v", err)
	}
	
	// Create main.go
	mainPath := filepath.Join(capDir, "main.go")
	mainContent := fmt.Sprintf(`package main

import (
	"fmt"
)

// %s - Auto-generated capability
type %s struct {
	Name string
	CID  string
}

// New%s - Create new %s instance
func New%s() *%s {
	return &%s{
		Name: "%s",
		CID:  "%s",
	}
}

// Execute - Main capability execution
func (c *%s) Execute() error {
	fmt.Printf("Executing capability: %%s\\n", c.Name)
	// TODO: Implement capability logic
	return nil
}

func main() {
	cap := New%s()
	if err := cap.Execute(); err != nil {
		fmt.Printf("Error executing capability: %%v\\n", err)
	}
}
`, name, name, name, name, name, name, name, name, cid, name, name)
	
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		return fmt.Errorf("failed to create main.go: %v", err)
	}
	
	fmt.Printf("%s: Created capability structure at %s\n", a.AgentID, capDir)
	return nil
}

// delegateDocumentation - Delegate documentation creation to AGT-SEMDOC-1
func (a *StructAgent) delegateDocumentation(name, structType, cid string) {
	docRequest := map[string]interface{}{
		"from":   a.AgentID,
		"action": "create_documentation",
		"params": map[string]interface{}{
			"name":        name,
			"type":        structType,
			"cid":         cid,
			"path":        filepath.Join("capabilities", name),
			"format":      "markdown",
			"include_api": true,
		},
	}
	
	fmt.Printf("%s: Delegating documentation creation to AGT-SEMDOC-1 for %s\n", a.AgentID, name)
	
	// Send delegation request via Redis
	requestData, _ := json.Marshal(docRequest)
	err := a.RedisClient.Publish(a.ctx, "agent.semdoc.request", requestData).Err()
	if err != nil {
		fmt.Printf("%s: Error delegating to AGT-SEMDOC-1: %v\n", a.AgentID, err)
	} else {
		fmt.Printf("%s: Delegated documentation creation to AGT-SEMDOC-1 via Redis\n", a.AgentID)
	}
}

func main() {
	agent := NewAgent()
	agent.Start()
}

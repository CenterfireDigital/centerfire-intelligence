package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// NamingAgent - Generated agent for NAMING domain
type NamingAgent struct {
	AgentID        string
	CID           string
	RequestChannel string
	ResponseChannel string
}

// NewAgent - Create new NAMING agent
func NewAgent() *NamingAgent {
	return &NamingAgent{
		AgentID:         "AGT-NAMING-1",
		CID:            "cid:centerfire:agent:01K4EAF14SC75RSJG1G9WV2APV",
		RequestChannel:  "agent.naming.request",
		ResponseChannel: "agent.naming.response",
	}
}

// Start - Start listening for requests
func (a *NamingAgent) Start() {
	fmt.Printf("%s starting...\n", a.AgentID)
	fmt.Printf("Listening on: %s\n", a.RequestChannel)
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Simulate agent running with heartbeat
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-sigChan:
			fmt.Printf("\n%s shutting down...\n", a.AgentID)
			return
		case <-ticker.C:
			fmt.Printf("%s heartbeat - ready for naming requests\n", a.AgentID)
		}
	}
}

// HandleRequest - Handle incoming request
func (a *NamingAgent) HandleRequest(request map[string]interface{}) map[string]interface{} {
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
		}
	}
	
	// Route to appropriate handler
	switch action {
	case "allocate_capability":
		return a.handleAllocateCapability(request)
	case "allocate_module":
		return a.handleAllocateModule(request)
	case "allocate_function":
		return a.handleAllocateFunction(request)
	case "validate_name":
		return a.handleValidateName(request)
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown action: %s", action),
		}
	}
}

// handleAllocateCapability - Allocate new capability name and delegate structure creation
func (a *NamingAgent) handleAllocateCapability(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	domain, ok := params["domain"].(string)
	if !ok {
		return map[string]interface{}{"error": "Domain required"}
	}
	
	purpose, ok := params["purpose"].(string)
	if !ok {
		purpose = "Generated capability"
	}
	
	// Generate name (this is what naming agent does)
	capability := a.generateCapabilityName(domain, purpose)
	
	// Delegate structure creation to AGT-STRUCT-1
	structRequest := map[string]interface{}{
		"from":   a.AgentID,
		"action": "create_structure",
		"params": map[string]interface{}{
			"name":     capability["slug"],
			"type":     "capability", 
			"template": "default_capability",
			"cid":      capability["cid"],
		},
	}
	
	fmt.Printf("%s: Allocated %s, delegating structure creation to AGT-STRUCT-1\n", 
		a.AgentID, capability["slug"])
	
	// In real implementation, would send via Redis
	// For now, just log the delegation
	fmt.Printf("%s: Would send to AGT-STRUCT-1: %+v\n", a.AgentID, structRequest)
	
	return capability
}

// generateCapabilityName - Generate new capability name with sequence
func (a *NamingAgent) generateCapabilityName(domain, purpose string) map[string]interface{} {
	// In real implementation, would check Redis for sequence
	// For now, simulate sequence increment
	sequence := 1 // TODO: Get from Redis
	
	// Generate ULID for uniqueness
	now := time.Now()
	ulid := fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
	
	slug := fmt.Sprintf("CAP-%s-%d", domain, sequence)
	cid := fmt.Sprintf("cid:centerfire:capability:%s", ulid)
	directory := fmt.Sprintf("%s__%s", slug, ulid)
	
	return map[string]interface{}{
		"slug":      slug,
		"cid":       cid, 
		"directory": directory,
		"domain":    domain,
		"purpose":   purpose,
		"sequence":  sequence,
	}
}

// Placeholder handlers
func (a *NamingAgent) handleAllocateModule(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"error": "Module allocation not implemented yet"}
}

func (a *NamingAgent) handleAllocateFunction(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"error": "Function allocation not implemented yet"}
}

func (a *NamingAgent) handleValidateName(request map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"error": "Name validation not implemented yet"}
}

func main() {
	// Check if we're in test mode
	if len(os.Args) > 1 && os.Args[1] == "test" {
		testNaming()
		return
	}
	
	agent := NewAgent()
	agent.Start()
}

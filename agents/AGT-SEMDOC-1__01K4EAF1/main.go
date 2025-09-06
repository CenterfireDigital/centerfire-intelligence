package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// SemdocAgent - Generated agent for SEMDOC domain
type SemdocAgent struct {
	AgentID        string
	CID           string
	RequestChannel string
	ResponseChannel string
}

// NewAgent - Create new SEMDOC agent
func NewAgent() *SemdocAgent {
	return &SemdocAgent{
		AgentID:         "AGT-SEMDOC-1",
		CID:            "cid:centerfire:agent:01K4EAF14T65H4PHVHGK9KK708",
		RequestChannel:  "agent.semdoc.request",
		ResponseChannel: "agent.semdoc.response",
	}
}

// Start - Start listening for requests
func (a *SemdocAgent) Start() {
	fmt.Printf("%s starting...\n", a.AgentID)
	fmt.Printf("Listening on: %s\n", a.RequestChannel)
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Simulate agent running
	for {
		select {
		case <-sigChan:
			fmt.Printf("\n%s shutting down...\n", a.AgentID)
			return
		default:
			// Agent work would go here
			// For now, just indicate it's running
		}
	}
}

// HandleRequest - Handle incoming request
func (a *SemdocAgent) HandleRequest(request map[string]interface{}) map[string]interface{} {
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
		}
	}
	
	// Route to appropriate handler
	switch action {
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown action: %s", action),
		}
	}
}

func main() {
	agent := NewAgent()
	agent.Start()
}

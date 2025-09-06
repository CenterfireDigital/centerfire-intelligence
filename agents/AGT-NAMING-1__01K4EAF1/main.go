package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
func (a *NamingAgent) HandleRequest(request map[string]interface{}) map[string]interface{} {
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

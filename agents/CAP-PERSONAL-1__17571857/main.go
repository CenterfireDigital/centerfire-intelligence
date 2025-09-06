package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v2"
)

// PersonalAgent - Configurable Personal AI Orchestration Agent
type PersonalAgent struct {
	Config        *AgentConfig
	Orchestrator  *TaskOrchestrator
	Memory        *ConversationMemory
	RedisClient   *redis.Client
	httpClient    *http.Client
	ctx           context.Context
	sessionID     string
	sessionStart  time.Time
	ciContext     string  // CI agent manifest context
	conversationHistory []ConversationTurn
	mutex         sync.RWMutex
}

type AgentConfig struct {
	AgentInfo struct {
		CID           string `yaml:"cid"`
		SemanticName  string `yaml:"semantic_name"`
		DisplayName   string `yaml:"display_name"`
		Version       string `yaml:"version"`
	} `yaml:"agent_info"`
	
	Personality struct {
		Style          string `yaml:"style"`
		Verbosity      string `yaml:"verbosity"`
		UseEmojis      bool   `yaml:"use_emojis"`
		ResponseFormat string `yaml:"response_format"`
		QuietTerminal  bool   `yaml:"quiet_terminal"`
	} `yaml:"personality"`
	
	Models struct {
		ConversationModel string `yaml:"conversation_model"`
		DecisionModel     string `yaml:"decision_model"`
		Specialists       map[string]string `yaml:"specialists"`
	} `yaml:"models"`
	
	Integrations struct {
		ManagerEndpoint   string `yaml:"manager_endpoint"`
		CommanderEndpoint string `yaml:"commander_endpoint"`
		LocalLLMEndpoint  string `yaml:"local_llm_endpoint"`
		RedisEndpoint     string `yaml:"redis_endpoint"`
		WeaviateEndpoint  string `yaml:"weaviate_endpoint"`
	} `yaml:"integrations"`
	
	Orchestration struct {
		MaxParallelTasks    int  `yaml:"max_parallel_tasks"`
		TaskTimeoutSeconds  int  `yaml:"task_timeout_seconds"`
		AutoDelegate        bool `yaml:"auto_delegate"`
		ConversationMemory  bool `yaml:"conversation_memory"`
		LearningEnabled     bool `yaml:"learning_enabled"`
	} `yaml:"orchestration"`
	
	DecisionRules map[string]struct {
		Patterns []string `yaml:"patterns"`
		Handler  string   `yaml:"handler"`
	} `yaml:"decision_rules"`
	
	Conversation struct {
		GreetingMessage          string `yaml:"greeting_message"`
		ContextWindow           int    `yaml:"context_window"`
		AutoSaveConversations   bool   `yaml:"auto_save_conversations"`
		SessionTimeoutMinutes   int    `yaml:"session_timeout_minutes"`
	} `yaml:"conversation"`
}

type TaskOrchestrator struct {
	agent         *PersonalAgent
	executionPlan *ExecutionPlan
}

type ExecutionPlan struct {
	Tasks         []Task                 `json:"tasks"`
	Dependencies  map[string][]string    `json:"dependencies"`
	Results       map[string]interface{} `json:"results"`
	Status        string                 `json:"status"`
}

type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Handler     string                 `json:"handler"`
	Parameters  map[string]interface{} `json:"parameters"`
	Status      string                 `json:"status"`
	Result      interface{}           `json:"result,omitempty"`
	StartTime   *time.Time            `json:"start_time,omitempty"`
	Duration    *time.Duration        `json:"duration,omitempty"`
}

type ConversationTurn struct {
	User      string    `json:"user"`
	Assistant string    `json:"assistant"`
	Timestamp time.Time `json:"timestamp"`
	Context   string    `json:"context,omitempty"`
}

type ConversationMemory struct {
	history []ConversationTurn
	mutex   sync.RWMutex
}

type AgentResponse struct {
	Success   bool                   `json:"success"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	RequestID string                 `json:"request_id"`
}

// generateULID8 creates an 8-character ULID for session identification
func generateULID8() string {
	now := time.Now().UTC()
	return fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
}

// NewPersonalAgent creates a new configurable personal agent
func NewPersonalAgent(configPath string) (*PersonalAgent, error) {
	config := &AgentConfig{}
	
	// Load configuration
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}
	
	if err := yaml.Unmarshal(configFile, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	
	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.Integrations.RedisEndpoint,
		Password: "",
		DB:       0,
	})
	
	// Test Redis connection
	_, err = redisClient.Ping(context.Background()).Result()
	if err != nil {
		log.Printf("⚠️ Redis connection failed: %v (continuing without Redis)", err)
		redisClient = nil
	}
	
	// Generate hierarchical session ID: CAP-PERSONAL-1:{ulid8}
	sessionStart := time.Now().UTC()
	sessionID := fmt.Sprintf("CAP-PERSONAL-1:%s", generateULID8())
	
	agent := &PersonalAgent{
		Config:       config,
		RedisClient:  redisClient,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		ctx:          context.Background(),
		sessionID:    sessionID,
		sessionStart: sessionStart,
		Memory:       &ConversationMemory{},
	}
	
	agent.Orchestrator = &TaskOrchestrator{agent: agent}
	
	// Load CI agent context from protocol manifest
	agent.loadCIContext()
	
	// Register with AGT-MANAGER-1 for singleton enforcement (if Redis available)
	if agent.RedisClient != nil {
		if err := agent.registerWithManager(); err != nil {
			log.Printf("⚠️ Failed to register with manager: %v", err)
		}
		
		// Stream session start to Neo4j via W/N pipeline
		agent.streamSessionEvent("session_started")
	}
	
	return agent, nil
}

// ProcessUserInput - Main entry point for user requests
func (pa *PersonalAgent) ProcessUserInput(input string) (string, error) {
	pa.mutex.Lock()
	defer pa.mutex.Unlock()
	
	// Always log to W/N streams, conditionally to terminal
	if !pa.Config.Personality.QuietTerminal {
		log.Printf("📥 Processing: %s", input)
	}
	
	// Analyze input and create execution plan
	plan, err := pa.Orchestrator.CreateExecutionPlan(input)
	if err != nil {
		return "", fmt.Errorf("failed to create execution plan: %v", err)
	}
	
	// Execute the plan
	response, err := pa.Orchestrator.ExecutePlan(plan)
	if err != nil {
		return "", fmt.Errorf("failed to execute plan: %v", err)
	}
	
	// Store conversation if enabled (with W/N streaming)
	if pa.Config.Orchestration.ConversationMemory {
		pa.AddTurnWithStreaming(input, response)
	}
	
	return response, nil
}

// CreateExecutionPlan analyzes user input and creates a task execution plan
func (to *TaskOrchestrator) CreateExecutionPlan(userInput string) (*ExecutionPlan, error) {
	plan := &ExecutionPlan{
		Tasks:    []Task{},
		Results:  make(map[string]interface{}),
		Status:   "created",
	}
	
	// Fast path: Rule-based routing for common patterns
	if handler := to.matchDecisionRules(userInput); handler != "" {
		task := Task{
			ID:         "main_task",
			Type:       "single",
			Handler:    handler,
			Parameters: map[string]interface{}{"input": userInput},
			Status:     "pending",
		}
		plan.Tasks = append(plan.Tasks, task)
		return plan, nil
	}
	
	// Smart path: Use decision model for complex requests
	return to.createSmartPlan(userInput)
}

// matchDecisionRules checks input against configured patterns
func (to *TaskOrchestrator) matchDecisionRules(input string) string {
	inputLower := strings.ToLower(input)
	
	for ruleType, rule := range to.agent.Config.DecisionRules {
		for _, pattern := range rule.Patterns {
			if strings.Contains(inputLower, strings.ToLower(pattern)) {
				if !to.agent.Config.Personality.QuietTerminal {
					log.Printf("🎯 Rule match: %s → %s", ruleType, rule.Handler)
				}
				return rule.Handler
			}
		}
	}
	
	return ""
}

// createSmartPlan uses conversation model to analyze complex requests
func (to *TaskOrchestrator) createSmartPlan(userInput string) (*ExecutionPlan, error) {
	// For now, default to conversation model
	plan := &ExecutionPlan{
		Tasks: []Task{{
			ID:         "conversation_task",
			Type:       "conversation",
			Handler:    "conversation",
			Parameters: map[string]interface{}{"input": userInput},
			Status:     "pending",
		}},
		Results: make(map[string]interface{}),
		Status:  "created",
	}
	
	return plan, nil
}

// ExecutePlan executes the tasks in the execution plan
func (to *TaskOrchestrator) ExecutePlan(plan *ExecutionPlan) (string, error) {
	if !to.agent.Config.Personality.QuietTerminal {
		log.Printf("🚀 Executing plan with %d tasks", len(plan.Tasks))
	}
	plan.Status = "executing"
	
	var finalResponse strings.Builder
	
	for i, task := range plan.Tasks {
		if !to.agent.Config.Personality.QuietTerminal {
			log.Printf("📋 Executing task %d: %s (%s)", i+1, task.ID, task.Handler)
		}
		
		startTime := time.Now()
		task.StartTime = &startTime
		task.Status = "running"
		
		result, err := to.executeTask(&task)
		duration := time.Since(startTime)
		task.Duration = &duration
		
		if err != nil {
			task.Status = "failed"
			task.Result = fmt.Sprintf("Error: %v", err)
			if !to.agent.Config.Personality.QuietTerminal {
				log.Printf("❌ Task %s failed: %v", task.ID, err)
			}
			continue
		}
		
		task.Status = "completed"
		task.Result = result
		plan.Results[task.ID] = result
		
		// Skip completion timing - just continue
		
		// Add result to final response
		if resultStr, ok := result.(string); ok {
			finalResponse.WriteString(resultStr)
		}
	}
	
	plan.Status = "completed"
	response := finalResponse.String()
	
	if response == "" {
		response = "I've processed your request, but didn't generate a specific response."
	}
	
	return response, nil
}

// executeTask executes a single task based on its handler
func (to *TaskOrchestrator) executeTask(task *Task) (interface{}, error) {
	switch task.Handler {
	case "commander":
		return to.executeCommanderTask(task)
	case "file_analyst":
		return to.executeSpecialistTask(task, "file_search")
	case "knowledge_curator":
		return to.executeSpecialistTask(task, "weaviate_query") 
	case "workflow_manager":
		return to.executeSpecialistTask(task, "todo_update")
	case "conversation":
		return to.executeConversationTask(task)
	default:
		return nil, fmt.Errorf("unknown handler: %s", task.Handler)
	}
}

// executeCommanderTask sends requests to System Commander
func (to *TaskOrchestrator) executeCommanderTask(task *Task) (interface{}, error) {
	input, ok := task.Parameters["input"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid input parameter")
	}
	
	// Use LLM to convert natural language intent to shell command
	command, err := to.generateShellCommand(input)
	if err != nil {
		log.Printf("⚠️ LLM command generation failed, using fallback: %v", err)
		command = input // Fallback to raw input
	}
	
	requestData := map[string]interface{}{
		"client_id":  "personal_agent",
		"command":    command,
		"request_id": fmt.Sprintf("apollo_%d", time.Now().UnixNano()),
	}
	
	return to.makeHTTPRequest(to.agent.Config.Integrations.CommanderEndpoint+"/execute_command", requestData)
}

// executeSpecialistTask sends requests to Local LLM specialists
func (to *TaskOrchestrator) executeSpecialistTask(task *Task, actionType string) (interface{}, error) {
	input, ok := task.Parameters["input"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid input parameter")
	}
	
	requestData := map[string]interface{}{
		"prompt":    input,
		"task_type": actionType,
	}
	
	return to.makeHTTPRequest(to.agent.Config.Integrations.LocalLLMEndpoint+"/"+actionType, requestData)
}

// executeConversationTask handles general conversation
func (to *TaskOrchestrator) executeConversationTask(task *Task) (interface{}, error) {
	input, ok := task.Parameters["input"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid input parameter")
	}
	
	// Use configured conversation model via direct Ollama call
	return to.queryOllama(to.agent.Config.Models.ConversationModel, input)
}

// generateShellCommand converts natural language intent to shell command using LLM
func (to *TaskOrchestrator) generateShellCommand(input string) (string, error) {
	// Use decision model (gemma:2b) for lightweight command generation
	model := to.agent.Config.Models.DecisionModel
	if model == "" {
		model = "gemma:2b" // Fallback
	}

	prompt := fmt.Sprintf(`Convert this natural language request to a single shell command. Return ONLY the command, no explanations.

Request: %s

Rules:
- For counting files: use find . -name "*.ext" | wc -l
- For listing files: use find . -name "*.ext" or ls
- For system info: use appropriate commands like ps, df, etc.
- For text operations: use grep, sed, awk as needed
- Return only the bare command, no markdown or quotes

Command:`, input)

	response, err := to.queryOllama(model, prompt)
	if err != nil {
		return "", fmt.Errorf("ollama query failed: %v", err)
	}

	// Clean up response - remove any markdown, quotes, or extra text
	command := strings.TrimSpace(response)
	command = strings.Trim(command, "`\"'")
	
	// Extract just the command if there's extra text
	lines := strings.Split(command, "\n")
	if len(lines) > 0 {
		command = strings.TrimSpace(lines[0])
	}

	log.Printf("🧠 LLM converted '%s' → '%s'", input, command)
	return command, nil
}

// loadCIContext loads agent information from the CI protocol file
func (pa *PersonalAgent) loadCIContext() {
	protocolPath := "../../DIR-SYS-1__genesis/claude-agent-protocol.yaml"
	content, err := os.ReadFile(protocolPath)
	if err != nil {
		log.Printf("⚠️ Could not load CI protocol: %v (using basic context)", err)
		pa.ciContext = "Basic CI system - System Commander and Local LLM agents available"
		return
	}
	
	// Extract key agent information for context
	lines := strings.Split(string(content), "\n")
	var activeAgents []string
	inActiveSection := false
	
	for _, line := range lines {
		if strings.Contains(line, "active_agents:") {
			inActiveSection = true
			continue
		}
		if inActiveSection && strings.HasPrefix(line, "  - id:") {
			agentID := strings.Trim(strings.Split(line, ":")[1], " \"")
			activeAgents = append(activeAgents, agentID)
		}
		if inActiveSection && !strings.HasPrefix(line, " ") && line != "" {
			break
		}
	}
	
	pa.ciContext = fmt.Sprintf("Active CI agents: %s", strings.Join(activeAgents, ", "))
}

// queryOllama makes direct requests to Ollama
func (to *TaskOrchestrator) queryOllama(model, prompt string) (string, error) {
	systemPrompt := fmt.Sprintf(`You are %s, the Centerfire Intelligence orchestrator for autonomous software engineering.

ROLE: Multi-agent task coordinator. Analyze requests and route to specialized CI agents.

%s

CAPABILITIES:
- AGT-SYSTEM-COMMANDER-1: System commands, process management, file operations
- AGT-LOCAL-LLM-1: Code analysis, documentation, intelligent file search
- AGT-SEMANTIC-1: Knowledge queries, concept storage, similarity search
- AGT-NAMING-1: Resource allocation, semantic naming
- Direct conversation: Simple questions, explanations, status updates

INSTRUCTIONS: Determine the best approach for each request. Route complex tasks to appropriate agents. Provide direct answers for simple queries.

RESPONSE STYLE: %s verbosity, task-focused, no unnecessary explanation.

User: %s`, 
		to.agent.Config.AgentInfo.DisplayName,
		to.agent.ciContext,
		to.agent.Config.Personality.Verbosity,
		prompt)

	requestData := map[string]interface{}{
		"model":  model,
		"prompt": systemPrompt,
		"stream": false,
	}
	
	reqBody, _ := json.Marshal(requestData)
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	var ollamaResp map[string]interface{}
	json.Unmarshal(body, &ollamaResp)
	
	if response, ok := ollamaResp["response"].(string); ok {
		return response, nil
	}
	
	return "", fmt.Errorf("no response from model")
}

// makeHTTPRequest makes HTTP requests to agent endpoints
func (to *TaskOrchestrator) makeHTTPRequest(endpoint string, data map[string]interface{}) (interface{}, error) {
	reqBody, _ := json.Marshal(data)
	
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-ID", "personal_agent")
	
	resp, err := to.agent.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	
	if success, ok := result["success"].(bool); ok && success {
		if data, ok := result["data"]; ok {
			return data, nil
		}
	}
	
	if errorMsg, ok := result["error"].(string); ok {
		return nil, fmt.Errorf("agent error: %s", errorMsg)
	}
	
	return string(body), nil
}

// AddTurn adds a conversation turn to memory
func (cm *ConversationMemory) AddTurn(user, assistant string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	turn := ConversationTurn{
		User:      user,
		Assistant: assistant,
		Timestamp: time.Now(),
	}
	
	cm.history = append(cm.history, turn)
	// Always log to W/N - fix struct reference
	log.Printf("💾 Stored conversation turn (W/N)")
}

// StartTerminalInterface starts the interactive terminal
func (pa *PersonalAgent) StartTerminalInterface() {
	displayName := pa.Config.AgentInfo.DisplayName
	greeting := strings.ReplaceAll(pa.Config.Conversation.GreetingMessage, "${display_name}", displayName)
	
	fmt.Printf("\n🤖 %s\n", greeting)
	fmt.Printf("📋 Agent ID: %s\n", pa.Config.AgentInfo.SemanticName)
	fmt.Printf("🧠 Conversation Model: %s\n", pa.Config.Models.ConversationModel)
	fmt.Printf("⚡ Type 'exit' to quit, 'help' for commands\n\n")
	
	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		
		if input == "exit" || input == "quit" {
			fmt.Printf("\n👋 Goodbye! %s signing off.\n", displayName)
			break
		}
		
		if input == "help" {
			pa.showHelp()
			continue
		}
		
		// Process the input
		fmt.Printf("\n%s: ", displayName)
		response, err := pa.ProcessUserInput(input)
		if err != nil {
			fmt.Printf("❌ Error: %v\n\n", err)
			continue
		}
		
		fmt.Printf("%s\n\n", response)
	}
}

func (pa *PersonalAgent) showHelp() {
	fmt.Printf("\n📖 %s Help:\n", pa.Config.AgentInfo.DisplayName)
	fmt.Printf("• I can execute system commands (check if Redis is running)\n")
	fmt.Printf("• Search and analyze files (find Redis config files)\n") 
	fmt.Printf("• Answer questions using local knowledge\n")
	fmt.Printf("• Help with todos and workflow management\n")
	fmt.Printf("• Have general conversations\n")
	fmt.Printf("• Type 'exit' to quit\n\n")
}

// registerWithManager registers APOLLO with AGT-MANAGER-1 for singleton enforcement
func (pa *PersonalAgent) registerWithManager() error {
	registrationData := map[string]interface{}{
		"agent_name":   "CAP-PERSONAL-1",
		"agent_type":   "personal_agent",
		"pid":          os.Getpid(),
		"session_id":   pa.sessionID,
		"capabilities": []string{"conversation", "orchestration", "memory_management", "task_routing"},
		"channels":     []string{"agent.personal.request"},
		"contracts":    []string{"standalone"},
	}
	
	data, _ := json.Marshal(registrationData)
	return pa.RedisClient.Publish(pa.ctx, "agent.manager.register", string(data)).Err()
}

// streamSessionEvent streams session lifecycle events to W/N pipeline for Neo4j
func (pa *PersonalAgent) streamSessionEvent(eventType string) {
	if pa.RedisClient == nil {
		return
	}
	
	sessionData := map[string]interface{}{
		"session_id":   pa.sessionID,
		"agent_id":     "CAP-PERSONAL-1",
		"event_type":   eventType,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"display_name": pa.Config.AgentInfo.DisplayName,
		"personality":  pa.Config.Personality.Style,
		"models": map[string]interface{}{
			"conversation": pa.Config.Models.ConversationModel,
			"decision":     pa.Config.Models.DecisionModel,
		},
		"session_start": pa.sessionStart.Format(time.RFC3339),
	}
	
	// Stream to Neo4j consumer for temporal relationship tracking
	data, _ := json.Marshal(sessionData)
	pa.RedisClient.XAdd(pa.ctx, &redis.XAddArgs{
		Stream: "centerfire:neo4j:sessions",
		Values: map[string]interface{}{
			"data": string(data),
		},
	})
	
	if !pa.Config.Personality.QuietTerminal {
		log.Printf("📊 Streamed %s event for session %s", eventType, pa.sessionID)
	}
}

// Enhanced AddTurn with W/N streaming for conversation persistence
func (pa *PersonalAgent) AddTurnWithStreaming(user, assistant string) {
	// Add to local memory
	pa.Memory.AddTurn(user, assistant)
	
	// Stream to W/N pipeline if Redis available
	if pa.RedisClient != nil {
		conversationData := map[string]interface{}{
			"session_id":  pa.sessionID,
			"user":        user,
			"assistant":   assistant,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"agent_id":    "CAP-PERSONAL-1",
			"turn_count":  len(pa.conversationHistory) + 1,
		}
		
		// Stream to Weaviate for semantic storage
		data, _ := json.Marshal(conversationData)
		pa.RedisClient.XAdd(pa.ctx, &redis.XAddArgs{
			Stream: "centerfire:semantic:conversations",
			Values: map[string]interface{}{
				"data": string(data),
			},
		})
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: ./apollo <config-path>")
	}
	
	configPath := os.Args[1]
	
	log.Printf("🚀 Starting Personal AI Agent...")
	log.Printf("📁 Loading config from: %s", configPath)
	
	agent, err := NewPersonalAgent(configPath)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	
	log.Printf("✅ %s initialized successfully (Session: %s)", 
		agent.Config.AgentInfo.DisplayName, agent.sessionID)
	
	// Start terminal interface
	agent.StartTerminalInterface()
	
	// Stream session end when exiting
	if agent.RedisClient != nil {
		agent.streamSessionEvent("session_ended")
	}
}
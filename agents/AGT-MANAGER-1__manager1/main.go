package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	
	"github.com/redis/go-redis/v9"
)

type AgentManager struct {
	AgentID     string
	RedisClient *redis.Client
	ctx         context.Context
	agents      map[string]*AgentProcess
	managerID   string // Unique manager instance ID
	agentRegistry map[string]*AgentDefinition // Agent registry for ephemeral lifecycle
}

type AgentProcess struct {
	Name      string
	Directory string
	Process   *exec.Cmd
	Running   bool
	StartTime time.Time
	SessionID string
	AgentType AgentType // persistent or ephemeral
	TaskID    string    // for ephemeral agents
}

type AgentType string

const (
	PersistentAgent AgentType = "persistent"
	EphemeralAgent  AgentType = "ephemeral"
)

type AgentDefinition struct {
	Name        string    `json:"name"`
	Directory   string    `json:"directory"`
	Type        AgentType `json:"type"`
	Capabilities []string `json:"capabilities"`
	Description string    `json:"description"`
	AutoShutdown bool     `json:"auto_shutdown"` // for ephemeral agents
	MaxRuntime  int64     `json:"max_runtime"`   // seconds, 0 = unlimited
}

type AgentRequest struct {
	RequestType string                 `json:"request_type"`
	AgentName   string                 `json:"agent_name,omitempty"`
	Action      string                 `json:"action,omitempty"`
	SessionData map[string]interface{} `json:"session_data,omitempty"`
	TaskID      string                 `json:"task_id,omitempty"`      // for ephemeral agents
	TaskData    map[string]interface{} `json:"task_data,omitempty"`    // task-specific data
	AgentDef    *AgentDefinition       `json:"agent_def,omitempty"`    // for registering agents
}

func NewAgentManager() *AgentManager {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	// Generate unique manager ID for instance tracking
	managerID := fmt.Sprintf("manager-%d", time.Now().Unix())

	am := &AgentManager{
		AgentID:     "AGT-MANAGER-1",
		RedisClient: rdb,
		ctx:         context.Background(),
		agents:      make(map[string]*AgentProcess),
		managerID:   managerID,
		agentRegistry: make(map[string]*AgentDefinition),
	}
	
	// Initialize agent registry with known agents
	am.initializeAgentRegistry()
	
	return am
}

func (am *AgentManager) Start() {
	fmt.Printf("%s starting...\n", am.AgentID)
	fmt.Printf("Listening on: centerfire:agent:manager\n")

	// Test Redis connection
	_, err := am.RedisClient.Ping(am.ctx).Result()
	if err != nil {
		fmt.Printf("Failed to connect to Redis: %v\n", err)
		return
	}
	fmt.Println("Connected to Redis successfully")

	// Subscribe to agent management requests
	pubsub := am.RedisClient.Subscribe(am.ctx, "centerfire:agent:manager")
	defer pubsub.Close()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("%s ready - listening for agent management requests\n", am.AgentID)

	for {
		select {
		case <-sigChan:
			fmt.Printf("\n%s shutting down...\n", am.AgentID)
			am.shutdown()
			return

		default:
			// Process agent management requests
			msg, err := pubsub.ReceiveTimeout(am.ctx, time.Second*1)
			if err != nil {
				continue
			}

			switch m := msg.(type) {
			case *redis.Message:
				am.processRequest(m.Payload)
			}
		}
	}
}

func (am *AgentManager) processRequest(payload string) {
	fmt.Printf("%s received request: %s\n", am.AgentID, payload)

	var request AgentRequest
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		fmt.Printf("Error parsing request: %v\n", err)
		return
	}

	switch request.RequestType {
	case "restart_agent":
		am.handleRestartAgent(request)
	case "stop_agent":
		am.handleStopAgent(request)
	case "start_agent":
		am.handleStartAgent(request)
	case "list_agents":
		am.handleListAgents(request)
	case "agent_status":
		am.handleAgentStatus(request)
	case "check_collisions":
		am.handleCheckCollisions(request)
	case "session_restore":
		am.handleSessionRestore(request)
	case "register_agent":
		am.handleRegisterAgent(request)
	case "spawn_ephemeral":
		am.handleSpawnEphemeral(request)
	case "list_registry":
		am.handleListRegistry(request)
	case "get_agent_definition":
		am.handleGetAgentDefinition(request)
	default:
		fmt.Printf("Unknown request type: %s\n", request.RequestType)
	}
}

func (am *AgentManager) handleRestartAgent(request AgentRequest) {
	agentName := request.AgentName
	fmt.Printf("%s: Restarting agent %s\n", am.AgentID, agentName)

	// Stop existing agent
	if process, exists := am.agents[agentName]; exists {
		am.stopAgentProcess(process)
	}

	// Start agent with session awareness
	if err := am.startAgent(agentName, request.SessionData); err != nil {
		fmt.Printf("Error restarting %s: %v\n", agentName, err)
		am.publishResponse(map[string]interface{}{
			"status": "error",
			"agent":  agentName,
			"error":  err.Error(),
		})
		return
	}

	fmt.Printf("%s: Agent %s restarted successfully\n", am.AgentID, agentName)
	am.publishResponse(map[string]interface{}{
		"status": "restarted",
		"agent":  agentName,
	})
}

func (am *AgentManager) handleStopAgent(request AgentRequest) {
	agentName := request.AgentName
	fmt.Printf("%s: Stopping agent %s\n", am.AgentID, agentName)

	if process, exists := am.agents[agentName]; exists {
		am.stopAgentProcess(process)
		delete(am.agents, agentName)
		
		am.publishResponse(map[string]interface{}{
			"status": "stopped",
			"agent":  agentName,
		})
	} else {
		am.publishResponse(map[string]interface{}{
			"status": "not_found",
			"agent":  agentName,
		})
	}
}

func (am *AgentManager) handleStartAgent(request AgentRequest) {
	agentName := request.AgentName
	fmt.Printf("%s: Starting agent %s\n", am.AgentID, agentName)

	if err := am.startAgent(agentName, request.SessionData); err != nil {
		am.publishResponse(map[string]interface{}{
			"status": "error",
			"agent":  agentName,
			"error":  err.Error(),
		})
		return
	}

	am.publishResponse(map[string]interface{}{
		"status": "started",
		"agent":  agentName,
	})
}

func (am *AgentManager) handleListAgents(request AgentRequest) {
	agents := make([]map[string]interface{}, 0)
	
	for name, process := range am.agents {
		agents = append(agents, map[string]interface{}{
			"name":       name,
			"running":    process.Running,
			"start_time": process.StartTime,
			"session_id": process.SessionID,
			"type":       process.AgentType,
			"task_id":    process.TaskID,
		})
	}

	am.publishResponse(map[string]interface{}{
		"status": "ok",
		"agents": agents,
	})
}

func (am *AgentManager) handleAgentStatus(request AgentRequest) {
	agentName := request.AgentName
	if process, exists := am.agents[agentName]; exists {
		am.publishResponse(map[string]interface{}{
			"status":     "ok",
			"agent":      agentName,
			"running":    process.Running,
			"start_time": process.StartTime,
			"session_id": process.SessionID,
			"type":       process.AgentType,
			"task_id":    process.TaskID,
		})
	} else {
		am.publishResponse(map[string]interface{}{
			"status": "not_found",
			"agent":  agentName,
		})
	}
}

func (am *AgentManager) handleCheckCollisions(request AgentRequest) {
	fmt.Printf("%s: Checking agent collisions\n", am.AgentID)

	collisions := make(map[string]interface{})
	singletonAgents := am.getSingletonAgents()

	// Check each singleton agent for multiple instances
	for agentName := range singletonAgents {
		isRunning, details, err := am.isAgentRunning(agentName)
		if err != nil {
			collisions[agentName] = map[string]interface{}{
				"error": err.Error(),
			}
			continue
		}

		collisions[agentName] = map[string]interface{}{
			"running":     isRunning,
			"singleton":   true,
			"details":     details,
		}
	}

	am.publishResponse(map[string]interface{}{
		"status":     "ok",
		"collisions": collisions,
		"manager_id": am.managerID,
	})
}

func (am *AgentManager) handleSessionRestore(request AgentRequest) {
	sessionID := request.SessionData["session_id"].(string)
	fmt.Printf("%s: Restoring session %s\n", am.AgentID, sessionID)

	// Get session data from Redis
	sessionKey := fmt.Sprintf("centerfire.dev.sessions:%s", sessionID)
	sessionJSON, err := am.RedisClient.Get(am.ctx, sessionKey).Result()
	if err != nil {
		am.publishResponse(map[string]interface{}{
			"status":     "error",
			"session_id": sessionID,
			"error":      "Session not found",
		})
		return
	}

	var sessionData map[string]interface{}
	json.Unmarshal([]byte(sessionJSON), &sessionData)

	// Restart agents with session context
	agentList := sessionData["agents"].([]interface{})
	for _, agent := range agentList {
		agentName := agent.(string)
		am.startAgent(agentName, map[string]interface{}{
			"session_id":      sessionID,
			"restore_context": true,
		})
	}

	am.publishResponse(map[string]interface{}{
		"status":     "restored",
		"session_id": sessionID,
		"agents":     agentList,
	})
}

// Agent Registry Management
func (am *AgentManager) initializeAgentRegistry() {
	// Register known persistent agents
	am.agentRegistry["AGT-NAMING-1"] = &AgentDefinition{
		Name:        "AGT-NAMING-1",
		Directory:   "/Users/larrydiffey/projects/CenterfireIntelligence/agents/AGT-NAMING-1__01K4EAF1",
		Type:        PersistentAgent,
		Capabilities: []string{"allocate_capability", "allocate_module", "allocate_namespace", "manage_sequences"},
		Description: "Core naming and identifier allocation service",
		AutoShutdown: false,
		MaxRuntime:  0, // unlimited
	}
	
	am.agentRegistry["AGT-SEMANTIC-1"] = &AgentDefinition{
		Name:        "AGT-SEMANTIC-1",
		Directory:   "/Users/larrydiffey/projects/CenterfireIntelligence/agents/AGT-SEMANTIC-1__01K4EAF1",
		Type:        PersistentAgent,
		Capabilities: []string{"semantic_similarity", "store_concept", "query_concepts"},
		Description: "Core semantic analysis and storage service",
		AutoShutdown: false,
		MaxRuntime:  0,
	}
	
	am.agentRegistry["AGT-STRUCT-1"] = &AgentDefinition{
		Name:        "AGT-STRUCT-1",
		Directory:   "/Users/larrydiffey/projects/CenterfireIntelligence/agents/AGT-STRUCT-1__01K4EAF1",
		Type:        PersistentAgent,
		Capabilities: []string{"create_structure", "delegate_documentation"},
		Description: "Core directory and file structure management service",
		AutoShutdown: false,
		MaxRuntime:  0,
	}
	
	am.agentRegistry["AGT-MANAGER-1"] = &AgentDefinition{
		Name:        "AGT-MANAGER-1",
		Directory:   "/Users/larrydiffey/projects/CenterfireIntelligence/agents/AGT-MANAGER-1__manager1",
		Type:        PersistentAgent,
		Capabilities: []string{"singleton_enforcement", "collision_detection", "process_monitoring", "agent_registry"},
		Description: "Agent lifecycle and process management service",
		AutoShutdown: false,
		MaxRuntime:  0,
	}
	
	// Register known ephemeral agents
	am.agentRegistry["AGT-CLEANUP-1"] = &AgentDefinition{
		Name:        "AGT-CLEANUP-1",
		Directory:   "/Users/larrydiffey/projects/CenterfireIntelligence/agents/AGT-CLEANUP-1__17571335",
		Type:        EphemeralAgent,
		Capabilities: []string{"cleanup_weaviate_classes", "cleanup_pre_semantic_data", "direct_cleanup_mode"},
		Description: "Data cleanup and maintenance service",
		AutoShutdown: true,
		MaxRuntime:  300, // 5 minutes max runtime
	}
	
	am.agentRegistry["AGT-SEMDOC-1"] = &AgentDefinition{
		Name:        "AGT-SEMDOC-1",
		Directory:   "/Users/larrydiffey/projects/CenterfireIntelligence/agents/AGT-SEMDOC-1__01K4EAF1",
		Type:        EphemeralAgent,
		Capabilities: []string{"generate_documentation", "semantic_documentation"},
		Description: "Documentation generation service",
		AutoShutdown: true,
		MaxRuntime:  600, // 10 minutes max runtime
	}
	
	am.agentRegistry["AGT-CODING-1"] = &AgentDefinition{
		Name:        "AGT-CODING-1",
		Directory:   "/Users/larrydiffey/projects/CenterfireIntelligence/agents/AGT-CODING-1__01K4EAF1",
		Type:        EphemeralAgent,
		Capabilities: []string{"generate_code", "refactor_code", "code_analysis"},
		Description: "Code generation and analysis service",
		AutoShutdown: true,
		MaxRuntime:  1800, // 30 minutes max runtime
	}
	
	fmt.Printf("%s: Agent registry initialized with %d agent definitions\n", am.AgentID, len(am.agentRegistry))
}

// Agent Registry Request Handlers
func (am *AgentManager) handleRegisterAgent(request AgentRequest) {
	if request.AgentDef == nil {
		am.publishResponse(map[string]interface{}{
			"status": "error",
			"error":  "No agent definition provided",
		})
		return
	}
	
	agentDef := request.AgentDef
	fmt.Printf("%s: Registering agent %s (type: %s)\n", am.AgentID, agentDef.Name, agentDef.Type)
	
	am.agentRegistry[agentDef.Name] = agentDef
	
	am.publishResponse(map[string]interface{}{
		"status":     "registered",
		"agent_name": agentDef.Name,
		"agent_type": agentDef.Type,
	})
}

func (am *AgentManager) handleSpawnEphemeral(request AgentRequest) {
	agentName := request.AgentName
	taskID := request.TaskID
	
	if taskID == "" {
		taskID = fmt.Sprintf("task_%d", time.Now().Unix())
	}
	
	fmt.Printf("%s: Spawning ephemeral agent %s for task %s\n", am.AgentID, agentName, taskID)
	
	// Check if agent is registered
	agentDef, exists := am.agentRegistry[agentName]
	if !exists {
		am.publishResponse(map[string]interface{}{
			"status":   "error",
			"error":    fmt.Sprintf("Agent %s not registered", agentName),
			"task_id":  taskID,
		})
		return
	}
	
	// Ensure agent is ephemeral
	if agentDef.Type != EphemeralAgent {
		am.publishResponse(map[string]interface{}{
			"status":  "error",
			"error":   fmt.Sprintf("Agent %s is not ephemeral (type: %s)", agentName, agentDef.Type),
			"task_id": taskID,
		})
		return
	}
	
	// Create unique instance name for ephemeral agent
	instanceName := fmt.Sprintf("%s_%s", agentName, taskID)
	
	// Start ephemeral agent
	if err := am.startEphemeralAgent(instanceName, agentDef, taskID, request.TaskData); err != nil {
		am.publishResponse(map[string]interface{}{
			"status":  "error",
			"error":   err.Error(),
			"task_id": taskID,
		})
		return
	}
	
	am.publishResponse(map[string]interface{}{
		"status":       "spawned",
		"agent_name":   agentName,
		"instance_name": instanceName,
		"task_id":      taskID,
	})
}

func (am *AgentManager) handleListRegistry(request AgentRequest) {
	registry := make([]map[string]interface{}, 0)
	
	for name, def := range am.agentRegistry {
		registry = append(registry, map[string]interface{}{
			"name":         name,
			"type":         def.Type,
			"capabilities": def.Capabilities,
			"description":  def.Description,
			"auto_shutdown": def.AutoShutdown,
			"max_runtime":  def.MaxRuntime,
		})
	}
	
	am.publishResponse(map[string]interface{}{
		"status":   "ok",
		"registry": registry,
	})
}

func (am *AgentManager) handleGetAgentDefinition(request AgentRequest) {
	agentName := request.AgentName
	if def, exists := am.agentRegistry[agentName]; exists {
		am.publishResponse(map[string]interface{}{
			"status":       "ok",
			"agent_name":   agentName,
			"type":         def.Type,
			"capabilities": def.Capabilities,
			"description":  def.Description,
			"auto_shutdown": def.AutoShutdown,
			"max_runtime":  def.MaxRuntime,
		})
	} else {
		am.publishResponse(map[string]interface{}{
			"status": "not_found",
			"agent":  agentName,
		})
	}
}

// Agent instance tracking and collision detection methods
func (am *AgentManager) getSingletonAgents() map[string]bool {
	// Define which agents should have only one instance running
	return map[string]bool{
		"AGT-NAMING-1":   true, // Core naming service - must be singleton
		"AGT-SEMANTIC-1": true, // Core semantic service - must be singleton  
		"AGT-STRUCT-1":   true, // Core structure service - must be singleton
		"AGT-MANAGER-1":  true, // Manager itself - must be singleton
	}
}

func (am *AgentManager) isAgentRunning(agentName string) (bool, string, error) {
	// Check Redis for active agent instances
	instanceKey := fmt.Sprintf("centerfire:agents:active:%s", agentName)
	instances, err := am.RedisClient.HGetAll(am.ctx, instanceKey).Result()
	if err != nil && err != redis.Nil {
		return false, "", err
	}

	if len(instances) == 0 {
		return false, "", nil
	}

	// Check if any instances are still alive by trying to verify heartbeat
	activeInstances := []string{}
	for instanceID, data := range instances {
		var instanceInfo map[string]interface{}
		json.Unmarshal([]byte(data), &instanceInfo)
		
		// Check heartbeat timestamp (if older than 30 seconds, consider dead)
		if heartbeat, ok := instanceInfo["heartbeat"].(float64); ok {
			if time.Now().Unix()-int64(heartbeat) < 30 {
				activeInstances = append(activeInstances, instanceID)
			} else {
				// Clean up dead instance
				am.RedisClient.HDel(am.ctx, instanceKey, instanceID)
			}
		}
	}

	if len(activeInstances) > 0 {
		return true, fmt.Sprintf("Active instances: %v", activeInstances), nil
	}

	return false, "", nil
}

func (am *AgentManager) registerAgentInstance(agentName string, sessionID string) error {
	instanceKey := fmt.Sprintf("centerfire:agents:active:%s", agentName)
	instanceID := fmt.Sprintf("%s-%s", am.managerID, agentName)
	
	instanceData := map[string]interface{}{
		"agent_name":    agentName,
		"instance_id":   instanceID,
		"manager_id":    am.managerID,
		"session_id":    sessionID,
		"started_at":    time.Now().Unix(),
		"heartbeat":     time.Now().Unix(),
	}

	instanceJSON, _ := json.Marshal(instanceData)
	return am.RedisClient.HSet(am.ctx, instanceKey, instanceID, string(instanceJSON)).Err()
}

func (am *AgentManager) unregisterAgentInstance(agentName string) error {
	instanceKey := fmt.Sprintf("centerfire:agents:active:%s", agentName)
	instanceID := fmt.Sprintf("%s-%s", am.managerID, agentName)
	return am.RedisClient.HDel(am.ctx, instanceKey, instanceID).Err()
}

func (am *AgentManager) updateHeartbeat(agentName string) error {
	instanceKey := fmt.Sprintf("centerfire:agents:active:%s", agentName)
	instanceID := fmt.Sprintf("%s-%s", am.managerID, agentName)
	
	// Get existing data
	data, err := am.RedisClient.HGet(am.ctx, instanceKey, instanceID).Result()
	if err != nil {
		return err
	}

	var instanceData map[string]interface{}
	json.Unmarshal([]byte(data), &instanceData)
	instanceData["heartbeat"] = time.Now().Unix()

	instanceJSON, _ := json.Marshal(instanceData)
	return am.RedisClient.HSet(am.ctx, instanceKey, instanceID, string(instanceJSON)).Err()
}

func (am *AgentManager) startAgent(agentName string, sessionData map[string]interface{}) error {
	// Check for singleton collision before starting
	singletonAgents := am.getSingletonAgents()
	if singletonAgents[agentName] {
		isRunning, details, err := am.isAgentRunning(agentName)
		if err != nil {
			return fmt.Errorf("error checking agent status: %v", err)
		}
		if isRunning {
			return fmt.Errorf("agent %s is already running (singleton constraint): %s", agentName, details)
		}
	}

	// Get agent definition from registry
	agentDef, exists := am.agentRegistry[agentName]
	if !exists {
		return fmt.Errorf("unknown agent: %s (not in registry)", agentName)
	}
	
	directory := agentDef.Directory

	// Create command to run the agent
	cmd := exec.Command("go", "run", "*.go")
	cmd.Dir = directory
	
	// Set environment variables for session context
	cmd.Env = os.Environ()
	if sessionData != nil {
		if sessionID, ok := sessionData["session_id"].(string); ok {
			cmd.Env = append(cmd.Env, fmt.Sprintf("SESSION_ID=%s", sessionID))
		}
		if restore, ok := sessionData["restore_context"].(bool); ok && restore {
			cmd.Env = append(cmd.Env, "RESTORE_CONTEXT=true")
		}
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %v", agentName, err)
	}

	// Track the process
	sessionID := ""
	if sessionData != nil {
		if sid, ok := sessionData["session_id"].(string); ok {
			sessionID = sid
		}
	}

	am.agents[agentName] = &AgentProcess{
		Name:      agentName,
		Directory: directory,
		Process:   cmd,
		Running:   true,
		StartTime: time.Now(),
		SessionID: sessionID,
		AgentType: agentDef.Type,
		TaskID:    "", // regular agents don't have task IDs
	}

	// Register agent instance in Redis for collision detection
	if err := am.registerAgentInstance(agentName, sessionID); err != nil {
		fmt.Printf("Warning: Failed to register agent instance %s: %v\n", agentName, err)
	}

	// Monitor process in background
	go am.monitorAgent(agentName)

	return nil
}

func (am *AgentManager) stopAgentProcess(process *AgentProcess) {
	if process.Process != nil && process.Running {
		process.Process.Process.Signal(syscall.SIGTERM)
		// Give it time to shutdown gracefully
		time.Sleep(time.Second * 2)
		if process.Process.ProcessState == nil {
			process.Process.Process.Kill()
		}
		process.Running = false
	}
}

// Ephemeral agent management
func (am *AgentManager) startEphemeralAgent(instanceName string, agentDef *AgentDefinition, taskID string, taskData map[string]interface{}) error {
	// Create command to run the agent
	cmd := exec.Command("go", "run", "*.go")
	cmd.Dir = agentDef.Directory
	
	// Set environment variables for ephemeral context
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("AGENT_TYPE=ephemeral"))
	cmd.Env = append(cmd.Env, fmt.Sprintf("TASK_ID=%s", taskID))
	if taskData != nil {
		taskJSON, _ := json.Marshal(taskData)
		cmd.Env = append(cmd.Env, fmt.Sprintf("TASK_DATA=%s", string(taskJSON)))
	}
	
	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ephemeral %s: %v", instanceName, err)
	}
	
	// Track the ephemeral process
	am.agents[instanceName] = &AgentProcess{
		Name:      agentDef.Name,
		Directory: agentDef.Directory,
		Process:   cmd,
		Running:   true,
		StartTime: time.Now(),
		SessionID: "", // ephemeral agents don't have sessions
		AgentType: EphemeralAgent,
		TaskID:    taskID,
	}
	
	// Monitor ephemeral agent with timeout
	go am.monitorEphemeralAgent(instanceName, agentDef)
	
	return nil
}

func (am *AgentManager) monitorAgent(agentName string) {
	process := am.agents[agentName]
	if process == nil {
		return
	}

	// Wait for process to complete
	err := process.Process.Wait()
	process.Running = false

	fmt.Printf("%s: Agent %s exited", am.AgentID, agentName)
	if err != nil {
		fmt.Printf(" with error: %v", err)
	}
	fmt.Println()

	// Unregister agent instance from Redis
	if err := am.unregisterAgentInstance(agentName); err != nil {
		fmt.Printf("Warning: Failed to unregister agent instance %s: %v\n", agentName, err)
	}

	// Publish agent exit event
	am.publishResponse(map[string]interface{}{
		"event":      "agent_exited",
		"agent":      agentName,
		"session_id": process.SessionID,
		"exit_time":  time.Now(),
		"agent_type": process.AgentType,
	})
}

func (am *AgentManager) monitorEphemeralAgent(instanceName string, agentDef *AgentDefinition) {
	process := am.agents[instanceName]
	if process == nil {
		return
	}
	
	fmt.Printf("%s: Monitoring ephemeral agent %s (task: %s)\n", am.AgentID, instanceName, process.TaskID)
	
	// Set up timeout if max runtime is specified
	var timeoutChan <-chan time.Time
	if agentDef.MaxRuntime > 0 {
		timeoutChan = time.After(time.Duration(agentDef.MaxRuntime) * time.Second)
		fmt.Printf("%s: Set %d second timeout for %s\n", am.AgentID, agentDef.MaxRuntime, instanceName)
	}
	
	// Monitor process completion or timeout
	done := make(chan error, 1)
	go func() {
		done <- process.Process.Wait()
	}()
	
	select {
	case err := <-done:
		// Process completed normally
		process.Running = false
		fmt.Printf("%s: Ephemeral agent %s completed task %s", am.AgentID, instanceName, process.TaskID)
		if err != nil {
			fmt.Printf(" with error: %v", err)
		}
		fmt.Println()
		
	case <-timeoutChan:
		// Process timed out - force kill
		fmt.Printf("%s: Ephemeral agent %s timed out after %d seconds - killing\n", am.AgentID, instanceName, agentDef.MaxRuntime)
		process.Process.Process.Kill()
		process.Running = false
		
		// Publish timeout event
		am.publishResponse(map[string]interface{}{
			"event":      "ephemeral_timeout",
			"agent":      agentDef.Name,
			"instance":   instanceName,
			"task_id":    process.TaskID,
			"max_runtime": agentDef.MaxRuntime,
			"exit_time":  time.Now(),
		})
	}
	
	// Cleanup ephemeral agent
	delete(am.agents, instanceName)
	
	// Publish ephemeral completion event
	am.publishResponse(map[string]interface{}{
		"event":     "ephemeral_completed",
		"agent":     agentDef.Name,
		"instance":  instanceName,
		"task_id":   process.TaskID,
		"exit_time": time.Now(),
	})
	
	fmt.Printf("%s: Ephemeral agent %s cleaned up\n", am.AgentID, instanceName)
}

func (am *AgentManager) publishResponse(response map[string]interface{}) {
	responseJSON, _ := json.Marshal(response)
	am.RedisClient.Publish(am.ctx, "centerfire:agent:manager:responses", string(responseJSON))
}

func (am *AgentManager) shutdown() {
	fmt.Printf("%s: Shutting down all managed agents...\n", am.AgentID)
	
	for name, process := range am.agents {
		fmt.Printf("Stopping %s...\n", name)
		am.stopAgentProcess(process)
	}
	
	am.RedisClient.Close()
	fmt.Printf("%s: Shutdown complete\n", am.AgentID)
}

// Utility function to create a session-aware agent restart command
func CreateRestartCommand(agentName string, sessionID string) string {
	request := AgentRequest{
		RequestType: "restart_agent",
		AgentName:   agentName,
		SessionData: map[string]interface{}{
			"session_id": sessionID,
		},
	}
	
	requestJSON, _ := json.Marshal(request)
	return fmt.Sprintf(`docker exec mem0-redis redis-cli PUBLISH centerfire:agent:manager '%s'`, 
		strings.ReplaceAll(string(requestJSON), "'", "\\'"))
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "restart":
			if len(os.Args) < 3 {
				fmt.Println("Usage: go run main.go restart <agent-name> [session-id]")
				return
			}
			sessionID := ""
			if len(os.Args) > 3 {
				sessionID = os.Args[3]
			}
			cmd := CreateRestartCommand(os.Args[2], sessionID)
			fmt.Printf("Execute: %s\n", cmd)
			return
		}
	}

	manager := NewAgentManager()
	manager.Start()
}
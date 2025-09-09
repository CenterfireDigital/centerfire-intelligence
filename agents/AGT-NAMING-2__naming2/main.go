package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
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

// NamingAgent represents the template-based naming authority agent
type NamingAgent struct {
	config      AgentConfig
	ctx         context.Context
	cancel      context.CancelFunc
	pidFile     string
	healthFile  string
	redisClient *redis.Client
	socketPath  string
}

// NewAgent creates a new naming agent from configuration
func NewAgent(configPath string) (*NamingAgent, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Load configuration
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	
	var config AgentConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	// Get socket path from communication config
	socketPath := "/tmp/agt-naming-2.sock"
	if comm, ok := config.Communication["unix_socket"].(string); ok {
		socketPath = comm
	}
	
	return &NamingAgent{
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		pidFile:     fmt.Sprintf("/tmp/%s.pid", config.AgentID),
		healthFile:  fmt.Sprintf("/tmp/%s.health", config.AgentID),
		redisClient: redisClient,
		socketPath:  socketPath,
	}, nil
}

// Start initializes and runs the agent
func (a *NamingAgent) Start() error {
	log.Printf("🚀 Starting %s (%s)", a.config.AgentID, a.config.FriendlyName)
	
	// Test Redis connection
	_, err := a.redisClient.Ping(a.ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Printf("Connected to Redis successfully")
	
	// Write PID file
	if err := a.writePIDFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	
	// Setup signal handling
	a.setupSignalHandling()
	
	// Register with monitor
	if err := a.registerWithMonitor(); err != nil {
		log.Printf("⚠️ Failed to register with monitor: %v", err)
	}
	
	// Start health reporting
	go a.healthReporter()
	
	// Start Redis listener for backward compatibility
	go a.startRedisListener()
	
	// Start Unix socket listener for new architecture
	go a.startSocketListener()
	
	// Run main agent logic
	return a.run()
}

// run contains the main agent logic
func (a *NamingAgent) run() error {
	log.Printf("✅ %s ready - listening on Redis and Unix socket", a.config.FriendlyName)
	
	// Register with manager and start heartbeat
	go a.registerWithManager()
	go a.startHeartbeat()
	
	for {
		select {
		case <-a.ctx.Done():
			log.Printf("🛑 %s shutting down", a.config.AgentID)
			return nil
		case <-time.After(30 * time.Second):
			a.logToCapture("Agent heartbeat", map[string]interface{}{
				"status": "running",
				"uptime": time.Now().Unix(),
			})
		}
	}
}

// startRedisListener handles Redis pub/sub for backward compatibility
func (a *NamingAgent) startRedisListener() {
	channels := []string{"agent.naming.request"}
	if comm, ok := a.config.Communication["redis_channels"].([]interface{}); ok {
		channels = make([]string, len(comm))
		for i, ch := range comm {
			channels[i] = ch.(string)
		}
	}
	
	pubsub := a.redisClient.Subscribe(a.ctx, channels[0])
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	log.Printf("%s: Redis listener started on %s", a.config.AgentID, channels[0])
	
	for {
		select {
		case <-a.ctx.Done():
			return
		case msg := <-ch:
			if msg != nil {
				response := a.processMessage(msg.Payload)
				responseJSON, _ := json.Marshal(response)
				a.redisClient.Publish(a.ctx, "agent.naming.response", responseJSON)
			}
		}
	}
}

// startSocketListener handles Unix socket communication
func (a *NamingAgent) startSocketListener() {
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			if err := a.connectToSocket(); err != nil {
				log.Printf("Socket connection failed: %v, retrying in 2s", err)
				time.Sleep(2 * time.Second)
			}
		}
	}
}

// connectToSocket establishes and handles socket connection
func (a *NamingAgent) connectToSocket() error {
	conn, err := net.Dial("unix", a.socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	
	log.Printf("%s: Connected to socket %s", a.config.AgentID, a.socketPath)
	
	buffer := make([]byte, 4096)
	for {
		select {
		case <-a.ctx.Done():
			return nil
		default:
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, err := conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return err
			}
			
			if n > 0 {
				payload := string(buffer[:n])
				response := a.processSocketMessage(payload)
				responseJSON, _ := json.Marshal(response)
				conn.Write(responseJSON)
			}
		}
	}
}

// processMessage handles both Redis and socket messages
func (a *NamingAgent) processMessage(payload string) map[string]interface{} {
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		return map[string]interface{}{
			"error": "Invalid JSON format",
		}
	}
	
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
		}
	}
	
	// Log the request
	a.logToCapture(fmt.Sprintf("Processing %s request", action), map[string]interface{}{
		"action": action,
	})
	
	var response map[string]interface{}
	
	switch action {
	case "allocate_capability":
		response = a.handleAllocateCapability(request)
	case "allocate_session":
		response = a.handleAllocateSession(request)
	case "allocate_namespace":
		response = a.handleAllocateNamespace(request)
	case "allocate_module":
		response = map[string]interface{}{"error": "not implemented yet"}
	case "allocate_function":
		response = map[string]interface{}{"error": "not implemented yet"}
	case "validate_name":
		response = map[string]interface{}{"error": "not implemented yet"}
	default:
		response = map[string]interface{}{"error": "Unknown action: " + action}
	}
	
	// Preserve request_id if provided
	if requestID, ok := request["request_id"]; ok {
		response["request_id"] = requestID
	}
	
	return response
}

// processSocketMessage handles socket-specific message wrapping
func (a *NamingAgent) processSocketMessage(payload string) map[string]interface{} {
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		return map[string]interface{}{
			"success":   false,
			"error":     "Invalid JSON format",
			"timestamp": time.Now(),
		}
	}
	
	response := a.processMessage(payload)
	
	return map[string]interface{}{
		"id":        request["id"],
		"success":   response["error"] == nil,
		"data":      response,
		"timestamp": time.Now(),
	}
}

// handleAllocateCapability allocates capability names with sequences
func (a *NamingAgent) handleAllocateCapability(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	domain, ok := params["domain"].(string)
	if !ok {
		return map[string]interface{}{"error": "Domain is required"}
	}
	
	// Get purpose from params (accept both 'purpose' and 'description')
	purpose := ""
	if p, ok := params["purpose"].(string); ok {
		purpose = p
	} else if p, ok := params["description"].(string); ok {
		purpose = p
	}
	
	// Generate capability allocation
	allocation := a.generateCapabilityID(domain, purpose)
	
	// Delegate structure creation to AGT-STRUCT-1
	a.delegateStructureCreation(allocation)
	
	return allocation
}

// handleAllocateSession allocates session IDs
func (a *NamingAgent) handleAllocateSession(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	sessionType := "claude_coding"
	if t, ok := params["type"].(string); ok {
		sessionType = t
	}
	
	context := "session"
	if c, ok := params["context"].(string); ok {
		context = c
	}
	
	sessionID := a.generateSessionID(sessionType)
	
	return map[string]interface{}{
		"session_id": sessionID,
		"type":       sessionType,
		"context":    context,
	}
}

// handleAllocateNamespace allocates semantic namespaces
func (a *NamingAgent) handleAllocateNamespace(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	project, ok := params["project"].(string)
	if !ok {
		return map[string]interface{}{"error": "Project required"}
	}
	
	environment, ok := params["environment"].(string)
	if !ok {
		return map[string]interface{}{"error": "Environment required"}
	}
	
	classType, _ := params["class_type"].(string)
	
	namespace := a.generateNamespaceID(project, environment)
	
	response := map[string]interface{}{
		"namespace":   namespace["namespace"],
		"cid":         namespace["cid"],
		"project":     project,
		"environment": environment,
	}
	
	if classType != "" {
		className := a.generateClassName(namespace["cid"].(string), classType)
		response["className"] = className
	}
	
	return response
}

// generateCapabilityID creates capability names with Redis sequences
func (a *NamingAgent) generateCapabilityID(domain, purpose string) map[string]interface{} {
	domainUpper := strings.ToUpper(domain)
	sequenceKey := fmt.Sprintf("centerfire.dev.sequence:CAP-%s", domainUpper)
	
	sequence, err := a.redisClient.Incr(a.ctx, sequenceKey).Result()
	if err != nil {
		log.Printf("Error getting sequence from Redis: %v, using fallback", err)
		sequence = 1
	}
	
	now := time.Now()
	ulid := fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
	
	slug := fmt.Sprintf("CAP-%s-%d", domainUpper, sequence)
	cid := fmt.Sprintf("cid:centerfire:capability:%s", ulid)
	directory := fmt.Sprintf("%s__%s", slug, ulid)
	
	nameKey := fmt.Sprintf("centerfire.dev.names:capability:%s", slug)
	allocated := time.Now().Format(time.RFC3339)
	
	nameData := map[string]interface{}{
		"slug":      slug,
		"cid":       cid,
		"directory": directory,
		"domain":    domain,
		"purpose":   purpose,
		"sequence":  sequence,
		"allocated": allocated,
	}
	
	nameJSON, _ := json.Marshal(nameData)
	a.redisClient.Set(a.ctx, nameKey, nameJSON, 0)
	
	// Publish semantic name event
	a.publishSemanticNameEvent(slug, cid, directory, domain, purpose, sequence, allocated)
	
	log.Printf("%s: Allocated capability: %s (CID: %s)", a.config.AgentID, slug, cid)
	
	return nameData
}

// generateSessionID creates session identifiers
func (a *NamingAgent) generateSessionID(sessionType string) string {
	sessionTypeUpper := strings.ToUpper(sessionType)
	sequenceKey := fmt.Sprintf("centerfire.dev.sequence:SES-%s", sessionTypeUpper)
	
	sequence, err := a.redisClient.Incr(a.ctx, sequenceKey).Result()
	if err != nil {
		log.Printf("Error getting session sequence: %v, using fallback", err)
		sequence = 1
	}
	
	now := time.Now()
	ulid := fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
	
	prefix := "SES-CLAUDE"
	switch sessionType {
	case "claude_coding":
		prefix = "SES-CLAUDE"
	case "agent_session":
		prefix = "SES-AGENT"
	default:
		prefix = "SES-GENERIC"
	}
	
	return fmt.Sprintf("%s-%d-%s", prefix, sequence, ulid)
}

// generateNamespaceID creates semantic namespaces with CIDs
func (a *NamingAgent) generateNamespaceID(project, environment string) map[string]interface{} {
	sequenceKey := fmt.Sprintf("centerfire.%s.sequence:NS-%s", environment, strings.ToUpper(project))
	sequence, err := a.redisClient.Incr(a.ctx, sequenceKey).Result()
	if err != nil {
		log.Printf("Error getting namespace sequence: %v, using fallback", err)
		sequence = 1
	}
	
	now := time.Now()
	ulid := fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
	
	cid := fmt.Sprintf("cid:%s:%s:namespace:%s", project, environment, ulid)
	namespace := fmt.Sprintf("%s.%s.ns%d", project, environment, sequence)
	
	nameKey := fmt.Sprintf("centerfire.%s.namespaces:%s", environment, namespace)
	allocated := time.Now().Format(time.RFC3339)
	
	namespaceData := map[string]interface{}{
		"namespace":   namespace,
		"cid":         cid,
		"project":     project,
		"environment": environment,
		"sequence":    sequence,
		"allocated":   allocated,
	}
	
	nameJSON, _ := json.Marshal(namespaceData)
	a.redisClient.Set(a.ctx, nameKey, nameJSON, 0)
	
	// Publish semantic namespace event
	a.publishSemanticNamespaceEvent(namespace, cid, project, environment, sequence, allocated)
	
	log.Printf("%s: Allocated namespace: %s (CID: %s)", a.config.AgentID, namespace, cid)
	
	return namespaceData
}

// generateClassName creates Weaviate class names from namespace CIDs
func (a *NamingAgent) generateClassName(namespaceCID, classType string) string {
	parts := strings.Split(namespaceCID, ":")
	if len(parts) >= 3 {
		project := strings.Title(strings.ToLower(parts[1]))
		env := strings.Title(strings.ToLower(parts[2]))
		class := strings.Title(strings.ToLower(classType))
		return fmt.Sprintf("%s_%s_%s", project, env, class)
	}
	
	class := strings.Title(strings.ToLower(classType))
	return fmt.Sprintf("Semantic_%s", class)
}

// publishSemanticNameEvent publishes capability events to Redis streams
func (a *NamingAgent) publishSemanticNameEvent(slug, cid, directory, domain, purpose string, sequence int64, allocated string) {
	streamName := "centerfire:semantic:names"
	
	eventData := map[string]interface{}{
		"slug":       slug,
		"cid":        cid,
		"directory":  directory,
		"domain":     domain,
		"purpose":    purpose,
		"sequence":   sequence,
		"allocated":  allocated,
		"event_type": "capability_allocated",
	}
	
	eventJSON, _ := json.Marshal(eventData)
	
	_, err := a.redisClient.XAdd(a.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data":      string(eventJSON),
			"timestamp": time.Now().Unix(),
			"source":    a.config.AgentID,
		},
	}).Result()
	
	if err != nil {
		log.Printf("%s: Error publishing to stream: %v", a.config.AgentID, err)
	} else {
		log.Printf("%s: Published semantic name event: %s", a.config.AgentID, slug)
	}
}

// publishSemanticNamespaceEvent publishes namespace events to Redis streams
func (a *NamingAgent) publishSemanticNamespaceEvent(namespace, cid, project, environment string, sequence int64, allocated string) {
	streamName := "centerfire:semantic:namespaces"
	
	eventData := map[string]interface{}{
		"namespace":   namespace,
		"cid":         cid,
		"project":     project,
		"environment": environment,
		"sequence":    sequence,
		"allocated":   allocated,
		"event_type":  "namespace_allocated",
	}
	
	eventJSON, _ := json.Marshal(eventData)
	
	_, err := a.redisClient.XAdd(a.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data":      string(eventJSON),
			"timestamp": time.Now().Unix(),
			"source":    a.config.AgentID,
		},
	}).Result()
	
	if err != nil {
		log.Printf("%s: Error publishing to namespace stream: %v", a.config.AgentID, err)
	} else {
		log.Printf("%s: Published namespace event: %s", a.config.AgentID, namespace)
	}
}

// delegateStructureCreation sends structure creation request to AGT-STRUCT-1
func (a *NamingAgent) delegateStructureCreation(allocation map[string]interface{}) {
	structRequest := map[string]interface{}{
		"action": "create_structure",
		"params": map[string]interface{}{
			"slug":      allocation["slug"],
			"cid":       allocation["cid"],
			"directory": allocation["directory"],
			"domain":    allocation["domain"],
			"purpose":   allocation["purpose"],
		},
		"source": a.config.AgentID,
	}
	
	requestJSON, _ := json.Marshal(structRequest)
	a.redisClient.Publish(a.ctx, "agent.struct.request", requestJSON)
	
	log.Printf("%s: Delegated structure creation to AGT-STRUCT-1", a.config.AgentID)
}

// registerWithManager registers with AGT-MANAGER-1
func (a *NamingAgent) registerWithManager() {
	responseChannel := fmt.Sprintf("centerfire:agent:manager:response:%s", a.config.AgentID)
	
	registrationRequest := map[string]interface{}{
		"request_type": "register_running",
		"agent_name":   a.config.AgentID,
		"session_data": map[string]interface{}{
			"pid": os.Getpid(),
		},
		"response_channel": responseChannel,
	}
	
	requestJSON, _ := json.Marshal(registrationRequest)
	a.redisClient.Publish(a.ctx, "centerfire:agent:manager", requestJSON)
	
	log.Printf("%s: Registered with AGT-MANAGER-1", a.config.AgentID)
}

// startHeartbeat sends periodic heartbeat to manager
func (a *NamingAgent) startHeartbeat() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			heartbeat := map[string]interface{}{
				"request_type": "heartbeat",
				"agent_name":   a.config.AgentID,
			}
			
			heartbeatJSON, _ := json.Marshal(heartbeat)
			a.redisClient.Publish(a.ctx, "centerfire:agent:manager", heartbeatJSON)
		}
	}
}

// Standard template methods below
func (a *NamingAgent) writePIDFile() error {
	pid := os.Getpid()
	return os.WriteFile(a.pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func (a *NamingAgent) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-sigChan
		log.Printf("🛑 Received %s, initiating graceful shutdown", sig)
		a.shutdown()
	}()
}

func (a *NamingAgent) registerWithMonitor() error {
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
	
	regFile := fmt.Sprintf("/tmp/agent-registry-%s.json", a.config.AgentID)
	data, _ := json.MarshalIndent(registrationData, "", "  ")
	
	return os.WriteFile(regFile, data, 0644)
}

func (a *NamingAgent) healthReporter() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			health := map[string]interface{}{
				"agent_id":   a.config.AgentID,
				"cid":        a.config.CID,
				"namespace":  a.config.Namespace,
				"status":     "healthy",
				"timestamp":  time.Now().Unix(),
				"pid":        os.Getpid(),
				"uptime":     time.Now().Unix(),
			}
			
			data, _ := json.Marshal(health)
			os.WriteFile(a.healthFile, data, 0644)
		}
	}
}

func (a *NamingAgent) logToCapture(message string, data map[string]interface{}) {
	logEntry := map[string]interface{}{
		"agent_id":  a.config.AgentID,
		"namespace": a.config.Namespace,
		"timestamp": time.Now().Unix(),
		"message":   message,
		"data":      data,
	}
	
	jsonData, _ := json.Marshal(logEntry)
	log.Printf("CAPTURE: %s", string(jsonData))
}

func (a *NamingAgent) shutdown() {
	log.Printf("🔄 %s performing graceful shutdown", a.config.AgentID)
	
	// Unregister with manager
	unregRequest := map[string]interface{}{
		"request_type": "unregister_running",
		"agent_name":   a.config.AgentID,
	}
	unregJSON, _ := json.Marshal(unregRequest)
	a.redisClient.Publish(a.ctx, "centerfire:agent:manager", unregJSON)
	
	a.logToCapture("Agent shutting down", map[string]interface{}{
		"reason": "graceful_shutdown",
		"uptime": time.Now().Unix(),
	})
	
	// Clean up files
	os.Remove(a.pidFile)
	os.Remove(a.healthFile)
	
	// Cancel context
	a.cancel()
	
	log.Printf("✅ %s shutdown complete", a.config.AgentID)
	os.Exit(0)
}

func main() {
	configPath := "agent.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	
	agent, err := NewAgent(configPath)
	if err != nil {
		log.Fatalf("❌ Failed to create agent: %v", err)
	}
	
	if err := agent.Start(); err != nil {
		log.Fatalf("❌ Agent failed to start: %v", err)
	}
}
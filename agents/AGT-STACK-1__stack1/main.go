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
)

type StackAgent struct {
	RedisClient   *redis.Client
	ctx           context.Context
	agentID       string
	dockerComposeFile string
	stackStates   map[string]*StackState
	statesMutex   sync.RWMutex
}

type StackState struct {
	Profile      string            `json:"profile"`
	Containers   []ContainerInfo   `json:"containers"`
	Status       string            `json:"status"` // "running", "stopped", "starting", "stopping"
	LastUpdated  time.Time         `json:"last_updated"`
	Dependencies []string          `json:"dependencies"`
}

type ContainerInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Ports  []string `json:"ports,omitempty"`
}

type StackRequest struct {
	Type      string `json:"type"`
	ClientID  string `json:"client_id"`
	Profile   string `json:"profile,omitempty"`
	Container string `json:"container,omitempty"`
	Operation string `json:"operation"`
}

type StackResponse struct {
	Type       string      `json:"type"`
	Status     string      `json:"status"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data,omitempty"`
	ClientID   string      `json:"client_id"`
	RequestID  string      `json:"request_id"`
}

func NewStackAgent() *StackAgent {
	ctx := context.Background()
	
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6380",
		DB:   0,
	})

	// Find docker-compose files
	composeFile := findDockerComposeFile()
	if composeFile == "" {
		log.Fatal("❌ No docker-compose file found")
	}
	
	log.Printf("🐳 Using docker-compose file: %s", composeFile)

	agent := &StackAgent{
		RedisClient:       redisClient,
		ctx:              ctx,
		agentID:          "AGT-STACK-1",
		dockerComposeFile: composeFile,
		stackStates:      make(map[string]*StackState),
	}
	
	// Initialize stack states by discovering current containers
	agent.discoverCurrentStacks()
	
	return agent
}

func findDockerComposeFile() string {
	possibleFiles := []string{
		"docker-compose.yaml",
		"docker-compose.yml", 
		"docker-compose-clickhouse-addition.yaml",
	}
	
	for _, file := range possibleFiles {
		if _, err := os.Stat(file); err == nil {
			return file
		}
	}
	return ""
}

func (sa *StackAgent) discoverCurrentStacks() {
	log.Println("🔍 Discovering current container states...")
	
	// Get running containers
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("⚠️  Warning: Could not discover containers: %v", err)
		return
	}
	
	containers := make(map[string]ContainerInfo)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			name := parts[0]
			status := parts[1]
			ports := []string{}
			if len(parts) > 2 {
				ports = strings.Fields(parts[2])
			}
			
			containers[name] = ContainerInfo{
				Name:   name,
				Status: status,
				Ports:  ports,
			}
		}
	}
	
	// Map containers to profiles (simplified detection)
	sa.statesMutex.Lock()
	defer sa.statesMutex.Unlock()
	
	// ClickHouse profile detection
	if container, exists := containers["centerfire-clickhouse"]; exists {
		sa.stackStates["analytics"] = &StackState{
			Profile:     "analytics",
			Containers:  []ContainerInfo{container},
			Status:      "running",
			LastUpdated: time.Now(),
		}
		log.Printf("🧊 Discovered analytics profile: ClickHouse running")
	}
	
	log.Printf("📊 Discovered %d active container profiles", len(sa.stackStates))
}

func (sa *StackAgent) registerWithManager() error {
	registration := map[string]interface{}{
		"agent_id": sa.agentID,
		"type":     "stack_orchestration",
		"capabilities": []string{
			"profile_management",
			"container_orchestration", 
			"ephemeral_startup",
			"dependency_tracking",
		},
		"status": "active",
	}
	
	data, _ := json.Marshal(registration)
	return sa.RedisClient.Publish(sa.ctx, "agent.registration", string(data)).Err()
}

func (sa *StackAgent) handleStackRequest(request StackRequest) StackResponse {
	log.Printf("🐳 Processing %s operation for profile '%s'", request.Operation, request.Profile)
	
	response := StackResponse{
		Type:     "stack_response",
		ClientID: request.ClientID,
		Status:   "success",
	}
	
	switch request.Operation {
	case "start_profile":
		err := sa.startProfile(request.Profile)
		if err != nil {
			response.Status = "error"
			response.Message = fmt.Sprintf("Failed to start profile %s: %v", request.Profile, err)
		} else {
			response.Message = fmt.Sprintf("Profile %s started successfully", request.Profile)
			response.Data = sa.getProfileStatus(request.Profile)
		}
		
	case "stop_profile":
		err := sa.stopProfile(request.Profile)
		if err != nil {
			response.Status = "error"
			response.Message = fmt.Sprintf("Failed to stop profile %s: %v", request.Profile, err)
		} else {
			response.Message = fmt.Sprintf("Profile %s stopped successfully", request.Profile)
		}
		
	case "profile_status":
		response.Data = sa.getProfileStatus(request.Profile)
		response.Message = fmt.Sprintf("Status for profile %s", request.Profile)
		
	case "list_profiles":
		response.Data = sa.listAllProfiles()
		response.Message = "Available profiles and their status"
		
	case "container_status":
		response.Data = sa.getContainerStatus(request.Container)
		response.Message = fmt.Sprintf("Status for container %s", request.Container)
		
	default:
		response.Status = "error"
		response.Message = fmt.Sprintf("Unknown operation: %s", request.Operation)
	}
	
	return response
}

func (sa *StackAgent) startProfile(profile string) error {
	sa.statesMutex.Lock()
	defer sa.statesMutex.Unlock()
	
	log.Printf("🚀 Starting profile: %s", profile)
	
	// Update state to starting
	if sa.stackStates[profile] == nil {
		sa.stackStates[profile] = &StackState{
			Profile: profile,
			Containers: []ContainerInfo{},
		}
	}
	sa.stackStates[profile].Status = "starting"
	sa.stackStates[profile].LastUpdated = time.Now()
	
	// Execute docker-compose up with profile
	cmd := exec.Command("docker-compose", 
		"-f", sa.dockerComposeFile,
		"--profile", profile,
		"up", "-d")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Failed to start profile %s: %v\nOutput: %s", profile, err, string(output))
		sa.stackStates[profile].Status = "error"
		return fmt.Errorf("docker-compose failed: %v", err)
	}
	
	log.Printf("✅ Profile %s started successfully", profile)
	sa.stackStates[profile].Status = "running"
	
	// Discover containers that were started
	sa.updateProfileContainers(profile)
	
	return nil
}

func (sa *StackAgent) stopProfile(profile string) error {
	sa.statesMutex.Lock()
	defer sa.statesMutex.Unlock()
	
	log.Printf("🛑 Stopping profile: %s", profile)
	
	if sa.stackStates[profile] != nil {
		sa.stackStates[profile].Status = "stopping"
		sa.stackStates[profile].LastUpdated = time.Now()
	}
	
	// Execute docker-compose down with profile
	cmd := exec.Command("docker-compose",
		"-f", sa.dockerComposeFile,
		"--profile", profile,
		"down")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Failed to stop profile %s: %v\nOutput: %s", profile, err, string(output))
		if sa.stackStates[profile] != nil {
			sa.stackStates[profile].Status = "error"
		}
		return fmt.Errorf("docker-compose failed: %v", err)
	}
	
	log.Printf("✅ Profile %s stopped successfully", profile)
	if sa.stackStates[profile] != nil {
		sa.stackStates[profile].Status = "stopped"
		sa.stackStates[profile].Containers = []ContainerInfo{}
	}
	
	return nil
}

func (sa *StackAgent) updateProfileContainers(profile string) {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("⚠️  Could not update container list: %v", err)
		return
	}
	
	var containers []ContainerInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			name := parts[0]
			// Simple heuristic: if container name contains profile-related terms
			if sa.containerBelongsToProfile(name, profile) {
				status := parts[1]
				ports := []string{}
				if len(parts) > 2 {
					ports = strings.Fields(parts[2])
				}
				
				containers = append(containers, ContainerInfo{
					Name:   name,
					Status: status,
					Ports:  ports,
				})
			}
		}
	}
	
	if sa.stackStates[profile] != nil {
		sa.stackStates[profile].Containers = containers
	}
}

func (sa *StackAgent) containerBelongsToProfile(containerName, profile string) bool {
	// Simple mapping logic - could be more sophisticated
	switch profile {
	case "analytics":
		return strings.Contains(containerName, "clickhouse")
	default:
		return false
	}
}

func (sa *StackAgent) getProfileStatus(profile string) interface{} {
	sa.statesMutex.RLock()
	defer sa.statesMutex.RUnlock()
	
	if state, exists := sa.stackStates[profile]; exists {
		return state
	}
	
	return map[string]interface{}{
		"profile": profile,
		"status":  "not_found",
		"message": "Profile not currently tracked",
	}
}

func (sa *StackAgent) getContainerStatus(containerName string) interface{} {
	cmd := exec.Command("docker", "inspect", containerName, "--format", "{{.State.Status}}")
	output, err := cmd.Output()
	if err != nil {
		return map[string]interface{}{
			"container": containerName,
			"status":    "not_found",
			"error":     err.Error(),
		}
	}
	
	status := strings.TrimSpace(string(output))
	return map[string]interface{}{
		"container": containerName,
		"status":    status,
	}
}

func (sa *StackAgent) listAllProfiles() interface{} {
	sa.statesMutex.RLock()
	defer sa.statesMutex.RUnlock()
	
	profiles := make(map[string]interface{})
	
	// Add tracked profiles
	for name, state := range sa.stackStates {
		profiles[name] = state
	}
	
	// Add available profiles that aren't tracked
	availableProfiles := []string{"analytics"} // Could be discovered from compose file
	for _, profile := range availableProfiles {
		if _, exists := profiles[profile]; !exists {
			profiles[profile] = map[string]interface{}{
				"profile": profile,
				"status":  "available",
				"containers": []ContainerInfo{},
			}
		}
	}
	
	return profiles
}

func (sa *StackAgent) startListening() {
	log.Printf("🐳 AGT-STACK-1 starting container orchestration...")
	log.Printf("📋 Monitoring Docker Compose profiles and ephemeral containers")
	
	// Register with manager
	if err := sa.registerWithManager(); err != nil {
		log.Printf("⚠️  Could not register with manager: %v", err)
	}
	
	// Subscribe to stack requests
	pubsub := sa.RedisClient.Subscribe(sa.ctx, "agent.stack.request")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	
	for msg := range ch {
		var request StackRequest
		if err := json.Unmarshal([]byte(msg.Payload), &request); err != nil {
			log.Printf("❌ Invalid request format: %v", err)
			continue
		}
		
		response := sa.handleStackRequest(request)
		
		// Send response
		responseData, _ := json.Marshal(response)
		sa.RedisClient.Publish(sa.ctx, "agent.stack.response", string(responseData))
	}
}

func main() {
	agent := NewStackAgent()
	agent.startListening()
}
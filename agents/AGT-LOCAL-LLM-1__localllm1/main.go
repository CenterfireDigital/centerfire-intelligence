package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type LocalLLMAgent struct {
	RedisClient *redis.Client
	ctx         context.Context
	agentID     string
	models      map[string]*ModelConfig
	activeModel string
	mutex       sync.RWMutex
}

type ModelConfig struct {
	Name         string   `json:"name" yaml:"name"`
	Model        string   `json:"model" yaml:"model"`
	Specialties  []string `json:"specialties" yaml:"specialties"`
	LoadedAt     *time.Time `json:"loaded_at,omitempty"`
	LastUsed     *time.Time `json:"last_used,omitempty"`
	ResourceCost int      `json:"resource_cost" yaml:"resource_cost"` // 1-10 scale
	AutoUnload   bool     `json:"auto_unload" yaml:"auto_unload"`
}

type LLMRequest struct {
	Action    string                 `json:"action"`
	Params    map[string]interface{} `json:"params"`
	ClientID  string                 `json:"client_id"`
	RequestID string                 `json:"request_id"`
}

type LLMResponse struct {
	Success   bool                   `json:"success"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	RequestID string                 `json:"request_id"`
	Model     string                 `json:"model,omitempty"`
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func NewLocalLLMAgent() *LocalLLMAgent {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	agent := &LocalLLMAgent{
		RedisClient: rdb,
		ctx:         ctx,
		agentID:     "AGT-LOCAL-LLM-1",
		models:      make(map[string]*ModelConfig),
	}

	agent.initializeModels()
	return agent
}

func (lla *LocalLLMAgent) initializeModels() {
	// Define model configurations with specialties
	models := []*ModelConfig{
		{
			Name:        "file_analyst",
			Model:       "codellama:13b-instruct",
			Specialties: []string{"file_search", "code_analysis", "project_understanding", "dependency_mapping"},
			ResourceCost: 8,
			AutoUnload:   true,
		},
		{
			Name:        "knowledge_curator", 
			Model:       "mistral:7b-instruct",
			Specialties: []string{"weaviate_queries", "conversation_assessment", "context_synthesis", "reasoning"},
			ResourceCost: 5,
			AutoUnload:   false, // Keep loaded for frequent queries
		},
		{
			Name:        "workflow_manager",
			Model:       "llama3.1:8b",
			Specialties: []string{"todo_management", "note_taking", "workflow_optimization", "progress_tracking"},
			ResourceCost: 6,
			AutoUnload:   true,
		},
	}

	for _, model := range models {
		lla.models[model.Name] = model
	}

	log.Printf("Initialized %d local LLM models", len(lla.models))
}

func (lla *LocalLLMAgent) selectModelForTask(taskType string) (*ModelConfig, error) {
	lla.mutex.RLock()
	defer lla.mutex.RUnlock()

	// Find best model for task
	var bestModel *ModelConfig
	maxScore := 0

	for _, model := range lla.models {
		score := 0
		for _, specialty := range model.Specialties {
			if strings.Contains(strings.ToLower(taskType), strings.ToLower(specialty)) ||
			   strings.Contains(strings.ToLower(specialty), strings.ToLower(taskType)) {
				score += 10
			}
		}
		
		// Prefer already loaded models (avoid context switching)
		if model.LoadedAt != nil {
			score += 5
		}

		if score > maxScore {
			maxScore = score
			bestModel = model
		}
	}

	if bestModel == nil {
		// Default to knowledge_curator for unknown tasks
		bestModel = lla.models["knowledge_curator"]
	}

	return bestModel, nil
}

func (lla *LocalLLMAgent) ensureModelLoaded(model *ModelConfig) error {
	if model.LoadedAt != nil {
		now := time.Now()
		model.LastUsed = &now
		return nil // Already loaded
	}

	log.Printf("Loading model %s (%s)...", model.Name, model.Model)
	
	// Test if model is available via Ollama
	testReq := OllamaRequest{
		Model:  model.Model,
		Prompt: "Hello",
		Stream: false,
	}

	reqBody, _ := json.Marshal(testReq)
	resp, err := http.Post("http://localhost:11434/api/generate", 
		"application/json", bytes.NewBuffer(reqBody))
	
	if err != nil {
		// Try to pull the model if not available
		log.Printf("Model not loaded, attempting to pull %s", model.Model)
		cmd := exec.Command("ollama", "pull", model.Model)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to pull model %s: %v", model.Model, err)
		}
	} else {
		resp.Body.Close()
	}

	now := time.Now()
	model.LoadedAt = &now
	model.LastUsed = &now
	
	log.Printf("✅ Model %s loaded successfully", model.Name)
	return nil
}

func (lla *LocalLLMAgent) queryOllama(model string, prompt string) (string, error) {
	reqData := OllamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	reqBody, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := http.Post("http://localhost:11434/api/generate",
		"application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	return ollamaResp.Response, nil
}

func (lla *LocalLLMAgent) processRequest(req LLMRequest) LLMResponse {
	startTime := time.Now()
	
	taskType := req.Action
	if params, ok := req.Params["task_type"].(string); ok {
		taskType = params
	}

	// Select best model for task
	selectedModel, err := lla.selectModelForTask(taskType)
	if err != nil {
		return LLMResponse{
			Success:   false,
			Error:     fmt.Sprintf("Model selection failed: %v", err),
			RequestID: req.RequestID,
		}
	}

	// Ensure model is loaded
	if err := lla.ensureModelLoaded(selectedModel); err != nil {
		return LLMResponse{
			Success:   false,
			Error:     fmt.Sprintf("Model loading failed: %v", err),
			RequestID: req.RequestID,
		}
	}

	// Build prompt based on action
	prompt := lla.buildPrompt(req)
	
	log.Printf("🧠 Processing %s with %s", taskType, selectedModel.Name)

	// Query the model
	response, err := lla.queryOllama(selectedModel.Model, prompt)
	if err != nil {
		return LLMResponse{
			Success:   false,
			Error:     fmt.Sprintf("LLM query failed: %v", err),
			RequestID: req.RequestID,
		}
	}

	duration := time.Since(startTime)
	log.Printf("✅ %s completed in %v", selectedModel.Name, duration)

	return LLMResponse{
		Success:   true,
		Data: map[string]interface{}{
			"response":      response,
			"model_used":    selectedModel.Name,
			"duration_ms":   duration.Milliseconds(),
		},
		RequestID: req.RequestID,
		Model:     selectedModel.Name,
	}
}

func (lla *LocalLLMAgent) buildPrompt(req LLMRequest) string {
	basePrompt := ""
	
	if prompt, ok := req.Params["prompt"].(string); ok {
		basePrompt = prompt
	} else if query, ok := req.Params["query"].(string); ok {
		basePrompt = query
	}

	// Add task-specific context
	switch req.Action {
	case "file_search":
		return fmt.Sprintf(`You are a expert file analyst. Help find files related to: %s

Context: Working in /Users/larrydiffey/projects/CenterfireIntelligence
Task: Suggest specific file paths, directories, or search patterns.
Format: Provide concrete actionable file locations.

Query: %s`, basePrompt, basePrompt)

	case "weaviate_query":
		return fmt.Sprintf(`You are a knowledge curator. Help query conversation history for: %s

Context: Weaviate vector database contains conversation embeddings
Task: Suggest semantic search terms and query strategies.
Format: Provide specific search terms and filters.

Query: %s`, basePrompt, basePrompt)

	case "todo_update":
		return fmt.Sprintf(`You are a workflow manager. Help organize tasks: %s

Context: Managing development project todos and priorities
Task: Structure, prioritize, and track progress.
Format: Provide clear actionable todo items.

Request: %s`, basePrompt, basePrompt)

	default:
		return basePrompt
	}
}

func (lla *LocalLLMAgent) startListener() {
	pubsub := lla.RedisClient.Subscribe(lla.ctx, "agent.localllm.request")
	defer pubsub.Close()

	log.Printf("🎧 %s listening on agent.localllm.request", lla.agentID)

	ch := pubsub.Channel()
	for msg := range ch {
		lla.handleRequest(msg.Payload)
	}
}

func (lla *LocalLLMAgent) handleRequest(payload string) {
	var req LLMRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		log.Printf("Error parsing request: %v", err)
		return
	}

	log.Printf("📥 Processing %s request from %s", req.Action, req.ClientID)

	response := lla.processRequest(req)
	responseData, _ := json.Marshal(response)

	lla.RedisClient.Publish(lla.ctx, "agent.localllm.response", string(responseData))
	log.Printf("📤 Response sent for %s", req.RequestID)
}

func (lla *LocalLLMAgent) registerWithManager() {
	registrationData := map[string]interface{}{
		"agent_name":   lla.agentID,
		"agent_type":   "local_llm_manager",
		"pid":          os.Getpid(),
		"capabilities": []string{"file_search", "weaviate_query", "todo_update", "code_analysis", "conversation_assessment"},
		"channels":     []string{"agent.localllm.request"},
		"models":       lla.getModelSummary(),
	}

	data, _ := json.Marshal(registrationData)
	lla.RedisClient.Publish(lla.ctx, "agent.manager.register", string(data))
	log.Printf("Registered %s with manager", lla.agentID)
}

func (lla *LocalLLMAgent) getModelSummary() map[string]interface{} {
	summary := make(map[string]interface{})
	for name, model := range lla.models {
		summary[name] = map[string]interface{}{
			"model":       model.Model,
			"specialties": model.Specialties,
			"loaded":      model.LoadedAt != nil,
		}
	}
	return summary
}

func main() {
	log.Println("Starting AGT-LOCAL-LLM-1...")

	agent := NewLocalLLMAgent()
	
	// Register with manager
	agent.registerWithManager()
	
	log.Println("Local LLM Agent ready for platform-independent AI processing")
	agent.startListener()
}
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	
	"github.com/redis/go-redis/v9"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

// SemanticAgent - Generated agent for SEMANTIC domain
type SemanticAgent struct {
	AgentID         string
	CID            string
	RequestChannel  string
	ResponseChannel string
	RedisClient    *redis.Client
	WeaviateClient *weaviate.Client
	Project        string  // Project name (e.g., "centerfire")
	Environment    string  // Environment (dev/test/prod)
	ctx            context.Context
}

// NewAgent - Create new SEMANTIC agent
func NewAgent() *SemanticAgent {
	// Connect to Redis container on port 6380
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})
	
	// Connect to Weaviate on port 8080
	cfg := weaviate.Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}
	
	client, err := weaviate.NewClient(cfg)
	if err != nil {
		fmt.Printf("Failed to create Weaviate client: %v\n", err)
		return nil
	}
	
	return &SemanticAgent{
		AgentID:         "AGT-SEMANTIC-1",
		CID:            "cid:centerfire:agent:01K4EAF14T65H4PHVHGFMPR501",
		RequestChannel:  "agent.semantic.request",
		ResponseChannel: "agent.semantic.response",
		RedisClient:    rdb,
		WeaviateClient: client,
		Project:        "centerfire",  // Default project
		Environment:    "dev",         // Default environment
		ctx:            context.Background(),
	}
}

// Start - Start listening for requests
func (a *SemanticAgent) Start() {
	fmt.Printf("%s starting...\n", a.AgentID)
	fmt.Printf("Listening on: %s\n", a.RequestChannel)
	
	// Test Redis connection
	_, err := a.RedisClient.Ping(a.ctx).Result()
	if err != nil {
		fmt.Printf("Failed to connect to Redis: %v\n", err)
		return
	}
	fmt.Printf("Connected to Redis successfully\n")
	
	// Test Weaviate connection
	err = a.testWeaviateConnection()
	if err != nil {
		fmt.Printf("Failed to connect to Weaviate: %v\n", err)
		return
	}
	fmt.Printf("Connected to Weaviate successfully\n")
	
	// Initialize schema if needed
	err = a.initializeSchema()
	if err != nil {
		fmt.Printf("Failed to initialize schema: %v\n", err)
		return
	}
	fmt.Printf("Schema initialized successfully\n")
	
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

// testWeaviateConnection - Test connection to Weaviate
func (a *SemanticAgent) testWeaviateConnection() error {
	ready, err := a.WeaviateClient.Misc().ReadyChecker().Do(a.ctx)
	if err != nil {
		return fmt.Errorf("readiness check failed: %v", err)
	}
	if !ready {
		return fmt.Errorf("weaviate not ready")
	}
	return nil
}

// initializeSchema - Initialize Weaviate schema for concepts with namespacing
func (a *SemanticAgent) initializeSchema() error {
	// Create namespaced classes for default environment and all supported environments
	environments := []string{"dev", "test", "prod"}
	
	for _, env := range environments {
		className := a.getClassName(a.Project, env, "concept")
		
		// Check if namespaced class exists
		exists, err := a.WeaviateClient.Schema().ClassExistenceChecker().WithClassName(className).Do(a.ctx)
		if err != nil {
			return fmt.Errorf("failed to check class existence for %s: %v", className, err)
		}
		
		if !exists {
			// Create namespaced Concept class
			conceptClass := &models.Class{
				Class:      className,
				Vectorizer: "text2vec-transformers",
				Properties: []*models.Property{
					{
						Name:         "name",
						DataType:     []string{"text"},
						Description:  "Name of the concept",
					},
					{
						Name:         "description", 
						DataType:     []string{"text"},
						Description:  "Description of the concept",
					},
					{
						Name:         "domain",
						DataType:     []string{"text"},
						Description:  "Domain or category of the concept",
					},
					{
						Name:         "cid",
						DataType:     []string{"text"},
						Description:  "Centerfire identifier",
					},
					{
						Name:         "metadata",
						DataType:     []string{"text"},
						Description:  "Additional metadata as JSON",
					},
					{
						Name:         "project",
						DataType:     []string{"text"},
						Description:  "Project namespace",
					},
					{
						Name:         "environment",
						DataType:     []string{"text"},
						Description:  "Environment (dev/test/prod)",
					},
				},
			}
			
			err = a.WeaviateClient.Schema().ClassCreator().WithClass(conceptClass).Do(a.ctx)
			if err != nil {
				return fmt.Errorf("failed to create %s class: %v", className, err)
			}
			fmt.Printf("%s: Created %s class in Weaviate\n", a.AgentID, className)
		}
	}
	
	// For backward compatibility, check if legacy "Concept" class exists
	exists, err := a.WeaviateClient.Schema().ClassExistenceChecker().WithClassName("Concept").Do(a.ctx)
	if err != nil {
		return fmt.Errorf("failed to check legacy Concept class existence: %v", err)
	}
	
	if exists {
		fmt.Printf("%s: Legacy Concept class exists - maintaining backward compatibility\n", a.AgentID)
	}
	
	return nil
}

// Helper methods for namespace management

// validateEnvironment - Validate environment parameter
func (a *SemanticAgent) validateEnvironment(env string) bool {
	validEnvs := []string{"dev", "test", "prod"}
	for _, validEnv := range validEnvs {
		if env == validEnv {
			return true
		}
	}
	return false
}

// getNamespace - Get semantic namespace from AGT-NAMING-1 instead of string concatenation
func (a *SemanticAgent) getNamespace(project, env string) string {
	// Call AGT-NAMING-1 for semantic namespace allocation
	namespaceResponse := a.requestSemanticNamespace(project, env, "")
	if namespace, ok := namespaceResponse["namespace"].(string); ok {
		return namespace
	}
	
	// Fallback to simple concatenation if naming agent fails
	fmt.Printf("%s: Warning - AGT-NAMING-1 namespace request failed, using fallback\n", a.AgentID)
	return fmt.Sprintf("%s.%s", project, env)
}

// getClassName - Generate semantic className from AGT-NAMING-1 instead of string concatenation
func (a *SemanticAgent) getClassName(project, env, classType string) string {
	// Call AGT-NAMING-1 for semantic namespace + className allocation
	namespaceResponse := a.requestSemanticNamespace(project, env, classType)
	if className, ok := namespaceResponse["className"].(string); ok {
		return className
	}
	
	// Fallback to simple concatenation if naming agent fails
	fmt.Printf("%s: Warning - AGT-NAMING-1 className request failed, using fallback\n", a.AgentID)
	projectTitle := strings.Title(strings.ToLower(project))
	envTitle := strings.Title(strings.ToLower(env))
	classTitle := strings.Title(strings.ToLower(classType))
	return fmt.Sprintf("%s_%s_%s", projectTitle, envTitle, classTitle)
}

// extractEnvironmentFromRequest - Extract environment from request, default to agent's environment
func (a *SemanticAgent) extractEnvironmentFromRequest(request map[string]interface{}) (string, error) {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return a.Environment, nil // Use default environment
	}
	
	if env, ok := params["environment"].(string); ok {
		if !a.validateEnvironment(env) {
			return "", fmt.Errorf("invalid environment: %s. Valid environments: dev, test, prod", env)
		}
		return env, nil
	}
	
	return a.Environment, nil // Use default environment
}

// generateNamespacedCID - Generate CID with namespace prefix
func (a *SemanticAgent) generateNamespacedCID(project, env, conceptType string) string {
	now := time.Now()
	ulid := fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
	namespace := a.getNamespace(project, env)
	return fmt.Sprintf("cid:%s:%s:%s", namespace, conceptType, ulid)
}

// requestSemanticNamespace - Request semantic namespace from AGT-NAMING-1 instead of string concatenation
func (a *SemanticAgent) requestSemanticNamespace(project, environment, classType string) map[string]interface{} {
	// Create request for AGT-NAMING-1
	request := map[string]interface{}{
		"from":   a.AgentID,
		"action": "allocate_namespace",
		"params": map[string]interface{}{
			"project":     project,
			"environment": environment,
		},
	}
	
	// Include class_type if provided
	if classType != "" {
		request["params"].(map[string]interface{})["class_type"] = classType
	}
	
	// Send request to AGT-NAMING-1
	requestData, _ := json.Marshal(request)
	err := a.RedisClient.Publish(a.ctx, "agent.naming.request", requestData).Err()
	if err != nil {
		fmt.Printf("%s: Error sending request to AGT-NAMING-1: %v\n", a.AgentID, err)
		return map[string]interface{}{}
	}
	
	// Subscribe to response channel temporarily to get the response
	pubsub := a.RedisClient.Subscribe(a.ctx, "agent.naming.response")
	defer pubsub.Close()
	
	// Wait for response with timeout
	ch := pubsub.Channel()
	timeout := time.After(5 * time.Second)
	
	select {
	case msg := <-ch:
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(msg.Payload), &response); err != nil {
			fmt.Printf("%s: Error parsing AGT-NAMING-1 response: %v\n", a.AgentID, err)
			return map[string]interface{}{}
		}
		return response
	case <-timeout:
		fmt.Printf("%s: Timeout waiting for AGT-NAMING-1 response\n", a.AgentID)
		return map[string]interface{}{}
	}
}

// processMessage - Process incoming Redis message
func (a *SemanticAgent) processMessage(payload string) {
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
func (a *SemanticAgent) HandleRequest(request map[string]interface{}) map[string]interface{} {
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
		}
	}
	
	// Route to appropriate handler
	switch action {
	case "semantic_similarity":
		return a.handleSemanticSimilarity(request)
	case "store_concept":
		return a.handleStoreConcept(request)
	case "query_concepts":
		return a.handleQueryConcepts(request)
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown action: %s", action),
		}
	}
}

// handleSemanticSimilarity - Find semantically similar concepts in namespaced classes
func (a *SemanticAgent) handleSemanticSimilarity(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	// Extract and validate environment
	environment, err := a.extractEnvironmentFromRequest(request)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	
	// Extract project or use default
	project := a.Project
	if p, ok := params["project"].(string); ok && p != "" {
		project = p
	}
	
	query, ok := params["query"].(string)
	if !ok {
		return map[string]interface{}{"error": "Query text required"}
	}
	
	limit := 5
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}
	
	// Get namespaced class name
	className := a.getClassName(project, environment, "concept")
	
	fmt.Printf("%s: Finding similar concepts for '%s' in class '%s'\n", a.AgentID, query, className)
	
	// Build GraphQL query for semantic similarity
	nearText := a.WeaviateClient.GraphQL().NearTextArgBuilder().
		WithConcepts([]string{query})
	
	result, err := a.WeaviateClient.GraphQL().Get().
		WithClassName(className).
		WithFields(
			graphql.Field{Name: "name"},
			graphql.Field{Name: "description"},
			graphql.Field{Name: "domain"},
			graphql.Field{Name: "cid"},
			graphql.Field{Name: "project"},
			graphql.Field{Name: "environment"},
			graphql.Field{Name: "_additional", Fields: []graphql.Field{
				{Name: "certainty"},
			}},
		).
		WithNearText(nearText).
		WithLimit(limit).
		Do(a.ctx)
	
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Failed to query %s: %v", className, err),
		}
	}
	
	// Parse results
	concepts := []map[string]interface{}{}
	if result.Data != nil {
		if get, ok := result.Data["Get"].(map[string]interface{}); ok {
			if conceptList, ok := get[className].([]interface{}); ok {
				for _, item := range conceptList {
					if concept, ok := item.(map[string]interface{}); ok {
						concepts = append(concepts, concept)
					}
				}
			}
		}
	}
	
	return map[string]interface{}{
		"success":     true,
		"query":       query,
		"concepts":    concepts,
		"count":       len(concepts),
		"project":     project,
		"environment": environment,
		"namespace":   a.getNamespace(project, environment),
		"class":       className,
	}
}

// handleStoreConcept - Store a new concept in Weaviate with namespacing
func (a *SemanticAgent) handleStoreConcept(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	// Extract and validate environment
	environment, err := a.extractEnvironmentFromRequest(request)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	
	// Extract project or use default
	project := a.Project
	if p, ok := params["project"].(string); ok && p != "" {
		project = p
	}
	
	name, ok := params["name"].(string)
	if !ok {
		return map[string]interface{}{"error": "Concept name required"}
	}
	
	description, ok := params["description"].(string)
	if !ok {
		description = ""
	}
	
	domain, ok := params["domain"].(string)
	if !ok {
		domain = "general"
	}
	
	cid, ok := params["cid"].(string)
	if !ok {
		// Generate namespaced CID if not provided
		cid = a.generateNamespacedCID(project, environment, "concept")
	}
	
	metadata := ""
	if m, ok := params["metadata"]; ok {
		if metadataBytes, err := json.Marshal(m); err == nil {
			metadata = string(metadataBytes)
		}
	}
	
	// Get namespaced class name
	className := a.getClassName(project, environment, "concept")
	
	fmt.Printf("%s: Storing concept '%s' via Redis stream (not direct Weaviate write)\n", a.AgentID, name)
	
	// CHANGED: Route through Redis streams instead of direct Weaviate write
	err = a.publishSemanticEvent(map[string]interface{}{
		"event_type":    "concept_stored",
		"name":          name,
		"description":   description,
		"domain":        domain,
		"cid":           cid,
		"metadata":      metadata,
		"project":       project,
		"environment":   environment,
		"className":     className,
		"namespace":     a.getNamespace(project, environment),
	})
	
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Failed to publish semantic event: %v", err),
		}
	}
	
	return map[string]interface{}{
		"success":     true,
		"message":     fmt.Sprintf("Stored concept: %s in namespace %s.%s", name, project, environment),
		"name":        name,
		"description": description,
		"domain":      domain,
		"cid":         cid,
		"project":     project,
		"environment": environment,
		"namespace":   a.getNamespace(project, environment),
	}
}

// handleQueryConcepts - Query concepts by filters within namespaced classes
func (a *SemanticAgent) handleQueryConcepts(request map[string]interface{}) map[string]interface{} {
	params, ok := request["params"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{"error": "No params provided"}
	}
	
	// Extract and validate environment
	environment, err := a.extractEnvironmentFromRequest(request)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	
	// Extract project or use default
	project := a.Project
	if p, ok := params["project"].(string); ok && p != "" {
		project = p
	}
	
	limit := 10
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}
	
	// Get namespaced class name
	className := a.getClassName(project, environment, "concept")
	
	fmt.Printf("%s: Querying concepts in class '%s' with limit: %d\n", a.AgentID, className, limit)
	
	query := a.WeaviateClient.GraphQL().Get().
		WithClassName(className).
		WithFields(
			graphql.Field{Name: "name"},
			graphql.Field{Name: "description"},
			graphql.Field{Name: "domain"},
			graphql.Field{Name: "cid"},
			graphql.Field{Name: "project"},
			graphql.Field{Name: "environment"},
		).
		WithLimit(limit)
	
	// TODO: Add domain filter support
	// For now, query returns all concepts in the namespace without additional filtering
	
	result, err := query.Do(a.ctx)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("Failed to query %s: %v", className, err),
		}
	}
	
	// Parse results
	concepts := []map[string]interface{}{}
	if result.Data != nil {
		if get, ok := result.Data["Get"].(map[string]interface{}); ok {
			if conceptList, ok := get[className].([]interface{}); ok {
				for _, item := range conceptList {
					if concept, ok := item.(map[string]interface{}); ok {
						concepts = append(concepts, concept)
					}
				}
			}
		}
	}
	
	return map[string]interface{}{
		"success":     true,
		"concepts":    concepts,
		"count":       len(concepts),
		"project":     project,
		"environment": environment,
		"namespace":   a.getNamespace(project, environment),
		"class":       className,
	}
}

// publishSemanticEvent - Publish semantic events to Redis streams for W/N consumers
func (a *SemanticAgent) publishSemanticEvent(eventData map[string]interface{}) error {
	streamName := "centerfire:semantic:concepts"
	
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		return fmt.Errorf("error marshaling event data: %v", err)
	}
	
	_, err = a.RedisClient.XAdd(a.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data":      string(eventJSON),
			"timestamp": time.Now().Unix(),
			"source":    a.AgentID,
		},
	}).Result()
	
	if err != nil {
		return fmt.Errorf("error publishing to stream: %v", err)
	}
	
	fmt.Printf("%s: Published semantic event to stream: %s (type: %s)\n", a.AgentID, streamName, eventData["event_type"])
	return nil
}

func main() {
	agent := NewAgent()
	if agent == nil {
		fmt.Printf("Failed to create agent\n")
		return
	}
	agent.Start()
}
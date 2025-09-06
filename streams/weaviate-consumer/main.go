package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

const (
	// Stream and connection configuration
	StreamName    = "centerfire.learning.conversations"
	ConsumerGroup = "weaviate_consumers"
	ConsumerName  = "weaviate_consumer_1"
	RedisAddr     = "mem0-redis:6380"
	WeaviateURL   = "http://centerfire-weaviate:8080"
	
	// Weaviate class name following semantic namespace
	WeaviateClass = "Centerfire_Learning_Conversation"
)

// ConversationEvent represents the stream event structure
type ConversationEvent struct {
	Timestamp    time.Time              `json:"timestamp"`
	SessionID    string                 `json:"session_id"`
	AgentID      string                 `json:"agent_id"`
	AgentActions []AgentAction          `json:"agent_actions"`
	Decisions    []Decision             `json:"decisions"`
	Outcomes     []Outcome              `json:"outcomes"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type AgentAction struct {
	Type        string                 `json:"type"`
	Tool        string                 `json:"tool,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Result      interface{}            `json:"result,omitempty"`
}

type Decision struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Context     map[string]interface{} `json:"context"`
	Options     []string               `json:"options"`
	Chosen      string                 `json:"chosen"`
	Reasoning   string                 `json:"reasoning"`
	Confidence  float64                `json:"confidence"`
	Timestamp   time.Time              `json:"timestamp"`
}

type Outcome struct {
	DecisionID  string                 `json:"decision_id"`
	Success     bool                   `json:"success"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Impact      string                 `json:"impact"`
	Metrics     map[string]float64     `json:"metrics"`
	Timestamp   time.Time              `json:"timestamp"`
}

// WeaviateConsumer handles consuming from Redis streams and storing in Weaviate
type WeaviateConsumer struct {
	redisClient   *redis.Client
	weaviateClient *weaviate.Client
	mu           sync.RWMutex
	stats        ConsumerStats
	stopChan     chan bool
}

type ConsumerStats struct {
	EventsProcessed int64     `json:"events_processed"`
	EventsStored    int64     `json:"events_stored"`
	Errors          int64     `json:"errors"`
	StartTime       time.Time `json:"start_time"`
	LastProcessed   time.Time `json:"last_processed"`
}

func NewWeaviateConsumer() *WeaviateConsumer {
	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     RedisAddr,
		Password: "",
		DB:       0,
	})

	// Initialize Weaviate client
	cfg := weaviate.Config{
		Host:   "centerfire-weaviate:8080",
		Scheme: "http",
	}
	
	weaviateClient, err := weaviate.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Weaviate client: %v", err)
	}

	return &WeaviateConsumer{
		redisClient:    rdb,
		weaviateClient: weaviateClient,
		stats: ConsumerStats{
			StartTime: time.Now(),
		},
		stopChan: make(chan bool, 1),
	}
}

func (wc *WeaviateConsumer) Connect(ctx context.Context) error {
	// Test Redis connection
	_, err := wc.redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis at %s: %v", RedisAddr, err)
	}

	// Test Weaviate connection
	ready, err := wc.weaviateClient.Misc().ReadyChecker().Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Weaviate at %s: %v", WeaviateURL, err)
	}
	if !ready {
		return fmt.Errorf("Weaviate at %s is not ready", WeaviateURL)
	}

	log.Printf("Connected to Redis at %s and Weaviate at %s", RedisAddr, WeaviateURL)
	return nil
}

func (wc *WeaviateConsumer) SetupWeaviateSchema(ctx context.Context) error {
	// Check if class already exists
	exists, err := wc.weaviateClient.Schema().ClassExistenceChecker().
		WithClassName(WeaviateClass).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check class existence: %v", err)
	}

	if exists {
		log.Printf("Weaviate class %s already exists", WeaviateClass)
		return nil
	}

	// Create the class schema
	class := &models.Class{
		Class:       WeaviateClass,
		Description: "Centerfire learning conversations with semantic analysis",
		Properties: []*models.Property{
			{
				Name:        "sessionId",
				Description: "Session identifier",
				DataType:    []string{"text"},
			},
			{
				Name:        "agentId",
				Description: "Agent identifier",
				DataType:    []string{"text"},
			},
			{
				Name:        "timestamp",
				Description: "Event timestamp",
				DataType:    []string{"date"},
			},
			{
				Name:        "agentActions",
				Description: "Agent actions taken during conversation",
				DataType:    []string{"text"},
			},
			{
				Name:        "decisions",
				Description: "Decisions made during conversation",
				DataType:    []string{"text"},
			},
			{
				Name:        "outcomes",
				Description: "Outcomes of decisions and actions",
				DataType:    []string{"text"},
			},
			{
				Name:        "conversationSummary",
				Description: "Semantic summary of the conversation for vector embedding",
				DataType:    []string{"text"},
			},
			{
				Name:        "learningContext",
				Description: "Learning context and insights extracted",
				DataType:    []string{"text"},
			},
			{
				Name:        "decisionPatterns",
				Description: "Decision patterns identified in conversation",
				DataType:    []string{"text"},
			},
			{
				Name:        "namespace",
				Description: "Semantic namespace for the conversation",
				DataType:    []string{"text"},
			},
		},
		Vectorizer: "text2vec-contextionary",
	}

	err = wc.weaviateClient.Schema().ClassCreator().
		WithClass(class).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Weaviate class: %v", err)
	}

	log.Printf("Created Weaviate class %s", WeaviateClass)
	return nil
}

func (wc *WeaviateConsumer) ProcessEvent(ctx context.Context, event ConversationEvent) error {
	// Generate semantic summary for vector embedding
	summary := wc.generateSemanticSummary(event)
	learningContext := wc.extractLearningContext(event)
	decisionPatterns := wc.identifyDecisionPatterns(event)

	// Convert complex objects to JSON strings for storage
	actionsJSON, _ := json.Marshal(event.AgentActions)
	decisionsJSON, _ := json.Marshal(event.Decisions)
	outcomesJSON, _ := json.Marshal(event.Outcomes)

	// Create Weaviate object
	properties := map[string]interface{}{
		"sessionId":           event.SessionID,
		"agentId":            event.AgentID,
		"timestamp":          event.Timestamp.Format(time.RFC3339),
		"agentActions":       string(actionsJSON),
		"decisions":          string(decisionsJSON),
		"outcomes":           string(outcomesJSON),
		"conversationSummary": summary,
		"learningContext":    learningContext,
		"decisionPatterns":   decisionPatterns,
		"namespace":          "centerfire.learning",
	}

	// Store in Weaviate with vector embedding
	result, err := wc.weaviateClient.Data().Creator().
		WithClassName(WeaviateClass).
		WithProperties(properties).
		Do(ctx)
	if err != nil {
		wc.mu.Lock()
		wc.stats.Errors++
		wc.mu.Unlock()
		return fmt.Errorf("failed to store in Weaviate: %v", err)
	}

	wc.mu.Lock()
	wc.stats.EventsStored++
	wc.mu.Unlock()

	log.Printf("Stored conversation in Weaviate with ID: %s", result.Object.ID)
	return nil
}

func (wc *WeaviateConsumer) generateSemanticSummary(event ConversationEvent) string {
	// Generate semantic summary for better vector embeddings
	summary := fmt.Sprintf("Agent %s in session %s performed %d actions, made %d decisions with %d outcomes.",
		event.AgentID, event.SessionID, len(event.AgentActions), len(event.Decisions), len(event.Outcomes))

	// Add decision context
	for _, decision := range event.Decisions {
		summary += fmt.Sprintf(" Decision '%s': chose '%s' from %v with confidence %.2f - %s.",
			decision.Type, decision.Chosen, decision.Options, decision.Confidence, decision.Reasoning)
	}

	// Add outcome context
	for _, outcome := range event.Outcomes {
		summary += fmt.Sprintf(" Outcome: %s with impact '%s'.", 
			map[bool]string{true: "successful", false: "failed"}[outcome.Success], outcome.Impact)
	}

	return summary
}

func (wc *WeaviateConsumer) extractLearningContext(event ConversationEvent) string {
	contexts := []string{}
	
	// Extract tools used
	toolsUsed := make(map[string]bool)
	for _, action := range event.AgentActions {
		if action.Tool != "" {
			toolsUsed[action.Tool] = true
		}
	}
	
	if len(toolsUsed) > 0 {
		tools := []string{}
		for tool := range toolsUsed {
			tools = append(tools, tool)
		}
		contexts = append(contexts, fmt.Sprintf("Tools utilized: %v", tools))
	}

	// Extract decision types
	decisionTypes := make(map[string]int)
	for _, decision := range event.Decisions {
		decisionTypes[decision.Type]++
	}
	
	if len(decisionTypes) > 0 {
		contexts = append(contexts, fmt.Sprintf("Decision types: %v", decisionTypes))
	}

	// Calculate success rates
	successfulActions := 0
	for _, action := range event.AgentActions {
		if action.Success {
			successfulActions++
		}
	}
	
	if len(event.AgentActions) > 0 {
		successRate := float64(successfulActions) / float64(len(event.AgentActions))
		contexts = append(contexts, fmt.Sprintf("Action success rate: %.2f", successRate))
	}

	if len(contexts) == 0 {
		return "No specific learning context identified"
	}

	return fmt.Sprintf("Learning context: %s", fmt.Sprintf("%v", contexts))
}

func (wc *WeaviateConsumer) identifyDecisionPatterns(event ConversationEvent) string {
	if len(event.Decisions) == 0 {
		return "No decision patterns identified"
	}

	patterns := []string{}
	
	// Identify high-confidence decisions
	highConfidenceCount := 0
	for _, decision := range event.Decisions {
		if decision.Confidence > 0.8 {
			highConfidenceCount++
		}
	}
	
	if highConfidenceCount > 0 {
		patterns = append(patterns, fmt.Sprintf("%d high-confidence decisions", highConfidenceCount))
	}

	// Identify sequential decision types
	if len(event.Decisions) > 1 {
		patterns = append(patterns, fmt.Sprintf("Sequential decision chain of %d decisions", len(event.Decisions)))
	}

	// Check for consistent decision reasoning patterns
	reasoningWords := make(map[string]int)
	for _, decision := range event.Decisions {
		// Simple word frequency analysis
		if len(decision.Reasoning) > 0 {
			if decision.Reasoning[:3] == "Sch" {
				reasoningWords["search-based"]++
			} else if decision.Reasoning[:3] == "Ana" {
				reasoningWords["analysis-based"]++
			}
		}
	}

	if len(reasoningWords) > 0 {
		patterns = append(patterns, fmt.Sprintf("Reasoning patterns: %v", reasoningWords))
	}

	if len(patterns) == 0 {
		return "Standard decision patterns observed"
	}

	return fmt.Sprintf("Decision patterns: %s", fmt.Sprintf("%v", patterns))
}

func (wc *WeaviateConsumer) StartConsuming(ctx context.Context) {
	log.Printf("Starting Weaviate consumer for stream %s", StreamName)

	// Create consumer group if it doesn't exist
	_, err := wc.redisClient.XGroupCreateMkStream(ctx, StreamName, ConsumerGroup, "0").Result()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Printf("Warning: Failed to create consumer group: %v", err)
	}

	for {
		select {
		case <-wc.stopChan:
			log.Println("Stopping Weaviate consumer...")
			return
		default:
			// Read from stream
			streams, err := wc.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    ConsumerGroup,
				Consumer: ConsumerName,
				Streams:  []string{StreamName, ">"},
				Count:    1,
				Block:    1 * time.Second,
			}).Result()

			if err != nil {
				if err != redis.Nil {
					log.Printf("Error reading from stream: %v", err)
					wc.mu.Lock()
					wc.stats.Errors++
					wc.mu.Unlock()
				}
				continue
			}

			// Process messages
			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := wc.processMessage(ctx, message); err != nil {
						log.Printf("Error processing message %s: %v", message.ID, err)
						wc.mu.Lock()
						wc.stats.Errors++
						wc.mu.Unlock()
					} else {
						// Acknowledge message
						wc.redisClient.XAck(ctx, StreamName, ConsumerGroup, message.ID)
						wc.mu.Lock()
						wc.stats.EventsProcessed++
						wc.stats.LastProcessed = time.Now()
						wc.mu.Unlock()
					}
				}
			}
		}
	}
}

func (wc *WeaviateConsumer) processMessage(ctx context.Context, message redis.XMessage) error {
	// Extract event data from message
	eventDataStr, exists := message.Values["data"].(string)
	if !exists {
		return fmt.Errorf("no event data found in message")
	}

	var event ConversationEvent
	if err := json.Unmarshal([]byte(eventDataStr), &event); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %v", err)
	}

	// Process the event
	return wc.ProcessEvent(ctx, event)
}

func (wc *WeaviateConsumer) Stop() {
	select {
	case wc.stopChan <- true:
	default:
	}
}

func (wc *WeaviateConsumer) GetStats() ConsumerStats {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.stats
}

func (wc *WeaviateConsumer) Close() error {
	if err := wc.redisClient.Close(); err != nil {
		return err
	}
	return nil
}

// HTTP API handlers
func (wc *WeaviateConsumer) handleGetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wc.GetStats())
}

func (wc *WeaviateConsumer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check Redis connection
	_, err := wc.redisClient.Ping(ctx).Result()
	if err != nil {
		http.Error(w, fmt.Sprintf("Redis connection failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Check Weaviate connection
	ready, err := wc.weaviateClient.Misc().ReadyChecker().Do(ctx)
	if err != nil || !ready {
		http.Error(w, fmt.Sprintf("Weaviate connection failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":         "healthy",
		"stream":         StreamName,
		"consumer_group": ConsumerGroup,
		"weaviate_class": WeaviateClass,
	})
}

func main() {
	log.Println("Starting Centerfire Learning Weaviate Consumer...")

	consumer := NewWeaviateConsumer()

	// Connect to services
	ctx := context.Background()
	if err := consumer.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to services: %v", err)
	}

	// Setup Weaviate schema
	if err := consumer.SetupWeaviateSchema(ctx); err != nil {
		log.Fatalf("Failed to setup Weaviate schema: %v", err)
	}

	// Set up HTTP server for monitoring
	router := mux.NewRouter()
	router.HandleFunc("/stats", consumer.handleGetStats).Methods("GET")
	router.HandleFunc("/health", consumer.handleHealthCheck).Methods("GET")

	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	// Start HTTP server in goroutine
	go func() {
		log.Println("Weaviate consumer HTTP API listening on :8081")
		log.Println("Endpoints:")
		log.Println("  GET /stats  - Get consumer statistics")
		log.Println("  GET /health - Health check")
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Start consuming in goroutine
	go consumer.StartConsuming(ctx)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down consumer...")

	// Stop consumer
	consumer.Stop()

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Close connections
	if err := consumer.Close(); err != nil {
		log.Printf("Consumer close error: %v", err)
	}

	log.Println("Weaviate consumer shutdown complete")
}
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
)

const (
	// Semantic namespace for learning streams
	StreamName = "centerfire.learning.conversations"
	RedisAddr  = "mem0-redis:6380"
)

// ConversationEvent represents a structured conversation log entry
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

// StreamProducer handles Redis stream operations
type StreamProducer struct {
	client *redis.Client
	mu     sync.RWMutex
	stats  ProducerStats
}

type ProducerStats struct {
	EventsPublished int64     `json:"events_published"`
	Errors          int64     `json:"errors"`
	StartTime       time.Time `json:"start_time"`
}

func NewStreamProducer() *StreamProducer {
	rdb := redis.NewClient(&redis.Options{
		Addr:     RedisAddr,
		Password: "",
		DB:       0,
	})

	return &StreamProducer{
		client: rdb,
		stats: ProducerStats{
			StartTime: time.Now(),
		},
	}
}

func (sp *StreamProducer) Connect(ctx context.Context) error {
	// Test connection
	_, err := sp.client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis at %s: %v", RedisAddr, err)
	}

	log.Printf("Connected to Redis at %s", RedisAddr)
	return nil
}

func (sp *StreamProducer) PublishEvent(ctx context.Context, event ConversationEvent) error {
	// Convert event to map for Redis stream
	eventData, err := json.Marshal(event)
	if err != nil {
		sp.mu.Lock()
		sp.stats.Errors++
		sp.mu.Unlock()
		return fmt.Errorf("failed to marshal event: %v", err)
	}

	// Create Redis stream entry
	args := &redis.XAddArgs{
		Stream: StreamName,
		Values: map[string]interface{}{
			"event_type":  "conversation",
			"session_id":  event.SessionID,
			"agent_id":    event.AgentID,
			"timestamp":   event.Timestamp.Unix(),
			"data":        string(eventData),
		},
	}

	// Add to stream
	result, err := sp.client.XAdd(ctx, args).Result()
	if err != nil {
		sp.mu.Lock()
		sp.stats.Errors++
		sp.mu.Unlock()
		return fmt.Errorf("failed to add to stream %s: %v", StreamName, err)
	}

	sp.mu.Lock()
	sp.stats.EventsPublished++
	sp.mu.Unlock()

	log.Printf("Published event to stream %s with ID: %s", StreamName, result)
	return nil
}

func (sp *StreamProducer) GetStats() ProducerStats {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.stats
}

func (sp *StreamProducer) Close() error {
	return sp.client.Close()
}

// HTTP API handlers
func (sp *StreamProducer) handlePublishEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event ConversationEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sp.PublishEvent(ctx, event); err != nil {
		http.Error(w, fmt.Sprintf("Failed to publish event: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Event published successfully",
	})
}

func (sp *StreamProducer) handleGetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sp.GetStats())
}

func (sp *StreamProducer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := sp.client.Ping(ctx).Result()
	if err != nil {
		http.Error(w, fmt.Sprintf("Redis connection failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"stream": StreamName,
	})
}

// Example event generator for testing
func (sp *StreamProducer) generateExampleEvent() ConversationEvent {
	return ConversationEvent{
		Timestamp: time.Now(),
		SessionID: fmt.Sprintf("session_%d", time.Now().Unix()),
		AgentID:   "centerfire_learning_agent",
		AgentActions: []AgentAction{
			{
				Type:      "tool_usage",
				Tool:      "search",
				Parameters: map[string]interface{}{
					"query": "semantic analysis patterns",
				},
				StartTime: time.Now().Add(-2 * time.Minute),
				EndTime:   time.Now().Add(-1 * time.Minute),
				Success:   true,
				Result:    "Found 15 relevant patterns",
			},
		},
		Decisions: []Decision{
			{
				ID:   "decision_1",
				Type: "tool_selection",
				Context: map[string]interface{}{
					"available_tools": []string{"search", "analyze", "categorize"},
				},
				Options:    []string{"search", "analyze"},
				Chosen:     "search",
				Reasoning:  "Search provides broader context for analysis",
				Confidence: 0.85,
				Timestamp:  time.Now().Add(-90 * time.Second),
			},
		},
		Outcomes: []Outcome{
			{
				DecisionID: "decision_1",
				Success:    true,
				Result:     "Successfully identified semantic patterns",
				Impact:     "Improved understanding of data relationships",
				Metrics: map[string]float64{
					"execution_time": 45.2,
					"accuracy":       0.92,
				},
				Timestamp: time.Now().Add(-30 * time.Second),
			},
		},
		Metadata: map[string]interface{}{
			"namespace":     "centerfire.learning",
			"version":       "1.0",
			"source":        "stream_producer",
			"environment":   "development",
		},
	}
}

func main() {
	log.Println("Starting Centerfire Learning Stream Producer...")

	producer := NewStreamProducer()

	// Connect to Redis
	ctx := context.Background()
	if err := producer.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Set up HTTP server
	router := mux.NewRouter()
	router.HandleFunc("/publish", producer.handlePublishEvent).Methods("POST")
	router.HandleFunc("/stats", producer.handleGetStats).Methods("GET")
	router.HandleFunc("/health", producer.handleHealthCheck).Methods("GET")

	// Test endpoint for generating example events
	router.HandleFunc("/test/publish", func(w http.ResponseWriter, r *http.Request) {
		event := producer.generateExampleEvent()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := producer.PublishEvent(ctx, event); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Test event published",
		})
	}).Methods("POST")

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Println("Stream producer HTTP API listening on :8080")
		log.Println("Endpoints:")
		log.Println("  POST /publish - Publish conversation event")
		log.Println("  GET  /stats   - Get producer statistics")
		log.Println("  GET  /health  - Health check")
		log.Println("  POST /test/publish - Publish test event")
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down producer...")

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Close Redis connection
	if err := producer.Close(); err != nil {
		log.Printf("Redis connection close error: %v", err)
	}

	log.Println("Producer shutdown complete")
}
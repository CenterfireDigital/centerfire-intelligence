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
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

const (
	// Stream and connection configuration
	StreamName     = "centerfire.learning.conversations"
	ConsumerGroup  = "neo4j_consumers"
	ConsumerName   = "neo4j_consumer_1"
	RedisAddr      = "mem0-redis:6380"
	Neo4jURI       = "bolt://centerfire-neo4j:7687"
	Neo4jUsername  = "neo4j"
	Neo4jPassword  = "centerfire123"
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

// Neo4jConsumer handles consuming from Redis streams and storing in Neo4j
type Neo4jConsumer struct {
	redisClient *redis.Client
	neo4jDriver neo4j.DriverWithContext
	mu          sync.RWMutex
	stats       ConsumerStats
	stopChan    chan bool
}

type ConsumerStats struct {
	EventsProcessed     int64     `json:"events_processed"`
	RelationshipsCreated int64     `json:"relationships_created"`
	NodesCreated        int64     `json:"nodes_created"`
	Errors              int64     `json:"errors"`
	StartTime           time.Time `json:"start_time"`
	LastProcessed       time.Time `json:"last_processed"`
}

func NewNeo4jConsumer() *Neo4jConsumer {
	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     RedisAddr,
		Password: "",
		DB:       0,
	})

	// Initialize Neo4j driver
	driver, err := neo4j.NewDriverWithContext(Neo4jURI, 
		neo4j.BasicAuth(Neo4jUsername, Neo4jPassword, ""))
	if err != nil {
		log.Fatalf("Failed to create Neo4j driver: %v", err)
	}

	return &Neo4jConsumer{
		redisClient: rdb,
		neo4jDriver: driver,
		stats: ConsumerStats{
			StartTime: time.Now(),
		},
		stopChan: make(chan bool, 1),
	}
}

func (nc *Neo4jConsumer) Connect(ctx context.Context) error {
	// Test Redis connection
	_, err := nc.redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis at %s: %v", RedisAddr, err)
	}

	// Test Neo4j connection
	err = nc.neo4jDriver.VerifyConnectivity(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Neo4j at %s: %v", Neo4jURI, err)
	}

	log.Printf("Connected to Redis at %s and Neo4j at %s", RedisAddr, Neo4jURI)
	return nil
}

func (nc *Neo4jConsumer) SetupNeo4jConstraints(ctx context.Context) error {
	session := nc.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	// Create constraints to ensure unique nodes
	constraints := []string{
		"CREATE CONSTRAINT session_id_unique IF NOT EXISTS FOR (s:Session) REQUIRE s.session_id IS UNIQUE",
		"CREATE CONSTRAINT decision_id_unique IF NOT EXISTS FOR (d:Decision) REQUIRE d.decision_id IS UNIQUE",
		"CREATE CONSTRAINT outcome_id_unique IF NOT EXISTS FOR (o:Outcome) REQUIRE o.outcome_id IS UNIQUE",
		"CREATE CONSTRAINT agent_id_unique IF NOT EXISTS FOR (a:Agent) REQUIRE a.agent_id IS UNIQUE",
		"CREATE CONSTRAINT action_id_unique IF NOT EXISTS FOR (ac:Action) REQUIRE ac.action_id IS UNIQUE",
	}

	for _, constraint := range constraints {
		_, err := session.Run(ctx, constraint, nil)
		if err != nil {
			log.Printf("Warning: Failed to create constraint: %v", err)
		}
	}

	// Create indexes for better performance
	indexes := []string{
		"CREATE INDEX session_timestamp IF NOT EXISTS FOR (s:Session) ON (s.timestamp)",
		"CREATE INDEX decision_timestamp IF NOT EXISTS FOR (d:Decision) ON (d.timestamp)",
		"CREATE INDEX outcome_timestamp IF NOT EXISTS FOR (o:Outcome) ON (o.timestamp)",
		"CREATE INDEX action_timestamp IF NOT EXISTS FOR (ac:Action) ON (ac.start_time)",
	}

	for _, index := range indexes {
		_, err := session.Run(ctx, index, nil)
		if err != nil {
			log.Printf("Warning: Failed to create index: %v", err)
		}
	}

	log.Println("Neo4j constraints and indexes setup complete")
	return nil
}

func (nc *Neo4jConsumer) ProcessEvent(ctx context.Context, event ConversationEvent) error {
	session := nc.neo4jDriver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	// Execute in a transaction
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		nodeCount, relationshipCount, err := nc.createGraphFromEvent(ctx, tx, event)
		if err != nil {
			return nil, err
		}

		nc.mu.Lock()
		nc.stats.NodesCreated += nodeCount
		nc.stats.RelationshipsCreated += relationshipCount
		nc.mu.Unlock()

		return nil, nil
	})

	if err != nil {
		nc.mu.Lock()
		nc.stats.Errors++
		nc.mu.Unlock()
		return fmt.Errorf("failed to create graph in Neo4j: %v", err)
	}

	log.Printf("Successfully processed conversation event for session %s", event.SessionID)
	return nil
}

func (nc *Neo4jConsumer) createGraphFromEvent(ctx context.Context, tx neo4j.ManagedTransaction, event ConversationEvent) (int64, int64, error) {
	var nodeCount, relationshipCount int64

	// 1. Create or merge Session node
	sessionQuery := `
		MERGE (s:Session {session_id: $session_id})
		ON CREATE SET 
			s.created_at = $timestamp,
			s.agent_id = $agent_id,
			s.namespace = 'centerfire.learning'
		ON MATCH SET 
			s.last_updated = $timestamp
		RETURN s
	`
	
	_, err := tx.Run(ctx, sessionQuery, map[string]interface{}{
		"session_id": event.SessionID,
		"timestamp":  event.Timestamp,
		"agent_id":   event.AgentID,
	})
	if err != nil {
		return 0, 0, err
	}
	nodeCount++

	// 2. Create or merge Agent node
	agentQuery := `
		MERGE (a:Agent {agent_id: $agent_id})
		ON CREATE SET 
			a.created_at = $timestamp,
			a.namespace = 'centerfire.learning'
		ON MATCH SET 
			a.last_seen = $timestamp
		RETURN a
	`
	
	_, err = tx.Run(ctx, agentQuery, map[string]interface{}{
		"agent_id":  event.AgentID,
		"timestamp": event.Timestamp,
	})
	if err != nil {
		return nodeCount, 0, err
	}
	nodeCount++

	// 3. Create Session -> Agent relationship
	sessionAgentQuery := `
		MATCH (s:Session {session_id: $session_id})
		MATCH (a:Agent {agent_id: $agent_id})
		MERGE (s)-[r:HANDLED_BY]->(a)
		ON CREATE SET r.created_at = $timestamp
		RETURN r
	`
	
	_, err = tx.Run(ctx, sessionAgentQuery, map[string]interface{}{
		"session_id": event.SessionID,
		"agent_id":   event.AgentID,
		"timestamp":  event.Timestamp,
	})
	if err != nil {
		return nodeCount, 0, err
	}
	relationshipCount++

	// 4. Process Actions
	for i, action := range event.AgentActions {
		actionID := fmt.Sprintf("%s_action_%d", event.SessionID, i)
		
		actionQuery := `
			MATCH (s:Session {session_id: $session_id})
			CREATE (ac:Action {
				action_id: $action_id,
				type: $type,
				tool: $tool,
				success: $success,
				start_time: $start_time,
				end_time: $end_time,
				duration: $duration,
				parameters: $parameters,
				result: $result,
				error: $error
			})
			CREATE (s)-[r:CONTAINS]->(ac)
			SET r.created_at = $timestamp
			RETURN ac
		`
		
		duration := action.EndTime.Sub(action.StartTime).Seconds()
		parametersJSON, _ := json.Marshal(action.Parameters)
		resultJSON, _ := json.Marshal(action.Result)
		
		_, err = tx.Run(ctx, actionQuery, map[string]interface{}{
			"session_id": event.SessionID,
			"action_id":  actionID,
			"type":       action.Type,
			"tool":       action.Tool,
			"success":    action.Success,
			"start_time": action.StartTime,
			"end_time":   action.EndTime,
			"duration":   duration,
			"parameters": string(parametersJSON),
			"result":     string(resultJSON),
			"error":      action.Error,
			"timestamp":  event.Timestamp,
		})
		if err != nil {
			return nodeCount, relationshipCount, err
		}
		nodeCount++
		relationshipCount++
	}

	// 5. Process Decisions
	for _, decision := range event.Decisions {
		decisionQuery := `
			MATCH (s:Session {session_id: $session_id})
			CREATE (d:Decision {
				decision_id: $decision_id,
				type: $type,
				chosen: $chosen,
				reasoning: $reasoning,
				confidence: $confidence,
				timestamp: $timestamp,
				options: $options,
				context: $context
			})
			CREATE (s)-[r:CONTAINS]->(d)
			SET r.created_at = $event_timestamp
			RETURN d
		`
		
		optionsJSON, _ := json.Marshal(decision.Options)
		contextJSON, _ := json.Marshal(decision.Context)
		
		_, err = tx.Run(ctx, decisionQuery, map[string]interface{}{
			"session_id":      event.SessionID,
			"decision_id":     decision.ID,
			"type":           decision.Type,
			"chosen":         decision.Chosen,
			"reasoning":      decision.Reasoning,
			"confidence":     decision.Confidence,
			"timestamp":      decision.Timestamp,
			"options":        string(optionsJSON),
			"context":        string(contextJSON),
			"event_timestamp": event.Timestamp,
		})
		if err != nil {
			return nodeCount, relationshipCount, err
		}
		nodeCount++
		relationshipCount++
	}

	// 6. Process Outcomes and create Decision -> Outcome relationships
	for i, outcome := range event.Outcomes {
		outcomeID := fmt.Sprintf("%s_outcome_%d", event.SessionID, i)
		
		// Create Outcome node
		outcomeQuery := `
			MATCH (s:Session {session_id: $session_id})
			CREATE (o:Outcome {
				outcome_id: $outcome_id,
				success: $success,
				impact: $impact,
				timestamp: $timestamp,
				result: $result,
				error: $error,
				metrics: $metrics
			})
			CREATE (s)-[r:CONTAINS]->(o)
			SET r.created_at = $event_timestamp
			RETURN o
		`
		
		resultJSON, _ := json.Marshal(outcome.Result)
		metricsJSON, _ := json.Marshal(outcome.Metrics)
		
		_, err = tx.Run(ctx, outcomeQuery, map[string]interface{}{
			"session_id":      event.SessionID,
			"outcome_id":      outcomeID,
			"success":        outcome.Success,
			"impact":         outcome.Impact,
			"timestamp":      outcome.Timestamp,
			"result":         string(resultJSON),
			"error":          outcome.Error,
			"metrics":        string(metricsJSON),
			"event_timestamp": event.Timestamp,
		})
		if err != nil {
			return nodeCount, relationshipCount, err
		}
		nodeCount++
		relationshipCount++

		// Create Decision -> Outcome relationship if decision exists
		if outcome.DecisionID != "" {
			decisionOutcomeQuery := `
				MATCH (d:Decision {decision_id: $decision_id})
				MATCH (o:Outcome {outcome_id: $outcome_id})
				MERGE (d)-[r:LEADS_TO]->(o)
				ON CREATE SET r.created_at = $timestamp
				RETURN r
			`
			
			_, err = tx.Run(ctx, decisionOutcomeQuery, map[string]interface{}{
				"decision_id": outcome.DecisionID,
				"outcome_id":  outcomeID,
				"timestamp":   event.Timestamp,
			})
			if err != nil {
				log.Printf("Warning: Failed to create decision-outcome relationship: %v", err)
			} else {
				relationshipCount++
			}
		}
	}

	// 7. Create temporal relationships between consecutive decisions
	if len(event.Decisions) > 1 {
		for i := 0; i < len(event.Decisions)-1; i++ {
			temporalQuery := `
				MATCH (d1:Decision {decision_id: $decision_id_1})
				MATCH (d2:Decision {decision_id: $decision_id_2})
				MERGE (d1)-[r:FOLLOWED_BY]->(d2)
				ON CREATE SET r.created_at = $timestamp
				RETURN r
			`
			
			_, err = tx.Run(ctx, temporalQuery, map[string]interface{}{
				"decision_id_1": event.Decisions[i].ID,
				"decision_id_2": event.Decisions[i+1].ID,
				"timestamp":     event.Timestamp,
			})
			if err != nil {
				log.Printf("Warning: Failed to create temporal relationship: %v", err)
			} else {
				relationshipCount++
			}
		}
	}

	return nodeCount, relationshipCount, nil
}

func (nc *Neo4jConsumer) StartConsuming(ctx context.Context) {
	log.Printf("Starting Neo4j consumer for stream %s", StreamName)

	// Create consumer group if it doesn't exist
	_, err := nc.redisClient.XGroupCreateMkStream(ctx, StreamName, ConsumerGroup, "0").Result()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Printf("Warning: Failed to create consumer group: %v", err)
	}

	for {
		select {
		case <-nc.stopChan:
			log.Println("Stopping Neo4j consumer...")
			return
		default:
			// Read from stream
			streams, err := nc.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    ConsumerGroup,
				Consumer: ConsumerName,
				Streams:  []string{StreamName, ">"},
				Count:    1,
				Block:    1 * time.Second,
			}).Result()

			if err != nil {
				if err != redis.Nil {
					log.Printf("Error reading from stream: %v", err)
					nc.mu.Lock()
					nc.stats.Errors++
					nc.mu.Unlock()
				}
				continue
			}

			// Process messages
			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := nc.processMessage(ctx, message); err != nil {
						log.Printf("Error processing message %s: %v", message.ID, err)
						nc.mu.Lock()
						nc.stats.Errors++
						nc.mu.Unlock()
					} else {
						// Acknowledge message
						nc.redisClient.XAck(ctx, StreamName, ConsumerGroup, message.ID)
						nc.mu.Lock()
						nc.stats.EventsProcessed++
						nc.stats.LastProcessed = time.Now()
						nc.mu.Unlock()
					}
				}
			}
		}
	}
}

func (nc *Neo4jConsumer) processMessage(ctx context.Context, message redis.XMessage) error {
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
	return nc.ProcessEvent(ctx, event)
}

func (nc *Neo4jConsumer) Stop() {
	select {
	case nc.stopChan <- true:
	default:
	}
}

func (nc *Neo4jConsumer) GetStats() ConsumerStats {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	return nc.stats
}

func (nc *Neo4jConsumer) Close() error {
	if err := nc.redisClient.Close(); err != nil {
		return err
	}
	if err := nc.neo4jDriver.Close(context.Background()); err != nil {
		return err
	}
	return nil
}

// HTTP API handlers
func (nc *Neo4jConsumer) handleGetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nc.GetStats())
}

func (nc *Neo4jConsumer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check Redis connection
	_, err := nc.redisClient.Ping(ctx).Result()
	if err != nil {
		http.Error(w, fmt.Sprintf("Redis connection failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Check Neo4j connection
	err = nc.neo4jDriver.VerifyConnectivity(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Neo4j connection failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":         "healthy",
		"stream":         StreamName,
		"consumer_group": ConsumerGroup,
	})
}

func main() {
	log.Println("Starting Centerfire Learning Neo4j Consumer...")

	consumer := NewNeo4jConsumer()

	// Connect to services
	ctx := context.Background()
	if err := consumer.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to services: %v", err)
	}

	// Setup Neo4j constraints and indexes
	if err := consumer.SetupNeo4jConstraints(ctx); err != nil {
		log.Fatalf("Failed to setup Neo4j constraints: %v", err)
	}

	// Set up HTTP server for monitoring
	router := mux.NewRouter()
	router.HandleFunc("/stats", consumer.handleGetStats).Methods("GET")
	router.HandleFunc("/health", consumer.handleHealthCheck).Methods("GET")

	server := &http.Server{
		Addr:    ":8082",
		Handler: router,
	}

	// Start HTTP server in goroutine
	go func() {
		log.Println("Neo4j consumer HTTP API listening on :8082")
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

	log.Println("Neo4j consumer shutdown complete")
}
# Centerfire Learning Stream Pipeline

A high-performance Redis Streams to Weaviate/Neo4j pipeline for capturing and analyzing conversation learning data from the Centerfire Intelligence ecosystem.

## Architecture

The pipeline follows semantic namespacing (`centerfire.learning`) and consists of three dedicated Go processes:

### 1. Stream Producer (`/producer/`)
- **Purpose**: Publishes structured conversation logs to Redis Streams
- **Stream**: `centerfire.learning.conversations`  
- **Connection**: Redis at `mem0-redis:6380`
- **Port**: 8080
- **Features**:
  - HTTP API for publishing events
  - Structured conversation event schema
  - Health checks and statistics
  - Example test event generation

### 2. Weaviate Consumer (`/weaviate-consumer/`)
- **Purpose**: Consumes stream events and stores semantic embeddings in Weaviate
- **Connection**: Weaviate at `centerfire-weaviate:8080`
- **Consumer Group**: `weaviate_consumers`
- **Port**: 8081
- **Features**:
  - Automatic Weaviate schema creation (`Centerfire_Learning_Conversation` class)
  - Vector embeddings for conversation summaries
  - Learning context extraction
  - Decision pattern identification

### 3. Neo4j Consumer (`/neo4j-consumer/`)
- **Purpose**: Consumes stream events and creates relationship graphs in Neo4j
- **Connection**: Neo4j at `centerfire-neo4j:7687`
- **Consumer Group**: `neo4j_consumers`
- **Port**: 8082
- **Features**:
  - Graph relationships: `(:Session)-[:CONTAINS]->(:Decision)-[:LEADS_TO]->(:Outcome)`
  - Temporal decision chains
  - Action and outcome tracking
  - Performance metrics storage

## Quick Start

### 1. Start the Complete Pipeline
```bash
./start-pipeline.sh start
```

### 2. Check Status
```bash
./start-pipeline.sh status
```

### 3. Test with Sample Event
```bash
./start-pipeline.sh test
```

### 4. Stop Pipeline
```bash
./start-pipeline.sh stop
```

## Event Schema

Conversation events follow this structure:

```json
{
  "timestamp": "2025-09-05T22:00:00Z",
  "session_id": "session_12345",
  "agent_id": "centerfire_learning_agent",
  "agent_actions": [
    {
      "type": "tool_usage",
      "tool": "search",
      "parameters": {"query": "semantic patterns"},
      "start_time": "2025-09-05T21:58:00Z",
      "end_time": "2025-09-05T21:59:00Z",
      "success": true,
      "result": "Found 15 patterns"
    }
  ],
  "decisions": [
    {
      "id": "decision_1",
      "type": "tool_selection",
      "context": {"available_tools": ["search", "analyze"]},
      "options": ["search", "analyze"],
      "chosen": "search",
      "reasoning": "Provides broader context",
      "confidence": 0.85,
      "timestamp": "2025-09-05T21:57:30Z"
    }
  ],
  "outcomes": [
    {
      "decision_id": "decision_1",
      "success": true,
      "result": "Successfully identified patterns",
      "impact": "Improved understanding",
      "metrics": {"execution_time": 45.2, "accuracy": 0.92},
      "timestamp": "2025-09-05T21:59:30Z"
    }
  ],
  "metadata": {
    "namespace": "centerfire.learning",
    "version": "1.0",
    "environment": "development"
  }
}
```

## API Endpoints

### Producer (Port 8080)
- `POST /publish` - Publish conversation event
- `POST /test/publish` - Publish test event
- `GET /stats` - Get producer statistics
- `GET /health` - Health check

### Weaviate Consumer (Port 8081)
- `GET /stats` - Get consumer statistics
- `GET /health` - Health check

### Neo4j Consumer (Port 8082)
- `GET /stats` - Get consumer statistics  
- `GET /health` - Health check

## Publishing Events

### Manual Event Publishing
```bash
curl -X POST http://localhost:8080/publish \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "test_session_1",
    "agent_id": "test_agent",
    "agent_actions": [...],
    "decisions": [...],
    "outcomes": [...],
    "metadata": {"namespace": "centerfire.learning"}
  }'
```

### Test Event Publishing
```bash
curl -X POST http://localhost:8080/test/publish
```

## Data Storage

### Weaviate Storage
- **Class**: `Centerfire_Learning_Conversation`
- **Features**: Vector embeddings, semantic search, learning context analysis
- **Properties**: sessionId, agentId, conversationSummary, learningContext, decisionPatterns

### Neo4j Storage  
- **Nodes**: Session, Agent, Decision, Action, Outcome
- **Relationships**: 
  - `(Session)-[:HANDLED_BY]->(Agent)`
  - `(Session)-[:CONTAINS]->(Decision|Action|Outcome)`
  - `(Decision)-[:LEADS_TO]->(Outcome)`
  - `(Decision)-[:FOLLOWED_BY]->(Decision)` (temporal)

## Monitoring

### Health Checks
The pipeline manager performs automated health checks every 30 seconds:
- Process status monitoring
- Service endpoint health verification  
- Automatic restart on failure

### Statistics
Real-time statistics available via HTTP APIs:
- Events processed/published
- Error counts
- Processing rates
- Connection status

### Logs
Logs are stored in `/streams/logs/`:
- `pipeline.log` - Pipeline management logs
- `producer.log` - Producer service logs
- `weaviate-consumer.log` - Weaviate consumer logs
- `neo4j-consumer.log` - Neo4j consumer logs

## Dependencies

Each component uses Go modules with the following key dependencies:

### Producer
- `github.com/go-redis/redis/v8` - Redis client
- `github.com/gorilla/mux` - HTTP router

### Weaviate Consumer
- `github.com/go-redis/redis/v8` - Redis client
- `github.com/weaviate/weaviate-go-client/v4` - Weaviate client
- `github.com/gorilla/mux` - HTTP router

### Neo4j Consumer
- `github.com/go-redis/redis/v8` - Redis client  
- `github.com/neo4j/neo4j-go-driver/v5` - Neo4j driver
- `github.com/gorilla/mux` - HTTP router

## Configuration

### Connection Settings
- Redis: `mem0-redis:6380`
- Weaviate: `centerfire-weaviate:8080`
- Neo4j: `centerfire-neo4j:7687` (neo4j/centerfire123)

### Stream Configuration
- Stream Name: `centerfire.learning.conversations`
- Consumer Groups: `weaviate_consumers`, `neo4j_consumers`
- Consumer Names: `weaviate_consumer_1`, `neo4j_consumer_1`

## Semantic Namespacing

All components follow the `centerfire.learning` semantic namespace:
- Stream names prefixed with namespace
- Weaviate class names: `Centerfire_Learning_*`  
- Neo4j node properties include namespace
- Metadata includes namespace identification

## Scalability

The pipeline is designed for horizontal scaling:
- **Consumer Groups**: Multiple consumers can join the same group for load balancing
- **Dedicated Processes**: Each component runs independently for optimal resource usage
- **Stateless Design**: Components can be restarted/scaled without data loss
- **Health Monitoring**: Automatic failure detection and recovery

## Troubleshooting

### Common Issues

1. **Connection Failures**: Verify Redis/Weaviate/Neo4j services are running
2. **Permission Errors**: Ensure script has execute permissions (`chmod +x start-pipeline.sh`)
3. **Port Conflicts**: Check ports 8080, 8081, 8082 are available
4. **Go Dependencies**: Run `go mod download` in each component directory

### Debug Commands
```bash
# Check service logs
./start-pipeline.sh logs

# Test individual components
cd producer && go run main.go
cd weaviate-consumer && go run main.go  
cd neo4j-consumer && go run main.go

# Check Redis stream
redis-cli -h mem0-redis -p 6380 XINFO STREAM centerfire.learning.conversations
```
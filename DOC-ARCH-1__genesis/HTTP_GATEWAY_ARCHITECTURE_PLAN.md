# HTTP Gateway Architecture Implementation Plan

## Overview
Transform the current Redis pub/sub + Socket dual-mode agent system into a **tiered HTTP Gateway architecture** with contract-based access control and socket-centric agent communication.

## Current State Analysis

### Existing Components:
- **AGT-MANAGER-1**: Enhanced PID-based collision detection, heartbeat monitoring
- **AGT-NAMING-1**: Enhanced with PID reporting and heartbeat system  
- **AGT-STRUCT-1, AGT-SEMANTIC-1**: Basic Redis pub/sub + socket listeners
- **orchestrator-go**: Socket-based multi-interface orchestrator (HTTP:8090)
- **Redis**: Agent communication backbone (mem0-redis:6380)

### Current Communication Flow:
```
Claude Code → Redis pub/sub → Agents
System Processes → Unix Sockets → Agents  
```

## Target Architecture

### New Communication Flow:
```
Claude Code → HTTP Gateway → Unix Sockets → Agents
Trusted Processes → Direct Unix Sockets → Agents
Web Clients → HTTP Gateway → Unix Sockets → Agents
```

### Components to Build:
1. **AGT-HTTP-GATEWAY-1**: HTTP→Socket proxy with contract validation
2. **Socket-only Agents**: Remove Redis pub/sub dependencies
3. **SemDoc Contracts**: Define access permissions per client
4. **Claude Code HTTP Client**: Replace Redis pub/sub calls

## Implementation Plan

### Phase 1: Foundation (Estimated: 4-6 hours)
**Goal**: Prepare existing components for migration

#### Step 1.1: Enhance Orchestrator as HTTP Gateway
**File**: `orchestrator-go/main.go`
**Changes**:
- Add SemDoc contract loading from filesystem
- Implement contract validation middleware
- Add structured agent routing: `/api/agents/{agent}/{action}`
- Add agent discovery endpoint: `/api/agents/available`
- Enhanced error handling with HTTP status codes

**Implementation Details**:
```go
// New endpoints to add:
GET  /api/agents/available           // List online agents
GET  /api/contracts/{client_id}      // Get client permissions  
POST /api/agents/{agent}/{action}    // Proxy to agent socket
GET  /api/health                     // System health check
```

#### Step 1.2: Create SemDoc Contract System
**File**: `contracts/claude_code_access.yaml`
```yaml
client_id: claude_code
version: "1.0"
access_permissions:
  allowed_agents: 
    - naming: ["allocate_capability", "allocate_module", "allocate_namespace"]
    - struct: ["create_structure", "delegate_documentation"]  
    - semantic: ["store_concept", "query_concepts", "semantic_similarity"]
  forbidden_agents: [manager, cleanup]
  rate_limits:
    requests_per_minute: 100
    burst_limit: 10
security:
  authentication: api_key
  authorization: contract_based
```

#### Step 1.3: Agent Socket Interface Standardization  
**Files**: All agent `main.go` files
**Changes**:
- Standardize socket message format (JSON request/response)
- Implement graceful socket handling with proper error responses
- Add socket listener health checking
- Remove Redis pub/sub dependencies (Phase 2)

**Standard Socket Message Format**:
```json
// Request
{
  "action": "allocate_capability",
  "data": {"domain": "AUTH", "description": "user authentication"},
  "client_id": "claude_code",
  "request_id": "req_12345"
}

// Response  
{
  "success": true,
  "data": {"capability_id": "CAP-AUTH-2__01K4EAF3"},
  "request_id": "req_12345",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### Phase 2: Agent Migration (Estimated: 3-4 hours)
**Goal**: Convert agents from dual-mode to socket-only

#### Step 2.1: AGT-NAMING-1 Socket-Only Migration
**File**: `agents/AGT-NAMING-1__01K4EAF1/main.go`
**Changes**:
- Remove Redis client initialization
- Remove Redis pub/sub listener
- Keep Unix socket listener only  
- Update heartbeat to use socket communication to manager
- Maintain PID tracking and collision detection via socket calls

#### Step 2.2: AGT-STRUCT-1, AGT-SEMANTIC-1 Migration
**Files**: `agents/AGT-STRUCT-1__01K4EAF1/main.go`, `agents/AGT-SEMANTIC-1__01K4EAF1/main.go`
**Changes**: Same pattern as AGT-NAMING-1

#### Step 2.3: AGT-MANAGER-1 Enhancement
**File**: `agents/AGT-MANAGER-1__manager1/main.go`  
**Changes**:
- Keep Redis for agent state persistence
- Add socket listener for agent management
- Implement socket-based heartbeat collection
- Enhanced agent registry with socket connection tracking

### Phase 3: HTTP Gateway Implementation (Estimated: 5-6 hours)
**Goal**: Complete HTTP Gateway with contract validation

#### Step 3.1: Contract Validation Engine
**New File**: `orchestrator-go/contracts.go`
```go
type ContractValidator struct {
    contracts map[string]*ClientContract
}

func (cv *ContractValidator) ValidateRequest(clientID, agent, action string) error
func (cv *ContractValidator) LoadContracts(contractsDir string) error  
func (cv *ContractValidator) GetClientPermissions(clientID string) *ClientContract
```

#### Step 3.2: Agent Proxy Engine
**New File**: `orchestrator-go/agent_proxy.go`
```go
type AgentProxy struct {
    socketConnections map[string]*net.Conn
}

func (ap *AgentProxy) ForwardToAgent(agent, action string, data interface{}) (*AgentResponse, error)
func (ap *AgentProxy) GetAvailableAgents() map[string]*AgentStatus
func (ap *AgentProxy) HealthCheckAgent(agent string) error
```

#### Step 3.3: HTTP Endpoint Implementation  
**File**: `orchestrator-go/main.go` enhancements
```go
// Enhanced HTTP handlers:
func (o *Orchestrator) handleAgentRequest(w http.ResponseWriter, r *http.Request)
func (o *Orchestrator) handleAgentDiscovery(w http.ResponseWriter, r *http.Request)
func (o *Orchestrator) handleContractInfo(w http.ResponseWriter, r *http.Request)
```

### Phase 4: Claude Code Integration (Estimated: 2-3 hours)
**Goal**: Replace Redis pub/sub with HTTP client

#### Step 4.1: HTTP Client Module  
**New File**: `claude_code_client/agent_client.go`
```go
type AgentClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func (ac *AgentClient) CallAgent(agent, action string, data interface{}) (*AgentResponse, error)
func (ac *AgentClient) GetAvailableAgents() (map[string]*AgentInfo, error)
func (ac *AgentClient) Health() error
```

#### Step 4.2: Integration Points
**Goal**: Replace Claude Code's current Redis calls with HTTP calls
- Naming allocations: `POST /api/agents/naming/allocate_capability`
- Structure creation: `POST /api/agents/struct/create_structure`  
- Semantic operations: `POST /api/agents/semantic/store_concept`

### Phase 5: Testing & Validation (Estimated: 2-3 hours)  
**Goal**: End-to-end validation of new architecture

#### Step 5.1: Unit Tests
- Contract validation logic
- Agent proxy functionality  
- Socket message handling
- HTTP endpoint responses

#### Step 5.2: Integration Tests  
- Claude Code → Gateway → Agent flow
- Contract enforcement (allowed/forbidden agents)
- Rate limiting functionality
- Error handling and recovery

#### Step 5.3: Performance Testing
- Socket vs HTTP performance comparison
- Concurrent request handling
- Memory usage under load

## Migration Strategy

### Backward Compatibility Plan:
1. **Dual Operation**: Run old Redis + new HTTP systems in parallel
2. **Gradual Migration**: Migrate agents one by one to socket-only
3. **Client Migration**: Update Claude Code to use HTTP gradually
4. **Deprecation**: Remove Redis pub/sub after full HTTP adoption

### Rollback Strategy:
```bash
# Emergency rollback to previous commit
git revert 43d807d  # Current enhanced collision detection commit
./start-agents.sh restart  # Restart with Redis pub/sub mode
```

### Risk Mitigation:
- **Socket Connection Failures**: Implement connection pooling and retry logic
- **HTTP Gateway Downtime**: Maintain direct socket access for critical processes
- **Contract Validation Errors**: Implement permissive mode for development
- **Performance Degradation**: Monitor latency and implement caching

## Success Metrics

### Technical Metrics:
- [ ] All agents respond via HTTP Gateway within 50ms
- [ ] Contract violations properly rejected with 403 status  
- [ ] Socket connections stable under 100 concurrent requests
- [ ] Zero Redis pub/sub dependencies in agents
- [ ] Claude Code successfully calls agents via HTTP

### Architectural Metrics:  
- [ ] Single point of access control (HTTP Gateway)
- [ ] Clear separation: HTTP clients vs Socket clients
- [ ] SemDoc contracts enforce security boundaries
- [ ] Agent discovery works dynamically
- [ ] Monitoring and logging centralized at gateway

## File Structure After Implementation

```
├── orchestrator-go/
│   ├── main.go              # Enhanced HTTP Gateway
│   ├── contracts.go         # Contract validation engine  
│   ├── agent_proxy.go       # Socket proxy functionality
│   └── middleware.go        # HTTP middleware (auth, logging)
├── contracts/
│   ├── claude_code_access.yaml
│   ├── web_ui_access.yaml
│   └── system_process_access.yaml
├── agents/                  # Socket-only agents
│   ├── AGT-NAMING-1__01K4EAF1/main.go
│   ├── AGT-STRUCT-1__01K4EAF1/main.go
│   ├── AGT-SEMANTIC-1__01K4EAF1/main.go
│   └── AGT-MANAGER-1__manager1/main.go
├── claude_code_client/
│   └── agent_client.go      # HTTP client for Claude Code
└── tests/
    ├── integration_test.go
    └── performance_test.go
```

## Context Recovery Instructions

### If You Lose Context:
1. **Read this file** for complete plan overview
2. **Check current commit**: `git log --oneline -5`
3. **Check running agents**: `./start-agents.sh status`  
4. **Review current architecture**: Read `DIR-SYS-1__genesis/claude-agent-protocol.yaml`
5. **Check infrastructure**: `docker ps | grep -E "(redis|weaviate|neo4j)"`

### Key Implementation Order:
1. **Start with orchestrator-go enhancements** (HTTP Gateway)
2. **Create contracts system** (SemDoc validation)  
3. **Migrate agents to socket-only** (Remove Redis dependencies)
4. **Build Claude Code HTTP client** (Replace Redis calls)
5. **Test end-to-end** (Validation and performance)

### Critical Design Decisions Made:
- **Tiered Access**: HTTP clients → Gateway → Socket agents
- **Contract-Based Security**: SemDoc YAML defines permissions
- **Socket-Centric**: Agents only listen on Unix sockets
- **Proxy Pattern**: Gateway proxies HTTP → Socket transparently
- **Backward Compatible**: Gradual migration strategy

This plan provides complete context recovery capability and step-by-step implementation guidance.
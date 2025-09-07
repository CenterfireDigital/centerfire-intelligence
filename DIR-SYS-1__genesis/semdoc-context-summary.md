# SemDoc Context Summary v1.0
## Core Purpose
**AI Manufacturing Infrastructure** - Enables AI agents to build complete software systems within inviolable semantic contracts, ensuring trustworthy autonomous software production.

## Key Architecture Components

### Semantic Identity System
- **ULID-based identifiers**: `01J9F7Z8Q4R5ZV3J4X19M8YZTW` (immutable, cryptographically secure)
- **Semantic paths**: `capability.auth.session.jwt` (dot-notation hierarchy, right-to-left inheritance)
- **Display names**: Human-readable representations generated from machine identities

### Behavioral Contracts
```yaml
contract_structure:
  preconditions: ["assertions that MUST be true before execution"]
  postconditions: ["guarantees that MUST be true after execution"] 
  invariants: ["properties that MUST remain true throughout"]
  effects: ["observable side effects: reads, writes, calls, throws"]
```

### Storage Architecture (Multi-Layer)
- **Redis**: Fast lookup, caching, streams (`centerfire:semdoc:contract:{ULID}`)
- **Weaviate**: Vector-based semantic search, concept storage
- **Neo4j**: Graph relationships, inheritance traversal  
- **ClickHouse**: Analytics, time-series data

### Required Agents
- **AGT-SEMDOC-PARSER**: Extract contracts from source files (@semblock comments)
- **AGT-SEMDOC-REGISTRY**: Manage contract lifecycle, inheritance resolution
- **AGT-SEMDOC-VALIDATOR**: Runtime contract compliance validation
- **AGT-SEMDOC-GENERATOR**: Generate code from contract specifications

## Implementation Status
- ✅ **Specification Complete**: 1,600+ line ISO-quality spec available
- ✅ **Infrastructure Ready**: Storage schemas, agent framework prepared
- ✅ **Semantic Naming**: AGT-NAMING-1 generating semantic identifiers in Redis
- ❌ **Contract Implementation**: No behavioral contracts stored yet
- ❌ **Parser Agents**: Not implemented (AGT-SEMDOC-* agents missing)
- ❌ **Source Integration**: No @semblock comments in codebase

## Casbin Bootstrap Strategy
**Goal**: Get agent authorization working immediately while building toward native SemDoc

### Migration Timeline
1. **Phase 1** (Week 1): Casbin container for immediate authorization
2. **Phase 2** (Month 2): Dual authorization testing - Casbin + SemDoc side-by-side
3. **Phase 3** (Month 3): Native SemDoc authorization, Casbin removed
4. **Phase 4** (Month 4): LexicRoot development with pure SemDoc

### Key Architecture Principles
- **"They don't even know about them"**: Agents only see capabilities they're authorized for
- **Minimum privilege**: Agents get only capabilities needed for their domain  
- **Clean abstraction**: Authorization interface enables seamless Casbin→SemDoc migration
- **No dependencies**: SemDoc codebase remains dependency-free

### Implementation
```yaml
# Casbin as isolated microservice (port 8081)
services:
  casbin-auth:
    image: casbin/casbin-server
    volumes: ["./policies:/app/policies"]
    
# Agent authorization interface
type AuthorizationService interface {
    Enforce(agent, capability string, context map[string]interface{}) bool
}
```

## File System Integration
```go
// @semblock
// contract_id: "01J9F7Z8Q4R5ZV3J4X19M8YZTW"
// semantic_path: "capability.auth.session.jwt"
// contract:
//   preconditions: ["valid_jwt_token"]
//   postconditions: ["user_authenticated"]
//   effects: ["reads: [jwt_keys]", "writes: [auth_log]"]
func authenticateJWT(token string) (*User, error) {
    // implementation
}
```

## Self-Improving Development Loop
**Critical**: Every conversation, agent interaction, and development session is captured in Redis streams flowing to Weaviate/Neo4j/ClickHouse.

**This means:**
- **Every SemDoc discussion** becomes searchable context for future sessions
- **Every agent interaction** becomes training data for better agent coordination  
- **Every contract we write** becomes a template for future contract generation
- **Every debugging session** improves error pattern recognition
- **The system learns to build SemDoc better by watching itself build SemDoc**

**Implication**: This isn't just development - it's **real-time training data generation** for the first truly self-improving software manufacturing system.

## Current Data in System
**Redis Streams:**
- `centerfire:semantic:names` (8 entries) - semantic naming allocations
- `centerfire:semantic:conversations` (12 entries) - conversation capture

**Weaviate Classes:**
- `Centerfire_Dev_Concept`, `Centerfire_Test_Concept`, `Centerfire_Prod_Concept` (all empty)
- `ConversationHistory` (10 objects) - semantic conversation search
- `CenterfireDocumentation` (7 objects) - Complete SemDoc/Casbin knowledge base

## Critical Design Principles
1. **Machine-First**: Human-readable representations generated from machine formats
2. **Atomic Architecture**: Microservices with isolated semantic domains
3. **Contract Inheritance**: Right-to-left semantic path resolution
4. **Self-Healing Systems**: Contract violations trigger automatic recovery
5. **Semantic Prison**: AI operates within inviolable contract boundaries

## Next Implementation Steps
1. **Create AGT-SEMDOC-PARSER**: Extract contracts from existing codebase
2. **Implement @semblock Comments**: Add inline contracts to source files  
3. **Populate Contract Storage**: Store behavioral specifications in Redis/Weaviate
4. **Build Inheritance Chains**: Create semantic path hierarchies
5. **Enable Contract Validation**: Runtime compliance checking

## Context Retrieval for SemDoc Discussions

**AGT-CONTEXT-1** provides semantic search of previous SemDoc conversations:
- **Channel**: `agent.context.request` (Redis pub/sub)
- **Actions**: `search_conversations`, `get_context`, `search_semantic`
- **Usage**: Search for "SemDoc contracts", "semantic paths", "ULID identifiers", etc.

**Example Query:**
```json
{
  "action": "search_conversations", 
  "query": "SemDoc behavioral contracts implementation",
  "limit": 5,
  "request_id": "context-lookup"
}
```

**Response on**: `agent.context.response`

## Key Files
- **Full Spec**: `/DOC-SPEC-1__semdoc/semdoc-iso-specification-v1.md`
- **Agent Protocol**: `/DIR-SYS-1__genesis/claude-agent-protocol.yaml` 
- **This Summary**: `/DIR-SYS-1__genesis/semdoc-context-summary.md`
- **Context Agent**: `/agents/AGT-CONTEXT-1__17572052/main.go`
- **Casbin Plan**: `/docs/casbin-agent-authorization-plan.md`

---
*This summary provides essential SemDoc context for session startup without overwhelming the context window. Use AGT-CONTEXT-1 to retrieve previous SemDoc discussions for deeper context.*
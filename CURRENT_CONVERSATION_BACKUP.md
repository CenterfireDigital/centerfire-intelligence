# Critical Architecture Conversation - 2025-09-07
## AGT-SEMDOC-PARSER-1 Design and Agent Infrastructure Analysis

**Session ID**: manual-backup-20250907
**Participants**: Human (Larry), Assistant (Claude)
**Topic**: SemDoc Parser implementation, agent architecture, Redis streams
**Importance**: HIGH - Core architecture decisions

---

## Key Architecture Decisions Made:

### 1. Storage Namespace Strategy
- **Decision**: Use `centerfire:semdocdev:*` namespace for Stage 1 development
- **Rationale**: Clear semantic separation from production SemDoc data
- **Impact**: Graph searches won't confuse dev vs production contracts

### 2. Data Flow Architecture 
- **Decision**: AGT-SEMDOC-PARSER-1 → Redis Streams → Stream Consumers → W/N/C
- **Discovery**: AGT-CLAUDE-CAPTURE-1 exists but is NOT running (critical gap)
- **Impact**: All architectural conversations being lost

### 3. Casbin Authorization Strategy
- **Decision**: Create AGT-CASBIN-1 for centralized authorization
- **Rationale**: Connection pooling, policy caching, request batching
- **Stage 1**: Start loose, tighten iteratively with EnableLog()

### 4. Error Handling Philosophy
- **Decision**: Permissive for Stage 1 - log errors and continue
- **Rationale**: Bad/wrong data acceptable if logged, focus on iteration
- **Implementation**: Full logging philosophy throughout system

### 5. ULID Assignment Priority
- **Critical Finding**: AGT-NAMING-1 needs refactor to use actual ULID format
- **Decision**: Support all three scenarios (existing, missing, invalid contract_ids)
- **Namespace**: Dev designated namespace required for development data

### 6. Stream Structure
- **Decision**: Multiple streams for different data types:
  - `centerfire:semdocdev:parsed` - Successfully extracted contracts
  - `centerfire:semdocdev:errors` - Parse failures, malformed @semblocks  
  - `centerfire:semdocdev:audit` - Who parsed what when
  - `centerfire:semdocdev:ulids` - ULID assignments and mappings
- **Rationale**: Tracking different things highly important for development process

### 7. Consumer Group Strategy  
- **Decision**: Separate consumer groups for dev vs test data:
  - `semdocdev-weaviate-consumers` (development contracts)
  - `semdoctest-weaviate-consumers` (ephemeral test data)
- **Rationale**: Prevent test data pollution, different data lifecycles

### 8. Agent Infrastructure Discovery
**Missing Running Agents** (should be operational but are NOT):
- AGT-CONTEXT-1 (context search)
- **AGT-CLAUDE-CAPTURE-1** (Redis streams writer) - THIS IS CRITICAL
- AGT-STACK-1 (container orchestration)  
- AGT-SYSTEM-COMMANDER-1 (system commands)
- AGT-NAMING-1 (ULID generation)
- AGT-MANAGER-1 (agent lifecycle)
- Stream consumers (conversation_consumer, clickhouse_consumer)

## Implementation Priorities Established:
1. **IMMEDIATE**: Start AGT-CLAUDE-CAPTURE-1 and preserve this conversation
2. Start missing operational agents
3. Refactor AGT-NAMING-1 for proper ULID format
4. Build AGT-SEMDOC-PARSER-1 with Redis streams integration
5. Create AGT-CASBIN-1 for centralized authorization

## Technical Specifications:
- **Language Priority**: Go first, then Python
- **File Discovery**: Directory scanning acceptable for Stage 1
- **Storage Pattern**: Redis + Weaviate for Stage 1, Neo4j in later stages
- **Authorization**: Casbin policies with staged tightening

## Critical Quotes:
- "Everything in all of this just development? We're not in production environment, this is meta."
- "Never be afraid to argue with me to make me prove my point because if I'm wrong, I'm happy to admit it."
- "The point is that future learning will come from these conversations."

## Conversation Status:
**URGENT**: This conversation contains critical architecture decisions that must be preserved in the semantic learning pipeline for future agent development.

---
*Generated: 2025-09-07 18:30 - Manual backup before starting AGT-CLAUDE-CAPTURE-1*
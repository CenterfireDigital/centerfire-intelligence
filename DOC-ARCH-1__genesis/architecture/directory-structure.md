# AI-Native Directory Structure for Centerfire Intelligence

## Core Principle: Capability-First, Language-Agnostic

Traditional human-centric organization (by language or team) creates AI friction. We organize by **what the system does** (capabilities), not **how it's implemented** (languages).

## Directory Structure

```
/CenterfireIntelligence/
│
├── /semdoc/                          # Control plane - the AI's map
│   ├── semdoc.yaml                   # Repository semantic README
│   ├── catalog.yaml                  # All capabilities & maturity
│   ├── ontology.jsonld               # Domain graph for Neo4j
│   └── policies/                     # Security, tenancy, governance
│       ├── risk.yaml
│       ├── tenancy.yaml
│       └── security.yaml
│
├── /platform/                        # Shared infrastructure
│   ├── schemas/                      # Cross-capability contracts
│   │   ├── llm-request.proto
│   │   ├── context-window.json
│   │   └── telemetry.yaml
│   ├── build/                        # Build configurations
│   │   ├── docker/
│   │   └── ci/
│   └── libs/                         # Shared libraries by language
│       ├── go/
│       ├── rust/
│       ├── node/
│       └── python/
│
├── /agents/                          # Agent orchestration & specs
│   ├── orchestrator/                 # The brain (Go)
│   │   ├── AgentSpec.yaml
│   │   ├── router/
│   │   ├── planner/
│   │   └── scheduler/
│   ├── agents/                       # Concrete agents
│   │   ├── code-refactor/
│   │   │   ├── AgentSpec.yaml
│   │   │   ├── prompts/
│   │   │   └── tests/
│   │   ├── context-manager/
│   │   ├── llm-router/
│   │   └── test-runner/
│   └── tools/                        # Agent capabilities
│       ├── git/
│       │   └── ToolSpec.yaml
│       ├── neo4j/
│       ├── vector-search/
│       └── llm-bridge/
│
├── /capabilities/                    # Core functionality units
│   │
│   ├── CAP-LLM-001-orchestration/    # LLM orchestration capability
│   │   ├── semdoc.yaml               # Capability contract
│   │   ├── contracts/                # Interface definitions
│   │   │   ├── grpc.proto
│   │   │   └── websocket.yaml
│   │   ├── impl/                     # Implementations by role
│   │   │   ├── service/              # Long-running services
│   │   │   │   ├── go/               # Go orchestrator
│   │   │   │   │   ├── main.go
│   │   │   │   │   └── router.go
│   │   │   │   └── rust/             # Rust performance layer
│   │   │   ├── lib/                  # Libraries
│   │   │   │   ├── node/             # Node.js client
│   │   │   │   └── python/           # Python adapters
│   │   │   └── ui/                   # User interfaces
│   │   │       └── react/            # Dashboard components
│   │   ├── tests/
│   │   │   ├── unit/
│   │   │   ├── integration/
│   │   │   └── property/
│   │   ├── telemetry/
│   │   │   └── events.yaml
│   │   └── maps/                     # Neo4j mappings
│   │       ├── code-map.json
│   │       └── test-map.json
│   │
│   ├── CAP-CTX-001-context-archive/  # Context management
│   │   ├── semdoc.yaml
│   │   ├── impl/
│   │   │   ├── service/
│   │   │   │   └── rust/             # High-performance archive
│   │   │   └── lib/
│   │   │       └── go/               # Go client library
│   │   └── tests/
│   │
│   ├── CAP-TERM-001-terminal/        # Terminal interface
│   │   ├── semdoc.yaml
│   │   ├── impl/
│   │   │   ├── service/
│   │   │   │   └── node/             # WebSocket server
│   │   │   └── ui/
│   │   │       └── react/            # xterm.js terminal
│   │   └── contracts/
│   │
│   ├── CAP-DASH-001-dashboard/       # Monitoring dashboard
│   │   ├── semdoc.yaml
│   │   ├── impl/
│   │   │   └── ui/
│   │   │       └── react/            # React dashboard
│   │   └── contracts/
│   │
│   └── CAP-VM-001-sandbox/           # VM/Container management
│       ├── semdoc.yaml
│       ├── impl/
│       │   └── service/
│       │       └── rust/             # Secure sandbox
│       └── contracts/
│
├── /products/                        # Product compositions
│   └── centerfire-dev/               # Development environment
│       ├── composition.yaml          # Which capabilities + versions
│       └── deployments/
│           ├── local/
│           └── docker/
│
├── /data/                           # Data & models
│   ├── neo4j/                       # Graph schemas & migrations
│   │   ├── schema.cypher
│   │   └── migrations/
│   ├── vectors/                     # Embedding collections
│   │   ├── semblocks/
│   │   └── documents/
│   └── models/                      # PSM adapters
│       ├── base/
│       └── fine-tuned/
│
└── /human/                          # Human-friendly views (generated)
    ├── by-language/                 # Symlinks organized by language
    │   ├── go/     -> ../capabilities/*/impl/*/go
    │   ├── rust/   -> ../capabilities/*/impl/*/rust
    │   ├── node/   -> ../capabilities/*/impl/*/node
    │   └── python/ -> ../capabilities/*/impl/*/python
    └── by-service/                  # Symlinks by service type
        ├── orchestrator/ -> ../capabilities/CAP-LLM-001/impl/service/go
        └── dashboard/    -> ../capabilities/CAP-DASH-001/impl/ui/react
```

## Key Design Decisions

### 1. Capability IDs
- Format: `CAP-<DOMAIN>-<NUMBER>-<name>`
- Examples: 
  - `CAP-LLM-001-orchestration`
  - `CAP-CTX-001-context-archive`
  - `CAP-TERM-001-terminal`

### 2. Language Placement Rules
- **Never** at top level
- Always under `impl/{role}/{language}/`
- Roles: `service`, `lib`, `ui`

### 3. Graph IDs for Neo4j
Every significant code element gets a graph ID:
```
centerfire:Function/CAP-LLM-001/routeRequest
centerfire:Service/CAP-CTX-001/ArchiveService
centerfire:Component/CAP-DASH-001/MetricsPanel
```

### 4. SemDoc Distribution
- Repository-level: `/semdoc/semdoc.yaml`
- Capability-level: `/capabilities/CAP-*/semdoc.yaml`
- File-level: SemBlocks inline with code

### 5. Agent-First Design
Agents aren't auxiliary - they're in `/agents/` at root level, equal to capabilities.

## Why This Structure Works for AI

### 1. **Semantic Locality**
When AI needs to modify "LLM orchestration", everything is in `CAP-LLM-001`:
- All implementations (Go service, Rust lib, React UI)
- Tests that validate it
- Contracts it must honor
- Telemetry it emits

### 2. **Graph Navigation**
Neo4j relationships:
```cypher
(:Capability {id: "CAP-LLM-001"})
  -[:CONTAINS]->(:Service {lang: "go"})
  -[:EXPOSES]->(:Endpoint {path: "/route"})
  -[:TESTED_BY]->(:Test {type: "integration"})
```

### 3. **Language Agnostic**
AI sees capabilities first, languages second:
- "Update orchestration" → `CAP-LLM-001`
- Then: "Go service needs this, Rust lib needs that"

### 4. **Contract-Driven**
Each capability declares its contracts upfront:
- Input/output schemas
- Performance budgets
- Security constraints
- Telemetry requirements

## Migration from Current Structure

### Phase 0: Setup (Immediate)
```bash
# Create new structure
mkdir -p semdoc platform agents capabilities products data

# Move Python daemon → CAP-LLM-001
mv daemon capabilities/CAP-LLM-001-orchestration/impl/service/python/

# Move C++ code → Rust rewrite location
mkdir -p capabilities/CAP-CTX-001-context-archive/impl/service/rust/
# (C++ code becomes reference for Rust rewrite)
```

### Phase 1: Core Capabilities
1. `CAP-LLM-001-orchestration` - Multi-LLM routing (Go)
2. `CAP-CTX-001-context-archive` - High-performance context (Rust)
3. `CAP-TERM-001-terminal` - Web terminal (Node + React)
4. `CAP-DASH-001-dashboard` - Monitoring UI (React)

### Phase 2: Support Capabilities
5. `CAP-VM-001-sandbox` - Safe execution (Rust)
6. `CAP-TEST-001-testing` - Test orchestration (Go)
7. `CAP-DEPLOY-001-deployment` - Release management (Go)

## Human Interface

For developers who prefer traditional views:
```bash
# Generate human-friendly symlinks
./scripts/generate-human-view.sh

# Creates:
# /human/by-language/go/     - All Go code
# /human/by-language/rust/   - All Rust code
# /human/by-service/          - Services grouped
```

## Tooling Integration

### VS Code Workspace
```json
{
  "folders": [
    {"path": "capabilities/CAP-LLM-001", "name": "LLM Orchestration"},
    {"path": "capabilities/CAP-CTX-001", "name": "Context Archive"},
    {"path": "agents", "name": "Agents"}
  ]
}
```

### Git Hooks
- Pre-commit: Validate SemDoc
- Post-commit: Update Neo4j graph
- Pre-push: Contract validation

## Success Metrics

1. **AI can navigate** without human help
2. **Cross-language changes** stay in one capability
3. **Tests colocated** with what they test
4. **Contracts enforced** at boundaries
5. **Neo4j graph** always current

## Anti-Patterns to Avoid

❌ `/src/go/`, `/src/rust/` - Language-first
❌ `/backend/`, `/frontend/` - Layer-first  
❌ Scattered tests in `/tests/`
❌ Documentation separate from code
❌ Contracts as afterthoughts

## The Revolution

This isn't just reorganization - it's designing for **AI-first maintenance**:
- Capabilities are stable anchors
- Languages are implementation details
- Contracts are enforced, not suggested
- The graph (Neo4j) is the source of truth
- Humans adapt to AI needs, not vice versa

Ready to restructure!
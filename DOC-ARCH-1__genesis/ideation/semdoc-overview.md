# SemDoc: Semantic Documentation System Overview

## Vision
A machine-first, human-friendly documentation standard that enables AI assistants to have **deep semantic understanding** of codebases through structured contracts, inline semantic blocks, and integrated vector/graph storage.

## Core Components

### 1. SemDoc Control Plane (`/sem-doc/`)
- **semdoc.yaml**: The "Semantic README" - machine-parsable project metadata
- **index.manifest.json**: Points to all vector/graph indices
- **domain.graph.jsonld**: Ontology and code entity schema
- **capabilities.yaml**: User-facing capabilities with guardrails
- **interfaces/**: API contracts (OpenAPI, AsyncAPI, queues)
- **maps/**: Code-to-symbol-to-purpose mappings

### 2. SemBlocks (Inline Documentation)
Structured JSON/YAML payloads embedded in comments directly above code elements:
- **Purpose**: Explicit contracts, effects, dependencies, invariants
- **Placement**: File-level and symbol-level
- **Graph Integration**: Each block has stable `graph_id` for tracking

### 3. SemDoc Contracts
Machine-enforceable specifications including:
- **Preconditions/Postconditions**: What must be true before/after
- **Invariants**: Properties that must never be violated
- **Effects**: All side effects (reads/writes/calls)
- **Performance**: Budgets and limits
- **Security**: Auth, PII handling, data residency
- **Tests/Telemetry**: Validation oracles and event schemas

## Key Innovation Points

### Machine-First Design
- Every statement is parsable and queryable
- Contracts are enforceable by CI/CD
- AI assistants can reason about impact and dependencies

### Continuous Validation
- Pre-commit hooks validate SemBlocks
- CI enforces contract-code alignment
- Performance budgets tracked automatically
- Security invariants checked continuously

### Legal/Compliance Ready
- Data tenancy flags for portfolio vs isolated models
- PII tracking at function level
- Audit trails through provenance metadata
- Clean separation for M&A scenarios

## Integration Architecture

```
Code + SemBlocks → Vector DB (semantic search)
                 ↓
                 → Graph DB (relationships)
                 ↓
                 → PSM Training (repo-specific LLM)
                 ↓
                 → CI/CD Validation
                 ↓
                 → Runtime Monitoring
```

## Adoption Path

### Level 0: Seed
- Add file-level SemBlocks to critical modules
- Document top 10 risky functions

### Level 1: Contracts
- Fill inputs/outputs/pre/post/invariants
- Add test oracles

### Level 2: Operations
- Add performance budgets, telemetry, flags
- Security annotations

### Level 3: Automation
- CI validation and graph sync
- Require SemChangePlans for stable functions

## Next Steps
See companion documents:
- `semdoc-contracts.md`: Detailed contract specifications
- `semdoc-commits.md`: Semantic commit message standard
- `ai-native-sdlc.md`: Post-Agile development methodology
- `per-repo-llm.md`: Project Specialist Model architecture
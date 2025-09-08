# Current Operating Stack - Centerfire Intelligence

> **Note**: Each update duplicates previous version with date stamp for historical tracking

---

## **Version 2025-09-07** *(Current)*

### **Core Infrastructure Containers**
| Service | Container | Image | Port(s) | Purpose |
|---------|-----------|--------|---------|----------|
| **Redis** | mem0-redis | redis:7-alpine | 6380 | Agent communication, caching, streams |
| **Weaviate** | centerfire-weaviate | semitechnologies/weaviate:1.25.0 | 8080 | Vector database, semantic search |
| **Neo4j** | centerfire-neo4j | neo4j:5.15 | 7474, 7687 | Graph database, relationships |
| **ClickHouse** | centerfire-clickhouse | clickhouse/clickhouse-server:23.12 | 8123, 9001 | Analytics, cold storage |
| **Casbin** | centerfire-casbin | casbin/casbin-server:latest | 50051 | Agent authorization (gRPC) |
| **Transformers** | centerfire-transformers | semitechnologies/transformers-inference | - | ML embeddings |

### **Languages in Active Use**
- **Go**: Primary agent development language (AGT-NAMING-1, AGT-CONTEXT-1, etc.)
- **Python**: Secondary agents, utilities (AGT-CLAUDE-CAPTURE-1, test scripts)
- **YAML**: Configuration, contracts, docker-compose
- **Bash**: Orchestration, deployment scripts
- **Markdown**: Documentation, specifications

### **Key Dependencies & Protocols**
- **Redis Pub/Sub**: Agent communication backbone
- **Redis Streams**: Data pipeline (conversations, semantic data)
- **GraphQL**: Weaviate queries for semantic search
- **Bolt Protocol**: Neo4j graph traversal
- **gRPC**: Casbin authorization service
- **HTTP/REST**: Service health checks, APIs
- **Docker Network**: mem0-network for service interconnection

### **Agent Ecosystem**
- **AGT-NAMING-1**: Semantic identifier allocation (operational)
- **AGT-CONTEXT-1**: Conversation search and retrieval (operational)
- **AGT-MANAGER-1**: Agent lifecycle and collision detection (operational)
- **AGT-SYSTEM-COMMANDER-1**: System command orchestration (operational)
- **AGT-CLAUDE-CAPTURE-1**: Claude Code session capture (operational)
- **AGT-STACK-1**: Container orchestration (operational)
- **SemDoc Agents**: Parser, Registry, Validator (planned - Stage 1)

### **Development Architecture**
- **Staged SemDoc Implementation**: Traditional → Pseudo-contracts → Evaluated → Enforced
- **RBAC Authorization**: Casbin policies for agent capabilities
- **Self-Training Loop**: All interactions feed semantic learning pipeline
- **Multi-Storage Pattern**: Redis→Weaviate/Neo4j/ClickHouse consumption

### **Notable Features**
- **Profile-based deployment**: Services start with specific profiles (analytics, casbin-auth)
- **Semantic naming convention**: PROJECT.ENV.TYPE-DOMAIN-N__ULID8 format
- **Contract-ready infrastructure**: Storage schemas prepared for behavioral contracts
- **Context preservation**: Conversation history semantically searchable

---

## **Version 2025-09-06** *(Previous)*

### **Core Infrastructure Containers**
| Service | Container | Image | Port(s) | Purpose |
|---------|-----------|--------|---------|----------|
| **Redis** | mem0-redis | redis:7-alpine | 6380 | Agent communication, caching, streams |
| **Weaviate** | centerfire-weaviate | semitechnologies/weaviate:1.25.0 | 8080 | Vector database, semantic search |
| **Neo4j** | centerfire-neo4j | neo4j:5.15 | 7474, 7687 | Graph database, relationships |
| **ClickHouse** | centerfire-clickhouse | clickhouse/clickhouse-server:23.12 | 8123, 9001 | Analytics, cold storage |
| **Transformers** | centerfire-transformers | semitechnologies/transformers-inference | - | ML embeddings |

### **Changes from Previous Version**
- ➕ **Added Casbin**: Authorization service for agent RBAC (port 50051, gRPC)
- ➕ **Added Authorization Policies**: Defined SemDoc agent capabilities and restrictions
- ➕ **Added gRPC Protocol**: Agent authorization communication method
- 📝 **Updated Agent Ecosystem**: SemDoc agents planned with Stage 1 traditional development

---

*This document tracks high-level infrastructure changes. For detailed deployment instructions, see service-specific documentation.*
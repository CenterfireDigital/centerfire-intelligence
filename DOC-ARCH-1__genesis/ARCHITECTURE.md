# Centerfire Intelligence - System Architecture

**A comprehensive semantic AI system for Claude Code with multi-storage conversation streaming and intelligent code analysis.**

## 🎯 Core Concept

Centerfire Intelligence creates a **semantic memory layer** for Claude Code sessions, enabling:
- **Persistent conversation memory** across sessions and projects
- **Intelligent code structure mapping** (not embeddings in Neo4j!)
- **Cross-project relationship discovery**
- **Training-safe data isolation** with automatic sensitive file protection

## 🏗️ Architecture Overview

### Three-Domain Separation

```
🗣️  CONVERSATION DOMAIN          🔗 LINKING DOMAIN              🏗️  CODE STRUCTURE DOMAIN
   (Neo4j + Qdrant)                (Neo4j Relationships)           (Neo4j + Weaviate)
                                                                    
Project → Conversation           Conversation ──DISCUSSED──→       CodeFile → CodeClass
   ↓         ↓                                                        ↓         ↓
Session   Turn/Topic             Turn ──MENTIONED_FUNCTION──→       CodeClass → CodeMethod
                                                                       ↓         ↓
                                 Project ──HAS_CODEBASE──→           Import    Function
```

### Storage System Specialization

| System | Purpose | Data Type | Example |
|--------|---------|-----------|---------|
| **Neo4j** | Graph relationships & code structure | Nodes + Relationships | `RedisManager ──CONTAINS──→ health_check()` |
| **Qdrant** | Semantic conversation search | Vector embeddings | "Docker service discovery issues" |
| **Weaviate** | Code semantic similarity | Vector embeddings | Functions similar to "async error handling" |
| **Redis** | Real-time streaming & caching | Streams + Cache | Conversation flow processing |

## 📊 Data Flow Architecture

### 1. Conversation Streaming Pipeline

```
Claude Code Session
        ↓
   Session Detection
   (working_dir, project_name, session_id)
        ↓
   Redis Global Stream
   "conversations:global"
        ↓
   ┌─────────────────────┬─────────────────────┬─────────────────────┐
   ↓                     ↓                     ↓                     ↓
Neo4j Storage         Qdrant Vectors        Weaviate Code        File Analysis
(Relationships)       (Conversations)       (Code Snippets)      (Security Filter)
```

### 2. Code Structure Mapping

```
Code File (Python/JS/etc)
        ↓
   Code Chunker
   (AST/Regex Parsing)
        ↓
   Logical Chunks:
   ├── MODULE (file-level)
   ├── CLASS (class definitions)
   ├── FUNCTION (function definitions)
   ├── METHOD (class methods)
   ├── IMPORT (dependencies)
   └── DECORATOR (annotations)
        ↓
   Neo4j Graph Storage
   (Structure & Relationships)
        ↓
   Weaviate Vector Embeddings
   (Semantic Code Search)
```

## 🗄️ Neo4j Graph Schema

### Core Node Types

#### Conversation Domain
- **`Project`**: Top-level project container
- **`Conversation`**: Individual Claude Code conversations
- **`Turn`**: Individual exchanges within conversations
- **`Topic`**: Conversation topic clustering
- **`Command`**: CLI commands executed

#### Code Structure Domain  
- **`CodeFile`**: File-level containers
- **`CodeModule`**: Module/file scope
- **`CodeClass`**: Class definitions
- **`CodeFunction`**: Function definitions
- **`CodeMethod`**: Class methods
- **`CodeImport`**: Import statements
- **`Dependency`**: External dependencies

### Key Relationships

#### Conversation Flow
```cypher
(Project)-[:HAS_CONVERSATION]->(Conversation)
(Conversation)-[:CONTAINS]->(Turn)
(Turn)-[:DISCUSSES]->(Topic)
(Command)-[:EXECUTED_IN]->(Conversation)
```

#### Code Structure
```cypher
(CodeFile)-[:CONTAINS]->(CodeClass)
(CodeClass)-[:CONTAINS]->(CodeMethod)
(CodeModule)-[:IMPORTS]->(Dependency)
(CodeFunction)-[:CALLS]->(CodeFunction)
```

#### Cross-Domain Linking
```cypher
(Conversation)-[:DISCUSSED_CODE]->(CodeFile)
(Turn)-[:MENTIONED_FUNCTION]->(CodeFunction)
(Project)-[:HAS_CODEBASE]->(CodeFile)
```

## 🧩 Code Chunking System

### Chunking Strategy

**Python Example:**
```python
# File: redis_manager.py
class RedisManager:           # ← CodeClass chunk
    def __init__(self):       # ← CodeMethod chunk
        self.pool = None
    
    async def health_check(self):  # ← CodeMethod chunk (complexity: 4)
        """Check Redis health"""   # ← Docstring stored
        try:
            await self.redis.ping()
            return {"status": "healthy"}
        except Exception as e:
            return {"status": "error"}
```

**Resulting Neo4j Structure:**
```
CodeFile: redis_manager.py
├── CodeModule: redis_manager (lines 1-100)
│   ├── CodeImport: asyncio
│   ├── CodeImport: redis.asyncio
│   └── CodeClass: RedisManager (lines 15-98)
│       ├── CodeMethod: __init__ (lines 16-18, complexity: 1)
│       └── CodeMethod: health_check (lines 20-28, complexity: 4)
```

### Chunk Properties

Each code chunk stores:
- **Location**: `start_line`, `end_line`, `file_path`
- **Metadata**: `complexity_score`, `docstring`, `decorators`
- **Relationships**: `parent_chunk_id`, `children_chunk_ids`, `dependencies`
- **Content**: Actual source code for the chunk

## 🛡️ Security & Privacy

### File Security Filter

**Automatic Exclusion:**
- **Environment files**: `.env`, `.env.*`, `environment.*`
- **Credentials**: `*secret*`, `*password*`, `*token*`, `*key*`
- **Certificates**: `*.pem`, `*.key`, `*.p12`, `*.crt`
- **Cloud configs**: `.aws/`, `.gcp/`, `.kube/`
- **Sensitive patterns**: API keys, private keys, tokens

**Content Analysis:**
- **Pattern detection**: `sk-[A-Za-z0-9]{48,}` (OpenAI keys)
- **Base64 secrets**: High entropy string detection
- **Binary files**: Non-ASCII ratio filtering
- **Size limits**: Files >1MB excluded

### Training-Safe Data Namespacing

**Test Data Isolation:**
```
Production Data: 
├── Project: "CenterfireIntelligence"
├── Conversations: Real development discussions
└── Code: Actual project structure

Test Data:
├── Project: "TEST_PROJECT_*" 
├── Conversations: "[TRAINING_IGNORE] Test conversation..."
└── Namespace: "CLAUDE_CODE_TEST_DATA"
```

## 🎯 Project Namespacing Strategy

### Multi-Project Support

Each project gets isolated namespaces:

```
Project: CenterfireIntelligence (Python)
├── Neo4j: "CenterfireIntelligence::python"
├── Qdrant: "centerfireintelligence_python" 
├── Weaviate: "Centerfireintelligence__Python"
└── Redis: "centerfireintelligence:python"

Project: MyReactApp (JavaScript)  
├── Neo4j: "MyReactApp::javascript"
├── Qdrant: "myreactapp_javascript"
├── Weaviate: "Myreactapp__Javascript" 
└── Redis: "myreactapp:javascript"
```

### Project Type Detection

**Automatic Classification:**
- **Language detection**: Primary/secondary languages from file extensions
- **Framework detection**: React, Django, Express, etc.
- **Project type**: Frontend, Backend, Library, CLI tool
- **Structure analysis**: Tests, docs, complexity metrics

## 🔄 Concurrent Session Support

### Multi-Session Architecture

**Single Daemon, Multiple Sessions:**
```
Session A (/project/frontend) ──┐
Session B (/project/backend)  ──┤──→ Shared Daemon (port 8081)
Session C (/other/project)    ──┘    ├── Docker Discovery
                                     ├── Redis Streams  
                                     ├── Neo4j Graph
                                     └── Qdrant/Weaviate
```

**Session Isolation:**
- **Unique session IDs**: `claude_session_${timestamp}`
- **Working directory detection**: Automatic project classification
- **Namespace separation**: Different projects → different storage namespaces
- **No output sharing**: Each session has isolated console output

## 🚀 Performance Characteristics

### Benchmarked Performance

**Streaming Throughput:**
- **Redis**: ~1-3ms latency, >1000 req/sec
- **Neo4j**: ~15-25ms latency, ~40-60 req/sec  
- **Qdrant**: ~25-45ms latency, ~25-40 req/sec
- **Overall**: ~100+ conversations/sec across all systems

**Storage Efficiency:**
- **Code chunking**: 20-50 chunks per typical Python file
- **Relationship density**: 5-15 relationships per code chunk
- **Memory usage**: Minimal overhead with connection pooling

### Docker Service Discovery

**Dynamic Port Resolution:**
```bash
# Discovers actual Docker ports automatically:
redis: mem0-redis → localhost:6380
neo4j: centerfire-neo4j → localhost:7687  
qdrant: mem0-qdrant → localhost:6333
weaviate: centerfire-weaviate → localhost:8080
```

**Benefits:**
- **No hardcoded ports**: Adapts to any Docker configuration
- **Automatic detection**: Discovers running containers dynamically
- **Fallback graceful**: Uses sensible defaults when Docker unavailable

## 🧪 Testing & Validation

### Test Toolkit

**Streaming Tests:**
```bash
centerfire-test basic              # Basic functionality
centerfire-test performance 100   # 100 conversation throughput test
centerfire-test measure           # Current storage state
centerfire-test flush             # Clean test data
```

**Development Tests:**  
```bash
./scripts/dev-end-to-end-test.sh  # Complete system validation
```

**Test Data Safety:**
- **Namespace isolation**: `TEST_PROJECT_*` prefixes
- **Training markers**: `[TRAINING_IGNORE]` in all test content
- **Automatic cleanup**: Flushes test data while preserving production
- **Performance validation**: Ensures 100+ req/sec baseline

## 📈 Scalability Design

### Horizontal Scaling Points

**Storage Layer:**
- **Redis clustering**: Stream partitioning across nodes
- **Neo4j clustering**: Causal clustering for read replicas
- **Qdrant scaling**: Collection sharding and replication
- **Weaviate scaling**: Multi-node semantic search

**Application Layer:**
- **Multiple daemons**: One per development machine
- **Service discovery**: Docker-based service location
- **Load balancing**: Session-based routing

### Data Growth Management

**Conversation Pruning:**
- **Time-based**: Archive conversations older than N months
- **Size-based**: Compress large conversation histories
- **Relevance-based**: Keep frequently accessed conversations

**Code Structure Updates:**
- **Incremental parsing**: Only re-chunk modified files
- **Relationship updates**: Update dependency graphs on file changes
- **Version tracking**: Track code evolution over time

## 🛠️ Development Workflow Integration

### Claude Code Session Lifecycle

1. **Session Start**: 
   - Project detection from working directory
   - Namespace generation for storage isolation
   - Health check of all storage systems

2. **Active Development**:
   - Real-time conversation streaming to all storage systems
   - Automatic code structure analysis on file changes
   - Security filtering prevents sensitive data embedding

3. **Cross-Session Memory**:
   - Previous conversations accessible in new sessions
   - Code structure knowledge persists across restarts
   - Project context maintained indefinitely

### Global CLI Tools

**Management Commands:**
```bash
centerfire-daemon start           # Start semantic AI daemon
centerfire-health                 # System health check
centerfire-test {command}         # Testing toolkit
```

**Installation:**
```bash
./scripts/install-global.sh       # Global system installation
./scripts/dev-end-to-end-test.sh  # Development validation
```

## 🎯 Future Capabilities

### Planned Enhancements

**Advanced Code Analysis:**
- **Function call graph mapping**: Cross-file function relationships
- **Dependency vulnerability tracking**: Security analysis integration
- **Code quality metrics**: Complexity, maintainability scoring
- **Refactoring suggestions**: Based on code structure knowledge

**Enhanced Semantic Search:**
- **Natural language queries**: "Find async functions that handle timeouts"
- **Cross-project patterns**: "Show similar error handling across projects" 
- **Conversation-driven search**: "Code we discussed last week about Redis"

**Developer Productivity:**
- **Context-aware suggestions**: Based on conversation history
- **Automatic documentation**: Generate docs from code + conversations
- **Knowledge transfer**: Onboard new developers with conversation history

---

## 📚 Quick Reference

### Key Architectural Principles

1. **Domain Separation**: Conversations ≠ Code Structure ≠ Vector Embeddings
2. **Explicit Relationships**: No implicit connections, everything is linked properly
3. **Security First**: Sensitive data automatically excluded from all storage
4. **Project Isolation**: Complete namespace separation between different projects
5. **Training Safety**: Test data clearly marked and easily flushed

### Terminology Clarification

- **Neo4j**: Stores **relationships and structure**, NOT embeddings
- **Qdrant/Weaviate**: Store **vector embeddings** for semantic search
- **"Code Chunking"**: Parsing code into logical units (functions, classes)
- **"Code Mapping"**: Creating relationship graphs in Neo4j  
- **"Code Embedding"**: Creating vector representations in Weaviate

This architecture provides the foundation for intelligent, persistent, and secure semantic AI assistance that grows smarter with every Claude Code session while maintaining complete data isolation and security.
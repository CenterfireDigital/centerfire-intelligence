# Centerfire Intelligence - Deployment Guide
*Complete setup guide for deploying semantic AI intelligence across development environments*

## Table of Contents
1. Overview
2. Architecture Decisions
3. Code Chunking Strategies
4. Deployment Options
5. IDE Integration Patterns
6. Scaling and Performance
7. Troubleshooting

---

## Overview

### What This System Provides

**Transform development from file-searching to semantic intelligence:**
- **Sub-second code discovery**: Find relevant code by concept, not keywords
- **Cross-service pattern recognition**: Discover proven implementations across microservices
- **Memory-informed development**: Build on previous solutions and architectural decisions
- **IDE-agnostic intelligence**: Works with Claude Code, Cursor, VS Code, JetBrains, or standalone

### Performance Characteristics
```
Traditional Development:
"How do I implement JWT auth?" → 20-30 minutes of file searching

Semantic AI Intelligence:
"How do I implement JWT auth?" → <10 seconds with examples and context
```

### Technology Stack Decision Matrix

| Component | Production Choice | Alternative | Why |
|-----------|------------------|-------------|-----|
| Vector DB | **Weaviate** | Qdrant | Better code understanding, easier deployment |
| Graph DB | **Neo4j** | ArangoDB | Mature graph queries, excellent tooling |
| Memory | **mem0** | Custom Redis | Purpose-built for conversational memory |
| Cache | **Redis** | Memcached | Data structures + persistence |

---

## Architecture Decisions

### Why This 4-Component Architecture

#### The Intelligence Stack
```
┌─────────────────────────────────────────────────────────────┐
│                    IDE Integration                          │
│              (VS Code, JetBrains, Claude)                  │
└──────────────────┬──────────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────────┐
│               Semantic AI Agent                             │
│         (Unified Intelligence Coordinator)                 │
└──┬────────┬────────┬────────┬────────────────────────────────┘
   │        │        │        │
   ▼        ▼        ▼        ▼
┌──────┐ ┌────────┐ ┌──────┐ ┌─────────┐
│Weaviate│ │ Neo4j  │ │ mem0 │ │  Redis  │
│Semantic│ │Graph   │ │Memory│ │ Cache   │
│<300ms  │ │Rels    │ │ Q&A  │ │ <5ms    │
└────────┘ └────────┘ └──────┘ └─────────┘
```

#### Component Responsibilities

**1. Weaviate (Semantic Search Engine)**
- **Purpose**: Lightning-fast conceptual code discovery
- **Data**: Function/class embeddings organized by service categories
- **Query Time**: <300ms for semantic search across entire codebase
- **Why Not Just Grep**: Finds `validateUser()` when you search for "authentication"

**2. Neo4j (Relationship Intelligence)**  
- **Purpose**: Understanding how code pieces connect and evolve
- **Data**: Code dependencies, pattern evolution, conversation links
- **Query Time**: 30-50ms for relationship analysis
- **Why Not Just AST**: Tracks architectural decisions and pattern maturity

**3. mem0 (Conversational Memory)**
- **Purpose**: Avoiding repeated questions, building on previous work
- **Data**: Question-answer pairs, architectural decisions, session context
- **Query Time**: 50-100ms for similarity search
- **Why Not Just Logs**: Semantic understanding of development conversations

**4. Redis (Performance Multiplier)**
- **Purpose**: Making intelligence instant instead of computed
- **Data**: Search results, embeddings, recommendations
- **Query Time**: <5ms for cached queries
- **Why Essential**: 40-80x performance improvement on repeat queries

### Performance Engineering Rationale

#### Multi-Tier Performance Strategy
```
Query Flow Performance Profile:

Cold Query (First Time):
1. Generate embedding: 50-100ms
2. Weaviate semantic search: 200-250ms  
3. Neo4j relationship lookup: 30-50ms
4. mem0 context search: 50-100ms
5. Result compilation: 10-20ms
Total: 340-520ms

Warm Query (Redis Cache Hit):
1. Redis lookup: 3-7ms
Total: <10ms

Performance Multiplier: 50-100x faster
```

This design ensures the first query is fast, subsequent queries are instant.

---

## Code Chunking Strategies

### The Art and Science of Semantic Chunking

**Code chunking is the most critical factor in system intelligence quality.**

#### Chunking Philosophy
```javascript
// BAD: File-level only chunking
{
  type: "file",
  content: "entire 500-line auth.js file",
  semantic: "authentication file"
}
// Result: Vague, hard to find specific implementations

// GOOD: Multi-level semantic chunking  
{
  type: "function",
  name: "validateJWT",
  content: "async function validateJWT(token) { ... }",
  semantic: "validates JWT tokens for user authentication with expiry checking",
  complexity: 3,
  dependencies: ["jsonwebtoken"],
  service_category: "core",
  usage_context: "middleware authentication"
}
// Result: Precise, actionable, discoverable
```

### Chunking Strategies by Code Type

#### 1. Function-Level Chunking (Primary)
```javascript
// Optimal chunk size: 10-50 lines
// Semantic focus: Single responsibility

{
  id: "func-auth-validate-jwt-a1b2c3",
  type: "function", 
  name: "validateJWT",
  code: `async function validateJWT(token, secret) {
    try {
      const decoded = jwt.verify(token, secret);
      if (decoded.exp < Date.now() / 1000) {
        throw new Error('Token expired');
      }
      return { valid: true, user: decoded.user };
    } catch (error) {
      return { valid: false, error: error.message };
    }
  }`,
  semantic_description: "validates JWT tokens for authentication, handles expiration checking and error handling",
  complexity_score: 3, // 1-5 scale
  dependencies: ["jsonwebtoken"],
  service_path: "core/api-gateway",
  usage_patterns: ["middleware", "authentication"],
  input_types: ["string", "string"],
  output_types: ["object"],
  error_handling: true,
  async_operation: true
}
```

#### 2. Class-Level Chunking (Secondary)
```javascript
// For classes with multiple related methods
{
  id: "class-redis-cache-manager-x1y2z3",
  type: "class",
  name: "RedisCacheManager", 
  code: "class RedisCacheManager { constructor... get... set... delete... }",
  semantic_description: "manages Redis caching operations with TTL support and error handling",
  methods: [
    {
      name: "get",
      semantic: "retrieves cached values with automatic JSON parsing",
      complexity: 2
    },
    {
      name: "set", 
      semantic: "stores values in cache with configurable TTL",
      complexity: 2
    }
  ],
  service_path: "infrastructure/memory-manager",
  dependencies: ["ioredis"],
  complexity_score: 4
}
```

#### 3. File-Level Chunking (Context)
```javascript
// For understanding overall file purpose
{
  id: "file-auth-middleware-f4g5h6",
  type: "file",
  name: "auth.js",
  code: "// First 200 lines + summary of remaining...",
  semantic_description: "authentication middleware for API gateway with JWT validation, role checking, and session management", 
  exports: ["validateJWT", "requireAuth", "checkRole"],
  imports: ["jsonwebtoken", "redis", "user-service"],
  file_purpose: "authentication_middleware",
  service_path: "core/api-gateway",
  line_count: 156,
  function_count: 8,
  complexity_score: 4
}
```

### Semantic Description Engineering

#### The Power of Rich Descriptions
**Bad semantic descriptions:**
```
"function that does auth"
"validates stuff" 
"helper method"
```

**Good semantic descriptions:**
```
"validates JWT tokens for user authentication with automatic expiration checking and detailed error reporting"
"caches Redis values with configurable TTL and automatic JSON serialization for session data"
"middleware function that requires user authentication and optionally checks user roles for API endpoints"
```

#### Description Template
```
"[ACTION] [WHAT] for [PURPOSE] with [KEY_FEATURES] and [ERROR_HANDLING/EDGE_CASES]"

Examples:
- "validates JWT tokens for user authentication with expiration checking and detailed error handling"
- "caches user session data for performance optimization with automatic TTL and JSON serialization"  
- "processes file uploads for content management with size validation and virus scanning"
```

### Service Category Organization

#### Optimal Category Structure
```javascript
const SERVICE_CATEGORIES = {
  'core': {
    description: 'Essential services (API gateway, admin, auth)',
    examples: ['api-gateway', 'admin-service', 'auth-service'],
    typical_patterns: ['authentication', 'routing', 'middleware'],
    search_priority: 'high' // Most commonly needed
  },
  
  'infrastructure': {
    description: 'Data and performance services',
    examples: ['redis-manager', 'database-service', 'message-queue'],
    typical_patterns: ['caching', 'data-persistence', 'messaging'],
    search_priority: 'medium'
  },
  
  'business': {
    description: 'Domain-specific business logic',
    examples: ['user-management', 'billing', 'analytics'],
    typical_patterns: ['validation', 'workflows', 'reporting'],
    search_priority: 'medium'
  },
  
  'integration': {
    description: 'External service connectors', 
    examples: ['payment-gateway', 'email-service', 'social-auth'],
    typical_patterns: ['API-clients', 'webhooks', 'oauth'],
    search_priority: 'low' // Project-specific
  }
}
```

#### Why Categories Matter
```bash
# Without categories: Search all 10,000 code chunks
semantic_search("authentication") → 300-500ms

# With categories: Search relevant 2,000 chunks  
semantic_search("authentication", categories=["core", "infrastructure"]) → 100-150ms
```

### Chunking Quality Metrics

#### Measuring Chunk Effectiveness
```javascript
const QUALITY_METRICS = {
  semantic_precision: {
    measurement: "Relevant results in top 5 / Total queries",
    target: "> 85%",
    indicator: "Good semantic descriptions"
  },
  
  search_recall: {
    measurement: "Found implementations / Known implementations", 
    target: "> 90%",
    indicator: "Complete code coverage"
  },
  
  chunk_coherence: {
    measurement: "Single-purpose chunks / Total chunks",
    target: "> 80%", 
    indicator: "Proper function-level chunking"
  }
}
```

---

## Deployment Options

### Option 1: Full Stack Deployment (Recommended for Teams)

#### Docker Compose Setup
```yaml
# docker-compose.yml
version: '3.8'
services:
  weaviate:
    image: semitechnologies/weaviate:latest
    ports:
      - "8080:8080"
    environment:
      QUERY_DEFAULTS_LIMIT: 25
      AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED: 'true'
      PERSISTENCE_DATA_PATH: '/var/lib/weaviate'
      DEFAULT_VECTORIZER_MODULE: 'text2vec-transformers'
      ENABLE_MODULES: 'text2vec-transformers'
    volumes:
      - weaviate_data:/var/lib/weaviate

  neo4j:
    image: neo4j:latest
    ports:
      - "7474:7474"
      - "7687:7687"
    environment:
      NEO4J_AUTH: neo4j/your_password
      NEO4J_dbms_memory_heap_initial__size: 512m
      NEO4J_dbms_memory_heap_max__size: 1G
    volumes:
      - neo4j_data:/data

  redis:
    image: redis:alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  mem0:
    image: mem0ai/mem0:latest
    ports:
      - "11434:11434"
    environment:
      REDIS_URL: redis://redis:6379
    depends_on:
      - redis

volumes:
  weaviate_data:
  neo4j_data:  
  redis_data:
```

#### Installation Script
```bash
#!/bin/bash
# install-semantic-ai.sh

echo "🚀 Installing Semantic AI Code Intelligence..."

# Verify Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Clone deployment repository
git clone https://github.com/your-org/semantic-ai-intelligence.git
cd semantic-ai-intelligence

# Start services
docker-compose up -d

# Wait for services to be healthy
echo "⏳ Waiting for services to start..."
sleep 30

# Verify services
curl -f http://localhost:8080/v1/.well-known/ready || echo "⚠️  Weaviate not ready"
curl -f http://localhost:7474 || echo "⚠️  Neo4j not ready" 
curl -f http://localhost:6379 || echo "⚠️  Redis not ready"

echo "✅ Services started. Run 'npm run setup' to initialize."
```

### Option 2: Minimal Deployment (Individual Developers)

#### Weaviate-Only Setup
```bash
# Fastest way to get started - just semantic search
docker run -p 8080:8080 \
  -e AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED='true' \
  -e PERSISTENCE_DATA_PATH='/var/lib/weaviate' \
  -e DEFAULT_VECTORIZER_MODULE='text2vec-transformers' \
  -e ENABLE_MODULES='text2vec-transformers' \
  semitechnologies/weaviate:latest

# Inject your codebase  
npm install -g semantic-code-ai
semantic-code-ai inject /path/to/your/project
```

### Option 3: Cloud Deployment

#### AWS/GCP/Azure Ready
```terraform
# terraform/main.tf
resource "aws_ecs_cluster" "semantic_ai" {
  name = "semantic-ai-intelligence"
}

resource "aws_ecs_service" "weaviate" {
  name            = "weaviate"
  cluster         = aws_ecs_cluster.semantic_ai.id
  task_definition = aws_ecs_task_definition.weaviate.arn
  desired_count   = 1
  
  # Auto-scaling configuration
  # Load balancer configuration
  # Health checks
}

# Additional resources for Neo4j, Redis, mem0
```

---

## IDE Integration Patterns

### Universal Integration Strategy

The system is designed to work with any development environment through multiple integration points.

#### Integration Levels

**Level 1: CLI Integration (Universal)**
```bash
# Works with any IDE through terminal/command palette
semantic-search "JWT validation patterns"
semantic-assist "implement Redis caching" 
```

**Level 2: Language Server Protocol (VS Code, Vim, Emacs)**
```json
// .vscode/settings.json
{
  "semantic-ai.endpoint": "http://localhost:8080",
  "semantic-ai.enableAutoCompletion": true,
  "semantic-ai.enableHover": true
}
```

**Level 3: Direct Plugin (VS Code Extension)**
```javascript
// VS Code Extension API
vscode.commands.registerCommand('semantic-ai.findSimilar', async () => {
  const selection = vscode.window.activeTextEditor.selection;
  const code = document.getText(selection);
  const results = await semanticSearch(code);
  // Display results in sidebar
});
```

### Claude Code Integration

#### Automatic Startup Integration
```bash
# Add to CLAUDE.md or startup script
echo "Semantic AI Intelligence available. Quick commands:
- Ask me to 'search for Redis patterns' (I'll use semantic search)  
- Ask me to 'find similar auth code' (I'll query the vector DB)
- I'll automatically check for similar previous questions (mem0)"
```

#### Session Hooks
```javascript
// Automatic context loading
if (semanticAIAvailable()) {
  loadRecentPatterns();
  checkSimilarQuestions();
  preloadFrequentSearches(); 
}
```

### VS Code Extension Architecture

#### Extension Capabilities
```typescript
// extension.ts
export function activate(context: vscode.ExtensionContext) {
  // Command: Search code semantically
  const searchCommand = vscode.commands.registerCommand(
    'semantic-ai.search', 
    async () => {
      const query = await vscode.window.showInputBox({
        prompt: 'Search for code patterns'
      });
      
      const results = await semanticAI.search(query);
      showResultsPanel(results);
    }
  );

  // Hover provider: Show similar code on hover
  const hoverProvider = vscode.languages.registerHoverProvider('*', {
    provideHover(document, position) {
      const range = document.getWordRangeAtPosition(position);
      const word = document.getText(range);
      
      // Find similar functions/patterns
      return semanticAI.findSimilar(word).then(results => {
        return new vscode.Hover(formatResults(results));
      });
    }
  });

  context.subscriptions.push(searchCommand, hoverProvider);
}
```

### JetBrains Plugin Pattern

#### IntelliJ Plugin Structure
```kotlin
// SemanticAIPlugin.kt
class SemanticAIAction : AnAction() {
    override fun actionPerformed(event: AnActionEvent) {
        val project = event.project ?: return
        val editor = event.getData(CommonDataKeys.EDITOR) ?: return
        
        val selectedText = editor.selectionModel.selectedText
        
        SemanticAIService.findSimilar(selectedText) { results ->
            // Display in tool window
            ToolWindowManager.getInstance(project)
                .getToolWindow("Semantic AI")
                ?.show(results)
        }
    }
}
```

### IDE-Agnostic API Design

#### Universal REST API
```javascript
// Universal endpoints that any IDE can consume
GET /api/search?q={query}&categories={cats}&limit={n}
POST /api/similar { "code": "function code...", "context": "auth" }
GET /api/patterns/{serviceCategory}
POST /api/assist { "task": "implement caching", "service": "api" }

// Response format (consistent across all endpoints)
{
  "query": "JWT validation",
  "took": 234, // milliseconds  
  "total": 12,
  "results": [
    {
      "score": 0.923,
      "type": "function",
      "name": "validateJWT", 
      "file": "core/api-gateway/auth.js",
      "code": "async function validateJWT...",
      "description": "validates JWT tokens with expiration checking",
      "dependencies": ["jsonwebtoken"],
      "complexity": 3
    }
  ]
}
```

---

## Scaling and Performance

### Performance Benchmarks

#### Target Performance Metrics
```
Search Performance:
- Cold semantic search: <300ms (95th percentile)
- Warm cached search: <5ms (99th percentile)  
- Complex multi-category search: <500ms
- Bulk pattern analysis: <2s for 50 patterns

Memory Usage:
- Weaviate: 2-4GB for 100K code chunks
- Neo4j: 1-2GB for relationships
- Redis: 500MB-1GB for active cache
- Total: 4-8GB for complete system

Storage:
- Vector embeddings: ~50MB per 10K functions  
- Graph relationships: ~10MB per 10K functions
- Conversation memory: ~1MB per 1K conversations
- Cache data: ~100MB active working set
```

#### Scaling Strategies

**Horizontal Scaling Pattern:**
```yaml
# Production scaling configuration
weaviate:
  replicas: 3
  shards: 6  # Distribute embeddings
  replication: 2  # Fault tolerance

neo4j:
  cluster_mode: true
  core_servers: 3
  read_replicas: 2

redis:
  cluster_mode: true
  nodes: 6  # 3 masters, 3 replicas
  
mem0:
  replicas: 2
  load_balancer: true
```

**Vertical Scaling Guidelines:**
```
Small Team (1-5 developers, <50K LOC):
- 4 CPU cores, 8GB RAM
- Single-node deployment
- Expected response time: <200ms

Medium Team (5-20 developers, <200K LOC):  
- 8 CPU cores, 16GB RAM
- Multi-container deployment
- Expected response time: <150ms

Large Team (20+ developers, >200K LOC):
- 16+ CPU cores, 32GB+ RAM  
- Clustered deployment
- Expected response time: <100ms
```

### Performance Optimization

#### Cache Strategy Optimization
```javascript
// Intelligent cache warming
const CACHE_WARMING_STRATEGY = {
  popular_queries: {
    // Pre-cache common searches
    patterns: ["authentication", "caching", "validation", "middleware"],
    refresh_interval: "1h",
    priority: "high"
  },
  
  user_patterns: {
    // Learn user's common queries
    track_frequency: true,
    auto_cache_threshold: 3, // Cache after 3 uses
    ttl: "4h"
  },
  
  service_specific: {
    // Pre-cache by service category
    categories: ["core", "infrastructure"], 
    warmup_schedule: "startup",
    refresh_on_code_change: true
  }
}
```

#### Query Optimization
```javascript
// Smart query routing
const optimizeQuery = (query, context) => {
  // Route simple queries to cache first
  if (isSimplePattern(query) && cache.has(query)) {
    return cache.get(query); // <5ms
  }
  
  // Route semantic queries to Weaviate
  if (isSemanticQuery(query)) {
    return weaviate.search(query); // <300ms
  }
  
  // Route relationship queries to Neo4j
  if (isRelationshipQuery(query)) {
    return neo4j.query(query); // <50ms
  }
  
  // Route memory queries to mem0
  if (isMemoryQuery(query)) {
    return mem0.search(query); // <100ms
  }
}
```

---

## Troubleshooting

### Common Issues and Solutions

#### 1. Vector Search Returns Poor Results

**Symptom**: Semantic search finds irrelevant code
```
Query: "JWT validation"  
Results: Random functions with "validation" in comments
```

**Root Cause**: Poor semantic descriptions during chunking

**Solution**: Improve chunking quality
```javascript
// BAD chunking
{
  semantic: "validates stuff"
}

// GOOD chunking  
{
  semantic: "validates JWT tokens for user authentication with automatic expiration checking and detailed error reporting for API middleware"
}
```

#### 2. Slow Search Performance

**Symptom**: Searches taking >1 second consistently

**Diagnosis Steps**:
```bash
# Check service health
curl http://localhost:8080/v1/.well-known/live  # Weaviate
curl http://localhost:7474/db/neo4j/tx/commit  # Neo4j  
redis-cli ping  # Redis

# Check resource usage
docker stats

# Check cache hit rates
redis-cli info stats | grep keyspace_hits
```

**Solutions**:
- Increase Docker memory allocation
- Enable Redis cache warming
- Optimize chunk sizes (aim for 10-50 line functions)
- Use service categories to narrow search scope

#### 3. Memory Issues

**Symptom**: Services crashing with OOM errors

**Monitoring**:
```bash
# Check memory usage by service
docker stats --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"

# Check Weaviate memory 
curl http://localhost:8080/v1/nodes | jq '.nodes[].stats.memoryUsage'
```

**Solutions**:
```yaml
# docker-compose.yml memory limits
services:
  weaviate:
    deploy:
      resources:
        limits:
          memory: 4G
        reservations:
          memory: 2G
          
  neo4j:
    environment:
      NEO4J_dbms_memory_heap_max__size: 2G
      NEO4J_dbms_memory_pagecache_size: 1G
```

#### 4. Code Changes Not Reflected

**Symptom**: Searching for newly added code returns nothing

**Root Cause**: Index not updated after code changes

**Solution**: Implement auto-refresh
```bash
# Manual refresh
npm run inject-codebase

# Automatic refresh with file watching
npm run watch-and-inject

# Git hook integration (recommended)
npm run hooks:setup  # Updates on commits
```

### Health Monitoring

#### Service Health Checks
```bash
#!/bin/bash
# health-check.sh

echo "🔍 Semantic AI Health Check"

# Weaviate
if curl -f http://localhost:8080/v1/.well-known/ready >/dev/null 2>&1; then
  echo "✅ Weaviate: Ready"
else
  echo "❌ Weaviate: Not responding"
fi

# Neo4j
if curl -f http://localhost:7474 >/dev/null 2>&1; then
  echo "✅ Neo4j: Ready" 
else
  echo "❌ Neo4j: Not responding"
fi

# Redis
if redis-cli ping | grep -q PONG; then
  echo "✅ Redis: Ready"
else 
  echo "❌ Redis: Not responding"
fi

# Test end-to-end
if npm run semantic:search "test query" >/dev/null 2>&1; then
  echo "✅ End-to-end: Working"
else
  echo "❌ End-to-end: Failed"
fi
```

#### Performance Monitoring
```javascript
// performance-monitor.js
const monitor = {
  async checkPerformance() {
    const tests = [
      { query: "authentication", expected: 300 },
      { query: "caching patterns", expected: 250 },
      { query: "validation middleware", expected: 400 }
    ];
    
    for (const test of tests) {
      const start = Date.now();
      await semanticSearch(test.query);
      const duration = Date.now() - start;
      
      if (duration > test.expected) {
        console.warn(`⚠️  Slow query: ${test.query} took ${duration}ms (expected <${test.expected}ms)`);
      } else {
        console.log(`✅ ${test.query}: ${duration}ms`);
      }
    }
  }
};
```

### Production Deployment Checklist

#### Pre-Deployment
- [ ] Resource requirements verified (CPU, RAM, storage)
- [ ] Network security configured (firewall rules, VPC)
- [ ] Backup strategy implemented
- [ ] Monitoring and alerting configured
- [ ] Health checks implemented
- [ ] Auto-scaling policies defined

#### Post-Deployment  
- [ ] End-to-end functionality tested
- [ ] Performance benchmarks verified
- [ ] Cache warming completed
- [ ] User access configured
- [ ] Documentation updated
- [ ] Team training completed

---

This deployment guide provides everything needed to reproduce the semantic AI intelligence system across different environments, IDEs, and team sizes. The modular architecture allows for gradual adoption - start with just Weaviate for semantic search, then add components as needed.
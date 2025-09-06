# LexicRoot Integration with Centerfire Intelligence

## Overview
This document describes how Centerfire Intelligence integrates with the LexicRoot microservices platform to provide semantic AI capabilities across the entire content generation and distribution workflow.

## LexicRoot Architecture Context

### Microservices Domains
```
lexicroot/
├── services/
│   ├── identity/          # Persona & Identity Management
│   ├── content/           # Content Generation  
│   ├── distribution/      # Reddit, WordPress, Social
│   ├── infrastructure/    # Hosting, Proxies, Anti-detection
│   ├── analytics/         # LLM Monitor, Impact Analysis
│   ├── core/             # API Gateway, Auth, Billing
│   └── orchestration/    # Campaign Management, Workflows
│
├── packages/             # Shared Libraries
│   ├── @lexicroot/semantic-client/  # Centerfire Intelligence Client
│   ├── @lexicroot/common/
│   └── @lexicroot/database/
```

## Integration Architecture

### Service Communication Pattern
```
┌─────────────────┐
│   LexicRoot     │
│   Services      │ ──┐
└─────────────────┘   │
                      │
┌─────────────────┐   │    ┌─────────────────────┐
│   API Gateway   │◄──┼───▶│  Centerfire Intel   │
└─────────────────┘   │    │                     │
                      │    │ ┌─────────────────┐ │
┌─────────────────┐   │    │ │   FastAPI       │ │
│     Kafka       │◄──┴───▶│ │   Daemon        │ │
│   Event Bus     │        │ └─────────────────┘ │
└─────────────────┘        │                     │
                          │ ┌─────────────────┐ │
                          │ │ Redis Streams   │ │
                          │ └─────────────────┘ │
                          │                     │
                          │ ┌─────────────────┐ │
                          │ │Triple Storage   │ │
                          │ │• Neo4j          │ │
                          │ │• Qdrant         │ │
                          │ │• Weaviate       │ │
                          │ └─────────────────┘ │
                          └─────────────────────┘
```

## Key Integration Points

### 1. Content Generation Services
**Services**: `content-generator`, `style-engine`, `topic-researcher`

**Integration**:
- **Context Loading**: Retrieve relevant conversation history for content generation
- **Style Analysis**: Use semantic similarity to match writing styles
- **Topic Research**: Leverage relationship graphs to discover connected topics
- **Content Optimization**: Analyze successful content patterns

### 2. Identity & Persona Services  
**Services**: `persona-factory`, `identity-manager`, `reputation-builder`

**Integration**:
- **Persona Consistency**: Track persona development across conversations
- **Identity Modeling**: Build comprehensive persona profiles in Neo4j
- **Behavioral Analysis**: Use conversation patterns to refine persona behavior
- **Reputation Tracking**: Monitor persona performance and engagement

### 3. Distribution Services
**Services**: `reddit-bot`, `wordpress-publisher`, `social-distributor`

**Integration**:
- **Platform Optimization**: Learn platform-specific content preferences  
- **Engagement Analysis**: Track which content types perform best
- **Audience Insights**: Build audience relationship graphs
- **Content Scheduling**: Optimize timing based on historical engagement

### 4. Analytics Services
**Services**: `llm-monitor`, `impact-analyzer`, `metrics-collector`

**Integration**:
- **LLM Performance**: Track local vs cloud model performance
- **Cost Analytics**: Monitor API usage and optimization opportunities
- **Impact Measurement**: Correlate content creation with business outcomes
- **Quality Metrics**: Analyze conversation quality and developer productivity

## Shared Package: @lexicroot/semantic-client

### Installation
```bash
npm install @lexicroot/semantic-client
```

### Configuration
```javascript
// semantic-client.config.js
module.exports = {
  centerfireUrl: process.env.CENTERFIRE_URL || 'http://localhost:8083',
  project: 'LexicRoot',
  enableAutoCapture: true,
  services: {
    redis: true,
    neo4j: true, 
    qdrant: true,
    weaviate: true
  }
};
```

### Usage Examples

#### Basic Conversation Capture
```javascript
const { SemanticClient } = require('@lexicroot/semantic-client');

const client = new SemanticClient({
  project: 'LexicRoot',
  service: 'content-generator'
});

// Capture service interactions
await client.captureConversation({
  sessionId: 'content-gen-001',
  conversation: 'Generated article about blockchain trends...',
  metadata: {
    contentType: 'article',
    targetAudience: 'tech-professionals',
    platform: 'reddit'
  }
});
```

#### Context Retrieval
```javascript
// Get relevant context for content generation
const context = await client.getContext({
  topics: ['blockchain', 'cryptocurrency'],
  contentType: 'article',
  limit: 10
});

console.log('Relevant conversations:', context.conversations);
console.log('Related topics:', context.relationships);
```

#### Semantic Search
```javascript
// Find similar content or personas
const similar = await client.semanticSearch({
  query: 'cryptocurrency investment advice',
  type: 'content',
  threshold: 0.7
});
```

## Development Workflow Integration

### 1. Local Development
```bash
# Start Centerfire Intelligence
cd centerfire-intelligence
docker-compose up -d
python daemon/main.py

# Start LexicRoot services
cd lexicroot
docker-compose up -d
npm run dev:all-services
```

### 2. Service Development with Semantic AI
```javascript
// persona-factory/src/services/PersonaService.js
class PersonaService {
  constructor() {
    this.semantic = new SemanticClient({
      project: 'LexicRoot',
      service: 'persona-factory'
    });
  }

  async createPersona(traits) {
    // Get similar personas for consistency
    const similar = await this.semantic.semanticSearch({
      query: `persona traits: ${traits.join(' ')}`,
      type: 'persona'
    });

    // Create persona with context
    const persona = await this.generatePersona(traits, similar);

    // Capture the creation process
    await this.semantic.captureConversation({
      sessionId: `persona-${persona.id}`,
      conversation: `Created persona: ${JSON.stringify(persona)}`,
      metadata: { operation: 'create', traits }
    });

    return persona;
  }
}
```

### 3. Testing with Semantic Context
```javascript
// tests/integration/content-generation.test.js
describe('Content Generation with Semantic Context', () => {
  beforeEach(async () => {
    // Ensure Centerfire Intelligence is available
    await semanticClient.healthCheck();
  });

  it('should generate contextually relevant content', async () => {
    const result = await contentGenerator.generate({
      topic: 'AI development trends',
      style: 'technical-blog',
      length: 'medium'
    });

    // Verify semantic relevance
    const similarity = await semanticClient.compareContent(
      result.content,
      'AI development best practices'
    );
    
    expect(similarity.score).toBeGreaterThan(0.7);
  });
});
```

## Production Deployment

### Docker Compose Integration
```yaml
# lexicroot/docker-compose.yml
version: '3.8'
services:
  # LexicRoot services...
  
  # External Centerfire Intelligence
  semantic-ai:
    image: centerfire/intelligence:latest
    ports:
      - "8083:8083"
    environment:
      - DAEMON_HOME=/app/.centerfire-intelligence
    volumes:
      - semantic_data:/app/.centerfire-intelligence
    networks:
      - lexicroot-network

volumes:
  semantic_data:
networks:
  lexicroot-network:
    external: true
    name: centerfire-intelligence
```

### Kubernetes Integration
```yaml
# k8s/semantic-ai-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: centerfire-intelligence
spec:
  selector:
    app: centerfire-intelligence
  ports:
    - port: 8083
      targetPort: 8083
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: centerfire-intelligence
spec:
  replicas: 2
  selector:
    matchLabels:
      app: centerfire-intelligence
  template:
    metadata:
      labels:
        app: centerfire-intelligence
    spec:
      containers:
      - name: semantic-daemon
        image: centerfire/intelligence:latest
        ports:
        - containerPort: 8083
        env:
        - name: DAEMON_HOME
          value: "/app/.centerfire-intelligence"
        volumeMounts:
        - name: semantic-storage
          mountPath: /app/.centerfire-intelligence
      volumes:
      - name: semantic-storage
        persistentVolumeClaim:
          claimName: semantic-pvc
```

## Configuration Management

### Environment Variables
```bash
# LexicRoot services
CENTERFIRE_URL=http://centerfire-intelligence:8083
CENTERFIRE_PROJECT=LexicRoot
CENTERFIRE_AUTO_CAPTURE=true

# Service-specific
PERSONA_FACTORY_SEMANTIC=true
CONTENT_GENERATOR_CONTEXT_LIMIT=50
REDDIT_BOT_ENGAGEMENT_TRACKING=true
```

### Feature Flags
```javascript
// config/features.js
module.exports = {
  semanticAI: {
    enabled: process.env.NODE_ENV === 'production',
    services: {
      conversationCapture: true,
      contextLoading: true,
      semanticSearch: true,
      relationshipMapping: process.env.NODE_ENV === 'production'
    }
  }
};
```

## Monitoring & Analytics

### Health Checks
```javascript
// monitoring/health-checks.js
const healthChecks = [
  {
    name: 'centerfire-intelligence',
    check: () => semanticClient.healthCheck(),
    critical: false // Graceful degradation
  }
];
```

### Metrics Collection
```javascript
// metrics/semantic-metrics.js
const metrics = {
  conversationsCaptured: new Counter('conversations_captured_total'),
  contextRetrievalTime: new Histogram('context_retrieval_duration_seconds'),
  semanticSearchQueries: new Counter('semantic_search_queries_total'),
  relationshipMappingTime: new Histogram('relationship_mapping_duration_seconds')
};
```

## Migration Strategy

### Phase 1: Standalone Integration (Current)
- Centerfire Intelligence as external service
- Manual integration via @lexicroot/semantic-client
- Optional feature with graceful degradation

### Phase 2: Deep Integration (3-6 months)
- Embedded semantic capabilities in core services
- Automated context injection
- Real-time relationship updates

### Phase 3: AI-Native Architecture (6-12 months)
- Semantic AI as first-class citizen
- Autonomous content optimization
- Predictive persona management

---

*This integration enables LexicRoot to leverage Centerfire Intelligence for enhanced content generation, persona management, and distribution optimization while maintaining service independence and scalability.*
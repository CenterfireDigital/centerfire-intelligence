# Socket-Based Multi-Interface Architecture - Key Decisions
*Documented: 2025-09-06*
*Context: Architectural discussion about decoupling agents from Claude Code*

## 🎯 Core Problem Identified
- **Current agents are tightly coupled to Claude Code** via direct `go run main.go &` spawning
- **Documentation shows vision for multi-interface support**: Web, VS Code, API clients, etc.
- **Need interface abstraction layer** to support multiple client types

## 🏗️ Socket-Based Architecture Decision

### **Why Sockets Over HTTP**
- **Team size**: Just user + Claude = rapid iteration, not enterprise overhead
- **Performance**: Unix domain sockets ~10x faster than HTTP for local communication
- **Direct communication**: Lower overhead between components
- **HTTP only for**: Web UI, LLM APIs (Claude/Codex/Gemini), external services

### **Architecture Stack**
```
┌─────────────────────────────────────────────────────────────┐
│                    Client Interfaces                        │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐             │
│  │ Claude Code │ │ Web Browser │ │ VS Code Ext │             │
│  │   (HTTP)    │ │ (WebSocket) │ │   (HTTP)    │             │
│  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘             │
└─────────┼─────────────────┼─────────────────┼────────────────┘
          │                 │                 │
    ┌─────▼─────────────────▼─────────────────▼─────┐
    │           Rust Core (Orchestrator)            │
    │  ┌─────────────────────────────────────────┐  │
    │  │        Socket Manager                   │  │
    │  │  ┌─────────┐ ┌─────────┐ ┌───────────┐  │  │
    │  │  │HTTP Srv │ │WebSocket│ │TCP Client │  │  │
    │  │  │for APIs │ │for Web  │ │for LLMs   │  │  │
    │  │  └─────────┘ └─────────┘ └───────────┘  │  │
    │  └─────────────────┬───────────────────────┘  │
    │                    │                          │
    │  ┌─────────────────▼───────────────────────┐  │
    │  │         Agent Socket Pool               │  │
    │  │   Unix Domain Socket Connections        │  │
    │  └─────────────────┬───────────────────────┘  │
    └────────────────────┼────────────────────────────┘
                         │
    ┌────────────────────▼────────────────────────┐
    │              Go Agents                      │
    │  ┌──────────┐ ┌──────────┐ ┌─────────────┐  │
    │  │ NAMING-1 │ │ STRUCT-1 │ │ SEMANTIC-1  │  │
    │  │(dual mode│ │(dual mode│ │ (dual mode) │  │
    │  │Redis+Sock│ │Redis+Sock│ │ Redis+Sock) │  │
    │  └──────────┘ └──────────┘ └─────────────┘  │
    └─────────────────────────────────────────────┘
```

## 🔄 Agent Conversion Strategy

### **Keep Current System Running**
- Current Redis-based pub/sub agents must remain operational
- New socket interface added in parallel
- Zero downtime during transition

### **Dual-Mode Agent Implementation**
```go
// agents/AGT-NAMING-1__01K4EAF1/main.go
func main() {
    agent := NewNamingAgent()
    
    // KEEP: Redis mode for current system
    go agent.startRedisMode()
    
    // ADD: Socket mode for Rust orchestrator
    if os.Getenv("SOCKET_MODE") == "true" {
        go agent.startSocketMode("/tmp/naming-agent.sock")
    }
}
```

### **Socket Protocol Design**
```rust
// Rust orchestrator
struct AgentManager {
    naming_socket: UnixStream,    // /tmp/naming-agent.sock
    struct_socket: UnixStream,    // /tmp/struct-agent.sock  
    semantic_socket: UnixStream,  // /tmp/semantic-agent.sock
    manager_socket: UnixStream,   // /tmp/manager-agent.sock
}

impl AgentManager {
    async fn call_naming_agent(&self, request: AgentRequest) -> AgentResponse {
        // Direct socket call to Go agent
        self.naming_socket.write_all(&serialize(request)).await?;
        let response = self.read_response().await?;
        deserialize(response)
    }
}
```

## 🎨 Multi-Language Orchestration

### **Rust - Performance Core**
- **Socket management** and connection pooling
- **Stream processing** for high-throughput data  
- **Token counting/windowing** for LLM context management
- **Circuit breakers** and rate limiting for external APIs
- **WebSocket server** for real-time web interface

### **Go - Agent Logic**  
- **Existing agent codebase** with minimal changes
- **Business logic** remains unchanged in Go
- **Dual transport**: Redis + Unix sockets
- **Domain expertise** in semantic naming, structure creation, etc.

### **Node.js - Web Interface**
- **Web UI** connecting via WebSocket to Rust core
- **LLM SDK integration** where needed
- **Web ecosystem** tools and libraries

## 🚀 Performance Benefits

### **Socket Communication**
- **Unix domain sockets**: ~10x faster than HTTP for local IPC
- **Zero network stack overhead** for local communication  
- **Direct memory sharing** where possible
- **Lower latency** for agent coordination

### **Selective Protocol Use**
- **Sockets**: Rust ↔ Go agents, high-frequency internal communication
- **HTTP**: External LLM APIs, Claude Code integration, VS Code extensions
- **WebSocket**: Real-time web interface, browser communication
- **TCP**: External database connections, service mesh communication

## 🎯 Interface Support Matrix

| Interface Type | Protocol | Implementation | Status |
|----------------|----------|----------------|---------|
| Claude Code    | HTTP     | Existing hooks + daemon API | Current |
| Web Browser    | WebSocket| Rust WebSocket server | Planned |
| VS Code Ext    | HTTP     | Extension → daemon API | Planned |
| API Clients    | HTTP     | REST API endpoints | Planned |
| Terminal Scripts| HTTP    | curl/wget commands | Planned |
| Internal IPC   | Sockets  | Rust ↔ Go communication | Planned |

## 🔧 Implementation Phases

### **Phase 1: Socket Foundation**
1. Create Rust orchestrator with socket management
2. Add socket listeners to existing Go agents  
3. Test socket communication Rust ↔ Go
4. Verify feature parity with Redis communication

### **Phase 2: HTTP API Layer**
1. Add HTTP endpoints to Rust orchestrator
2. Route HTTP requests to appropriate agents via sockets
3. Test Claude Code integration via HTTP instead of direct spawning
4. Maintain backward compatibility with current system

### **Phase 3: Multi-Interface Support**  
1. Add WebSocket server for web interface
2. Create web UI connecting to WebSocket
3. Test multi-client scenarios (Claude Code + Web simultaneously)
4. Add API documentation and client SDKs

### **Phase 4: Advanced Features**
1. LLM integration (Claude/Codex/Gemini API clients)
2. Context window management and token optimization  
3. Session management across interfaces
4. Monitoring and health check systems

## 🔐 Environment Isolation Challenge

### **The Problem**
- **Claude Code needs access to host filesystem** for project work
- **But Claude Code cannot access new agent system** if agents running in isolated environment
- **Network proxy/interception approach** needed for complete conversation capture
- **Lightweight isolation** required (not full VM)

### **Solution Options** (detailed in next section)
- Container with bind mounts
- Namespace isolation  
- Process sandbox with selective access
- Network proxy with transparent interception

## 📝 Key Architectural Principles

1. **Socket-first for internal communication** - performance and simplicity
2. **HTTP for external interfaces** - compatibility and standards
3. **Keep existing agents running** - zero downtime transitions  
4. **Dual-mode implementation** - gradual migration path
5. **Language specialization** - Rust for performance, Go for logic, Node for web
6. **Interface abstraction** - same agent logic, multiple access methods

---

**This document captures the architectural decisions made on 2025-09-06 regarding the transition from Claude Code-coupled agents to a multi-interface socket-based system. Reference this document in future sessions to avoid re-discovering these architectural decisions.**
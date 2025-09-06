# Project Restructure Plan: C++ Migration

## New Directory Structure

```
CenterfireIntelligence/
├── cpp/                          # All C++ daemons (NEW)
│   ├── CMakeLists.txt           # Root CMake config
│   ├── conanfile.txt            # C++ dependencies
│   ├── common/                  # Shared C++ code
│   │   ├── semdoc/              # SemDoc parser/validator
│   │   ├── contracts/           # Contract definitions
│   │   ├── messaging/           # Inter-daemon communication
│   │   └── storage/             # Storage client interfaces
│   │
│   ├── daemons/                 # Individual daemons
│   │   ├── stream-processor/    # High-performance stream consumer
│   │   ├── semdoc-engine/       # Contract validation service
│   │   ├── context-restorer/    # Context management
│   │   ├── conversation-proxy/  # Claude Code interceptor
│   │   ├── task-router/         # LLM routing logic
│   │   └── health-monitor/      # Service health tracking
│   │
│   └── build/                   # Build output
│
├── python-deprecated/            # Current Python code (TO BE REMOVED)
│   ├── daemon/                  # Move existing daemon here
│   └── DEPRECATION.md          # Deprecation timeline
│
├── contracts/                    # SemDoc contracts (language-agnostic)
│   ├── core/                    # Core system contracts
│   ├── daemons/                 # Per-daemon contracts
│   └── registry.yaml            # Contract registry
│
├── scripts/                      # Orchestration & deployment
│   ├── start-all.sh             # Start all daemons
│   ├── migrate.sh               # Python -> C++ migration
│   └── test-integration.sh      # End-to-end tests
│
├── docker/                       # Container definitions
│   ├── Dockerfile.cpp           # C++ daemon image
│   ├── Dockerfile.python        # Legacy Python (temporary)
│   └── docker-compose.yml       # Full stack
│
└── docs/
    ├── architecture/            # System design docs
    ├── migration/               # Migration guides
    └── semdoc/                  # SemDoc specifications
```

## Migration Strategy

### Phase 1: Parallel Structure (Week 1)
```bash
# Create C++ structure without breaking Python
mkdir -p cpp/{common,daemons,build}
mkdir -p contracts/{core,daemons}

# Move Python to deprecated (but keep running)
mv daemon python-deprecated/daemon
ln -s python-deprecated/daemon daemon  # Symlink for compatibility
```

### Phase 2: First C++ Daemon (Week 1-2)
Build stream-processor in C++ running on different port:
- Python daemon: port 8081 (current)
- C++ daemon: port 8091 (new)
- Both consume same Redis stream (different consumer groups)

### Phase 3: SemDoc Engine (Week 2)
New daemon, no Python equivalent:
- Port 8082
- Parses contracts from `/contracts/`
- Provides validation API
- Stores in Redis/Neo4j

### Phase 4: Gradual Migration (Week 3-4)
For each Python component:
1. Build C++ replacement with SemDoc
2. Run both in parallel
3. Verify C++ version works
4. Switch traffic to C++
5. Remove Python component

## Inter-Daemon Communication Protocol

### Message Format (with SemDoc)
```cpp
// @semblock
// contract:
//   purpose: "Standard message format for inter-daemon communication"
//   invariants: ["message_id is unique", "timestamp is UTC"]
//   schema:
//     type: "protobuf"
//     version: "1.0"
struct DaemonMessage {
    std::string message_id;      // UUID v4
    std::string source_daemon;   // Sender identification
    std::string target_daemon;   // Recipient (or "broadcast")
    std::string message_type;    // Command/Event/Query/Response
    
    // SemDoc contract for this message
    Contract contract;
    
    // Actual payload
    std::variant<
        ConversationData,
        ContractValidation,
        HealthCheck,
        RouterDecision
    > payload;
    
    // Metadata
    uint64_t timestamp;
    std::string correlation_id;  // For request/response tracking
    uint32_t priority;           // Message priority (0-9)
};
```

### Service Registry Protocol
```cpp
// @semblock
// contract:
//   purpose: "Service discovery and health tracking"
//   effects:
//     writes: ["redis.service_registry"]
//   performance:
//     registration_time: "<100ms"
class ServiceRegistry {
    struct ServiceInfo {
        std::string daemon_name;
        std::string version;
        std::string host;
        uint16_t port;
        std::vector<std::string> capabilities;
        Contract service_contract;  // What this service promises
        
        // Health info
        std::string status;  // "starting", "healthy", "degraded", "stopping"
        uint64_t last_heartbeat;
        std::map<std::string, double> metrics;
    };
    
    void register_service(const ServiceInfo& info);
    std::optional<ServiceInfo> discover(const std::string& daemon_name);
    std::vector<ServiceInfo> discover_all();
};
```

## Build System Setup

### CMakeLists.txt (Root)
```cmake
cmake_minimum_required(VERSION 3.20)
project(CenterfireIntelligence VERSION 1.0.0)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Enable SemDoc validation at compile time
add_compile_definitions(SEMDOC_VALIDATION=1)

# Common libraries
add_subdirectory(common)

# Daemons
add_subdirectory(daemons/stream-processor)
add_subdirectory(daemons/semdoc-engine)
add_subdirectory(daemons/context-restorer)

# Testing
enable_testing()
add_subdirectory(tests)
```

### Conan Dependencies
```ini
[requires]
redis-plus-plus/1.3.3
nlohmann_json/3.11.2
spdlog/1.11.0
crow/1.0+5          # HTTP server
protobuf/3.21.12
grpc/1.51.1
neo4j-cpp-driver/1.0.0
catch2/3.3.2        # Testing

[generators]
CMakeDeps
CMakeToolchain
```

## Development Workflow

### 1. Start Existing Python Stack
```bash
# Keep Python running for now
./scripts/install-global.sh
~/.local/bin/centerfire-daemon start
```

### 2. Build C++ Components
```bash
# Set up C++ build
cd cpp
mkdir build && cd build
conan install .. --build=missing
cmake ..
make -j8
```

### 3. Run C++ Daemons Alongside Python
```bash
# Start C++ daemons (different ports)
./cpp/build/bin/stream-processor --port 8091 &
./cpp/build/bin/semdoc-engine --port 8082 &
```

### 4. Test Integration
```bash
# Send test messages to both Python and C++
./scripts/test-integration.sh
```

### 5. Monitor Both Systems
```bash
# Custom health check for all daemons
curl http://localhost:8080/health  # API gateway
curl http://localhost:8081/health  # Python daemon
curl http://localhost:8091/health  # C++ stream processor
curl http://localhost:8082/health  # SemDoc engine
```

## Deprecation Timeline

### Week 1
- [ ] Set up C++ project structure
- [ ] Build basic C++ stream processor
- [ ] Implement service registry
- [ ] Create inter-daemon protocol

### Week 2
- [ ] Complete C++ stream processor
- [ ] Build SemDoc engine
- [ ] Add contract validation
- [ ] Start parallel operation

### Week 3
- [ ] Build context restorer in C++
- [ ] Add conversation proxy
- [ ] Implement task router
- [ ] Full integration testing

### Week 4
- [ ] Switch primary traffic to C++
- [ ] Deprecate Python components
- [ ] Performance validation
- [ ] Production deployment

## Success Criteria

1. **Performance**: C++ daemons show 10x throughput improvement
2. **Reliability**: Zero message loss during migration
3. **SemDoc Coverage**: 100% of C++ code has contracts
4. **Compatibility**: C++ daemons fully replace Python functionality
5. **Monitoring**: Complete observability of all daemons

## Next Immediate Steps

1. Create C++ directory structure
2. Set up CMake build system
3. Write first SemDoc contract for stream processor
4. Build minimal C++ daemon with health endpoint
5. Test alongside Python daemon

---

*This restructure enables parallel development while maintaining system stability. Python keeps running until C++ is proven stable.*
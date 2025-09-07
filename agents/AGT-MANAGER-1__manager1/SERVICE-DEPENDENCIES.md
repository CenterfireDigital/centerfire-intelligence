# AGT-MANAGER-1 Service Dependency Tracking

## Overview
AGT-MANAGER-1 has been enhanced with comprehensive service dependency tracking and validation capabilities. This addresses the issue identified during ClickHouse consumer work where restart failures highlighted the need for dependency management.

## New Capabilities

### 1. Dependency Definition
Each agent can now define dependencies in their registry entry:

```go
Dependencies: []ServiceDependency{
    {Service: "redis", Type: "infrastructure", Endpoint: "localhost:6380", Critical: true, RetryCount: 3, RetryDelay: 5},
    {Service: "weaviate", Type: "infrastructure", Endpoint: "localhost:8080", Critical: true, RetryCount: 3, RetryDelay: 10},
    {Service: "AGT-NAMING-1", Type: "agent", Endpoint: "centerfire:agent:naming", Critical: true, RetryCount: 2, RetryDelay: 3},
}
```

### 2. Dependency Types
- **Infrastructure**: Redis, Weaviate, Docker, Neo4j, ClickHouse
- **Agent**: Other agents that must be running (AGT-NAMING-1, etc.)
- **Container**: Docker containers that must be in running state

### 3. New Request Types

#### `check_dependencies`
Validates all dependencies for a specific agent:
```json
{
  "request_type": "check_dependencies",
  "agent_name": "AGT-SEMANTIC-1"
}
```

#### `validate_service_health` 
Checks health of a specific service and shows affected agents:
```json
{
  "request_type": "validate_service_health",
  "agent_name": "redis"
}
```

#### `restart_with_dependencies`
Performs dependency-aware agent restart:
```json
{
  "request_type": "restart_with_dependencies",
  "agent_name": "AGT-STACK-1",
  "dependency_check": true,
  "force_restart": false
}
```

### 4. Automatic Features

#### Pre-Start Validation
All agent starts now validate critical dependencies unless explicitly disabled:
- Checks Redis connectivity
- Validates Weaviate endpoints  
- Confirms required agents are running
- Validates Docker daemon for container agents

#### Automatic Recovery
When persistent agents die, AGT-MANAGER-1 now:
1. Waits 10 seconds for graceful cleanup
2. Validates dependencies before restart attempt
3. Only restarts if critical dependencies are available
4. Logs detailed diagnostic information

#### Retry Logic
Each dependency check includes configurable retry logic:
- `RetryCount`: Number of attempts before failing
- `RetryDelay`: Seconds between retry attempts
- Critical vs non-critical failure handling

## Agent Registry Updates

### Enhanced Agent Definitions
All core agents now have dependency definitions:

- **AGT-NAMING-1**: Requires Redis
- **AGT-SEMANTIC-1**: Requires Redis, Weaviate, AGT-NAMING-1
- **AGT-STRUCT-1**: Requires Redis, AGT-NAMING-1  
- **AGT-STACK-1**: Requires Redis, Docker daemon
- **AGT-CLEANUP-1**: Requires Weaviate, Neo4j (optional)

### Health Check Configuration
Agents can define health check commands:
```go
HealthCheck: &HealthCheckConfig{
    Command: "curl -s http://localhost:8080/v1/meta",
    Interval: 60,
    Timeout: 10, 
    Retries: 2,
}
```

## Testing

Use `test-dependencies.go` to validate the new functionality:

```bash
cd agents/AGT-MANAGER-1__manager1
go run test-dependencies.go
```

Tests include:
1. Dependency validation for AGT-SEMANTIC-1
2. Redis service health validation
3. Dependency-aware restart of AGT-STACK-1

## Architecture Benefits

### Reliability
- Prevents agents from starting without required services
- Automatic recovery with dependency validation
- Clear failure diagnostics

### Monitoring
- Service health validation on demand
- Dependency status visibility
- Failed dependency logging

### Operational Intelligence
- Retry logic prevents transient failures
- Critical vs non-critical dependency classification
- Impact analysis (which agents affected by service failure)

## Future Enhancements

1. **Dependency Chains**: Automatically start dependencies in correct order
2. **Health Check Automation**: Periodic health validation
3. **Service Discovery Integration**: Dynamic endpoint resolution
4. **Notification System**: Alert on critical dependency failures
5. **Metrics Collection**: Dependency availability statistics

## Implementation Details

- **Location**: `/agents/AGT-MANAGER-1__manager1/main.go`
- **New Functions**: 
  - `handleCheckDependencies()`
  - `handleValidateServiceHealth()`
  - `handleRestartWithDependencies()`
  - `checkServiceDependency()`
  - `validateAgentDependencies()`
- **Enhanced Capabilities**: Added to AGT-MANAGER-1 registry entry
- **Backward Compatible**: Existing functionality unchanged

This enhancement resolves the ClickHouse consumer restart failures by ensuring all service dependencies are validated before agent startup attempts.
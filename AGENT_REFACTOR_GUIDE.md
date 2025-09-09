# Agent Refactoring Guide - The Builder's Bible

**Reference this document for EVERY agent refactor. No exceptions.**

## Pre-Build Phase

### 1. Agent Interface Contract Documentation
- [ ] **Current behavior documented**: What does the agent do exactly?
- [ ] **Input/output specifications**: Message formats, data structures
- [ ] **Dependencies mapped**: Which agents does this depend on?
- [ ] **Dependents identified**: Which agents depend on this?
- [ ] **Failure modes documented**: How does it behave under stress/failure?
- [ ] **Performance characteristics**: Response time, throughput, resource usage
- [ ] **State management**: Does it maintain state? How is it persisted?

### 2. Communication Analysis
- [ ] **Current communication method**: HTTP/Redis/Files/Process?
- [ ] **Message patterns**: Request/response, pub/sub, streaming?
- [ ] **Protocol specification**: Exact message format and sequence
- [ ] **Error handling**: How are failures communicated?
- [ ] **Security considerations**: Authentication, authorization, data protection

### 3. Architecture Planning
- [ ] **New communication method chosen**: Unix socket (default), HTTP (exception), Redis (exception)
- [ ] **Socket path planned**: `/tmp/agt-{type}-{instance}.sock`
- [ ] **Namespace defined**: `centerfire.agents.{type}`
- [ ] **Health check method**: File-based (default), HTTP (exception), process (simple)
- [ ] **Parent-child relationships**: Is this a parent? Child? Standalone?

## Build Phase

### 4. Template Customization
- [ ] **Template copied**: `cp -r templates/agent-template agents/AGT-{TYPE}-{VERSION}__{INSTANCE}/`
- [ ] **agent.yaml configured**: All placeholders replaced with actual values
- [ ] **Unnecessary components removed**: No HTTP server unless needed, no Redis unless needed
- [ ] **Communication layer implemented**: Unix socket server/client as appropriate
- [ ] **Health check implemented**: File-based status reporting
- [ ] **Namespace properly set**: All logging/communication includes namespace

### 5. Core Logic Implementation
- [ ] **Main agent logic ported**: Business logic from old agent
- [ ] **Message routing implemented**: Handle incoming requests appropriately
- [ ] **Error handling added**: Graceful failure and error reporting
- [ ] **Performance optimizations**: No regressions from old agent
- [ ] **Resource cleanup**: Proper cleanup on shutdown

### 6. Integration Points
- [ ] **Manager registration**: Properly registers with AGT-MANAGER via file/message
- [ ] **Claude Capture integration**: All logs sent to capture agent with namespace
- [ ] **Monitor discovery**: Health status reportable by monitoring systems
- [ ] **Signal handling**: SIGTERM/SIGINT handled gracefully
- [ ] **PID management**: PID file created and cleaned up

## Testing Phase

### 7. Unit Testing
- [ ] **Template compliance**: Agent follows template structure
- [ ] **Configuration validation**: agent.yaml loads correctly
- [ ] **Communication layer**: Unix socket works correctly
- [ ] **Core logic**: Business functions work as expected
- [ ] **Error conditions**: Failures handled gracefully
- [ ] **Resource cleanup**: No leaks, proper shutdown

### 8. Integration Testing
- [ ] **Dependency integration**: Works with required agents
- [ ] **Manager registration**: Successfully registers and deregisters
- [ ] **Health reporting**: Monitoring can detect agent status
- [ ] **Logging integration**: Logs appear in capture system with correct namespace
- [ ] **Performance testing**: Meets or exceeds old agent performance
- [ ] **Load testing**: Handles expected concurrent load

### 9. Behavioral Equivalence
- [ ] **Input/output matching**: Same responses for same inputs as old agent
- [ ] **Side effects preserved**: All expected side effects occur
- [ ] **Error conditions identical**: Same error responses as old agent
- [ ] **Performance equivalent**: No significant performance regressions
- [ ] **Resource usage**: Memory and CPU usage within acceptable range

## Deployment Phase

### 10. Pre-Deployment
- [ ] **Documentation updated**: Agent-specific README created
- [ ] **Configuration verified**: All environment-specific settings correct
- [ ] **Dependencies available**: Required agents running and accessible
- [ ] **Monitoring configured**: Health checks and alerts configured
- [ ] **Rollback plan ready**: Clear steps to revert if needed

### 11. Parallel Deployment
- [ ] **Old agent still running**: Keep original running during validation
- [ ] **New agent deployed**: Start new agent in parallel
- [ ] **Traffic splitting**: Route some traffic to new agent for validation
- [ ] **Monitoring comparison**: Compare metrics between old and new
- [ ] **Error rate monitoring**: No increase in errors from new agent

### 12. Cutover
- [ ] **Full traffic to new agent**: All requests routed to new agent
- [ ] **Old agent gracefully stopped**: Clean shutdown of original
- [ ] **Cleanup performed**: Remove old agent files and configurations
- [ ] **Monitoring verified**: All metrics showing healthy operation
- [ ] **Documentation updated**: Deployment notes and lessons learned

## Communication Standards

### Message Format (Standard)
```json
{
  "version": "1.0",
  "namespace": "centerfire.agents.{type}",
  "agent_id": "AGT-{TYPE}-{VERSION}",
  "cid": "cid:centerfire:agent:{instance}",
  "request_id": "uuid",
  "timestamp": "iso8601",
  "payload": {
    "action": "action_name",
    "data": {...}
  },
  "metadata": {
    "retry_count": 0,
    "priority": "normal"
  }
}
```

### Health Check Format (File-based)
```json
{
  "agent_id": "AGT-{TYPE}-{VERSION}",
  "cid": "cid:centerfire:agent:{instance}",
  "namespace": "centerfire.agents.{type}",
  "status": "healthy|degraded|unhealthy",
  "timestamp": "iso8601",
  "uptime": "seconds",
  "pid": "process_id",
  "version": "agent_version",
  "metrics": {
    "requests_processed": 0,
    "errors": 0,
    "memory_usage": "bytes",
    "cpu_usage": "percentage"
  }
}
```

## Error Conditions & Rollback Triggers

### Automatic Rollback Triggers
- Error rate > 5% increase from baseline
- Response time > 2x baseline
- Memory usage > 150% of old agent
- Health check failures > 3 consecutive
- Dependency connection failures

### Manual Rollback Process
1. Stop new agent: `kill -TERM {pid}`
2. Restart old agent: Follow old startup procedure
3. Update routing: Redirect traffic back to old agent
4. Verify operation: Confirm normal operation resumed
5. Investigate: Determine root cause before retry

## Refactoring Order (Dependency-First)

### Phase 1: Leaf Agents (No Dependencies)
- AGT-BOOTSTRAP-1 (utility)
- AGT-CLEANUP-1 (maintenance)
- AGT-LOCAL-LLM-1 (isolated)

### Phase 2: Data Processors
- AGT-NAMING-1 (basic services)
- AGT-STRUCT-1 (data processing)
- AGT-SEMANTIC-1 (analysis)

### Phase 3: Infrastructure Agents
- AGT-CLAUDE-CAPTURE-2 (conversation capture - new version)
- AGT-MONITOR-2 (monitoring - new version)
- AGT-MANAGER-2 (management - new version)

### Phase 4: Gateway Agents
- AGT-HTTP-GATEWAY-2 (external interface - new version)
- AGT-CONTEXT-1 (context management)

### Phase 5: Complex Agents
- AGT-SEMDOC-PARSER-1 (semantic processing)
- AGT-CODING-1 (code generation)
- AGT-STACK-1 (full stack)

## Development Environment Setup

### VPS Configuration (LexicRootDev - 137.220.51.98)
- Ubuntu 22.04 LTS
- Docker + Docker Compose
- Go 1.21+, Python 3.11+, Node.js 20+
- Redis 7.0+ (clean instance)
- Git repository clone
- Development tools (vim, tmux, htop, etc.)

### Testing Protocol
1. **Build agent on VPS**: Clean environment, no legacy interference
2. **Test against minimal infrastructure**: Only essential components running
3. **Compare with production**: Behavioral equivalence testing
4. **Load testing**: Ensure performance meets requirements
5. **Deploy to production**: Only after VPS validation complete

## Golden Rules

1. **Always use the template**: No shortcuts, no exceptions
2. **Document everything first**: Interface contract before coding
3. **Test behavioral equivalence**: New agent must match old agent exactly
4. **Keep old agent running**: Until new agent is proven in production
5. **Monitor everything**: Health, performance, errors, dependencies
6. **Have a rollback plan**: Always know how to go back
7. **One agent at a time**: No parallel refactoring of dependent agents

## Success Criteria

An agent refactor is successful when:
- ✅ All template requirements met
- ✅ Behavioral equivalence proven
- ✅ Performance equivalent or better
- ✅ Integration tests pass
- ✅ Running in production without issues for 48 hours
- ✅ Old agent cleanly removed
- ✅ Documentation complete

---

**Remember: This is not just a refactor - it's building the foundation for the entire Centerfire Intelligence agent ecosystem. Do it right.**
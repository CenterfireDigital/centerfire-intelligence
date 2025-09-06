# System Commander Enhancement Plan - Multi-Shell Orchestration

**Status**: 📋 PLANNED - Post-Compact Implementation  
**Version**: 2.0 Design  
**Date**: 2025-09-06  

## Vision: Process Orchestration Engine

Transform AGT-SYSTEM-COMMANDER-1 from simple command executor to sophisticated **multi-shell process coordination engine** capable of managing complex, interdependent system operations.

## Core Architecture Concepts

### Dual Execution Strategy

#### Tmux for Session Management
- **Use Cases**: Long-running processes, interactive sessions, persistent environments
- **Benefits**: Full TTY, session persistence, visual debugging, easy attach/detach
- **Perfect for**: Development servers, monitoring processes, interactive shells
- **Example**: `redis-server`, `npm run dev`, database servers

#### Pexpect for Programmatic Control
- **Use Cases**: Automated interactions, response-driven workflows, precise control  
- **Benefits**: Programmatic expect/send patterns, fine-grained control, automation
- **Perfect for**: Installation wizards, interactive prompts, conditional workflows
- **Example**: SSH interactions, password prompts, installation scripts

### Advanced Orchestration Scenarios

#### Parallel Process Coordination
```bash
Request: "Start Redis server, wait for ready, then run tests"

Execution Plan:
- Shell A (tmux): redis-server --port 6379 (long-running)
- Shell B (pexpect): Monitor Redis logs for "Ready to accept connections"  
- Shell C (tmux): npm test (once Shell B confirms Redis ready)
```

#### Inter-Process Communication
```bash
Request: "Build frontend, build backend, start both when ready"

Execution Plan:
- Shell A: npm run build → Signal completion via Redis/file
- Shell B: go build → Signal completion via Redis/file  
- Shell C: Monitor signals, start both services when A && B complete
```

#### Conditional Workflow Execution
```bash
Request: "Deploy if tests pass, rollback if they fail"

Execution Plan:
- Shell A (pexpect): Run tests, expect success/failure patterns
- Shell B (tmux): Deploy (if Shell A succeeds)
- Shell C (tmux): Rollback (if Shell A fails)
```

## Implementation Strategy

### Phase 1: Basic Multi-Shell Support
- **Goal**: Multiple tmux sessions per client
- **Features**: Shell pool management, basic parallel execution
- **Test**: Run 3 commands simultaneously, verify all complete

### Phase 2: Pexpect Integration  
- **Goal**: Add programmatic shell control alongside tmux
- **Features**: Expect/send patterns, interactive automation
- **Test**: SSH login, password prompt handling, conditional responses

### Phase 3: Orchestration Engine
- **Goal**: Complex workflow coordination
- **Features**: Dependencies, signaling, conditional execution
- **Test**: Multi-step deployment with rollback capability

### Phase 4: Semantic Integration
- **Goal**: Integration with AGT-NAMING-1 for process identification
- **Features**: Semantic shell naming, dependency graphing
- **Test**: Named processes with AGT-GRAPH-1 relationship mapping

## Technical Architecture

### Request Classification Engine
```go
type ExecutionStrategy struct {
    Mode         string        // "tmux", "pexpect", "parallel", "sequential"
    Shells       []ShellConfig
    Dependencies []Dependency
    Timeout      time.Duration
    Signals      []SignalConfig
}

func (sc *SystemCommander) classifyRequest(cmd string) ExecutionStrategy {
    if isLongRunning(cmd) { return tmuxStrategy }
    if needsInteraction(cmd) { return pexpectStrategy }  
    if isMultiStep(cmd) { return parallelStrategy }
}
```

### Shell Pool Management
```go
type ShellPool struct {
    TmuxSessions map[string]*TmuxSession    // Persistent sessions
    PexpectProcs map[string]*PexpectProcess // Interactive processes
    Dependencies map[string][]string        // Dependency graph
    Signals      map[string]chan bool       // Inter-shell communication
    Resources    *ResourceMonitor           // CPU/memory tracking
}
```

### Enhanced Command Types

#### Orchestration Commands
- `PARALLEL: cmd1 && cmd2 && cmd3` - Run simultaneously, coordinate completion
- `SEQUENCE: cmd1 THEN cmd2 THEN cmd3` - Run in order, pass state/context  
- `CONDITIONAL: cmd1 IF_SUCCESS cmd2 IF_FAIL cmd3` - Branching logic

#### Process Management
- `BACKGROUND: long_running_process` - Tmux session, return immediately
- `MONITOR: process_name UNTIL ready_signal` - Pexpect monitoring
- `SIGNAL: shell_id message` - Inter-shell communication

## Request Language Design (Future)

### Simple DSL for Complex Orchestration
```yaml
request:
  type: orchestration
  client_id: claude_code
  steps:
    - name: redis_server
      shell: redis  
      command: "redis-server --port 6379"
      mode: tmux
      wait_for: "Ready to accept connections"
      
    - name: run_tests
      shell: tests
      command: "npm test -- --redis-port=6379"
      mode: pexpect  
      depends_on: [redis_server]
      timeout: 300s
      
    - name: cleanup
      shell: cleanup
      command: "pkill redis-server"
      mode: direct
      depends_on: [run_tests]
      condition: always
```

## Key Design Questions for Implementation

### 1. State Management
- **Redis-based signaling**: Shells publish/subscribe coordination messages
- **File-based handoffs**: Temporary files for data passing between shells
- **Memory-based coordination**: System Commander tracks all shell states

### 2. Error Handling & Recovery
- **Cascade failure**: Kill all dependent processes when parent fails
- **Retry logic**: Attempt process restart with exponential backoff
- **Graceful degradation**: Continue with available shells, report partial success

### 3. Resource Management  
- **Per-client limits**: Each client_id gets N shells maximum
- **System monitoring**: CPU/memory-based throttling
- **Priority queuing**: Critical processes get priority shell allocation

### 4. Security & Isolation
- **Shell sandboxing**: Limit shell access to specific directories
- **Resource constraints**: Memory/CPU limits per shell
- **Process cleanup**: Automatic termination of orphaned processes

## Integration Points

### AGT-NAMING-1 Integration
- **Process naming**: Each shell gets semantic identifier
- **Dependency tracking**: Named process relationships
- **Log aggregation**: Shell outputs tagged with semantic IDs

### AGT-MANAGER-1 Integration  
- **Health monitoring**: Shell pool status and resource usage
- **Process registry**: Track all active shells across clients
- **Automatic cleanup**: Terminate shells when System Commander dies

### HTTP Gateway Integration
- **WebSocket support**: Real-time shell output streaming  
- **Process control**: Start/stop/status endpoints for shell management
- **Bulk operations**: Submit complex orchestration requests via HTTP

## Implementation Phases

### Phase 1: Foundation (Post-Compact Priority)
1. **Multi-tmux support**: Create/manage multiple tmux sessions per client
2. **Shell pool**: Basic shell lifecycle management  
3. **Parallel execution**: Simple concurrent command execution
4. **Basic testing**: Verify 3+ simultaneous commands work correctly

### Phase 2: Programmatic Control
1. **Pexpect integration**: Add programmatic shell interaction
2. **Pattern matching**: Expect/send automation for interactive processes
3. **Mode detection**: Auto-select tmux vs pexpect based on command type
4. **Interactive testing**: SSH, password prompts, conditional workflows

### Phase 3: Orchestration Engine
1. **Dependency engine**: Process coordination and signaling
2. **Conditional execution**: Success/failure branching logic
3. **State management**: Cross-shell communication and data passing
4. **Complex testing**: Multi-step deployment with rollback scenarios

### Phase 4: Production Readiness
1. **Resource monitoring**: CPU/memory limits and throttling
2. **Security hardening**: Process isolation and sandboxing  
3. **Semantic integration**: AGT-NAMING-1 and AGT-GRAPH-1 connectivity
4. **Performance testing**: High-load orchestration scenarios

## Success Metrics

### Phase 1 Success
- ✅ 10 concurrent tmux sessions running simultaneously  
- ✅ Shell isolation (commands don't interfere with each other)
- ✅ Proper cleanup (no orphaned processes after client disconnect)

### Phase 2 Success
- ✅ Interactive SSH session automation via pexpect
- ✅ Password prompt handling and conditional responses
- ✅ Mixed tmux/pexpect execution in single request

### Phase 3 Success  
- ✅ Multi-step deployment: build → test → deploy → rollback
- ✅ Process dependency coordination (A waits for B, then triggers C)
- ✅ Error propagation and cascading cleanup

### Phase 4 Success
- ✅ 100+ shells managed simultaneously without resource exhaustion
- ✅ Semantic process naming and relationship mapping
- ✅ Production-grade security and monitoring

---

**Next Steps Post-Compact**: Start with Phase 1 - Basic multi-tmux session support and shell pool management. Build iteratively with comprehensive testing at each stage.

**Architecture Philosophy**: Transform System Commander from simple command executor to sophisticated process orchestration engine, enabling complex system automation while maintaining security and reliability.
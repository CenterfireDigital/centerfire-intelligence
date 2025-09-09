# Minimal Agent Template

This template provides the **absolute minimum** infrastructure needed for a Centerfire Intelligence agent.

## Universal Requirements

Every agent built from this template gets:

1. **Discoverability**: Registers with monitor, reports friendly name and capabilities
2. **Graceful Shutdown**: SIGTERM/SIGINT handling with cleanup notification  
3. **PID Management**: Process tracking for monitoring
4. **Namespace Identification**: All communications include namespace for proper routing
5. **Health Reporting**: File-based health status (no unnecessary HTTP servers)
6. **Structured Logging**: All logs routed to Claude Capture agent with namespace

## What's NOT Included (Add Only If Needed)

- ❌ HTTP server (unless agent serves web requests)
- ❌ Redis integration (unless agent needs pub/sub) 
- ❌ Database connections (unless agent stores data)
- ❌ Complex configuration (keep it simple)

## Usage

1. Copy this template to create a new agent
2. Update `agent.yaml` with your agent's specifics
3. Replace the `run()` method in main.go with your agent's logic
4. Remove any components you don't need

## Communication Patterns

- **Most agents**: Unix sockets for local communication
- **Web agents**: Add HTTP server only if needed
- **Data agents**: Send all data to Claude Capture agent 
- **Monitor registration**: File-based registration (no Redis required)

## File Structure

```
agents/AGT-TYPE-VERSION__INSTANCE/
├── agent.yaml    # Minimal configuration
├── main.go       # Core agent logic
└── README.md     # Agent-specific documentation
```

## Key Principle

**Each agent gets exactly what it needs, nothing more.**

No bloat, no unnecessary dependencies, no over-engineering.
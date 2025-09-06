# Centerfire Intelligence

**Agent-First Autonomous Development Platform**

## Architecture

This system is built with agent-oriented architecture where specialized agents own domains completely:

- **AGT-BOOTSTRAP-1** - The genesis agent that creates all other agents  
- **AGT-NAMING-1** - Authority for all naming decisions
- **AGT-STRUCT-1** - Creates and manages directory structures
- **AGT-SEMDOC-1** - Semantic documentation management
- **AGT-CODING-1** - Code generation and analysis

## Directory Structure

```
/agents/                    # All system agents
  AGT-BOOTSTRAP-1__<ulid>/  # Bootstrap agent (Genesis)
  AGT-NAMING-1__<ulid>/     # Naming authority
  
/capabilities/              # Generated capabilities
  CAP-DOMAIN-1__<ulid>/     # Individual capabilities
  
/contracts/                 # System contracts and schemas

/DOC-ARCH-1__genesis/       # Architecture documentation
```

## Naming Convention

- **Capabilities**: `CAP-DOMAIN-N` (e.g., CAP-AUTH-1)
- **Agents**: `AGT-DOMAIN-N` (e.g., AGT-NAMING-1) 
- **Documentation**: `DOC-TYPE-N` (e.g., DOC-ARCH-1)
- **No zero padding**: Simple integers (1, 2, 10, not 001, 002, 010)

## Getting Started

1. Bootstrap the system:
```bash
cd agents/AGT-BOOTSTRAP-1__<ulid>
go run main.go bootstrap
```

2. Core agents will be created automatically
3. Use the system to build your first product

## Philosophy

Everything is an agent. Agents build agents. The system bootstraps itself from one manually created agent and becomes fully autonomous.

Built for the future of AI-native development.
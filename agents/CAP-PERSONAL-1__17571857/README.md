# CAP-PERSONAL-1 - Configurable Personal AI Orchestration Agent

**Semantic ID**: `CAP-PERSONAL-1__17571857`  
**CID**: `cid:centerfire:capability:17571857`

## Overview

Your personal AI agent that orchestrates multi-modal tasks across local LLMs, system commands, and knowledge systems. Configurable name (default: APOLLO), personality, and capabilities.

## Architecture

```
You → APOLLO → Task Orchestrator → [Commander|LocalLLM|Ollama|W/N] → Results → You
```

**Key Features**:
- **Configurable Identity**: Change name, personality, response style
- **Multi-Modal Orchestration**: System commands, file analysis, conversations, knowledge queries
- **Intelligent Routing**: Rule-based fast path + LLM decision engine
- **Memory System**: Conversation history and context preservation
- **Local-First**: Platform-independent using local models

## Configuration

Edit `config.yaml` to customize:

```yaml
agent_info:
  display_name: "APOLLO"  # Change to any name you want

personality:
  style: "technical_assistant"  # or "casual_friend", "professional"
  use_emojis: true

models:
  conversation_model: "llama2:latest"    # Your main chat model
  decision_model: "gemma:2b"             # Task routing (fallback: rule-based)
```

## Usage

```bash
# Start your personal agent
./apollo config.yaml

# Interactive terminal interface
You: check if redis is running
APOLLO: I'll check that for you! [calls Commander] ✅ Redis is running on port 6379

You: find files related to docker
APOLLO: Let me search for Docker-related files... [calls FileAnalyst] Found 15 Docker files in your project

You: what did we discuss about Redis yesterday?  
APOLLO: [searches conversation memory via W/N] We discussed Redis configuration and connection pooling...
```

## Orchestration Flow

1. **Input Analysis**: Rule matching or LLM decision
2. **Task Planning**: Single task or multi-step execution plan  
3. **Execution**: Parallel/sequential task processing
4. **Result Aggregation**: Combine responses intelligently
5. **Memory Storage**: Save conversation context

## Integration Endpoints

- **Commander**: `http://localhost:8090/api/agents/system` (system commands)
- **Local LLM**: `http://localhost:8090/api/agents/localllm` (file analysis, knowledge queries)
- **Ollama**: `http://localhost:11434/api/generate` (direct conversation model)
- **W/N Pipeline**: Redis streams for conversation memory

## Decision Rules (Fast Path)

- **System Commands**: "check if", "is running", "status" → Commander
- **File Operations**: "find files", "search for", "locate" → FileAnalyst  
- **Knowledge Queries**: "remember when", "what did we discuss" → KnowledgeCurator
- **Workflow Tasks**: "todo", "task", "organize" → WorkflowManager

## Commands

- `help` - Show available capabilities
- `exit` or `quit` - Terminate session

## Path to Claude Code Background Role

**Phase 1** (Current): Direct terminal interaction
- 80% of requests handled locally  
- Complex file operations → Claude Code

**Phase 2** (Next): Learned autonomy
- Pattern recognition from conversation history
- Proactive suggestions
- Claude Code for specialized coding tasks

**Phase 3** (Future): True digital assistant  
- Anticipates needs based on context
- Manages entire project lifecycle
- Claude Code becomes exception handler

## File Structure

```
CAP-PERSONAL-1__17571857/
├── config.yaml     # Agent configuration
├── main.go         # Core orchestration engine
├── apollo          # Compiled executable  
├── go.mod          # Go module definition
└── README.md       # This documentation
```

## Development

```bash
# Modify configuration
vim config.yaml

# Rebuild after changes
go build -o apollo main.go

# Test with different personalities
# Edit display_name: "JARVIS" or "FRIDAY" in config.yaml
```

## Integration Status

- ✅ **Terminal Interface**: Interactive conversation
- ✅ **Hierarchical Session Management**: `CAP-PERSONAL-1:{ulid8}` format
- ✅ **UTC Timestamp Tracking**: RFC3339 format for global consistency
- ✅ **W/N Pipeline Integration**: Session events → Neo4j, conversations → Weaviate
- ✅ **Singleton Enforcement**: AGT-MANAGER-1 collision prevention
- ✅ **Local LLM Integration**: Specialist task routing
- ✅ **Conversation Memory**: In-memory + persistent W/N streaming
- ✅ **Rule-Based Routing**: Fast pattern matching with system analysis
- ✅ **CI Awareness**: Dynamic agent discovery from protocol manifest
- ✅ **Commander Integration**: personal_agent contract established
- ✅ **Multi-Agent Orchestration**: Task routing to specialized CI agents
- 🚧 **Smart Decision Engine**: LLM-powered task analysis
- 🚧 **Multi-Step Orchestration**: Complex workflow execution

---

## Phase 2 Roadmap

**Session Analytics & Temporal Intelligence**:
- Cross-session memory queries ("what did we build yesterday?")
- Application build timeline analysis
- Agent collaboration pattern recognition
- Performance optimization via temporal data

**Enhanced Orchestration**:
- AGT-CONTRACT-MANAGER-1 for unified contract management across all agents
- Smart decision engine using gemma:2b for task analysis
- Multi-step workflow execution with dependency management
- Cross-agent communication optimization and dependency resolution

**CI Intelligence Enhancements**:
- Real-time agent status monitoring and health checks
- Dynamic capability discovery and load balancing
- Context-aware task routing based on agent performance
- Advanced orchestration patterns for complex workflows

**Next Steps**: Phase 1.5 complete! APOLLO is now CI-aware and ready for intelligent multi-agent orchestration with full session management and W/N streaming integration.
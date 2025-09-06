# Claude Code Conversation Proxy/Emulator - Long-term Solution

## Problem Statement

The current conversation capture system has a fundamental limitation: Claude Code's hook system (PostToolUse, SessionEnd) only provides technical context (git changes, file modifications, system state) but not the actual conversation content between user and assistant. This creates a "context gap" where the semantic AI system (Neo4j, Qdrant, Weaviate) receives rich technical metadata but misses the reasoning, discussions, and actual dialogue that drives development decisions.

## Current Quick Fix

**Transcript Monitor System** (`scripts/monitor-claude-transcripts.py`)
- Monitors `~/.claude/projects/*/` for session transcript files
- Parses JSONL transcript files to extract real conversation content
- Feeds actual user-assistant exchanges to semantic AI services
- Status: ✅ Implemented and working

## Long-term Vision: Conversation Proxy/Emulator

### Core Concept

Create a transparent layer between the user and Claude Code that acts as a conversation proxy, capturing everything bidirectionally while maintaining the native Claude Code experience.

### Architecture Overview

```
User ↔ Conversation Proxy ↔ Claude Code (in VM/Container)
                ↓
        Semantic AI System
        (Neo4j, Qdrant, etc.)
```

### Implementation Approaches

#### Option A: Network Proxy
- Intercept Claude Code's API calls to Anthropic's servers
- Capture requests/responses at network level
- Requires SSL certificate handling for HTTPS interception
- Most transparent to user experience

#### Option B: Input/Output Wrapper
- Wrapper around Claude Code binary that captures stdin/stdout
- Parse terminal interactions to extract conversations
- Handle Claude Code's interactive features (autocomplete, etc.)
- May impact user experience with input delays

#### Option C: Virtual Machine/Container
- Run Claude Code in controlled environment (VM/Docker)
- Monitor filesystem, network, and process interactions
- Complete isolation allows comprehensive monitoring
- Resource overhead but maximum capture capability

### Key Features

#### 1. Comprehensive Capture
- **Input**: All user messages, commands, file paths, context
- **Output**: All assistant responses, tool calls, results
- **Metadata**: Timestamps, session info, project context
- **State**: Working directory, git branch, environment

#### 2. Real-time Processing
- Stream conversation data to semantic AI services as it happens
- No reliance on Claude Code's hook system
- Immediate context availability for other tools/services

#### 3. Context Restoration
- Detect when Claude Code auto-compacts due to token limits
- Automatically inject relevant context from semantic AI services
- Seamless continuation without user intervention

#### 4. Multi-session Awareness
- Track conversation threads across multiple Claude Code sessions
- Maintain project context even with session breaks
- Link related conversations across time gaps

#### 5. Intelligent Task Routing
- **Local LLM First**: Route simple tasks to local models for fast processing
- **Claude Escalation**: Pass complex semantic analysis and problem-solving to Claude
- **Seamless Handoff**: No terminal window switching required
- **Context Preservation**: Full conversation context maintained across routing decisions
- **Autonomous Operation**: System determines appropriate routing without user intervention

### Technical Challenges

#### 1. Claude Code Integration
- **Challenge**: Maintaining native Claude Code experience
- **Solution**: Transparent proxy with minimal latency
- **Risk**: Breaking Claude Code's built-in features

#### 2. Port Management & Service Discovery
- **Challenge**: Port conflicts with system services and other applications
- **Current Issue**: Daemon uses fixed ports (8080-8081) causing conflicts with rapportd, adb, etc.
- **Solution**: Dynamic port allocation with persistent configuration
  - Scan for available ports during installation
  - Store port mappings in Redis or local config file  
  - Health checks read from config rather than guessing ports
  - Robust service discovery for all daemon components
- **Implementation**: Enhanced installation script with port allocation and tracking

#### 2. Authentication & Security
- **Challenge**: Handling Anthropic API authentication through proxy
- **Solution**: Secure credential passthrough or token delegation
- **Risk**: Exposing user credentials

#### 3. Performance Impact
- **Challenge**: Additional processing layer adds latency
- **Solution**: Async processing, minimal blocking operations
- **Risk**: Degraded user experience

#### 4. Version Compatibility
- **Challenge**: Claude Code updates may break proxy integration
- **Solution**: Version detection and adaptive parsing
- **Risk**: Maintenance overhead

### Implementation Phases

#### Phase 1: Proof of Concept
- [ ] Basic network proxy capturing HTTP/HTTPS traffic
- [ ] Parse Claude Code API requests/responses
- [ ] Extract conversation content from API payloads
- [ ] Test with simple conversations

#### Phase 2: Full Proxy
- [ ] Handle authentication and session management
- [ ] Support all Claude Code features (file editing, git operations)
- [ ] Real-time streaming to semantic AI services
- [ ] Error handling and fallback mechanisms

#### Phase 3: Context Restoration
- [ ] Auto-compact detection algorithms
- [ ] Context injection system
- [ ] Smart context selection (relevance scoring)
- [ ] User preference controls

#### Phase 4: Advanced Features
- [ ] Multi-session conversation threading
- [ ] Project-aware context management
- [ ] Performance optimizations
- [ ] Monitoring and analytics dashboard

### Alternative Approaches Considered

#### 1. Claude Code Modification
- **Pros**: Direct access to all conversation data
- **Cons**: Requires maintaining custom Claude Code fork
- **Verdict**: Too maintenance-heavy, breaks update path

#### 2. Terminal Session Recording
- **Pros**: Captures everything user sees
- **Cons**: Complex parsing, terminal formatting issues
- **Verdict**: Unreliable conversation extraction

#### 3. Filesystem Monitoring
- **Pros**: Simple implementation
- **Cons**: Only captures file changes, not conversations
- **Verdict**: Insufficient for conversation capture

### Success Metrics

#### Technical
- **Capture Rate**: >99% of conversations captured successfully
- **Latency**: <100ms additional delay per interaction
- **Reliability**: <0.1% failed conversation processing
- **Compatibility**: Works with latest Claude Code versions

#### User Experience
- **Transparency**: Users don't notice proxy layer
- **Context Quality**: Improved context restoration after auto-compact
- **Feature Parity**: All Claude Code features work normally
- **Performance**: No noticeable slowdown in typical usage

### Risk Mitigation

#### 1. Backup Systems
- If proxy fails, fall back to transcript monitoring
- Maintain existing hook-based technical context capture
- Queue failed conversation processing for retry

#### 2. Security Measures
- Encrypt all captured conversation data
- Secure credential handling and storage
- User consent and privacy controls

#### 3. Update Strategy
- Monitor Claude Code releases for compatibility
- Automated testing pipeline for new versions
- Graceful degradation when compatibility breaks

### Timeline Estimate

- **Phase 1**: 2-3 weeks (proof of concept)
- **Phase 2**: 1-2 months (full proxy implementation)
- **Phase 3**: 2-3 weeks (context restoration)
- **Phase 4**: 1-2 months (advanced features)

**Total: 3-4 months for complete implementation**

### Current Status

- ✅ Problem identified and documented
- ✅ Quick fix implemented (transcript monitoring)
- ✅ Architecture planned
- 🔄 Phase 1 ready to begin when resources available

---

*This document represents the long-term vision for solving the "conversation context gap" in the Centerfire Intelligence semantic AI system. The immediate need is addressed by the transcript monitoring system, but this proxy/emulator approach would provide a more robust and comprehensive solution.*
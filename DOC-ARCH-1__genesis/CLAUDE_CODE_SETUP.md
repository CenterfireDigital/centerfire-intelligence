# Claude Code Auto-Compaction Context Restoration

## Overview

This system automatically detects when Claude Code auto-compacts due to token limits and restores relevant context from project logs. This ensures seamless continuity even when conversations are interrupted by context limits.

## Setup Instructions

### 1. Enable Context Restoration Hook

Add this to your Claude Code settings to automatically inject context on startup:

```json
{
  "hooks": {
    "session_start": "python3 /Users/larrydiffey/projects/CenterfireIntelligence/scripts/claude-startup-context.py"
  }
}
```

### 2. Alternative: Manual Context Check

You can also manually check for and restore context using these commands:

```bash
# Check if context is available
curl -X POST http://localhost:8081/api/context/check \
  -H "Content-Type: application/json" \
  -d '{"session_id": "your_session_id", "working_dir": "/path/to/project"}'

# Restore context
curl -X POST http://localhost:8081/api/context/restore \
  -H "Content-Type: application/json" \
  -d '{"session_id": "your_session_id", "working_dir": "/path/to/project"}'
```

## How It Works

### Auto-Compaction Detection

The system detects auto-compaction events by monitoring:

1. **Session ID Changes**: New sessions starting in the same working directory
2. **Time Gaps**: Short gaps between session end and restart (<15 minutes)
3. **Token Thresholds**: Previous sessions that approached token limits (~1.5M+ tokens)
4. **Activity Patterns**: High conversation volume in previous session

### Context Restoration Process

1. **Detection**: When a new session starts, check if it follows an auto-compact pattern
2. **Log Analysis**: Read recent conversations from project-specific logs (`.centerfire/logs/`)
3. **Smart Summarization**: Generate concise context summary with key technical details
4. **Token Management**: Respect token budget (~50K tokens max for context)
5. **Emoji Restoration**: Automatically restore escaped emojis from logs

### Context Summary Format

The restored context includes:

```markdown
# 🔄 Context Restored for ProjectName
Auto-compaction detected. Restoring context from N recent conversations.

## Recent Activity Summary:
### Session abc123... (timestamp to timestamp)
- **Exchange 1**: Recent conversation summary...
- **Exchange 2**: Key technical details...
- *...and N more exchanges*

## Current Status:
- Working Directory: `/path/to/project`
- Project: ProjectName
- Context restored from N conversations

---
*This context was automatically restored after Claude Code auto-compaction.*
```

## Token Management

- **Token Estimation**: Uses 4 chars/token approximation for quick calculations
- **Context Budget**: Maximum 50K tokens for context restoration (configurable)
- **Smart Truncation**: Prioritizes most recent and relevant conversations
- **Emoji Optimization**: Restores escaped emojis to preserve formatting while managing tokens

## Configuration

### Environment Variables

```bash
# Optional: Override daemon port detection
export CENTERFIRE_DAEMON_PORT=8081

# Optional: Override session ID detection
export CLAUDE_SESSION_ID=custom_session_id

# Optional: Override context token budget
export CONTEXT_RESTORE_MAX_TOKENS=50000
```

### Project-Specific Settings

Each project automatically gets:

- `.centerfire/logs/conversations.jsonl` - Token-aware rotating conversation logs
- `.centerfire/.gitignore` - Excludes logs from version control
- Token-based rotation (default: ~2M tokens per file)
- Gzip compression of old log files

## API Endpoints

- `POST /api/context/check` - Check if context restoration is available
- `POST /api/context/restore` - Restore context for a session
- `GET /api/context/stats` - Get context restoration service statistics
- `POST /api/context/cleanup` - Clean up old session tracking data

## Troubleshooting

### Context Not Being Restored

1. **Check Daemon Status**: `~/.local/bin/centerfire-daemon status`
2. **Verify Logs Exist**: Look for `.centerfire/logs/conversations.jsonl` in project
3. **Check Hook Setup**: Ensure Claude Code hook is properly configured
4. **Manual Test**: Try manual API calls to verify service

### Performance Considerations

- Context restoration adds ~1-2 seconds to session startup
- Log files rotate automatically based on token count, not time
- Old session tracking data is cleaned up after 24 hours
- Emoji escaping reduces log file size by 20-40% for emoji-heavy conversations

### Privacy and Security

- Logs are stored locally in project directories
- Sensitive file patterns are automatically excluded
- Training-safe data isolation with namespace prefixes
- Logs are not committed to version control (via `.gitignore`)
- Context summaries respect token budgets to prevent overwhelming

## Integration with Claude Code Startup Preferences

This context restoration integrates seamlessly with the existing Claude Code startup preferences system. The hook runs before other startup scripts, ensuring context is available when needed.

Add to your `claude-startup-preferences.sh`:

```bash
# Auto-restore context if available
if [ -f "/Users/larrydiffey/projects/CenterfireIntelligence/scripts/claude-startup-context.py" ]; then
    python3 "/Users/larrydiffey/projects/CenterfireIntelligence/scripts/claude-startup-context.py"
fi
```
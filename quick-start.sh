#!/bin/bash

# Quick Startup for Claude Code Sessions
# Starts only the 3 essential working agents

set -e

PROJECT_ROOT="/Users/larrydiffey/projects/CenterfireIntelligence"
cd "$PROJECT_ROOT"

echo "🚀 Quick startup of essential Centerfire Intelligence agents..."

# Stop any existing agents
echo "🧹 Stopping existing agents..."
pkill -f "AGT-|gateway|manager" 2>/dev/null || true
sleep 1

# Start AGT-MANAGER-1 (use binary if available)
echo "📋 Starting AGT-MANAGER-1..."
cd agents/AGT-MANAGER-1__manager1
if [[ -f "manager" ]]; then
    ./manager &
    echo "   ✅ Started with binary"
else
    go run main.go &
    echo "   ✅ Started with Go"
fi
sleep 2

# Start AGT-HTTP-GATEWAY-1 (use binary)
echo "🌐 Starting AGT-HTTP-GATEWAY-1..."
cd "$PROJECT_ROOT/agents/AGT-HTTP-GATEWAY-1__01K4EAF1"
./gateway &
echo "   ✅ Started HTTP Gateway"
sleep 2

# Start AGT-CLAUDE-CAPTURE-1 (Python)
echo "🎯 Starting AGT-CLAUDE-CAPTURE-1..."
cd "$PROJECT_ROOT/agents/AGT-CLAUDE-CAPTURE-1__0368F157"
python3 main.py &
echo "   ✅ Started Claude Code capture"
sleep 2

# Start AGT-CONTEXT-1 (Go)
echo "🧠 Starting AGT-CONTEXT-1..."
cd "$PROJECT_ROOT/agents/AGT-CONTEXT-1__17572052"
go run main.go &
echo "   ✅ Started Context retrieval agent"
sleep 2

# Quick health check
echo ""
echo "🏥 Quick health check..."
if curl -s http://localhost:8090/api/system/health >/dev/null 2>&1; then
    echo "✅ System is healthy - HTTP Gateway responding"
    echo "📊 Dashboard: http://localhost:9191/"
    echo "🔗 Health API: curl http://localhost:8090/api/system/health | jq"
else
    echo "⚠️  Health check failed - agents may still be starting"
fi

echo ""
echo "🎉 Essential agents started!"
echo "   AGT-MANAGER-1: Agent management (8380)"
echo "   AGT-HTTP-GATEWAY-1: HTTP API (8090)" 
echo "   AGT-CLAUDE-CAPTURE-1: Session capture"
echo "   AGT-CONTEXT-1: Context retrieval"
echo ""
echo "   Use 'pkill -f \"AGT-|gateway\"' to stop all"
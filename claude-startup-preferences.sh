#!/bin/bash

# Claude Code Startup Health Check & Smart Agent Startup
# Starts critical agents if not running, but never restarts running agents

set -e

echo "🔍 Checking Centerfire Intelligence system health..."

PROJECT_ROOT="/Users/larrydiffey/projects/CenterfireIntelligence"
cd "$PROJECT_ROOT"

# Quick health check first
if curl -s http://localhost:8090/api/system/health > /dev/null 2>&1; then
    echo "✅ System already healthy - all essential agents running"
    echo "📊 Dashboard: http://localhost:9191/"
    exit 0
fi

# System is unhealthy - start missing critical agents (no restarts)
echo "🚀 System unhealthy - starting missing critical agents..."
echo "⚠️  Note: Will NOT restart already running agents"

# DON'T use quick-start.sh - it kills existing agents
# Instead use smart manual startup that checks each agent
echo "🧠 Using smart startup - checking each agent individually..."

# Smart startup - check each agent before starting

# AGT-MANAGER-1 (check if already running)
if ! pgrep -f "AGT-MANAGER-1" > /dev/null; then
    echo "📋 Starting AGT-MANAGER-1..."
    cd agents/AGT-MANAGER-1__manager1 && ./manager &
    sleep 2
    cd "$PROJECT_ROOT"
else
    echo "📋 AGT-MANAGER-1 already running - skipping"
fi

# AGT-HTTP-GATEWAY-1 (check if already running)
if ! pgrep -f "gateway" > /dev/null; then
    echo "🌐 Starting AGT-HTTP-GATEWAY-1..."  
    cd agents/AGT-HTTP-GATEWAY-1__01K4EAF1 && ./gateway &
    sleep 2
    cd "$PROJECT_ROOT"
else
    echo "🌐 AGT-HTTP-GATEWAY-1 already running - skipping"
fi

# AGT-CLAUDE-CAPTURE-1 (check if already running)
if ! pgrep -f "AGT-CLAUDE-CAPTURE-1" > /dev/null; then
    echo "🎯 Starting AGT-CLAUDE-CAPTURE-1..."
    cd agents/AGT-CLAUDE-CAPTURE-1__0368F157 && python3 main.py &
    sleep 2
    cd "$PROJECT_ROOT"
else
    echo "🎯 AGT-CLAUDE-CAPTURE-1 already running - skipping"
fi

# AGT-CONTEXT-1 (check if already running)
if ! pgrep -f "AGT-CONTEXT-1" > /dev/null; then
    echo "🧠 Starting AGT-CONTEXT-1..."
    cd agents/AGT-CONTEXT-1__17572052 && go run main.go &
    sleep 2
    cd "$PROJECT_ROOT"
else
    echo "🧠 AGT-CONTEXT-1 already running - skipping"
fi

echo ""
echo "🎉 Critical agent startup complete!"
echo "📊 Health Dashboard: http://localhost:9191/"
echo "🔗 Health API: curl http://localhost:8090/api/system/health | jq"
echo ""
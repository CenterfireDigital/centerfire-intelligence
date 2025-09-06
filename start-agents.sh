#!/bin/bash

# Centerfire Intelligence Agent Startup Script
# Starts core persistent agents for the socket-based orchestrator

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_DIR="$SCRIPT_DIR/agents"

# Core persistent agents (order matters for dependencies)
CORE_AGENTS=(
    "AGT-MANAGER-1__manager1"
    "AGT-NAMING-1__01K4EAF1" 
    "AGT-STRUCT-1__01K4EAF1"
    "AGT-SEMANTIC-1__01K4EAF1"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to check if an agent is already running
check_agent_running() {
    local agent_name="$1"
    # Check for running go processes with this agent's path
    if ps aux | grep -v grep | grep -q "go run.*$agent_name.*main.go"; then
        return 0  # Running
    else
        return 1  # Not running
    fi
}

# Function to start a single agent
start_agent() {
    local agent_dir="$1"
    local agent_name=$(basename "$agent_dir")
    
    log "Starting agent: $agent_name"
    
    if check_agent_running "$agent_name"; then
        warn "Agent $agent_name is already running"
        return 0
    fi
    
    if [[ ! -d "$AGENTS_DIR/$agent_dir" ]]; then
        error "Agent directory not found: $AGENTS_DIR/$agent_dir"
        return 1
    fi
    
    if [[ ! -f "$AGENTS_DIR/$agent_dir/main.go" ]]; then
        error "No main.go found in $AGENTS_DIR/$agent_dir"
        return 1
    fi
    
    # Change to agent directory and start in background
    cd "$AGENTS_DIR/$agent_dir"
    
    # Start agent with output redirection
    log "Executing: go run main.go (background)"
    nohup go run main.go > "/tmp/$agent_name.log" 2>&1 &
    local pid=$!
    
    # Give it a moment to start
    sleep 2
    
    # Check if it's still running
    if kill -0 $pid 2>/dev/null; then
        success "Agent $agent_name started (PID: $pid)"
        echo "$pid" > "/tmp/$agent_name.pid"
    else
        error "Agent $agent_name failed to start"
        error "Check log: /tmp/$agent_name.log"
        return 1
    fi
    
    cd "$SCRIPT_DIR"
}

# Function to stop all agents
stop_agents() {
    log "Stopping all agents..."
    
    for agent_dir in "${CORE_AGENTS[@]}"; do
        local agent_name=$(basename "$agent_dir")
        local pid_file="/tmp/$agent_name.pid"
        
        if [[ -f "$pid_file" ]]; then
            local pid=$(cat "$pid_file")
            if kill -0 "$pid" 2>/dev/null; then
                kill "$pid"
                success "Stopped agent $agent_name (PID: $pid)"
            fi
            rm -f "$pid_file"
        fi
        
        # Fallback: kill by process name
        if pgrep -f "agents/$agent_dir/main.go" > /dev/null; then
            pkill -f "agents/$agent_dir/main.go"
            log "Killed remaining processes for $agent_name"
        fi
    done
    
    success "All agents stopped"
}

# Function to show agent status
status_agents() {
    log "Agent Status:"
    echo
    
    local all_running=true
    
    for agent_dir in "${CORE_AGENTS[@]}"; do
        local agent_name=$(basename "$agent_dir")
        
        if check_agent_running "$agent_name"; then
            local pid=$(ps aux | grep -v grep | grep "go run.*$agent_name.*main.go" | awk '{print $2}' | head -1)
            success "Agent $agent_name: RUNNING (PID: $pid)"
        else
            error "Agent $agent_name: STOPPED"
            all_running=false
        fi
    done
    
    echo
    if $all_running; then
        success "All core agents are running"
    else
        warn "Some agents are not running"
    fi
    
    # Show orchestrator status
    if pgrep -f "orchestrator-go/main.go" > /dev/null; then
        success "Orchestrator: RUNNING"
    else
        warn "Orchestrator: NOT RUNNING"
    fi
}

# Function to check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        error "Go is not installed or not in PATH"
        exit 1
    fi
    success "Go found: $(go version)"
    
    # Check if Redis is running
    if ! docker ps --format "table {{.Names}}\t{{.Status}}" | grep -q "mem0-redis.*Up"; then
        error "Redis container (mem0-redis) is not running"
        error "Start it with: docker start mem0-redis"
        exit 1
    fi
    success "Redis container is running"
    
    # Check if orchestrator is running
    if ! pgrep -f "orchestrator-go/main.go" > /dev/null; then
        warn "Orchestrator is not running"
        warn "Start it with: cd orchestrator-go && go run main.go &"
    else
        success "Orchestrator is running"
    fi
}

# Main execution
case "${1:-start}" in
    "start")
        log "🚀 Starting Centerfire Intelligence Agents"
        check_prerequisites
        
        for agent_dir in "${CORE_AGENTS[@]}"; do
            start_agent "$agent_dir"
            sleep 1  # Brief pause between starts
        done
        
        echo
        success "🎉 All core agents startup initiated"
        log "💡 Use './start-agents.sh status' to check agent health"
        log "💡 Use './start-agents.sh stop' to stop all agents"
        ;;
        
    "stop")
        stop_agents
        ;;
        
    "restart")
        stop_agents
        sleep 2
        exec "$0" start
        ;;
        
    "status")
        status_agents
        ;;
        
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        echo
        echo "Commands:"
        echo "  start   - Start all core agents"
        echo "  stop    - Stop all agents"
        echo "  restart - Restart all agents" 
        echo "  status  - Show agent status"
        exit 1
        ;;
esac
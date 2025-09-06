#!/bin/bash

# Centerfire Learning Stream Pipeline Manager
# Manages Redis Streams to Weaviate/Neo4j pipeline with health checks and graceful shutdown

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PRODUCER_DIR="$SCRIPT_DIR/producer"
WEAVIATE_CONSUMER_DIR="$SCRIPT_DIR/weaviate-consumer"
NEO4J_CONSUMER_DIR="$SCRIPT_DIR/neo4j-consumer"

# Process tracking
PRODUCER_PID=""
WEAVIATE_CONSUMER_PID=""
NEO4J_CONSUMER_PID=""

# Health check configuration
HEALTH_CHECK_INTERVAL=30
MAX_HEALTH_CHECK_FAILURES=3
PRODUCER_HEALTH_URL="http://localhost:8080/health"
WEAVIATE_HEALTH_URL="http://localhost:8081/health"
NEO4J_HEALTH_URL="http://localhost:8082/health"

# Log file paths
LOG_DIR="$SCRIPT_DIR/logs"
PRODUCER_LOG="$LOG_DIR/producer.log"
WEAVIATE_LOG="$LOG_DIR/weaviate-consumer.log"
NEO4J_LOG="$LOG_DIR/neo4j-consumer.log"
PIPELINE_LOG="$LOG_DIR/pipeline.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${timestamp} [${level}] ${message}" | tee -a "$PIPELINE_LOG"
}

log_info() {
    log "INFO" "${GREEN}$*${NC}"
}

log_warn() {
    log "WARN" "${YELLOW}$*${NC}"
}

log_error() {
    log "ERROR" "${RED}$*${NC}"
}

log_debug() {
    log "DEBUG" "${BLUE}$*${NC}"
}

# Setup function
setup() {
    log_info "Setting up Centerfire Learning Stream Pipeline..."
    
    # Create logs directory
    mkdir -p "$LOG_DIR"
    
    # Initialize log files
    touch "$PRODUCER_LOG" "$WEAVIATE_LOG" "$NEO4J_LOG" "$PIPELINE_LOG"
    
    # Check if Go modules are ready
    if [ ! -f "$PRODUCER_DIR/go.mod" ]; then
        log_error "Producer Go module not found. Run 'go mod init' in $PRODUCER_DIR"
        exit 1
    fi
    
    if [ ! -f "$WEAVIATE_CONSUMER_DIR/go.mod" ]; then
        log_error "Weaviate consumer Go module not found. Run 'go mod init' in $WEAVIATE_CONSUMER_DIR"
        exit 1
    fi
    
    if [ ! -f "$NEO4J_CONSUMER_DIR/go.mod" ]; then
        log_error "Neo4j consumer Go module not found. Run 'go mod init' in $NEO4J_CONSUMER_DIR"
        exit 1
    fi
    
    log_info "Setup complete"
}

# Start individual components
start_producer() {
    log_info "Starting stream producer..."
    cd "$PRODUCER_DIR"
    nohup go run main.go > "$PRODUCER_LOG" 2>&1 &
    PRODUCER_PID=$!
    cd - > /dev/null
    log_info "Producer started with PID: $PRODUCER_PID"
    
    # Wait a moment for startup
    sleep 2
}

start_weaviate_consumer() {
    log_info "Starting Weaviate consumer..."
    cd "$WEAVIATE_CONSUMER_DIR"
    nohup go run main.go > "$WEAVIATE_LOG" 2>&1 &
    WEAVIATE_CONSUMER_PID=$!
    cd - > /dev/null
    log_info "Weaviate consumer started with PID: $WEAVIATE_CONSUMER_PID"
    
    # Wait a moment for startup
    sleep 2
}

start_neo4j_consumer() {
    log_info "Starting Neo4j consumer..."
    cd "$NEO4J_CONSUMER_DIR"
    nohup go run main.go > "$NEO4J_LOG" 2>&1 &
    NEO4J_CONSUMER_PID=$!
    cd - > /dev/null
    log_info "Neo4j consumer started with PID: $NEO4J_CONSUMER_PID"
    
    # Wait a moment for startup
    sleep 2
}

# Health check functions
check_service_health() {
    local service_name="$1"
    local health_url="$2"
    local max_retries=3
    local retry_delay=5
    
    for ((i=1; i<=max_retries; i++)); do
        if curl -f -s "$health_url" > /dev/null 2>&1; then
            log_debug "$service_name health check passed"
            return 0
        else
            log_warn "$service_name health check failed (attempt $i/$max_retries)"
            if [ $i -lt $max_retries ]; then
                sleep $retry_delay
            fi
        fi
    done
    
    log_error "$service_name health check failed after $max_retries attempts"
    return 1
}

perform_health_checks() {
    local failures=0
    
    # Check producer
    if [ -n "$PRODUCER_PID" ] && kill -0 "$PRODUCER_PID" 2>/dev/null; then
        if ! check_service_health "Producer" "$PRODUCER_HEALTH_URL"; then
            ((failures++))
        fi
    else
        log_error "Producer process not running (PID: $PRODUCER_PID)"
        ((failures++))
    fi
    
    # Check Weaviate consumer
    if [ -n "$WEAVIATE_CONSUMER_PID" ] && kill -0 "$WEAVIATE_CONSUMER_PID" 2>/dev/null; then
        if ! check_service_health "Weaviate Consumer" "$WEAVIATE_HEALTH_URL"; then
            ((failures++))
        fi
    else
        log_error "Weaviate consumer process not running (PID: $WEAVIATE_CONSUMER_PID)"
        ((failures++))
    fi
    
    # Check Neo4j consumer
    if [ -n "$NEO4J_CONSUMER_PID" ] && kill -0 "$NEO4J_CONSUMER_PID" 2>/dev/null; then
        if ! check_service_health "Neo4j Consumer" "$NEO4J_HEALTH_URL"; then
            ((failures++))
        fi
    else
        log_error "Neo4j consumer process not running (PID: $NEO4J_CONSUMER_PID)"
        ((failures++))
    fi
    
    if [ $failures -eq 0 ]; then
        log_info "All services healthy"
    else
        log_warn "$failures service(s) failing health checks"
    fi
    
    return $failures
}

# Get service statistics
get_service_stats() {
    log_info "=== Service Statistics ==="
    
    # Producer stats
    if curl -f -s "http://localhost:8080/stats" > /dev/null 2>&1; then
        log_info "Producer Stats:"
        curl -s "http://localhost:8080/stats" | jq '.' 2>/dev/null || curl -s "http://localhost:8080/stats"
    fi
    
    # Weaviate consumer stats
    if curl -f -s "http://localhost:8081/stats" > /dev/null 2>&1; then
        log_info "Weaviate Consumer Stats:"
        curl -s "http://localhost:8081/stats" | jq '.' 2>/dev/null || curl -s "http://localhost:8081/stats"
    fi
    
    # Neo4j consumer stats
    if curl -f -s "http://localhost:8082/stats" > /dev/null 2>&1; then
        log_info "Neo4j Consumer Stats:"
        curl -s "http://localhost:8082/stats" | jq '.' 2>/dev/null || curl -s "http://localhost:8082/stats"
    fi
    
    log_info "=========================="
}

# Graceful shutdown
shutdown_services() {
    log_info "Shutting down pipeline services..."
    
    # Send SIGTERM to all processes
    if [ -n "$PRODUCER_PID" ] && kill -0 "$PRODUCER_PID" 2>/dev/null; then
        log_info "Stopping producer (PID: $PRODUCER_PID)..."
        kill -TERM "$PRODUCER_PID" 2>/dev/null || true
    fi
    
    if [ -n "$WEAVIATE_CONSUMER_PID" ] && kill -0 "$WEAVIATE_CONSUMER_PID" 2>/dev/null; then
        log_info "Stopping Weaviate consumer (PID: $WEAVIATE_CONSUMER_PID)..."
        kill -TERM "$WEAVIATE_CONSUMER_PID" 2>/dev/null || true
    fi
    
    if [ -n "$NEO4J_CONSUMER_PID" ] && kill -0 "$NEO4J_CONSUMER_PID" 2>/dev/null; then
        log_info "Stopping Neo4j consumer (PID: $NEO4J_CONSUMER_PID)..."
        kill -TERM "$NEO4J_CONSUMER_PID" 2>/dev/null || true
    fi
    
    # Wait for graceful shutdown
    log_info "Waiting for graceful shutdown..."
    sleep 10
    
    # Force kill if still running
    for pid in "$PRODUCER_PID" "$WEAVIATE_CONSUMER_PID" "$NEO4J_CONSUMER_PID"; do
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            log_warn "Force killing process $pid"
            kill -KILL "$pid" 2>/dev/null || true
        fi
    done
    
    log_info "All services stopped"
}

# Signal handlers
handle_sigterm() {
    log_info "Received SIGTERM, initiating graceful shutdown..."
    shutdown_services
    exit 0
}

handle_sigint() {
    log_info "Received SIGINT, initiating graceful shutdown..."
    shutdown_services
    exit 0
}

# Test event publishing
test_pipeline() {
    log_info "Testing pipeline with sample event..."
    
    # Wait for services to be ready
    sleep 5
    
    # Publish test event
    if curl -f -s -X POST "http://localhost:8080/test/publish" > /dev/null 2>&1; then
        log_info "Test event published successfully"
        
        # Wait a moment for processing
        sleep 3
        
        # Show stats after test
        get_service_stats
    else
        log_error "Failed to publish test event"
        return 1
    fi
}

# Main execution
main() {
    local command="${1:-start}"
    
    case "$command" in
        "start")
            setup
            
            # Set up signal handlers
            trap handle_sigterm SIGTERM
            trap handle_sigint SIGINT
            
            # Start all services
            start_producer
            start_weaviate_consumer
            start_neo4j_consumer
            
            # Initial health check
            log_info "Performing initial health checks..."
            sleep 10  # Give services time to fully start
            
            if perform_health_checks; then
                log_info "Initial health checks passed"
            else
                log_warn "Some initial health checks failed, but continuing..."
            fi
            
            # Show startup information
            log_info "=== Pipeline Started ==="
            log_info "Producer PID: $PRODUCER_PID (http://localhost:8080)"
            log_info "Weaviate Consumer PID: $WEAVIATE_CONSUMER_PID (http://localhost:8081)"
            log_info "Neo4j Consumer PID: $NEO4J_CONSUMER_PID (http://localhost:8082)"
            log_info "Logs: $LOG_DIR/"
            log_info "======================="
            
            # Health check loop
            local health_failure_count=0
            
            while true; do
                sleep $HEALTH_CHECK_INTERVAL
                
                if perform_health_checks; then
                    health_failure_count=0
                else
                    ((health_failure_count++))
                    
                    if [ $health_failure_count -ge $MAX_HEALTH_CHECK_FAILURES ]; then
                        log_error "Max health check failures reached. Shutting down pipeline."
                        shutdown_services
                        exit 1
                    fi
                fi
            done
            ;;
        "stop")
            log_info "Stopping pipeline..."
            # Find and stop running processes
            pkill -f "go run.*producer" || true
            pkill -f "go run.*weaviate-consumer" || true
            pkill -f "go run.*neo4j-consumer" || true
            log_info "Pipeline stopped"
            ;;
        "status")
            log_info "Pipeline Status:"
            perform_health_checks
            get_service_stats
            ;;
        "test")
            test_pipeline
            ;;
        "logs")
            log_info "=== Recent Pipeline Logs ==="
            tail -n 20 "$PIPELINE_LOG" 2>/dev/null || echo "No pipeline logs found"
            ;;
        "help"|*)
            echo "Centerfire Learning Stream Pipeline Manager"
            echo ""
            echo "Usage: $0 [command]"
            echo ""
            echo "Commands:"
            echo "  start   - Start the complete pipeline (default)"
            echo "  stop    - Stop all pipeline services"
            echo "  status  - Check status and show statistics"
            echo "  test    - Test pipeline with sample event"
            echo "  logs    - Show recent pipeline logs"
            echo "  help    - Show this help message"
            echo ""
            echo "The pipeline consists of:"
            echo "  - Redis Stream Producer (port 8080)"
            echo "  - Weaviate Consumer (port 8081)"
            echo "  - Neo4j Consumer (port 8082)"
            ;;
    esac
}

# Run main function with all arguments
main "$@"
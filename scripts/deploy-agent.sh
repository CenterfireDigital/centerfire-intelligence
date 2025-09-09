#!/bin/bash
# 5D Agent Deployment Script
# Usage: ./deploy-agent.sh <agent-name>
# Future: This will be called by distributed orchestration layer

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load deployment library
source "$SCRIPT_DIR/5d-deployment-lib.sh"

# Usage function
usage() {
    echo "Usage: $0 <agent-name>"
    echo ""
    echo "Examples:"
    echo "  $0 AGT-STRUCT-2__struct2"
    echo "  $0 AGT-SEMANTIC-2__semantic2"
    echo ""
    echo "5D Deployment Pipeline - Military-grade agent transitions"
    exit 1
}

# Check arguments
if [[ $# -ne 1 ]]; then
    usage
fi

AGENT_NAME=$1

# Header
echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    5D DEPLOYMENT PIPELINE                    ║${NC}"
echo -e "${BLUE}║                Military-Grade Agent Deployment               ║${NC}"
echo -e "${BLUE}║              (Future: Distributed Orchestration)             ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}🎯 TARGET:${NC} $AGENT_NAME"
echo -e "${YELLOW}📁 PROJECT:${NC} $PROJECT_DIR"
echo -e "${YELLOW}⏰ TIMESTAMP:${NC} $(date)"
echo ""

# Change to project directory
cd "$PROJECT_DIR"

# Execute deployment
echo -e "${BLUE}🚀 EXECUTING 5D DEPLOYMENT${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if deploy_agent_from_vps "$AGENT_NAME"; then
    echo ""
    echo -e "${GREEN}🎉 DEPLOYMENT SUCCESSFUL: $AGENT_NAME${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}❌ DEPLOYMENT FAILED: $AGENT_NAME${NC}"
    exit 1
fi
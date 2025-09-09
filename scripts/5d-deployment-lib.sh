#!/bin/bash
# 5D Deployment Pipeline Library
# Military-grade deployment functions for agent transitions

# Load configuration
load_config() {
    if [[ ! -f "deployment.env" ]]; then
        echo "❌ ERROR: deployment.env not found"
        exit 1
    fi
    source deployment.env
    
    # Create temp directory
    mkdir -p "$TEMP_DIR"
    
    if [[ "$VERBOSE" == "true" ]]; then
        echo "✅ Configuration loaded"
        echo "   VPS: ${VPS_USER}@${VPS_HOST}"
        echo "   Local: ${LOCAL_PROJECT_PATH}"
    fi
}

# Logging function
log() {
    local level=$1
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    echo "[$timestamp] [$level] $message" | tee -a "$DEPLOY_LOG"
}

# Test VPS connection
test_vps_connection() {
    log "INFO" "Testing VPS connection..."
    
    if ssh -o ConnectTimeout=10 -o BatchMode=yes "${VPS_USER}@${VPS_HOST}" "echo 'Connection test successful'" >/dev/null 2>&1; then
        log "SUCCESS" "VPS connection established"
        return 0
    else
        log "ERROR" "VPS connection failed - may need password authentication"
        return 1
    fi
}

# Create deployment package on VPS
create_vps_package() {
    local agent_name=$1
    local package_name="${agent_name}-validated-$(date +%Y%m%d-%H%M%S).tar.gz"
    
    log "INFO" "Creating deployment package: $package_name"
    
    # Create package on VPS
    ssh "${VPS_USER}@${VPS_HOST}" "
        cd ${VPS_AGENTS_PATH}/${agent_name} && \
        tar -czf /tmp/${package_name} \
        --exclude='*.pid' \
        --exclude='*.health' \
        --exclude='*.log' \
        --exclude='capabilities' \
        --exclude='modules' \
        .
    " 2>/dev/null
    
    if [[ $? -eq 0 ]]; then
        log "SUCCESS" "Package created: $package_name"
        echo "$package_name"
        return 0
    else
        log "ERROR" "Failed to create VPS package"
        return 1
    fi
}

# Transfer package from VPS
transfer_package() {
    local package_name=$1
    local local_package="${TEMP_DIR}/${package_name}"
    
    log "INFO" "Transferring package: $package_name"
    
    # Transfer package
    scp "${VPS_USER}@${VPS_HOST}:/tmp/${package_name}" "$local_package" 2>/dev/null
    
    if [[ $? -eq 0 ]] && [[ -f "$local_package" ]]; then
        # Cleanup VPS temp file
        ssh "${VPS_USER}@${VPS_HOST}" "rm -f /tmp/${package_name}" 2>/dev/null
        
        log "SUCCESS" "Package transferred: $(ls -lh $local_package | awk '{print $5}')"
        echo "$local_package"
        return 0
    else
        log "ERROR" "Package transfer failed"
        return 1
    fi
}

# Backup current agent
backup_agent() {
    local agent_path=$1
    local backup_path="${agent_path}${BACKUP_SUFFIX}"
    
    if [[ -d "$agent_path" ]]; then
        log "INFO" "Backing up current agent..."
        cp -r "$agent_path" "$backup_path"
        
        if [[ $? -eq 0 ]]; then
            log "SUCCESS" "Backup created: $backup_path"
            echo "$backup_path"
            return 0
        else
            log "ERROR" "Backup failed"
            return 1
        fi
    else
        log "INFO" "No existing agent to backup"
        echo ""
        return 0
    fi
}

# Deploy package
deploy_package() {
    local package_path=$1
    local target_path=$2
    
    log "INFO" "Deploying package to: $target_path"
    
    # Ensure target directory exists
    mkdir -p "$target_path"
    
    # Extract package
    tar -xzf "$package_path" -C "$target_path"
    
    if [[ $? -eq 0 ]]; then
        # Make binaries executable
        find "$target_path" -name "agt-*" -type f -exec chmod +x {} \; 2>/dev/null
        find "$target_path" -name "*.sh" -type f -exec chmod +x {} \; 2>/dev/null
        
        log "SUCCESS" "Package deployed successfully"
        return 0
    else
        log "ERROR" "Package deployment failed"
        return 1
    fi
}

# Main deployment function
deploy_agent_from_vps() {
    local agent_name=$1
    local target_path="${LOCAL_AGENTS_PATH}/${agent_name}"
    
    log "INFO" "🚀 Starting 5D deployment: $agent_name"
    
    # Load configuration
    load_config
    
    # Create backup
    backup_path=$(backup_agent "$target_path")
    
    # Create VPS package
    package_name=$(create_vps_package "$agent_name")
    if [[ $? -ne 0 ]]; then
        log "ERROR" "Deployment aborted - package creation failed"
        return 1
    fi
    
    # Transfer package
    local_package=$(transfer_package "$package_name")
    if [[ $? -ne 0 ]]; then
        log "ERROR" "Deployment aborted - package transfer failed"
        return 1
    fi
    
    # Deploy package
    if ! deploy_package "$local_package" "$target_path"; then
        log "ERROR" "Deployment failed"
        return 1
    fi
    
    # Success - remove backup
    if [[ -n "$backup_path" ]] && [[ -d "$backup_path" ]]; then
        rm -rf "$backup_path"
        log "INFO" "Backup removed - deployment successful"
    fi
    
    # Cleanup
    rm -f "$local_package"
    
    log "SUCCESS" "🎉 5D deployment completed successfully: $agent_name"
    return 0
}
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// ClientContract represents a SemDoc access contract
type ClientContract struct {
	ClientID    string    `yaml:"client_id"`
	Version     string    `yaml:"version"`
	Created     string    `yaml:"created"`
	Description string    `yaml:"description"`
	
	AccessPermissions AccessPermissions `yaml:"access_permissions"`
	RateLimits       RateLimits        `yaml:"rate_limits"`
	Security         SecuritySettings  `yaml:"security"`
	Protocol         ProtocolSettings  `yaml:"protocol"`
	Monitoring       MonitoringSettings `yaml:"monitoring"`
}

// AccessPermissions defines what agents and actions are allowed
type AccessPermissions struct {
	AllowedAgents   map[string]AgentPermissions `yaml:"allowed_agents"`
	ForbiddenAgents []string                   `yaml:"forbidden_agents"`
}

// AgentPermissions defines allowed actions for a specific agent
type AgentPermissions struct {
	Actions     []string `yaml:"actions"`
	Description string   `yaml:"description"`
}

// RateLimits defines request rate limiting
type RateLimits struct {
	RequestsPerMinute   int `yaml:"requests_per_minute"`
	BurstLimit         int `yaml:"burst_limit"`
	ConcurrentRequests int `yaml:"concurrent_requests"`
}

// SecuritySettings defines authentication and authorization
type SecuritySettings struct {
	Authentication string `yaml:"authentication"`
	Authorization  string `yaml:"authorization"`
	RequireHTTPS   bool   `yaml:"require_https"`
	CORSEnabled    bool   `yaml:"cors_enabled,omitempty"`
}

// ProtocolSettings defines communication protocol details
type ProtocolSettings struct {
	RequestFormat  string `yaml:"request_format"`
	ResponseFormat string `yaml:"response_format"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	RetryAttempts  int    `yaml:"retry_attempts"`
}

// MonitoringSettings defines logging and metrics
type MonitoringSettings struct {
	LogRequests      bool `yaml:"log_requests"`
	LogResponses     bool `yaml:"log_responses"`
	TrackUsageMetrics bool `yaml:"track_usage_metrics"`
}

// ContractValidator manages client contracts and validates requests
type ContractValidator struct {
	contracts    map[string]*ClientContract
	contractsDir string
}

// ValidationError represents a contract validation error
type ValidationError struct {
	ClientID string
	Agent    string
	Action   string
	Reason   string
}

func (ve *ValidationError) Error() string {
	return fmt.Sprintf("Contract violation for client %s: %s (agent: %s, action: %s)", 
		ve.ClientID, ve.Reason, ve.Agent, ve.Action)
}

// NewContractValidator creates a new contract validator
func NewContractValidator(contractsDir string) *ContractValidator {
	return &ContractValidator{
		contracts:    make(map[string]*ClientContract),
		contractsDir: contractsDir,
	}
}

// LoadContracts loads all contracts from the contracts directory
func (cv *ContractValidator) LoadContracts() error {
	fmt.Printf("📋 Loading contracts from: %s\n", cv.contractsDir)
	
	if _, err := os.Stat(cv.contractsDir); os.IsNotExist(err) {
		return fmt.Errorf("contracts directory does not exist: %s", cv.contractsDir)
	}
	
	files, err := ioutil.ReadDir(cv.contractsDir)
	if err != nil {
		return fmt.Errorf("failed to read contracts directory: %v", err)
	}
	
	contractsLoaded := 0
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") && !strings.HasSuffix(file.Name(), ".yml") {
			continue
		}
		
		contractPath := filepath.Join(cv.contractsDir, file.Name())
		if err := cv.loadContract(contractPath); err != nil {
			fmt.Printf("❌ Failed to load contract %s: %v\n", file.Name(), err)
			continue
		}
		contractsLoaded++
	}
	
	fmt.Printf("✅ Loaded %d contracts\n", contractsLoaded)
	return nil
}

// loadContract loads a single contract file
func (cv *ContractValidator) loadContract(contractPath string) error {
	data, err := ioutil.ReadFile(contractPath)
	if err != nil {
		return fmt.Errorf("failed to read contract file: %v", err)
	}
	
	var contract ClientContract
	if err := yaml.Unmarshal(data, &contract); err != nil {
		return fmt.Errorf("failed to parse contract YAML: %v", err)
	}
	
	if contract.ClientID == "" {
		return fmt.Errorf("contract missing client_id")
	}
	
	cv.contracts[contract.ClientID] = &contract
	fmt.Printf("📄 Loaded contract for client: %s (version: %s)\n", contract.ClientID, contract.Version)
	return nil
}

// ValidateRequest validates a request against the client's contract
func (cv *ContractValidator) ValidateRequest(clientID, agent, action string) error {
	contract, exists := cv.contracts[clientID]
	if !exists {
		return &ValidationError{
			ClientID: clientID,
			Agent:    agent,
			Action:   action,
			Reason:   "no contract found for client",
		}
	}
	
	// Check forbidden agents
	for _, forbiddenAgent := range contract.AccessPermissions.ForbiddenAgents {
		if forbiddenAgent == agent {
			return &ValidationError{
				ClientID: clientID,
				Agent:    agent,
				Action:   action,
				Reason:   "access to agent is forbidden",
			}
		}
	}
	
	// Check allowed agents and actions
	agentPermissions, exists := contract.AccessPermissions.AllowedAgents[agent]
	if !exists {
		return &ValidationError{
			ClientID: clientID,
			Agent:    agent,
			Action:   action,
			Reason:   "agent not in allowed list",
		}
	}
	
	// Check specific action permissions
	if !cv.isActionAllowed(agentPermissions.Actions, action) {
		return &ValidationError{
			ClientID: clientID,
			Agent:    agent,
			Action:   action,
			Reason:   "action not permitted for this agent",
		}
	}
	
	return nil
}

// isActionAllowed checks if an action is permitted
func (cv *ContractValidator) isActionAllowed(allowedActions []string, action string) bool {
	for _, allowedAction := range allowedActions {
		if allowedAction == "*" || allowedAction == action {
			return true
		}
	}
	return false
}

// GetClientContract returns the contract for a specific client
func (cv *ContractValidator) GetClientContract(clientID string) (*ClientContract, bool) {
	contract, exists := cv.contracts[clientID]
	return contract, exists
}

// GetAllowedAgents returns the list of agents a client can access
func (cv *ContractValidator) GetAllowedAgents(clientID string) map[string]AgentPermissions {
	contract, exists := cv.contracts[clientID]
	if !exists {
		return make(map[string]AgentPermissions)
	}
	return contract.AccessPermissions.AllowedAgents
}

// GetContractInfo returns contract information for API responses
func (cv *ContractValidator) GetContractInfo(clientID string) map[string]interface{} {
	contract, exists := cv.contracts[clientID]
	if !exists {
		return map[string]interface{}{
			"error": "Contract not found",
		}
	}
	
	return map[string]interface{}{
		"client_id":        contract.ClientID,
		"version":          contract.Version,
		"description":      contract.Description,
		"allowed_agents":   contract.AccessPermissions.AllowedAgents,
		"forbidden_agents": contract.AccessPermissions.ForbiddenAgents,
		"rate_limits":      contract.RateLimits,
		"loaded_at":        time.Now(),
	}
}
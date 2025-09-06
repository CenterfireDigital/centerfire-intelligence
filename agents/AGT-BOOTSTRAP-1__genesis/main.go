package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"gopkg.in/yaml.v3"
)

// BootstrapAgent - The genesis agent that creates all others
type BootstrapAgent struct {
	AgentID   string `json:"agent_id"`
	CID       string `json:"cid"`
	BasePath  string `json:"base_path"`
	Sequences map[string]int `json:"sequences"`
}

// AgentSpec - Specification for creating new agents
type AgentSpec struct {
	Domain       string            `yaml:"domain"`
	Purpose      string            `yaml:"purpose"`
	Name         string            `yaml:"name,omitempty"`
	Capabilities []string          `yaml:"capabilities,omitempty"`
	Dependencies []string          `yaml:"dependencies,omitempty"`
	State        map[string]string `yaml:"state,omitempty"`
}

// AgentMetadata - Generated metadata for new agent
type AgentMetadata struct {
	Slug      string `json:"slug"`
	CID       string `json:"cid"`
	Directory string `json:"directory"`
	Path      string `json:"path"`
}

// NewBootstrapAgent - Create new bootstrap agent instance
func NewBootstrapAgent() (*BootstrapAgent, error) {
	basePath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %v", err)
	}
	
	// Go up to parent agents directory
	basePath = filepath.Dir(basePath)
	
	agent := &BootstrapAgent{
		AgentID:   "AGT-BOOTSTRAP-1",
		CID:       "cid:centerfire:agent:genesis", 
		BasePath:  basePath,
		Sequences: make(map[string]int),
	}
	
	// Load existing sequences
	if err := agent.loadSequences(); err != nil {
		log.Printf("Warning: Could not load sequences: %v", err)
	}
	
	return agent, nil
}

// loadSequences - Load or initialize sequence tracking
func (b *BootstrapAgent) loadSequences() error {
	sequencesFile := filepath.Join(b.BasePath, "sequences.json")
	
	data, err := os.ReadFile(sequencesFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with empty sequences
			return nil
		}
		return err
	}
	
	return json.Unmarshal(data, &b.Sequences)
}

// saveSequences - Persist sequence numbers
func (b *BootstrapAgent) saveSequences() error {
	sequencesFile := filepath.Join(b.BasePath, "sequences.json")
	
	data, err := json.MarshalIndent(b.Sequences, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(sequencesFile, data, 0644)
}

// getNextSequence - Get next sequence number for domain
func (b *BootstrapAgent) getNextSequence(domain string) (int, error) {
	key := fmt.Sprintf("AGT-%s", domain)
	nextSeq := b.Sequences[key] + 1
	b.Sequences[key] = nextSeq
	
	if err := b.saveSequences(); err != nil {
		return 0, fmt.Errorf("failed to save sequences: %v", err)
	}
	
	return nextSeq, nil
}

// CreateAgent - Create a new agent from specification
func (b *BootstrapAgent) CreateAgent(spec AgentSpec) (*AgentMetadata, error) {
	// Generate identifiers
	sequence, err := b.getNextSequence(spec.Domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get sequence: %v", err)
	}
	
	agentULID := ulid.Make()
	slug := fmt.Sprintf("AGT-%s-%d", spec.Domain, sequence)
	cid := fmt.Sprintf("cid:centerfire:agent:%s", agentULID.String())
	dirName := fmt.Sprintf("%s__%s", slug, agentULID.String()[:8])
	
	// Create directory structure
	agentDir := filepath.Join(b.BasePath, dirName)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}
	
	// Write .id file
	idFile := filepath.Join(agentDir, ".id")
	if err := os.WriteFile(idFile, []byte(cid), 0644); err != nil {
		return nil, fmt.Errorf("failed to write .id file: %v", err)
	}
	
	// Create agent specification
	agentSpec := map[string]interface{}{
		"version":      "1.0",
		"id":           slug,
		"cid":          cid,
		"name":         getAgentName(spec),
		"purpose":      spec.Purpose,
		"domain":       spec.Domain,
		"sequence":     sequence,
		"capabilities": spec.Capabilities,
		"dependencies": spec.Dependencies,
		"service": map[string]interface{}{
			"type": "redis_pubsub",
			"channels": map[string]string{
				"request":  fmt.Sprintf("agent.%s.request", spec.Domain),
				"response": fmt.Sprintf("agent.%s.response", spec.Domain),
			},
		},
		"state":      spec.State,
		"created_by": b.AgentID,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}
	
	// Write spec.yaml
	specFile := filepath.Join(agentDir, "spec.yaml")
	specData, err := yaml.Marshal(agentSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec: %v", err)
	}
	if err := os.WriteFile(specFile, specData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write spec.yaml: %v", err)
	}
	
	// Generate basic agent code
	agentCode := generateAgentCode(agentSpec)
	codeFile := filepath.Join(agentDir, "main.go")
	if err := os.WriteFile(codeFile, []byte(agentCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write main.go: %v", err)
	}
	
	// Initialize Go module
	modInit := fmt.Sprintf("cd %s && go mod init centerfire/%s", agentDir, slug)
	if err := runCommand(modInit); err != nil {
		log.Printf("Warning: Failed to initialize Go module: %v", err)
	}
	
	fmt.Printf("Created agent: %s\n", slug)
	fmt.Printf("  CID: %s\n", cid)
	fmt.Printf("  Directory: %s\n", dirName)
	
	return &AgentMetadata{
		Slug:      slug,
		CID:       cid,
		Directory: dirName,
		Path:      agentDir,
	}, nil
}

// getAgentName - Generate human-friendly agent name
func getAgentName(spec AgentSpec) string {
	if spec.Name != "" {
		return spec.Name
	}
	return fmt.Sprintf("%s Agent", spec.Domain)
}

// runCommand - Execute shell command (simplified)
func runCommand(cmd string) error {
	// This is a simplified version - in production would use proper exec
	log.Printf("Would run: %s", cmd)
	return nil
}

// generateAgentCode - Generate basic Go agent implementation
func generateAgentCode(spec map[string]interface{}) string {
	domain := spec["domain"].(string)
	agentID := spec["id"].(string)
	cid := spec["cid"].(string)
	
	return fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// %sAgent - Generated agent for %s domain
type %sAgent struct {
	AgentID        string
	CID           string
	RequestChannel string
	ResponseChannel string
}

// NewAgent - Create new %s agent
func NewAgent() *%sAgent {
	return &%sAgent{
		AgentID:         "%s",
		CID:            "%s",
		RequestChannel:  "agent.%s.request",
		ResponseChannel: "agent.%s.response",
	}
}

// Start - Start listening for requests
func (a *%sAgent) Start() {
	fmt.Printf("%%s starting...\n", a.AgentID)
	fmt.Printf("Listening on: %%s\n", a.RequestChannel)
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Simulate agent running
	for {
		select {
		case <-sigChan:
			fmt.Printf("\n%%s shutting down...\n", a.AgentID)
			return
		default:
			// Agent work would go here
			// For now, just indicate it's running
		}
	}
}

// HandleRequest - Handle incoming request
func (a *%sAgent) HandleRequest(request map[string]interface{}) map[string]interface{} {
	action, ok := request["action"].(string)
	if !ok {
		return map[string]interface{}{
			"error": "No action specified",
		}
	}
	
	// Route to appropriate handler
	switch action {
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown action: %%s", action),
		}
	}
}

func main() {
	agent := NewAgent()
	agent.Start()
}
`, 
		strings.Title(strings.ToLower(domain)), // NamingAgent
		domain,                                 // NAMING domain
		strings.Title(strings.ToLower(domain)), // NamingAgent
		domain,                                 // NAMING agent
		strings.Title(strings.ToLower(domain)), // NamingAgent  
		strings.Title(strings.ToLower(domain)), // NamingAgent
		agentID,                               // AGT-NAMING-1
		cid,                                   // cid:...
		strings.ToLower(domain),               // naming.request
		strings.ToLower(domain),               // naming.response
		strings.Title(strings.ToLower(domain)), // NamingAgent
		strings.Title(strings.ToLower(domain)), // NamingAgent
	)
}

// BootstrapCoreAgents - Create the core agent set needed for the system
func (b *BootstrapAgent) BootstrapCoreAgents() error {
	coreAgents := []AgentSpec{
		{
			Domain:  "NAMING",
			Purpose: "Exclusive authority over all naming decisions",
			Capabilities: []string{
				"allocate_capability",
				"allocate_module",
				"allocate_function", 
				"validate_name",
				"manage_sequences",
			},
		},
		{
			Domain:  "STRUCT",
			Purpose: "Creates and manages directory structures",
			Capabilities: []string{
				"create_directory",
				"create_structure", 
				"validate_structure",
			},
			Dependencies: []string{"AGT-NAMING-1"},
		},
		{
			Domain:  "SEMDOC", 
			Purpose: "Creates and maintains semantic documentation",
			Capabilities: []string{
				"create_semblock",
				"create_contract",
				"validate_documentation",
			},
			Dependencies: []string{"AGT-NAMING-1", "AGT-STRUCT-1"},
		},
		{
			Domain:  "CODING",
			Purpose: "Generates code from specifications", 
			Capabilities: []string{
				"generate_code",
				"refactor_code",
				"analyze_patterns",
			},
			Dependencies: []string{"AGT-NAMING-1", "AGT-SEMDOC-1"},
		},
	}
	
	fmt.Println("Bootstrapping core agents...")
	for _, agentSpec := range coreAgents {
		metadata, err := b.CreateAgent(agentSpec)
		if err != nil {
			return fmt.Errorf("failed to create %s agent: %v", agentSpec.Domain, err)
		}
		log.Printf("✓ Created %s", metadata.Slug)
	}
	fmt.Println("Core agents created!")
	
	return nil
}

func main() {
	bootstrap, err := NewBootstrapAgent()
	if err != nil {
		log.Fatalf("Failed to create bootstrap agent: %v", err)
	}
	
	// Check command line arguments
	if len(os.Args) > 1 && os.Args[1] == "bootstrap" {
		if err := bootstrap.BootstrapCoreAgents(); err != nil {
			log.Fatalf("Failed to bootstrap core agents: %v", err)
		}
		return
	}
	
	fmt.Printf("Bootstrap Agent %s ready\n", bootstrap.AgentID)
	fmt.Println("Run with 'bootstrap' argument to create core agents")
}
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/net/context"
)

// Stage 1 SemDoc Parser Implementation
// Traditional development without SemDoc contracts
// Focus: Extract @semblock comments and prepare for future contract enforcement

type SemDocParser struct {
	redisClient *redis.Client
	agentID     string
}

type ParseRequest struct {
	Action    string `json:"action"`
	FilePath  string `json:"file_path,omitempty"`
	Directory string `json:"directory,omitempty"`
	RequestID string `json:"request_id"`
}

type ParseResponse struct {
	Success   bool        `json:"success"`
	RequestID string      `json:"request_id"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type SemBlockContract struct {
	ContractID     string            `json:"contract_id"`
	SemanticPath   string            `json:"semantic_path"`
	FunctionName   string            `json:"function_name"`
	FilePath       string            `json:"file_path"`
	LineNumber     int               `json:"line_number"`
	Preconditions  []string          `json:"preconditions"`
	Postconditions []string          `json:"postconditions"`
	Invariants     []string          `json:"invariants"`
	Effects        []string          `json:"effects"`
	RawComment     string            `json:"raw_comment"`
	ParsedAt       time.Time         `json:"parsed_at"`
	Metadata       map[string]string `json:"metadata"`
}

func NewSemDocParser() *SemDocParser {
	return &SemDocParser{
		redisClient: redis.NewClient(&redis.Options{
			Addr: "localhost:6380",
		}),
		agentID: "AGT-SEMDOC-PARSER-1",
	}
}

func (p *SemDocParser) Start() {
	ctx := context.Background()
	
	// Test Redis connection
	_, err := p.redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	log.Printf("%s starting...", p.agentID)
	log.Println("STAGE 1: Traditional development mode")
	log.Println("Redis channel: agent.semdoc-parser.request")

	// Subscribe to parser requests
	pubsub := p.redisClient.Subscribe(ctx, "agent.semdoc-parser.request")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		p.handleRequest(ctx, msg.Payload)
	}
}

func (p *SemDocParser) handleRequest(ctx context.Context, payload string) {
	var req ParseRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		log.Printf("Failed to parse request: %v", err)
		return
	}

	log.Printf("Processing request: %s", req.Action)

	var response ParseResponse
	response.RequestID = req.RequestID
	response.Success = true

	switch req.Action {
	case "parse_file":
		contracts, err := p.parseFile(req.FilePath)
		if err != nil {
			response.Success = false
			response.Error = err.Error()
		} else {
			response.Data = contracts
			// Store contracts in Redis for Stage 2/3 processing
			p.storeContracts(ctx, contracts)
		}

	case "parse_directory":
		contracts, err := p.parseDirectory(req.Directory)
		if err != nil {
			response.Success = false
			response.Error = err.Error()
		} else {
			response.Data = contracts
			p.storeContracts(ctx, contracts)
		}

	case "list_contracts":
		contracts, err := p.listStoredContracts(ctx)
		if err != nil {
			response.Success = false
			response.Error = err.Error()
		} else {
			response.Data = contracts
		}

	default:
		response.Success = false
		response.Error = "Unknown action: " + req.Action
	}

	// Publish response
	responseBytes, _ := json.Marshal(response)
	p.redisClient.Publish(ctx, "agent.semdoc-parser.response", string(responseBytes))
}

// parseFile extracts @semblock comments from a single file
func (p *SemDocParser) parseFile(filePath string) ([]SemBlockContract, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	var contracts []SemBlockContract
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	inSemBlock := false
	var currentBlock strings.Builder
	var blockStartLine int

	// Regex to detect @semblock start
	semBlockRegex := regexp.MustCompile(`@semblock`)

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		
		if semBlockRegex.MatchString(line) {
			inSemBlock = true
			blockStartLine = lineNumber
			currentBlock.Reset()
			currentBlock.WriteString(line + "\n")
			continue
		}

		if inSemBlock {
			currentBlock.WriteString(line + "\n")
			
			// Check if we're ending the comment block
			if strings.Contains(line, "*/") || (!strings.HasPrefix(strings.TrimSpace(line), "//") && strings.TrimSpace(line) != "") {
				// Parse the collected block
				contract, err := p.parseSemBlock(currentBlock.String(), filePath, blockStartLine)
				if err != nil {
					log.Printf("Warning: Failed to parse semblock at line %d: %v", blockStartLine, err)
				} else if contract != nil {
					contracts = append(contracts, *contract)
				}
				inSemBlock = false
			}
		}
	}

	return contracts, scanner.Err()
}

// parseSemBlock parses a single @semblock comment into a contract
func (p *SemDocParser) parseSemBlock(block, filePath string, lineNumber int) (*SemBlockContract, error) {
	contract := &SemBlockContract{
		FilePath:    filePath,
		LineNumber:  lineNumber,
		RawComment:  block,
		ParsedAt:    time.Now(),
		Metadata:    make(map[string]string),
	}

	lines := strings.Split(block, "\n")
	
	// Parse contract components
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "contract_id:") {
			contract.ContractID = p.extractValue(line, "contract_id:")
		} else if strings.HasPrefix(line, "semantic_path:") {
			contract.SemanticPath = p.extractValue(line, "semantic_path:")
		} else if strings.HasPrefix(line, "function:") {
			contract.FunctionName = p.extractValue(line, "function:")
		} else if strings.HasPrefix(line, "preconditions:") {
			contract.Preconditions = p.extractArray(line, "preconditions:")
		} else if strings.HasPrefix(line, "postconditions:") {
			contract.Postconditions = p.extractArray(line, "postconditions:")
		} else if strings.HasPrefix(line, "invariants:") {
			contract.Invariants = p.extractArray(line, "invariants:")
		} else if strings.HasPrefix(line, "effects:") {
			contract.Effects = p.extractArray(line, "effects:")
		}
	}

	// Skip if no contract_id found (not a proper semblock)
	if contract.ContractID == "" {
		return nil, nil
	}

	return contract, nil
}

// parseDirectory recursively parses all files in a directory
func (p *SemDocParser) parseDirectory(dirPath string) ([]SemBlockContract, error) {
	var allContracts []SemBlockContract
	
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Only parse source files
		if p.isSourceFile(path) {
			contracts, err := p.parseFile(path)
			if err != nil {
				log.Printf("Warning: Failed to parse file %s: %v", path, err)
				return nil // Continue processing other files
			}
			allContracts = append(allContracts, contracts...)
		}
		
		return nil
	})
	
	return allContracts, err
}

// isSourceFile checks if a file should be parsed for semblocks
func (p *SemDocParser) isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	sourceExts := []string{".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".hpp", ".rs", ".rb", ".php"}
	
	for _, sourceExt := range sourceExts {
		if ext == sourceExt {
			return true
		}
	}
	return false
}

// storeContracts stores parsed contracts in Redis for Stage 2/3 processing
func (p *SemDocParser) storeContracts(ctx context.Context, contracts []SemBlockContract) {
	for _, contract := range contracts {
		contractBytes, err := json.Marshal(contract)
		if err != nil {
			log.Printf("Failed to marshal contract %s: %v", contract.ContractID, err)
			continue
		}
		
		// Store in Redis hash
		key := fmt.Sprintf("centerfire:semdoc:contract:%s", contract.ContractID)
		p.redisClient.HSet(ctx, key, "data", string(contractBytes))
		
		// Add to contracts index
		p.redisClient.SAdd(ctx, "centerfire:semdoc:contracts:index", contract.ContractID)
		
		// Store in stream for processing
		p.redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: "centerfire:semdoc:contracts",
			Values: map[string]interface{}{
				"contract_id":    contract.ContractID,
				"semantic_path":  contract.SemanticPath,
				"file_path":      contract.FilePath,
				"event_type":     "contract_parsed",
				"parser_version": "stage1",
				"data":           string(contractBytes),
			},
		})
	}
	
	log.Printf("Stored %d contracts in Redis", len(contracts))
}

// listStoredContracts retrieves all stored contracts
func (p *SemDocParser) listStoredContracts(ctx context.Context) ([]SemBlockContract, error) {
	contractIDs, err := p.redisClient.SMembers(ctx, "centerfire:semdoc:contracts:index").Result()
	if err != nil {
		return nil, err
	}
	
	var contracts []SemBlockContract
	for _, contractID := range contractIDs {
		key := fmt.Sprintf("centerfire:semdoc:contract:%s", contractID)
		contractData, err := p.redisClient.HGet(ctx, key, "data").Result()
		if err != nil {
			log.Printf("Failed to get contract %s: %v", contractID, err)
			continue
		}
		
		var contract SemBlockContract
		if err := json.Unmarshal([]byte(contractData), &contract); err != nil {
			log.Printf("Failed to unmarshal contract %s: %v", contractID, err)
			continue
		}
		
		contracts = append(contracts, contract)
	}
	
	return contracts, nil
}

// Helper functions
func (p *SemDocParser) extractValue(line, prefix string) string {
	value := strings.TrimPrefix(line, prefix)
	value = strings.Trim(value, ` "'"`)
	return value
}

func (p *SemDocParser) extractArray(line, prefix string) []string {
	value := strings.TrimPrefix(line, prefix)
	value = strings.Trim(value, ` []"'`)
	
	if value == "" {
		return []string{}
	}
	
	// Simple array parsing - split by comma
	parts := strings.Split(value, ",")
	var result []string
	for _, part := range parts {
		part = strings.Trim(part, ` "'"`)
		if part != "" {
			result = append(result, part)
		}
	}
	
	return result
}

func main() {
	parser := NewSemDocParser()
	parser.Start()
}
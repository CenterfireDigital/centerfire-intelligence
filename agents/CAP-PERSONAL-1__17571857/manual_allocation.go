package main

import (
	"fmt"
	"time"
)

// generateULID8 creates an 8-character ULID for agent identification
func generateULID8() string {
	now := time.Now().UTC()
	return fmt.Sprintf("%d%08X", now.Unix(), now.Nanosecond()%0xFFFFFFFF)[:8]
}

// generateCID creates a semantic CID following CI protocol
func generateCID(project, env, objectType, ulid string) string {
	return fmt.Sprintf("cid:%s.%s:%s:%s", project, env, objectType, ulid)
}

func main() {
	fmt.Println("🏷️  MANUAL SEMANTIC NAME ALLOCATION")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("Following CI Protocol Pattern: PROJECT.ENV.TYPE-DOMAIN-N__ULID8")
	fmt.Println()

	// Generate ULID8 for the new agent
	ulid8 := generateULID8()
	
	// Following CI Protocol pattern
	domain := "CONTEXT"
	sequence := 1 // First CONTEXT agent
	project := "centerfire"
	environment := "dev"
	
	// Generate semantic name using CI protocol pattern
	semanticName := fmt.Sprintf("AGT-%s-%d__%s", domain, sequence, ulid8)
	
	// Generate CID
	cid := generateCID(project, environment, "agent", ulid8)
	
	// Create slug for directory naming
	slug := fmt.Sprintf("AGT-%s-%d__%s", domain, sequence, ulid8)
	
	fmt.Printf("✅ ALLOCATED SEMANTIC NAME\n")
	fmt.Printf("🏷️  Semantic Name: %s\n", semanticName)
	fmt.Printf("🔑 CID: %s\n", cid)
	fmt.Printf("📂 Directory Slug: %s\n", slug)
	fmt.Printf("🌐 Project: %s\n", project)
	fmt.Printf("🏗️  Environment: %s\n", environment)
	fmt.Printf("📝 Domain: %s\n", domain)
	fmt.Printf("🔢 Sequence: %d\n", sequence)
	fmt.Printf("🎯 Purpose: Fast Weaviate GraphQL context retrieval agent for conversation history and semantic search\n")
	fmt.Printf("🔄 Type: Agent (persistent)\n")
	fmt.Printf("⏰ Generated: %s\n", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	
	fmt.Printf("\n🚀 NEXT STEPS:\n")
	fmt.Printf("   1. Create agent directory: agents/%s/\n", semanticName)
	fmt.Printf("   2. Implement Weaviate GraphQL context retrieval\n")
	fmt.Printf("   3. Add conversation history search capabilities\n")
	fmt.Printf("   4. Add semantic search for contextual information\n")
	fmt.Printf("   5. Register with AGT-MANAGER-1 when ready\n")
	fmt.Printf("   6. Update CI protocol with new agent entry\n")
	
	fmt.Printf("\n📋 AGENT SPEC TEMPLATE:\n")
	fmt.Printf(`
capabilities:
    - weaviate_query
    - context_retrieval
    - conversation_search
    - semantic_search
cid: %s
created_at: "%s"
created_by: manual_allocation
dependencies: ["weaviate", "redis"]
domain: CONTEXT
id: %s
name: Context Retrieval Agent
purpose: Fast Weaviate GraphQL context retrieval agent for conversation history and semantic search
sequence: %d
service:
    channels:
        request: agent.context.request
        response: agent.context.response
    type: redis_pubsub
state: {}
version: "1.0"
`, cid, time.Now().UTC().Format("2006-01-02T15:04:05Z"), semanticName, sequence)
}
# Casbin Agent Authorization Integration Plan

## Overview

This document outlines the plan to integrate Casbin as a bootstrap authorization service for Centerfire Intelligence agents, with the goal of accelerating agent development while building toward native SemDoc authorization.

## Genesis of the Approach: Active Directory → Agent Authorization

### The Network Engineering Insight

*From the conversation that led to this approach:*

**The Core Insight:** "I used to be a network engineer and I built a lot of the early NT and later, Active Directory networks. Active Directory roles and permissions were essentially contracts, but mere gatekeepers, not logic. The salient point is that each role and permission was an inviolable contract and they were only supposed to be given the minimum permissions needed to access whatever it was they needed to do."

**The Translation to Agents:** "In our case, the people are agents and agents have capabilities. Not only will agents not be able to request services they don't have permissions to access, they won't even know about them."

### Why This Architectural Pattern Works

**Active Directory Principles Applied to Agents:**
- **Roles and Groups**: Agent types with inherited capabilities
- **Minimum Privilege**: Agents get only capabilities needed for their domain
- **Hierarchical Inheritance**: Groups can be assigned to groups → Agent capabilities can inherit from domain capabilities
- **Inviolable Contracts**: Permissions are enforced at system level, not discretionary

**The "They Don't Even Know About Them" Principle:**
- Content generation agents cannot see infrastructure capabilities
- Infrastructure orchestration agents cannot see brand/content systems
- Account management agents see only their assigned account groups
- Session orchestrators respect domain boundaries

### The LexicRoot Complexity Challenge

**The operational requirements that drove this approach:**
- Hundreds of hosting accounts with different behavioral contracts
- SSH sessions spawning tmux sessions with nested reporting chains
- VPN rotation coordination across distributed infrastructure
- Map-reduce content operations across nodes
- Content timing that must appear human-like
- Account suspension recovery without revealing coordination patterns

**Why traditional RBAC isn't sufficient:**
"Obviously this is bigger and more complicated than just an RBAC solution, but that's where we start I think."

### The Casbin Discovery

**Why Casbin was the breakthrough:**
- Proven authorization engine with sophisticated routing features
- RBAC + ABAC support for complex conditional logic
- Domain/tenant support for account group isolation
- RESTful path matching for session hierarchy management
- Policy-as-code approach that matches SemDoc contracts
- Multi-language support for polyglot agent architecture

**The Strategic Decision:**
"Honestly, if we just had to start with Casbin to get the development cycle going, would that be so bad?"

**The answer:** "No - Starting With Casbin Would Be Brilliant" because it provides proven authorization foundation while keeping the path open for native SemDoc migration.

## The Moron Detection Agent Concept

**From the ideation discussion:**
"Later I want to build a moron detection agent as an ideation sub agent just to shoot down my ideas."

This led to the insight that specialized agents with narrow, well-defined contracts could collaborate effectively:

```yaml
agent.ideation.critic:
  capabilities:
    - CAP-LOGIC-ANALYSIS-1
    - CAP-FEASIBILITY-ASSESSMENT-1  
    - CAP-RISK-EVALUATION-1
  semantic_constraints:
    - idea.complexity.analyze_realistic_implementation_time
    - idea.dependencies.identify_circular_or_impossible
    - idea.resource_requirements.validate_availability
  postconditions:
    - criticism.constructive == true
    - alternatives.suggested.count >= 2
    - reasoning.clear_and_specific == true
```

## Strategic Context

- **Primary Goal**: Build Centerfire Intelligence to manufacture LexicRoot
- **Current Challenge**: Need agent isolation and authorization to enable autonomous agent operations
- **Bootstrap Strategy**: Use Casbin as authorization service, migrate to native SemDoc later
- **Timeline**: Start tomorrow, working authorization within a week

## Architecture Principles

### Clean Separation
- Casbin deployed as isolated authorization microservice (container)
- SemDoc codebase remains dependency-free
- Authorization abstracted behind interface for easy migration
- No external library dependencies in core agent code

### Migration Path
1. **Phase 1**: Casbin container for immediate authorization (Week 1)
2. **Phase 2**: Dual authorization testing - Casbin + SemDoc side-by-side (Month 2)
3. **Phase 3**: Native SemDoc authorization, Casbin container removed (Month 3)
4. **Phase 4**: LexicRoot development with pure SemDoc (Month 4)

## Technical Implementation

### Container Strategy
```yaml
# docker-compose.yml
services:
  casbin-auth:
    image: casbin/casbin-server
    ports:
      - "8081:8080"
    volumes:
      - ./policies:/app/policies
      - ./model.conf:/app/model.conf
    depends_on:
      - mem0-redis
    
  centerfire-agents:
    build: ./agents
    depends_on:
      - casbin-auth
      - mem0-redis
    environment:
      - AUTH_SERVICE_URL=http://casbin-auth:8080
```

### Authorization Interface Abstraction
```go
// agents/auth/interface.go
type AuthorizationService interface {
    Enforce(agent string, capability string, context map[string]interface{}) bool
    LoadPolicies(agent string) error
    AddPolicy(policy Policy) error
    RemovePolicy(policy Policy) error
}

// Implementation 1: Casbin Service (Bootstrap)
type CasbinAuthService struct {
    endpoint string
    client   *http.Client
}

// Implementation 2: SemDoc Service (Future Native)
type SemDocAuthService struct {
    contractRegistry *ContractRegistry
    validator       *ContractValidator
}
```

### Agent Integration
```go
func (a *Agent) Execute(capability string, context map[string]interface{}) error {
    // Authorization check through abstracted interface
    if !a.authService.Enforce(a.ID, capability, context) {
        return fmt.Errorf("agent %s not authorized for capability %s", a.ID, capability)
    }
    
    // Execute within authorized boundaries
    return a.executeCapability(capability, context)
}
```

## LexicRoot Domain Modeling

### Agent Domain Isolation
```ini
# Content Generation Agents
p, CAP-CONTENT-GENERATOR-1, content.create, allow
p, CAP-CONTENT-GENERATOR-1, brand.voice.validate, allow
p, CAP-CONTENT-GENERATOR-1, infrastructure.*, deny
p, CAP-CONTENT-GENERATOR-1, account.manage.*, deny

# Infrastructure Orchestration Agents  
p, CAP-INFRA-ORCHESTRATOR-1, ssh.session.manage, allow
p, CAP-INFRA-ORCHESTRATOR-1, vpn.rotate, allow
p, CAP-INFRA-ORCHESTRATOR-1, content.*, deny
p, CAP-INFRA-ORCHESTRATOR-1, brand.*, deny

# Account Management Agents (Domain-based)
p, CAP-ACCOUNT-MANAGER-1, account.create, domain_1, allow
p, CAP-ACCOUNT-MANAGER-1, account.manage, domain_1, allow  
p, CAP-ACCOUNT-MANAGER-1, account.*, domain_2, deny
p, CAP-ACCOUNT-MANAGER-1, infrastructure.*, deny
```

### Session Management Hierarchy
```ini
# Nested session permissions for complex orchestration
p, session_orchestrator, /session/*, manage
p, content_session, /session/content/*, read_write
p, infra_session, /session/infra/level_*/*, read_write  
p, account_session, /session/account/group_*/*, read_write

# Cross-domain access prevention
p, content_session, /session/infra/*, deny
p, content_session, /session/account/*, deny
p, infra_session, /session/content/*, deny
```

## "They Don't Even Know About Them" Implementation

### Filtered Policy Loading
```go
func NewAgent(agentID string, enforcer *casbin.Enforcer) *Agent {
    // Agent only loads policies for capabilities it can access
    filter := &casbin.Filter{
        P: []string{"", agentID, "", ""},
    }
    
    // Agent literally cannot see unauthorized capabilities
    enforcer.LoadFilteredPolicy(filter)
    
    return &Agent{
        ID: agentID,
        enforcer: enforcer,
        capabilities: getAuthorizedCapabilities(agentID, enforcer),
    }
}
```

### Operational Security Benefits
- Content agents cannot access infrastructure capabilities
- Infrastructure agents cannot access brand/content systems  
- Account managers see only their assigned account groups
- Session orchestrators respect domain boundaries
- No agent can accidentally violate operational security

## Development Timeline

### Week 1: Casbin Container + Agent Interface
- [ ] Deploy Casbin as authorization microservice
- [ ] Create authorization abstraction layer in agent framework
- [ ] Convert existing agents (AGT-NAMING-1, AGT-STRUCT-1, etc.) to use authorization service
- [ ] Test basic agent isolation and capability enforcement
- [ ] Validate "they don't even know about them" principle

### Week 2: LexicRoot Domain Policies  
- [ ] Define Casbin policies for content/infrastructure/account domains
- [ ] Implement session management hierarchy permissions
- [ ] Test account group isolation across multiple domains
- [ ] Load test with multiple agent types operating simultaneously
- [ ] Validate nested session orchestration under policy enforcement

### Week 3: SemDoc Authorization Development
- [ ] Begin building native SemDoc contract validator
- [ ] Implement dual authorization testing framework
- [ ] Performance comparison between Casbin and SemDoc approaches
- [ ] Design migration strategy and timeline
- [ ] Document contract-to-policy mapping patterns

### Week 4: Production Deployment
- [ ] Deploy Centerfire Intelligence with Casbin authorization
- [ ] Real multi-agent orchestration under policy enforcement
- [ ] System ready for LexicRoot development
- [ ] Complete Casbin → SemDoc migration roadmap
- [ ] Performance metrics and operational validation

## Container Profile Strategy

Following existing container orchestration patterns:

```yaml
# docker-compose profiles for migration stages
profiles:
  - casbin-auth      # Bootstrap authorization (Phase 1)
  - dual-auth        # Both systems for migration testing (Phase 2)  
  - semdoc-native    # Pure SemDoc authorization (Phase 3)
```

```bash
# Deployment commands
docker-compose --profile casbin-auth up      # Start with Casbin
docker-compose --profile dual-auth up        # Migration testing
docker-compose --profile semdoc-native up    # Final state
```

## Integration with Existing Infrastructure

### Redis Policy Storage
- Casbin policies stored in existing Redis instance (mem0-redis:6380)
- No additional storage infrastructure required
- Policies persist across agent restarts
- Eliminates context retraining cycles

### Agent Manager Enhancement
```go
// Enhanced AGT-MANAGER-1 with authorization
type AgentManager struct {
    redis       *redis.Client
    authService AuthorizationService  // Abstracted authorization
    agents      map[string]*Agent
}

func (am *AgentManager) SpawnAgent(agentType string, config AgentConfig) (*Agent, error) {
    // Load agent's authorized policies
    policies := am.loadAgentPolicies(agentType)
    
    // Create agent with authorization enforcement
    agent := NewAgent(agentType, am.authService)
    
    // Agent can only execute authorized capabilities
    return agent, nil
}
```

## Success Metrics

### Week 1 Success Criteria
- [ ] All existing agents operate under Casbin authorization
- [ ] Zero unauthorized capability access attempts succeed
- [ ] Agent isolation verified through policy enforcement
- [ ] Authorization service responds < 10ms for policy checks

### Month 1 Success Criteria  
- [ ] Complex multi-agent orchestration working under authorization
- [ ] Account domain isolation tested with hundreds of mock accounts
- [ ] Session management hierarchy operational with nested sessions
- [ ] System ready for LexicRoot development phase

### Migration Success Criteria
- [ ] Native SemDoc authorization achieves feature parity with Casbin
- [ ] Zero downtime migration from Casbin to SemDoc
- [ ] Performance improvements with native authorization
- [ ] Casbin container successfully removed from infrastructure

## Risk Mitigation

### Technical Risks
- **Authorization service downtime**: Implement circuit breaker pattern with fallback policies
- **Performance bottleneck**: Load test authorization service, implement caching
- **Policy complexity**: Start simple, iterate based on operational needs
- **Migration complexity**: Dual-system testing validates before switchover

### Operational Risks  
- **Agent capability explosion**: Use hierarchical policies and inheritance
- **Policy maintenance burden**: Automate policy generation from agent definitions
- **Security misconfiguration**: Implement policy validation and testing frameworks
- **Debugging complexity**: Comprehensive logging and audit trails

## Next Steps

1. **Tomorrow**: Begin Casbin container integration
2. **This Week**: Complete Phase 1 implementation  
3. **Next Week**: LexicRoot domain policy definition
4. **Month End**: Production-ready Centerfire Intelligence with agent authorization

## Notes

- This document captures the plan developed 2025-09-07
- Casbin serves as bootstrap only - not permanent dependency  
- Focus remains on building Centerfire Intelligence to manufacture LexicRoot
- SemDoc specification development continues in parallel
- Authorization abstraction enables clean migration path
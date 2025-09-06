# Agent Bootstrap Sequence

## The Realization: Agents ARE Capabilities

Agents aren't separate from the system - they ARE the system. Each agent is a capability with:
- A CID (immutable identifier)
- A slug (AGT-DOMAIN-###)
- A directory structure
- SemDoc contracts
- The ability to act autonomously

## The Chicken-Egg Solution: AGT-BOOTSTRAP-001

### The Prime Mover
We need ONE manually-built agent that can build other agents. This is our genesis block.

```yaml
agent: AGT-BOOTSTRAP-001
cid: cid:centerfire:agent:01J9F8MANUALLY0CREATED
purpose: "Builds other agents according to specification"
capabilities:
  - Create agent directory structure
  - Generate agent contracts
  - Wire up agent communication
  - Deploy agent services
```

## Agent Naming Convention

```yaml
pattern: AGT-<DOMAIN>-<NNN>
examples:
  AGT-NAMING-001:   # Naming authority agent
  AGT-SEMDOC-001:   # Documentation agent
  AGT-CODING-001:   # Code generation agent
  AGT-STRUCT-001:   # Structure creation agent
  AGT-GRAPH-001:    # Graph database agent
  AGT-VECTOR-001:   # Vector database agent
  AGT-TEST-001:     # Testing agent
  AGT-DEPLOY-001:   # Deployment agent
```

## Bootstrap Sequence

### Iteration 0: Manual Creation
```bash
# The only manual step - create the bootstrap agent
/agents/
  AGT-BOOTSTRAP-001__manual/
    agent.py          # Simple agent builder
    spec.yaml         # Agent specification
    .id               # Contains bootstrap CID
```

### Iteration 1: Bootstrap Creates Naming Agent
```python
# AGT-BOOTSTRAP-001 executes:
bootstrap.create_agent({
    "domain": "NAMING",
    "purpose": "Authority for all naming decisions",
    "capabilities": ["allocate_ids", "manage_sequences", "validate_names"]
})

# Creates:
/agents/
  AGT-NAMING-001__01J9F8H8/
    agent.py
    spec.yaml
    .id
```

### Iteration 2: Naming + Bootstrap Create Others
Now we have TWO agents working together:
- AGT-BOOTSTRAP-001 creates the structure
- AGT-NAMING-001 provides the names

```python
# Bootstrap asks Naming for a name
name = naming_agent.allocate_agent("SEMDOC", "Documentation management")
# Returns: AGT-SEMDOC-001

# Bootstrap creates the agent
bootstrap.create_agent(name, semdoc_spec)
```

### Iteration 3: Exponential Growth
Each new agent can help create others:
- AGT-SEMDOC-001 documents new agents
- AGT-STRUCT-001 creates directory structures
- AGT-CODING-001 generates agent code
- AGT-TEST-001 tests agent behavior

## Agent Communication Protocol

### Redis Pub/Sub for Speed
```python
# Agent registers its capabilities
redis.publish("agent.online", {
    "agent": "AGT-NAMING-001",
    "capabilities": ["allocate_capability", "allocate_module", "allocate_function"],
    "endpoint": "redis://agent.naming.request"
})

# Other agents can request services
redis.publish("agent.naming.request", {
    "from": "AGT-CODING-001",
    "action": "allocate_capability",
    "params": {"domain": "AUTH", "purpose": "User authentication"}
})

# Naming agent responds
redis.publish("agent.coding.response", {
    "to": "AGT-CODING-001",
    "result": {"slug": "CAP-AUTH-001", "cid": "cid:..."}
})
```

## Agent Directory Structure

```bash
/agents/
  AGT-BOOTSTRAP-001__manual/     # The only manual one
    agent.py
    spec.yaml
    .id
    
  AGT-NAMING-001__01J9F8H8/      # Created by bootstrap
    agent.py
    spec.yaml
    .id
    contracts/
      naming-contract.yaml
    state/
      sequences.json
      
  AGT-SEMDOC-001__01J9F8J2/      # Created by bootstrap + naming
    agent.py
    spec.yaml
    .id
    templates/
      semblock.template
      contract.template
```

## Why This Changes Everything

### 1. Agents Build Agents
Once AGT-BOOTSTRAP-001 exists, it can create ALL other agents. No more manual creation.

### 2. Agents ARE the System
Not just tools - they're the living, autonomous parts of the system.

### 3. Self-Improving
Each agent can be rebuilt by other agents as the system learns.

### 4. True Autonomy
Agents negotiate with each other, no human coordination needed.

## The Complete Bootstrap Plan

### Step 1: Build AGT-BOOTSTRAP-001 (Manual - One Time Only)
```python
class BootstrapAgent:
    def create_agent(self, spec):
        # 1. Get name from naming agent (or generate first one)
        # 2. Create directory structure
        # 3. Generate agent code from template
        # 4. Write spec.yaml
        # 5. Register in graph
        # 6. Start agent service
```

### Step 2: Bootstrap Creates Core Agents
```python
# Order matters - each enables the next
bootstrap.create_agent(naming_agent_spec)    # AGT-NAMING-001
bootstrap.create_agent(struct_agent_spec)    # AGT-STRUCT-001
bootstrap.create_agent(semdoc_agent_spec)    # AGT-SEMDOC-001
bootstrap.create_agent(coding_agent_spec)    # AGT-CODING-001
```

### Step 3: Agents Take Over
After core agents exist, they handle everything:
- AGT-CODING-001 improves other agents' code
- AGT-TEST-001 ensures agents work correctly
- AGT-DEPLOY-001 manages agent deployment
- AGT-MONITOR-001 tracks agent health

## Implementation Priority

1. **AGT-BOOTSTRAP-001** - Without this, nothing else can be created
2. **AGT-NAMING-001** - Provides names for everything 
3. **AGT-STRUCT-001** - Creates proper directory structures
4. **AGT-SEMDOC-001** - Documents what's being built
5. **AGT-CODING-001** - Generates actual code

## The Beautiful Part

Once this works, adding new capabilities is just:
```python
# Human says: "I need email sending capability"
orchestrator.request("Create email sending capability")

# Orchestrator coordinates:
# 1. AGT-NAMING-001 allocates: CAP-NET-002
# 2. AGT-STRUCT-001 creates directories
# 3. AGT-CODING-001 generates implementation
# 4. AGT-TEST-001 creates tests
# 5. AGT-SEMDOC-001 documents it
# 6. AGT-DEPLOY-001 deploys it

# Human gets: Working email capability in minutes
```

## Agents All The Way Down

Every piece of functionality is an agent:
- Database access? AGT-DATA-001
- Authentication? AGT-AUTH-001  
- Monitoring? AGT-MONITOR-001
- Even the orchestrator? AGT-ORCHESTRATE-001

The system becomes a swarm of specialized agents, each perfect at its one job, working in harmony.

This is how we achieve true autonomous development.
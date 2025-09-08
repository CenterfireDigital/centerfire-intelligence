# General Todo List - Centerfire Intelligence

*Last Updated: 2025-09-07*

## Infrastructure & Security

### High Priority
- [ ] **Sensitive data scrubber (PII)** for the Redis intake agent to be added as another agent
  - Create AGT-PII-SCRUBBER-1 to sanitize data before Redis storage
  - Handle SSNs, credit cards, emails, phone numbers, names, addresses
  - Integration point: Redis streams before Weaviate/Neo4j/ClickHouse consumption
  - Added: 2025-09-07

### Medium Priority
- [ ] **Session management cleanup** - Remove orphaned background bash sessions
- [ ] **Container health monitoring** - Automated alerts for service failures
- [ ] **Backup strategy** - Regular backups of Redis, Neo4j, Weaviate data

## Agent Development

### Stage 1 (Traditional + RBAC)
- [ ] **AGT-SEMDOC-PARSER-1** - Extract @semblock contracts from source files
- [ ] **AGT-SEMDOC-REGISTRY-1** - Manage contract lifecycle and storage
- [ ] **AGT-SEMDOC-VALIDATOR-1** - Validate contract syntax and semantics
- [ ] **Casbin integration** - gRPC client implementation for agent authorization

### Future Stages
- [ ] **Stage 2 preparation** - Add @semblock comments to existing agents
- [ ] **Stage 3 preparation** - Contract validation framework
- [ ] **Stage 4 preparation** - Full contract enforcement system

## Documentation & Maintenance
- [ ] **API documentation** - Document all agent communication protocols
- [ ] **Deployment guides** - Step-by-step setup for new environments
- [ ] **Performance benchmarking** - Baseline metrics for all services
- [ ] **Security audit** - Review all agent communication channels

## LexicRoot Manufacturing Preparation
- [ ] **Content generation contracts** - Define behavioral specifications
- [ ] **Account management contracts** - Define operational boundaries  
- [ ] **Infrastructure coordination contracts** - Define timing and coordination rules
- [ ] **Brand consistency contracts** - Define content quality standards

---

*This is a general todo list for infrastructure and development tasks not specific to SemDoc implementation. For SemDoc-specific tasks, see the staged implementation plan.*
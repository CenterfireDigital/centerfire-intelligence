# SemDoc Contracts: Detailed Specifications

## Overview
Machine-enforceable specifications that define the complete behavioral contract of code elements, enabling AI assistants to reason about impact, dependencies, and correctness.

## Contract Structure

### Core Components

```yaml
contract:
  preconditions:
    - description: "What must be true before execution"
      type: "assertion | assumption | requirement"
      validation: "runtime | compile-time | static-analysis"
  
  postconditions:
    - description: "What is guaranteed after execution"
      type: "guarantee | side-effect | state-change"
      validation: "test | proof | monitoring"
  
  invariants:
    - description: "What must always remain true"
      type: "data | state | security | performance"
      scope: "function | class | module | system"
  
  effects:
    all:
      reads: ["list of data/resources read"]
      writes: ["list of data/resources written"]
      calls: ["external functions/services called"]
      throws: ["exceptions that may be raised"]
      allocates: ["resources allocated"]
      releases: ["resources released"]
```

## Contract Types

### 1. Data Contracts
Define how data flows through the system:

```yaml
data_contract:
  input:
    schema: "JSON Schema or type definition"
    validation: "strict | lenient | custom"
    transformations: ["normalization", "sanitization"]
  
  output:
    schema: "JSON Schema or type definition"
    guarantees: ["non-null", "normalized", "validated"]
  
  state_mutations:
    - target: "database.users"
      operation: "update"
      fields: ["last_login", "session_count"]
```

### 2. Performance Contracts
Specify performance requirements and budgets:

```yaml
performance_contract:
  time_complexity: "O(n log n)"
  space_complexity: "O(n)"
  
  budgets:
    execution_time:
      p50: "10ms"
      p95: "50ms"
      p99: "100ms"
      max: "500ms"
    
    memory:
      heap: "100MB"
      stack: "1MB"
    
    io:
      database_queries: 5
      network_calls: 2
      file_operations: 0
```

### 3. Security Contracts
Define security requirements and constraints:

```yaml
security_contract:
  authentication:
    required: true
    methods: ["jwt", "oauth2"]
    roles: ["admin", "user"]
  
  authorization:
    permissions: ["read:users", "write:own-profile"]
    data_access: "row-level"
  
  data_handling:
    pii_fields: ["email", "ssn", "credit_card"]
    encryption: "at-rest and in-transit"
    retention: "90 days"
    deletion: "hard-delete with audit"
  
  compliance:
    frameworks: ["GDPR", "HIPAA", "SOC2"]
    audit_events: ["access", "modification", "deletion"]
```

### 4. Concurrency Contracts
Specify thread-safety and concurrent behavior:

```yaml
concurrency_contract:
  thread_safety: "thread-safe | not-thread-safe | conditionally-safe"
  
  synchronization:
    locks: ["mutex:user_data", "rwlock:cache"]
    atomic_operations: ["counter_increment"]
  
  ordering_guarantees:
    happens_before: ["init() before start()"]
    synchronizes_with: ["producer synchronizes-with consumer"]
  
  deadlock_prevention:
    lock_ordering: ["always acquire user_lock before db_lock"]
    timeout: "5s"
```

## Contract Validation

### Static Validation
Checked at development time:

```yaml
static_validation:
  tools:
    - type_checker: "mypy | typescript | flow"
    - contract_verifier: "custom AST analysis"
    - security_scanner: "semgrep | codeql"
  
  rules:
    - "all functions must have contracts"
    - "PII handlers must specify encryption"
    - "database operations must specify transactions"
```

### Runtime Validation
Checked during execution:

```yaml
runtime_validation:
  mode: "development | staging | production"
  
  development:
    check_preconditions: true
    check_postconditions: true
    check_invariants: true
    performance_tracking: true
  
  production:
    check_preconditions: false  # Already validated
    check_postconditions: sample(0.01)  # 1% sampling
    check_invariants: critical_only
    performance_tracking: true
```

### Test Validation
Verified through test oracles:

```yaml
test_validation:
  oracle:
    type: "deterministic | statistical | property-based"
    
    property_tests:
      - "output is always normalized"
      - "no PII in logs"
      - "response time < 100ms for n < 1000"
    
    regression_tests:
      - "maintains backward compatibility"
      - "preserves performance characteristics"
```

## Contract Evolution

### Versioning
How contracts change over time:

```yaml
contract_version:
  version: "2.0.0"
  compatible_with: ["1.x"]
  
  changes:
    breaking:
      - "removed support for MD5 hashing"
      - "changed return type from int to string"
    
    non_breaking:
      - "added optional timeout parameter"
      - "improved performance by 50%"
    
  migration:
    from_1_x:
      - "update hash algorithm to SHA256"
      - "convert integer IDs to UUIDs"
```

### Deprecation
Managing contract deprecation:

```yaml
deprecation:
  status: "deprecated"
  since: "2.1.0"
  removal_version: "3.0.0"
  alternative: "use processUserV2() instead"
  
  migration_guide:
    - "change function call from processUser to processUserV2"
    - "add new required 'context' parameter"
    - "handle new error types"
```

## Contract Composition

### Inheritance
How contracts inherit from parent contracts:

```yaml
inheritance:
  extends: "BaseProcessor"
  
  inherited_contracts:
    - preconditions: all
    - postconditions: all
    - invariants: all
  
  overrides:
    performance:
      execution_time:
        p95: "25ms"  # Tighter than parent's 50ms
  
  additions:
    effects:
      calls: ["cache.get", "cache.set"]
```

### Composition
Combining multiple contracts:

```yaml
composition:
  includes:
    - "Cacheable"
    - "Retryable"
    - "Auditable"
  
  conflict_resolution:
    performance: "most_restrictive"
    security: "union_all"
    effects: "union_all"
```

## Contract Enforcement

### CI/CD Pipeline
Automated contract enforcement:

```yaml
ci_enforcement:
  pre_commit:
    - validate_contract_syntax
    - check_contract_completeness
  
  pull_request:
    - verify_contract_changes
    - run_contract_tests
    - measure_performance_budgets
  
  deployment:
    - validate_production_contracts
    - configure_runtime_checks
    - setup_monitoring_alerts
```

### Monitoring & Alerting
Runtime contract monitoring:

```yaml
monitoring:
  metrics:
    - contract_violations_total
    - precondition_failures_rate
    - performance_budget_exceeded
  
  alerts:
    - name: "Critical invariant violated"
      condition: "invariant_violations > 0"
      severity: "page"
    
    - name: "Performance degradation"
      condition: "p95_latency > contract.performance.p95"
      severity: "warning"
```

## AI Integration

### Contract-Aware Code Generation
How AI uses contracts for code generation:

```yaml
ai_generation:
  input:
    - function_signature
    - contract_specification
    - existing_codebase_context
  
  validation:
    - generated_code_meets_preconditions
    - generated_code_ensures_postconditions
    - maintains_all_invariants
  
  optimization:
    - minimize_effects
    - meet_performance_budgets
    - maintain_security_requirements
```

### Impact Analysis
How AI reasons about contract changes:

```yaml
impact_analysis:
  change_type: "tightening_precondition"
  
  affected:
    direct_callers: 15
    transitive_callers: 127
    
  risk_assessment:
    - "3 callers may not meet new precondition"
    - "performance impact on 7 critical paths"
    - "no security implications"
  
  migration_plan:
    - "update 3 non-compliant callers"
    - "add validation layer for external calls"
    - "deploy with feature flag"
```

## Examples

### Complete Function Contract
Real-world example:

```python
# @semblock
# contract:
#   preconditions:
#     - "user_id exists in database"
#     - "user has write permissions"
#     - "document is not locked"
#   postconditions:
#     - "document saved to database"
#     - "version number incremented"
#     - "audit log entry created"
#   invariants:
#     - "document.owner never changes"
#     - "document.created_at never changes"
#   effects:
#     reads: ["database.users", "database.documents"]
#     writes: ["database.documents", "database.audit_log"]
#     calls: ["validator.validate", "cache.invalidate"]
#   performance:
#     p95: "100ms"
#     database_queries: 3
#   security:
#     requires_auth: true
#     pii_handling: "logs sanitized"
def save_document(user_id: str, doc_id: str, content: str) -> Document:
    # Implementation here
    pass
```

### Service-Level Contract
Microservice contract example:

```yaml
# /sem-doc/contracts/user-service.yaml
service_contract:
  name: "UserService"
  version: "1.0.0"
  
  endpoints:
    - path: "/users/{id}"
      method: "GET"
      contract:
        rate_limit: "1000/minute"
        cache: "5 minutes"
        response_time_p99: "50ms"
        
    - path: "/users"
      method: "POST"
      contract:
        rate_limit: "10/minute"
        validation: "strict"
        effects: ["creates user", "sends email"]
        rollback: "supported"
  
  dependencies:
    - service: "EmailService"
      contract: "async-delivery"
      fallback: "queue for retry"
    
    - service: "Database"
      contract: "ACID transactions"
      timeout: "5s"
  
  sla:
    availability: "99.95%"
    error_rate: "< 0.1%"
    response_time_p99: "100ms"
```

## Best Practices

### Contract Design
1. **Start simple**: Begin with basic pre/postconditions
2. **Be explicit**: State all effects and side effects
3. **Think defensively**: Include error conditions
4. **Version carefully**: Plan for contract evolution
5. **Test thoroughly**: Contracts are only as good as their validation

### Common Patterns
1. **Builder pattern**: Accumulate preconditions through method chain
2. **Transaction pattern**: All-or-nothing with rollback support
3. **Retry pattern**: Idempotent operations with exponential backoff
4. **Circuit breaker**: Fail fast when contracts repeatedly violated
5. **Compensation pattern**: Define reverse operations for rollback

### Anti-patterns to Avoid
1. **Over-specification**: Don't constrain implementation details
2. **Under-specification**: Don't leave critical behaviors undefined
3. **Circular dependencies**: Avoid contracts that reference each other
4. **Runtime-only validation**: Catch issues early with static analysis
5. **Ignored violations**: Always handle contract failures explicitly

---

*This document specifies the complete contract system for SemDoc, enabling machine-enforceable specifications that AI assistants can reason about and validate.*
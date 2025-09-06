# Universal AI-Native Naming Convention

## Purpose
A naming convention that prioritizes AI comprehension and navigation over human preferences, applicable to any AI-maintained project.

## Core Principles

1. **Semantic First**: Names encode meaning, not just identity
2. **Graph-Friendly**: Every name can be a node in a knowledge graph
3. **Stable Anchors**: Core concepts get permanent IDs that never change
4. **Self-Describing**: Names contain their type, domain, and purpose
5. **Machine-Parseable**: Consistent patterns that regex/AI can decompose

## Universal Naming Patterns

### 1. Capability Identifiers
```
CAP-<DOMAIN>-<SEQUENCE>[-<descriptor>]
```
- **DOMAIN**: 3-4 letter semantic domain (LLM, CTX, AUTH, DATA, UI, NET, etc.)
- **SEQUENCE**: 3-digit number (001-999)
- **descriptor**: Optional human-friendly name (kebab-case)

**Examples:**
- `CAP-AUTH-001-user-authentication`
- `CAP-LLM-002-model-selection`
- `CAP-DATA-001-persistence`

### 2. Graph IDs (Universal Resource Identifiers)
```
<namespace>:<type>/<capability>/<name>[#version]
```
- **namespace**: Project or org identifier
- **type**: Node type (Function, Service, Component, Contract, Test, etc.)
- **capability**: Parent capability ID
- **name**: Specific identifier
- **version**: Optional semantic version

**Examples:**
- `centerfire:Function/CAP-LLM-001/routeRequest#1.0`
- `acme:Service/CAP-AUTH-001/TokenService`
- `project:Contract/CAP-DATA-001/UserSchema#2.1`

### 3. Agent & Tool Names
```
AGENT-<PURPOSE>-<VARIANT>
TOOL-<ACTION>-<TARGET>
```

**Agent Examples:**
- `AGENT-REFACTOR-SAFE`
- `AGENT-TEST-RUNNER`
- `AGENT-DEPLOY-CANARY`

**Tool Examples:**
- `TOOL-EXEC-SHELL`
- `TOOL-QUERY-GRAPH`
- `TOOL-WRITE-FILE`

### 4. File Naming
```
<purpose>.<namespace>.<type>.<ext>
```
- **purpose**: What the file does
- **namespace**: Capability or module namespace
- **type**: File type (service, lib, test, config, schema)
- **ext**: Language extension

**Examples:**
- `router.llm.service.go`
- `auth.cap001.test.py`
- `dashboard.ui.component.tsx`
- `user.data.schema.json`

### 5. Contract & Schema Names
```
<domain>.<operation>.<version>.<format>
```

**Examples:**
- `llm.request.v1.proto`
- `auth.token.v2.json`
- `data.user.v1.graphql`

### 6. Event & Telemetry Names
```
<capability>.<entity>.<action>[.<result>]
```

**Examples:**
- `llm.request.started`
- `auth.token.validated.success`
- `data.user.created`
- `ctx.archive.compressed.failed`

### 7. Configuration Keys
```
<capability>.<component>.<property>
```

**Examples:**
- `llm.router.timeout_ms`
- `auth.jwt.secret_key`
- `data.postgres.max_connections`

### 8. Test Names
```
<target>_<scenario>_<expectation>
```

**Examples:**
- `routeRequest_highLoad_returnsWithin100ms`
- `authToken_expired_throwsUnauthorized`
- `contextArchive_largePayload_compressesSuccessfully`

### 9. Semantic Commit Prefixes
```
<capability>/<change-type>: <description>
```

**Change Types:**
- `feat` - New feature
- `fix` - Bug fix
- `perf` - Performance improvement
- `refactor` - Code restructuring
- `contract` - Contract change
- `test` - Test changes
- `docs` - Documentation

**Examples:**
- `CAP-LLM-001/feat: Add GPT-4 support to router`
- `CAP-AUTH-001/fix: Token expiration check`
- `CAP-CTX-001/perf: Optimize compression algorithm`

### 10. Environment & Deployment Names
```
<product>-<environment>-<region>[-<variant>]
```

**Examples:**
- `centerfire-prod-us-east`
- `centerfire-staging-eu`
- `centerfire-dev-local`
- `acme-prod-asia-canary`

## Hierarchical Naming

### Directory Paths Follow Semantic Hierarchy
```
/capabilities/CAP-<DOMAIN>-<SEQ>/impl/<role>/<language>/
```

### Import Paths Include Semantic Context
```go
import "centerfire/capabilities/llm/orchestration/router"
```

```python
from capabilities.auth.token import TokenService
```

```typescript
import { Dashboard } from '@capabilities/ui/dashboard';
```

## Reserved Prefixes

These prefixes are reserved for system use:

- `SYS-` : System-level components
- `TEST-` : Test-only artifacts
- `TEMP-` : Temporary/ephemeral items
- `DEPRECATED-` : Marked for removal
- `EXPERIMENTAL-` : Unstable/research code

## Anti-Patterns to Avoid

### ❌ Bad Names
- `utils.js` - Too vague
- `helpers.py` - No semantic meaning
- `index.ts` - No context
- `main.go` - Insufficient detail
- `stuff.rs` - Meaningless

### ✅ Good Names
- `auth.token.lib.js`
- `llm.request.helpers.py`
- `dashboard.entry.component.ts`
- `orchestrator.main.service.go`
- `context.compression.lib.rs`

## Validation Rules

### 1. Capability IDs
- Must match: `^CAP-[A-Z]{2,4}-\d{3}(-[a-z-]+)?$`
- Domain must be registered in `/semdoc/domains.yaml`

### 2. Graph IDs
- Must match: `^[a-z]+:[A-Z][a-z]+\/CAP-[A-Z]+-\d{3}\/[a-zA-Z]+`
- Must resolve to actual code element

### 3. File Names
- Must match: `^[a-z-]+\.[a-z]+\.(service|lib|test|schema|config)\.[a-z]+$`
- Purpose must be a verb or noun, not adjective

### 4. Events
- Must match: `^[a-z]+\.[a-z]+\.[a-z]+(\.[a-z]+)?$`
- Must be registered in capability's `telemetry/events.yaml`

## Language-Specific Adaptations

### Go
```go
package orchestration // Package names are single words
type LLMRouter struct {} // Types are PascalCase
func (r *LLMRouter) RouteRequest() {} // Public methods are PascalCase
```

### Rust
```rust
mod llm_orchestration; // Modules are snake_case
struct LLMRouter {} // Types are PascalCase
impl LLMRouter {
    pub fn route_request() {} // Functions are snake_case
}
```

### TypeScript/JavaScript
```typescript
// Files: dashboard.ui.component.tsx
export class DashboardComponent {} // Classes are PascalCase
export function routeRequest() {} // Functions are camelCase
export const MAX_TIMEOUT_MS = 5000; // Constants are SCREAMING_SNAKE_CASE
```

### Python
```python
# Files: llm_router.service.py
class LLMRouter: # Classes are PascalCase
    def route_request(self): # Methods are snake_case
        pass
MAX_TIMEOUT_MS = 5000 # Constants are SCREAMING_SNAKE_CASE
```

## Tooling Support

### Validation Script
```bash
# validate-names.sh
#!/bin/bash
# Validates all names in the repository against conventions

# Check capability IDs
find capabilities -maxdepth 1 -type d | grep -v "^capabilities$" | while read dir; do
    basename "$dir" | grep -qE "^CAP-[A-Z]{2,4}-[0-9]{3}(-[a-z-]+)?$" || echo "Invalid: $dir"
done

# Check file names
find . -type f -name "*.go" -o -name "*.rs" -o -name "*.ts" -o -name "*.py" | while read file; do
    basename "$file" | grep -qE "^[a-z-]+\.[a-z]+\.(service|lib|test|schema|config)\." || echo "Invalid: $file"
done
```

### IDE Integration
Configure IDE to enforce naming:
- Template for new files
- Linter rules for names
- Auto-complete with conventions

## Evolution & Governance

### Adding New Patterns
1. Propose in `/semdoc/naming-proposals.yaml`
2. AI validates against existing patterns
3. Test with sample implementation
4. Update this document
5. Update validation tooling

### Deprecating Patterns
1. Mark as deprecated with timeline
2. AI assists in migration
3. Update validation to warn
4. Remove after migration period

## Success Metrics

- **AI Navigation Speed**: Time to locate code element by semantic description
- **Naming Conflicts**: Near-zero collision rate
- **Pattern Compliance**: >95% automated validation pass rate
- **Cross-Language Consistency**: Same concept, same naming pattern
- **Graph Completeness**: Every named element is a graph node

## Conclusion

This naming convention prioritizes:
1. **Machine comprehension** over human preference
2. **Semantic clarity** over brevity
3. **Stability** over flexibility
4. **Patterns** over creativity

When AI maintains the code, names become the primary navigation system. This convention ensures that navigation is deterministic, semantic, and universal.
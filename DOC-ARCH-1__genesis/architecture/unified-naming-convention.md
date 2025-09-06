# Unified AI-First Naming Convention v2.0

> **The Best of Both Worlds**: Combining GPT's immutability with Claude's practicality

## Core Innovation: The Triad System

Every entity in the system has three names:

### 1. **Canonical ID (CID)** - The Immutable Truth
```
cid:centerfire:capability:01J9F7Z8Q4R5ZV3J4X19M8YZTW
```
- **Never changes**, even if renamed or moved
- **ULID-based**: Time-sortable, globally unique
- **AI navigation anchor**: What AI uses internally

### 2. **Semantic Slug** - The Stable Reference
```
CAP-AUTH-001
```
- **Rarely changes**, can have aliases
- **Human-readable**: What developers type
- **Pattern-based**: Follows strict conventions

### 3. **Display Name** - The Friendly Face
```
"Authentication Service"
```
- **Changes freely**: For UI/documentation
- **Natural language**: What users see
- **No downstream impact**: Pure presentation

## Universal Naming Patterns

### Capability Identifiers
```
CID:  cid:centerfire:capability:01J9F7Z8Q4R5ZV3J4X19M8YZTW
Slug: CAP-<DOMAIN>-<N>  # No zero padding!
Dir:  CAP-AUTH-1__01J9F7Z8/   # Slug + ULID first 8 chars
```

### Graph IDs (for Neo4j)
```
lexi:<Type>/<CapabilitySlug>/<Name>
lexi:Function/CAP-AUTH-1/issueToken
lexi:Service/CAP-LLM-1/Orchestrator
```

### URNs (for Effects & Resources)
```
urn:lexi:db:CAP-AUTH-1:users
urn:lexi:event:CAP-AUTH-1:auth.issued.v1
urn:lexi:endpoint:CAP-AUTH-1:POST:/v1/auth/tokens
```

## Directory Structure

### Capability Folders
```
/capabilities/
  CAP-AUTH-1__01J9F7Z8/          # Slug + ULID tail for uniqueness
    .id                           # Contains full CID
    semdoc.yaml                   # Capability contract
    contracts/                    # Interface definitions
    impl/                         # Implementations
      service/go/
      lib/rust/
      ui/react/
    tests/
    telemetry/
```

### Why ULID Suffix?
- Prevents collisions during merges
- Enables multiple versions during migration
- Makes every directory globally unique

## File Naming (Practical)

### Clean Semantic Names (Not Verbose)
```
✅ Good (Our Choice):
auth.token.service.go
router.orchestration.lib.rs
dashboard.metrics.component.tsx

❌ Too Verbose (Avoided):
issueToken.lexi.Function.CAP-AUTH-001.issueToken.ts
```

### Pattern
```
<purpose>.<domain>.<type>.<ext>
```

### Language-Specific Adaptations

#### Go
```go
// File: orchestrator.llm.service.go
package orchestration
type LLMOrchestrator struct {}
func (o *LLMOrchestrator) RouteRequest() {}
```

#### Rust
```rust
// File: context.archive.lib.rs
mod context_archive;
struct ContextArchive {}
impl ContextArchive {
    pub fn compress_context() {}
}
```

#### TypeScript/React
```typescript
// File: dashboard.metrics.component.tsx
export const MetricsDashboard: React.FC = () => {}
export function calculateMetrics() {}
const MAX_METRICS = 1000;
```

#### Python
```python
# File: llm_router.orchestration.service.py
class LLMRouter:
    def route_request(self):
        pass
MAX_TIMEOUT_MS = 5000
```

## Event & Telemetry Naming

### Events (Dot-Namespaced)
```
Slug: EVT-AUTH-001.auth.issued.v1
URN:  urn:lexi:event:CAP-AUTH-001:auth.issued.v1
Code: "auth.issued.v1"  # What you emit
```

### Metrics
```
Slug: MET-AUTH-001.latency
Code: "auth.latency.p95"
```

### Structured Logs
```
Slug: LOG-AUTH-001.access.v1
Schema: /capabilities/CAP-AUTH-001/telemetry/logs.schema.json
```

## Test Naming (Descriptive)

### Pattern: `<target>_<scenario>_<expectation>`
```
✅ Clear Test Names:
routeRequest_highLoad_returnsWithin100ms
authToken_expired_throwsUnauthorized
contextArchive_largePayload_compressesSuccessfully

❌ Vague Test Names:
test1
testAuth
checkFunction
```

### File Pattern
```
test_<function>_<scenario>.spec.ts
test_auth_token_expiry.spec.go
```

## Cross-Reference System

### When to Use What

| Context | Use | Example |
|---------|-----|---------|
| **Code References** | Graph ID | `lexi:Function/CAP-AUTH-001/issueToken` |
| **Effects** | URN | `urn:lexi:db:CAP-AUTH-001:users` |
| **Human Communication** | Slug | `CAP-AUTH-001` |
| **Internal Storage** | CID | `cid:centerfire:capability:01J9F7Z8...` |
| **UI Display** | Display Name | "Authentication Service" |

## Evolution & Aliases

### Renaming Without Breaking

```yaml
# /semdoc/catalog.yaml
capabilities:
  - cid: cid:centerfire:capability:01J9F7Z8Q4R5ZV3J4X19M8YZTW
    slug: CAP-AUTH-1
    aliases: 
      - CAP-IDENTITY-1    # Old name
      - CAP-AUTHN-1        # Alternative name
    display: "Authentication Service"
```

### Directory Moves
```bash
# .id file at directory root
$ cat /capabilities/CAP-AUTH-001__01J9F7Z8/.id
cid:centerfire:capability:01J9F7Z8Q4R5ZV3J4X19M8YZTW

# Even if moved, tools can find by CID
```

## Validation Rules (SemDoc Contract)

### Machine-Enforceable Policy
```yaml
# /semdoc/policies/naming.yaml
version: "1.0"
namespace: "centerfire"

ids:
  canonical:
    format: "cid:<namespace>:<kind>:<ULID>"
    ulid_length: 26
    
  slugs:
    capability: 
      regex: "^CAP-[A-Z]{2,4}-[0-9]+$"  # No padding!
      max_length: 20
    function:
      regex: "^FN-[A-Z]{2,4}-[0-9]+-[a-zA-Z]+$"
      max_length: 50
      
  graph_ids:
    regex: "^lexi:[A-Z][a-z]+/CAP-[A-Z]+-[0-9]+/[a-zA-Z]+$"
    max_length: 100
    
  urns:
    regex: "^urn:lexi:(db|event|endpoint|topic):[A-Z-]+:[a-z.]+$"
    max_length: 150

validation:
  enforce_on_commit: true
  block_invalid_names: true
  require_cid_in_semdoc: true
```

## Anti-Patterns to Avoid

### ❌ Never Do This
```
utils.js                    # Too vague
helpers.py                  # No semantic meaning
stuff.go                    # Meaningless
index.ts                    # No context
CAP-AUTH-authentication    # Mixing conventions
MyAuthService              # Personal naming
auth-service-v2            # Version in name (use CID)
```

### ✅ Always Do This
```
auth.token.lib.js          # Clear purpose
llm.request.helpers.py     # Domain-specific
orchestrator.main.go       # Semantic meaning
dashboard.entry.ts         # Clear entry point
CAP-AUTH-1                # Simple integers
AuthenticationService     # Display name only
CAP-AUTH-2                # Next capability
```

## Reserved Prefixes

| Prefix | Purpose | Example |
|--------|---------|---------|
| `AGT-` | Agent components | `AGT-BOOTSTRAP-1` |
| `TEST-` | Test artifacts | `TEST-FIXTURE-001` |
| `TEMP-` | Temporary items | `TEMP-MIGRATION-001` |
| `DEPRECATED-` | Marked for removal | `DEPRECATED-CAP-OLD-001` |
| `EXPERIMENTAL-` | Unstable/research | `EXPERIMENTAL-AI-001` |

## Commit Message Convention

### Semantic Commits with Capability Context
```
CAP-AUTH-1/feat: Add OAuth2 support

CID: cid:centerfire:capability:01J9F7Z8Q4R5ZV3J4X19M8YZTW
Graph-IDs: 
  - lexi:Function/CAP-AUTH-001/validateOAuth
  - lexi:Endpoint/CAP-AUTH-001/oauth/callback
Effects:
  - urn:lexi:db:CAP-AUTH-001:oauth_tokens [write]
  - urn:lexi:event:CAP-AUTH-001:auth.oauth.completed.v1 [emit]
```

## Success Metrics

### How We Know It's Working

1. **AI Navigation Speed**: <100ms to locate any entity by semantic description
2. **Zero Broken References**: CIDs never break, even during renames
3. **Pattern Compliance**: >95% automated validation pass rate
4. **Cross-Language Consistency**: Same concept = same naming pattern
5. **Graph Completeness**: Every named entity is a Neo4j node
6. **Rename Safety**: Aliases prevent breaking changes
7. **Merge Conflict Reduction**: ULID suffixes prevent collisions

## Implementation Tooling

### Validation Script
```bash
#!/bin/bash
# validate-naming.sh

# Check for .id files
find capabilities -name ".id" -exec sh -c '
  dir=$(dirname "$1")
  cid=$(cat "$1")
  if [[ ! "$cid" =~ ^cid:centerfire:[a-z]+:[A-Z0-9]{26}$ ]]; then
    echo "Invalid CID in $dir"
  fi
' _ {} \;

# Validate directory names
find capabilities -maxdepth 1 -type d | while read dir; do
  name=$(basename "$dir")
  if [[ ! "$name" =~ ^CAP-[A-Z]{2,4}-[0-9]+__[A-Z0-9]{8}$ ]]; then
    echo "Invalid directory name: $name"
  fi
done
```

### Git Hooks
```bash
# .git/hooks/pre-commit
#!/bin/bash
./scripts/validate-naming.sh || exit 1
./scripts/check-semdoc-cids.sh || exit 1
```

## Migration Path

### From Old to New
1. **Generate CIDs** for all existing entities
2. **Add .id files** to all capability directories
3. **Create aliases** for any existing names
4. **Update references** gradually using aliases
5. **Validate continuously** with CI checks

## The Revolution

This naming convention represents:
- **Immutability** through CIDs (GPT's insight)
- **Practicality** through clean file names (Claude's insight)
- **Evolution** through aliases (GPT's insight)
- **Education** through anti-patterns (Claude's insight)
- **Enforcement** through SemDoc contracts (GPT's insight)
- **Clarity** through examples (Claude's insight)

We're not choosing between human and machine needs.
We're satisfying both with a triad system that gives each audience what it needs.

**This is how AI-maintained systems should be named.**
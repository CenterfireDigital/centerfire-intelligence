# Naming Convention Comparison: Claude vs GPT

## Core Philosophy Comparison

### GPT's Approach: **Triad System**
- **Canonical ID (CID)**: Immutable ULID - `cid:centerfire:capability:01J9F7Z8Q4R5ZV3J4X19M8YZTW`
- **Semantic Slug**: Stable human name - `CAP-AUTH-001`
- **Display Name**: Free-form - "Authentication Service"

### Claude's Approach: **Single Semantic ID**
- **Capability ID**: `CAP-AUTH-001-user-authentication`
- **Graph ID**: `centerfire:Function/CAP-AUTH-001/routeRequest#1.0`
- **No separate display name system**

**Winner: GPT's Triad** ✅
- Solves rename problem permanently
- Allows human-friendly evolution without breaking references
- CIDs provide true immutability for AI navigation

## Directory Structure

### GPT's Approach
```
CAP-AUTH-001__01J9F7Z8/    # Slug + ULID tail
  .id                       # Contains full CID
  semdoc.yaml
```

### Claude's Approach
```
CAP-AUTH-001-user-authentication/
  semdoc.yaml
```

**Winner: GPT's Approach** ✅
- ULID tail prevents collisions
- .id file enables directory moves
- More robust for merges/imports

## File Naming

### GPT's Approach
```
issueToken.lexi.Function.CAP-AUTH-001.issueToken.ts
```

### Claude's Approach
```
auth.token.service.go
router.llm.service.ts
```

**Winner: Claude's Approach** ✅
- GPT's is too verbose
- Claude's is cleaner while still semantic
- Easier to type and read

## Cross-Reference System

### GPT's Approach
- Graph IDs: `lexi:Function/CAP-AUTH-001/issueToken`
- URNs for effects: `urn:lexi:event:CAP-AUTH-001:auth.issued.v1`
- Dual system for different contexts

### Claude's Approach
- Single Graph ID: `centerfire:Function/CAP-AUTH-001/routeRequest#1.0`
- No separate URN system

**Winner: GPT's URN System** ✅
- URNs better for effects specification
- Clear separation of concerns
- More extensible

## Event & Telemetry Naming

### GPT's Approach
```
EVT-AUTH-001.auth.issued.v1
MET-auth.cap.latency
LOG-auth.cap.access.v1
```

### Claude's Approach
```
auth.token.validated.success
llm.request.started
```

**Winner: Tie** 🤝
- GPT's is more formal/traceable
- Claude's is more readable
- Combine: Use GPT's slugs with Claude's patterns

## Test Naming

### GPT's Approach
```
TEST-AUTH-001-issueToken-validates_expiry
test_issueToken__validates_expiry.lexi.Test.CAP-AUTH-001.issueToken.spec.ts
```

### Claude's Approach
```
routeRequest_highLoad_returnsWithin100ms
authToken_expired_throwsUnauthorized
```

**Winner: Claude's Approach** ✅
- More descriptive test names
- Clear scenario_expectation pattern
- Easier to understand test purpose

## Language Adaptations

### GPT's Approach
- Brief mentions, relies on language conventions

### Claude's Approach
- Detailed per-language guidelines
- Examples for Go, Rust, TypeScript, Python
- Clear constant/variable patterns

**Winner: Claude's Approach** ✅
- More comprehensive
- Practical examples
- Respects language idioms

## Rename & Evolution

### GPT's Approach
```yaml
capabilities:
  - cid: cid:centerfire:capability:01J9F7Z8...
    slug: CAP-AUTH-001
    aliases: [CAP-IDENTITY-001]  # Old name
```

### Claude's Approach
- No formal alias system
- Relies on git history

**Winner: GPT's Approach** ✅
- Explicit alias tracking
- Supports gradual migration
- AI can understand evolution

## Validation & Enforcement

### GPT's Approach
```yaml
# /sem-doc/policies/naming.yaml
ids:
  slugs:
    capability: 
      regex: "^CAP-[A-Z]{2,12}-[0-9]{3}(-[a-z0-9]+)*$"
      max: 80
```

### Claude's Approach
- Validation rules in documentation
- Simple bash script example
- Less formal enforcement

**Winner: GPT's Approach** ✅
- Machine-readable validation
- CI/CD integration ready
- SemDoc contract for names

## Anti-Patterns & Education

### GPT's Approach
- Minimal "what not to do"

### Claude's Approach
- Clear anti-patterns section
- Bad vs Good examples
- Educational focus

**Winner: Claude's Approach** ✅
- Better for onboarding
- Prevents common mistakes
- More pedagogical

## Summary Scorecard

| Aspect | GPT | Claude | Winner |
|--------|-----|--------|--------|
| Core System | Triad (CID+Slug+Display) | Single Semantic | GPT ✅ |
| Directory Structure | ULID suffix + .id file | Simple semantic | GPT ✅ |
| File Naming | Verbose with full graph ID | Clean semantic | Claude ✅ |
| Cross-References | URN + Graph ID | Single Graph ID | GPT ✅ |
| Events/Telemetry | Formal slugs | Readable patterns | Tie 🤝 |
| Test Naming | Basic | Scenario_Expectation | Claude ✅ |
| Language Guidelines | Minimal | Comprehensive | Claude ✅ |
| Rename/Evolution | Alias system | None | GPT ✅ |
| Validation | SemDoc contract | Documentation | GPT ✅ |
| Education | Minimal | Anti-patterns | Claude ✅ |

**Final Score: GPT 6, Claude 5, Tie 1**

## Synthesis Recommendation

Combine the best of both:
1. **Use GPT's triad system** for core identity
2. **Use Claude's file naming** for practicality
3. **Use GPT's URN system** for effects
4. **Use Claude's test patterns** for clarity
5. **Use Claude's language guidelines** for completeness
6. **Use GPT's alias system** for evolution
7. **Use GPT's SemDoc validation** for enforcement
8. **Use Claude's anti-patterns** for education

This creates the ultimate AI-first naming convention!
# SemDoc Implementation Design

## Overview
Practical design for implementing SemDoc - the semantic documentation system that enables machine-understanding of code through contracts and semantic blocks.

## Core Components

### 1. SemBlock Format Specification

#### Basic Structure
```yaml
# @semblock
# version: 1.0
# contract:
#   id: "unique-contract-id"  # Optional, auto-generated if missing
#   preconditions: []
#   postconditions: []
#   invariants: []
#   effects:
#     reads: []
#     writes: []
#     calls: []
# metadata:
#   author: "developer-id"
#   created: "timestamp"
#   intent: "related-change-intent-id"
```

#### Language Integration
Different languages, same semantic structure:

**Python:**
```python
# @semblock
# contract:
#   preconditions: ["user.is_authenticated", "payment.amount > 0"]
#   postconditions: ["payment.status == 'completed'", "receipt.sent"]
def process_payment(user, payment):
    pass
```

**C++:**
```cpp
// @semblock
// contract:
//   preconditions: ["buffer != nullptr", "size > 0"]
//   postconditions: ["return >= 0", "buffer modified"]
//   performance:
//     time: "O(n)"
//     memory: "O(1)"
int process_buffer(char* buffer, size_t size) {
    // implementation
}
```

**JavaScript:**
```javascript
// @semblock
// contract:
//   preconditions: ["typeof data === 'object'", "data.id exists"]
//   postconditions: ["cache updated", "promise resolved"]
//   effects:
//     calls: ["cache.set", "api.notify"]
async function updateRecord(data) {
    // implementation
}
```

### 2. Parser Implementation

#### Multi-Language Parser Engine
```python
class SemDocParser:
    """Universal SemDoc parser with language-specific backends"""
    
    def __init__(self):
        self.parsers = {
            '.py': PythonSemDocParser(),
            '.cpp': CppSemDocParser(),
            '.js': JavaScriptSemDocParser(),
            '.rs': RustSemDocParser(),
        }
        self.validator = ContractValidator()
        self.registry = ContractRegistry()
    
    def parse_file(self, filepath: Path) -> List[SemBlock]:
        """Extract all SemBlocks from a file"""
        suffix = filepath.suffix
        parser = self.parsers.get(suffix, GenericRegexParser())
        
        blocks = parser.extract_blocks(filepath)
        validated = [self.validator.validate(b) for b in blocks]
        
        # Register contracts for lookup
        for block in validated:
            self.registry.register(block)
        
        return validated
```

#### Language-Specific Parsers

**Python Parser (AST-based):**
```python
import ast

class PythonSemDocParser:
    def extract_blocks(self, filepath: Path) -> List[SemBlock]:
        with open(filepath) as f:
            tree = ast.parse(f.read())
        
        blocks = []
        for node in ast.walk(tree):
            if isinstance(node, (ast.FunctionDef, ast.ClassDef)):
                # Extract docstring
                docstring = ast.get_docstring(node)
                if docstring and '@semblock' in docstring:
                    block = self.parse_semblock(docstring, node)
                    blocks.append(block)
                
                # Also check comments above
                comments = self.extract_comments_above(node)
                if '@semblock' in comments:
                    block = self.parse_semblock(comments, node)
                    blocks.append(block)
        
        return blocks
```

**Generic Regex Parser (fallback):**
```python
import re

class GenericRegexParser:
    SEMBLOCK_PATTERN = r'@semblock\s*(.*?)(?=\n(?:[^#]|$))'
    
    def extract_blocks(self, filepath: Path) -> List[SemBlock]:
        content = filepath.read_text()
        blocks = []
        
        for match in re.finditer(self.SEMBLOCK_PATTERN, content, re.MULTILINE | re.DOTALL):
            yaml_content = match.group(1)
            # Clean comment markers
            yaml_content = re.sub(r'^[#/*\s]+', '', yaml_content, flags=re.MULTILINE)
            
            try:
                data = yaml.safe_load(yaml_content)
                blocks.append(SemBlock(**data))
            except yaml.YAMLError as e:
                logger.warning(f"Invalid SemBlock in {filepath}: {e}")
        
        return blocks
```

### 3. Contract Validator

```python
class ContractValidator:
    """Validates contracts for consistency and completeness"""
    
    def validate(self, block: SemBlock) -> SemBlock:
        # Validate structure
        self._validate_structure(block)
        
        # Validate preconditions are checkable
        self._validate_preconditions(block.contract.preconditions)
        
        # Validate postconditions are verifiable
        self._validate_postconditions(block.contract.postconditions)
        
        # Validate effects are traceable
        self._validate_effects(block.contract.effects)
        
        # Check for conflicts
        self._check_conflicts(block)
        
        return block
    
    def _validate_preconditions(self, conditions: List[str]):
        """Ensure preconditions can be evaluated"""
        for condition in conditions:
            # Parse condition
            # Check if variables are accessible
            # Verify condition is deterministic
            pass
```

### 4. Contract Registry

```python
class ContractRegistry:
    """Central registry for all contracts in the system"""
    
    def __init__(self):
        self.redis = Redis()  # Fast lookup
        self.neo4j = Neo4j()  # Relationships
        self.qdrant = Qdrant()  # Semantic search
        
    def register(self, block: SemBlock):
        """Register a contract in all stores"""
        # Store in Redis for fast lookup
        key = f"contract:{block.id}"
        self.redis.set(key, block.to_json())
        
        # Store in Neo4j for relationship tracking
        self.neo4j.create_node(
            label="Contract",
            properties=block.to_dict()
        )
        
        # Store in Qdrant for semantic search
        embedding = self.generate_embedding(block)
        self.qdrant.upsert(
            collection="contracts",
            points=[{
                "id": block.id,
                "vector": embedding,
                "payload": block.to_dict()
            }]
        )
    
    def find_by_function(self, function_name: str) -> Optional[SemBlock]:
        """Find contract for a specific function"""
        return self.redis.get(f"contract:function:{function_name}")
    
    def find_similar(self, block: SemBlock, limit: int = 5) -> List[SemBlock]:
        """Find similar contracts using semantic search"""
        embedding = self.generate_embedding(block)
        results = self.qdrant.search(
            collection="contracts",
            query_vector=embedding,
            limit=limit
        )
        return [SemBlock(**r.payload) for r in results]
```

### 5. Runtime Integration

#### Pre-commit Hook
```python
#!/usr/bin/env python3
# .git/hooks/pre-commit

def check_semdocs():
    """Validate all SemBlocks in staged files"""
    parser = SemDocParser()
    
    # Get staged files
    staged_files = get_staged_files()
    
    errors = []
    for filepath in staged_files:
        try:
            blocks = parser.parse_file(filepath)
            for block in blocks:
                # Validate contract
                validator.validate(block)
                
                # Check if implementation matches contract
                if not verify_implementation(block, filepath):
                    errors.append(f"Contract violation in {filepath}")
        except Exception as e:
            errors.append(f"SemDoc error in {filepath}: {e}")
    
    if errors:
        print("SemDoc validation failed:")
        for error in errors:
            print(f"  - {error}")
        return 1
    
    return 0
```

#### IDE Integration (VS Code Extension)
```typescript
// Real-time contract validation in IDE
export function activate(context: vscode.ExtensionContext) {
    // Register diagnostic provider
    const diagnostics = vscode.languages.createDiagnosticCollection('semdoc');
    
    // Watch for changes
    vscode.workspace.onDidChangeTextDocument(event => {
        const document = event.document;
        
        // Parse SemBlocks
        const blocks = parseSemBlocks(document.getText());
        
        // Validate and show errors
        const errors = validateBlocks(blocks);
        diagnostics.set(document.uri, errors);
    });
    
    // Provide code completion for contracts
    vscode.languages.registerCompletionItemProvider('*', {
        provideCompletionItems(document, position) {
            // Suggest contract templates
            return generateContractSuggestions(document, position);
        }
    });
}
```

### 6. Contract Enforcement

#### Compile-Time Validation (C++)
```cpp
// Macro for compile-time contract checking
#define SEMBLOCK_CONTRACT(pre, post) \
    static_assert(ValidateContract<pre, post>::value, "Contract violation")

// Template metaprogramming for contract validation
template<typename Preconditions, typename Postconditions>
struct ValidateContract {
    static constexpr bool value = 
        CheckPreconditions<Preconditions>::value &&
        CheckPostconditions<Postconditions>::value;
};
```

#### Runtime Monitoring
```python
class ContractMonitor:
    """Monitor contract compliance in production"""
    
    def __init__(self):
        self.metrics = PrometheusMetrics()
        self.alerts = AlertManager()
    
    def monitor_function(self, func, contract: Contract):
        """Decorator to monitor function execution"""
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            # Check preconditions
            if not self.check_preconditions(contract.preconditions, args, kwargs):
                self.metrics.increment('contract.precondition.failed')
                self.alerts.send('Precondition failed', func.__name__)
                raise ContractViolation("Precondition failed")
            
            # Execute function
            start_time = time.time()
            result = func(*args, **kwargs)
            duration = time.time() - start_time
            
            # Check postconditions
            if not self.check_postconditions(contract.postconditions, result):
                self.metrics.increment('contract.postcondition.failed')
                self.alerts.send('Postcondition failed', func.__name__)
                raise ContractViolation("Postcondition failed")
            
            # Check performance contracts
            if contract.performance and duration > contract.performance.max_time:
                self.metrics.increment('contract.performance.exceeded')
                self.alerts.send('Performance budget exceeded', func.__name__)
            
            return result
        return wrapper
```

### 7. Storage Schema

#### Redis Schema
```yaml
# Contract storage in Redis
contract:<id>:
  version: "1.0"
  function: "process_payment"
  file: "/path/to/file.py"
  line: 42
  contract:
    preconditions: [...]
    postconditions: [...]
    effects: {...}
  metadata:
    created: "timestamp"
    modified: "timestamp"
    validated: "timestamp"
```

#### Neo4j Schema
```cypher
// Contract node
CREATE (c:Contract {
  id: 'contract-id',
  function: 'process_payment',
  file: '/path/to/file.py'
})

// Relationships
CREATE (c1:Contract)-[:CALLS]->(c2:Contract)
CREATE (c:Contract)-[:READS]->(d:Data)
CREATE (c:Contract)-[:WRITES]->(d:Data)
CREATE (c:Contract)-[:DEPENDS_ON]->(c2:Contract)
```

### 8. CLI Tools

```bash
# SemDoc CLI for development
semdoc parse file.py          # Parse and validate SemBlocks
semdoc validate .              # Validate all contracts in directory
semdoc generate file.py        # Generate contracts from code
semdoc check file.py:42        # Check specific function contract
semdoc search "payment"        # Semantic search for contracts
semdoc impact analyze file.py  # Analyze impact of changes
semdoc monitor start           # Start runtime monitoring
```

## Implementation Phases

### Phase 1: Basic Parser (Week 1)
- [ ] Define SemBlock YAML schema
- [ ] Build Python parser
- [ ] Create validation engine
- [ ] Implement Redis storage

### Phase 2: Multi-Language Support (Week 2)
- [ ] Add C++ parser
- [ ] Add JavaScript parser
- [ ] Create generic regex parser
- [ ] Build contract registry

### Phase 3: Tooling (Week 3)
- [ ] Pre-commit hooks
- [ ] CLI tools
- [ ] IDE extension prototype
- [ ] Runtime monitoring

### Phase 4: Integration (Week 4)
- [ ] Neo4j relationship tracking
- [ ] Qdrant semantic search
- [ ] Contract enforcement
- [ ] Production monitoring

## Next Steps

1. **Immediate**: Start adding SemBlocks to existing Python daemon
2. **This Week**: Build basic parser and validator
3. **Next Week**: Integrate with C++ daemon development
4. **Month End**: Full SemDoc system operational

---

*This design provides a practical, implementable approach to SemDoc that can start simple and evolve into a comprehensive semantic documentation system.*
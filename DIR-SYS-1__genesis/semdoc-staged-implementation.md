# SemDoc Staged Implementation - Mini-Spec

## The Bootstrap Problem
SemDoc is a self-describing system that needs itself to exist before it can be built (chicken/egg problem).

## Solution: Progressive Enforcement Stages

### **Stage 1: Traditional + RBAC** (Weeks 1-4)
**Build SemDoc infrastructure using proven traditional methods**
- AGT-SEMDOC-PARSER, AGT-SEMDOC-REGISTRY, AGT-SEMDOC-VALIDATOR
- Traditional Go/Python development (no contracts)
- Casbin RBAC for agent authorization 
- Traditional testing and validation
- **Result**: Working SemDoc tools without SemDoc constraints

### **Stage 2: Pseudo-Contracts** (Weeks 5-8) 
**Add contract syntax without enforcement**
```go
// @semblock
// contract_id: "01J9F7Z8Q4R5ZV3J4X19M8YZTW"
// semantic_path: "capability.semdoc.parse.extract"
// contract:
//   preconditions: ["file_readable"]  
//   postconditions: ["contracts_extracted"]
func ParseFile(path string) ([]Contract, error) {
    // Traditional implementation - contracts are documentation only
}
```
- **Result**: Contract database populated, developers familiar with syntax

### **Stage 3: Evaluated But Not Enforced** (Weeks 9-12)
**Add validation without blocking execution**
```go
func ParseFile(path string) ([]Contract, error) {
    result, err := parseFile(path)
    
    // NEW: Validate but don't block on violations
    contractValidator.ValidatePostconditions("capability.semdoc.parse.extract", ...)
    // Violations logged, execution continues
    
    return result, err
}
```
- **Result**: Contract violations detected, performance measured, specifications refined

### **Stage 4: Fully Enforced** (Weeks 13-16)
**Complete contract enforcement**
```go
func ParseFile(path string) ([]Contract, error) {
    // Preconditions must pass or execution fails
    if !contractValidator.ValidatePreconditions(...) {
        return nil, ContractViolationError{"Preconditions not met"}
    }
    
    result, err := parseFile(path)
    
    // Postconditions must pass or execution fails  
    if !contractValidator.ValidatePostconditions(...) {
        return nil, ContractViolationError{"Postconditions violated"}
    }
    
    return result, err
}
```
- **Result**: SemDoc system operates under inviolable semantic contracts
- **Bootstrap Complete**: System can now build more SemDoc systems

## Key Benefits
- **Breaks circular dependencies** - build contract system without contracts
- **Progressive risk management** - validate each stage before proceeding  
- **Practical timeline** - working system in 4-16 weeks
- **Self-bootstrapping** - Stage 4 system can manufacture more SemDoc

## Current Status
- **Stage 0**: Basic semantic naming (AGT-NAMING-1) ✅
- **Stage 1**: Ready to implement traditional SemDoc infrastructure

## Next Action
Begin Stage 1 implementation - traditional development of AGT-SEMDOC-PARSER, AGT-SEMDOC-REGISTRY, AGT-SEMDOC-VALIDATOR using Casbin RBAC.

---
*This solves the fundamental chicken/egg problem by building the SemDoc system traditionally, then using that system to enforce SemDoc on everything else.*
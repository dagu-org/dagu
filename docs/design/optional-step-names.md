# Design Document: Optional Step Names in Dagu

## Executive Summary

This document outlines the design for making step names optional in Dagu DAG definitions. When a step name is not provided, the system will automatically generate a simple, predictable name using the format `step-{index}` where index starts from 1.

## Current State Analysis

### Step Name Usage

Currently, step names in Dagu are **mandatory** and serve multiple critical purposes:

1. **Step Identification**: Unique identifier within a DAG for each step
2. **Dependency Resolution**: Referenced in `depends` fields to establish execution order
3. **Output Variable Scoping**: Used to reference step outputs via `${step_name.output}`
4. **UI Display**: Shown in web interface for workflow visualization
5. **Logging & Monitoring**: Used in logs, error messages, and execution traces
6. **Execution Graph Management**: Internal node identification in the scheduler
7. **Handler References**: Referenced in onSuccess/onFailure/onCancel/onExit handlers
8. **Status Tracking**: Used in dagrun status files and history

### Current Validation Rules

- Step names are required (error: `ErrStepNameRequired`)
- Maximum length: 100 characters (error: `ErrStepNameTooLong`)
- Must be unique within a DAG (error: `ErrStepNameDuplicate`)
- Cannot conflict with step IDs
- No format restrictions (allows spaces, special characters)

### Key Code Locations

- **Definition**: `internal/digraph/step.go` - Step struct with Name field
- **Building**: `internal/digraph/builder.go:buildSteps()` - Step creation from YAML
- **Validation**: `internal/digraph/builder.go:validateSteps()` - Name validation logic
- **Dependency Resolution**: References to step names in depends fields
- **Execution**: `internal/digraph/scheduler/graph.go` - Node identification by name

## Design Goals

1. **Backward Compatibility**: Existing DAGs with explicit names must work unchanged
2. **Simplicity**: Keep the implementation as simple as possible
3. **Predictable**: Generated names follow a clear, consistent pattern
4. **Unique & Stable**: Names must be unique and consistent across runs
5. **Minimal Disruption**: Changes should be localized to the build phase

## Proposed Solution

### Auto-Generation Strategy

When a step name is not provided, generate a simple, predictable name based on the step's position:

```yaml
steps:
  - command: echo "Hello"     # Generated: "step-1"
  - command: npm test          # Generated: "step-2"
  - script: |                  # Generated: "step-3"
      echo "complex"
      echo "script"
```

### Name Generation Rules

1. **Simple Format**: `step-{index}` where index starts from 1
   - Predictable and consistent
   - Easy to reference in dependencies
   - No complex parsing logic needed

2. **Uniqueness**: Guaranteed by incrementing index
   - No conflicts possible
   - Simple counter-based approach

3. **Mixed Mode**: Allow mixing explicit and auto-generated names
   ```yaml
   steps:
     - command: setup.sh       # Generated: "step-1"
     - name: build            # Explicit: "build"
       command: make all
     - command: test.sh       # Generated: "step-3"
       depends: build
   ```

### Implementation Details

#### Phase 1: Core Name Generation

Location: `internal/digraph/builder.go`

```go
// generateStepName generates an automatic name for a step
func generateStepName(existingNames map[string]struct{}, index int) string {
    // Simple, predictable naming: step-1, step-2, etc.
    name := fmt.Sprintf("step-%d", index+1)
    
    // Handle the rare case where user explicitly named a step "step-N"
    for {
        if _, exists := existingNames[name]; !exists {
            existingNames[name] = struct{}{}
            return name
        }
        index++
        name = fmt.Sprintf("step-%d", index+1)
    }
```

#### Phase 2: Modify buildSteps Function

```go
func buildSteps(ctx BuildContext, spec *definition, dag *DAG) error {
    buildCtx := StepBuildContext{BuildContext: ctx, dag: dag}
    existingNames := make(map[string]struct{})
    
    switch v := spec.Steps.(type) {
    case []any:
        var stepDefs []stepDef
        // ... decode logic ...
        
        for i, stepDef := range stepDefs {
            // Auto-generate name if not provided
            if stepDef.Name == "" {
                stepDef.Name = generateStepName(existingNames, i)
            } else {
                existingNames[stepDef.Name] = struct{}{}
            }
            
            step, err := buildStep(buildCtx, stepDef)
            if err != nil {
                return err
            }
            dag.Steps = append(dag.Steps, *step)
        }
        // ... rest of logic ...
    }
}
```

#### Phase 3: Update Validation

Modify `validateSteps` to skip the name requirement check:

```go
func validateSteps(ctx BuildContext, spec *definition, dag *DAG) error {
    // Names should already be populated (explicit or generated)
    // Proceed with uniqueness and other validations
    
    stepNames := make(map[string]struct{})
    stepIDs := make(map[string]struct{})
    
    for _, step := range dag.Steps {
        // Name should always exist at this point (generated if not provided)
        if step.Name == "" {
            // This should not happen if generation works correctly
            return fmt.Errorf("internal error: step name not generated")
        }
        
        // Continue with existing validation logic...
    }
}
```

## Migration & Compatibility

### Backward Compatibility

- All existing DAGs with explicit step names continue to work unchanged
- No changes to DAG file format or schema
- No changes to API or UI contracts

### Migration Path

1. **Phase 1**: Deploy name generation logic (backward compatible)
2. **Phase 2**: Update documentation with examples
3. **Phase 3**: Update DAG templates and examples to showcase optional names
4. **Phase 4**: Consider deprecation warnings for very generic names (future)

## Testing Strategy

### Unit Tests

1. **Name Generation Tests** (`builder_test.go`):
   - Test various command formats
   - Test executor-based naming
   - Test uniqueness handling
   - Test edge cases (empty commands, special characters)

2. **Integration Tests**:
   - DAGs with mixed explicit and auto-generated names
   - Dependency resolution with generated names
   - Output variable references

3. **Regression Tests**:
   - Ensure all existing tests pass
   - Verify backward compatibility

### Test Cases

```yaml
# Test Case 1: Basic auto-generation
steps:
  - command: echo "hello"        # Expected: "step-1"
  - command: npm test            # Expected: "step-2"
  - command: go build ./...      # Expected: "step-3"

# Test Case 2: Mixed explicit and generated
steps:
  - name: setup
    command: mkdir temp
  - command: echo "test"         # Expected: "step-2"
    depends: setup
  - name: cleanup
    command: rm -rf temp
    depends: step-2              # Reference to generated name

# Test Case 3: All auto-generated with dependencies
steps:
  - command: git pull            # Expected: "step-1"
  - command: npm install         # Expected: "step-2"
    depends: step-1
  - command: npm test            # Expected: "step-3"
    depends: step-2
  - command: npm build           # Expected: "step-4"
    depends: step-3

# Test Case 4: Conflict handling
steps:
  - name: step-2                # Explicit: "step-2"
    command: echo "explicit"
  - command: echo "auto1"       # Expected: "step-1" (not conflicting)
  - command: echo "auto2"       # Expected: "step-3" (skips step-2)
```

## UI/UX Considerations

### User Experience

1. **Debugging**: Generated names should be meaningful enough for debugging
2. **Logs**: Clear indication when names are auto-generated (optional log message)
3. **Documentation**: Clear examples showing both explicit and implicit naming

### Web UI

- No changes required - generated names display like explicit names
- Consider adding visual indicator for auto-generated names (future enhancement)

## Performance Considerations

- Name generation happens once during DAG parsing (minimal overhead)
- No runtime performance impact
- Memory: Small additional map for tracking names during generation

## Security Considerations

- Command sanitization prevents injection in generated names
- Length limits prevent resource exhaustion
- Special character filtering prevents UI/log corruption

## Future Enhancements

1. **Smart Naming**: Use AI/ML to generate more meaningful names from complex commands
2. **User Preferences**: Allow configuration of naming strategies
3. **Name Templates**: Support custom name generation templates
4. **IDE Support**: Auto-completion for generated names in IDEs

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Name collision with user expectations | Medium | Clear documentation, predictable patterns |
| Breaking changes in depends resolution | High | Thorough testing, backward compatibility |
| Confusing generated names | Low | Meaningful extraction algorithms |
| Performance degradation | Low | Efficient generation, one-time cost |

## Implementation Timeline

1. **Week 1**: Implement core name generation logic
2. **Week 2**: Integration and testing
3. **Week 3**: Documentation and examples
4. **Week 4**: Release and monitoring

## Decision Log

- **Why allow optional names?**: Reduces boilerplate for simple workflows
- **Why not use UUIDs?**: Not human-readable, hard to debug
- **Why command-based naming?**: Most intuitive and meaningful
- **Why maintain backward compatibility?**: Large existing user base

## Additional Considerations

### Edge Cases & Examples

#### Dependencies with Generated Names

```yaml
# Example: Complex dependency graph with auto-generated names
steps:
  - command: git pull                    # Generated: "step-1"
  
  - command: npm install                 # Generated: "step-2"
    depends: step-1
  
  - command: npm test                    # Generated: "step-3"
    depends: step-2
  
  - command: npm build                   # Generated: "step-4"
    depends: step-3
  
  - command: docker build -t app:latest  # Generated: "step-5"
    depends: step-4
  
  - command: docker push app:latest      # Generated: "step-6"
    depends: step-5
```

#### Output Variables with Generated Names

```yaml
# Using output variables with auto-generated step names
steps:
  - command: git rev-parse HEAD          # Generated: "step-1"
    output: COMMIT_SHA
  
  - command: echo "Deploying ${COMMIT_SHA}"  # Generated: "step-2"
    depends: step-1
  
  # Alternative: Still can use explicit names when needed
  - name: get-version
    command: cat VERSION
    output: VERSION
  
  - command: docker build -t app:${VERSION}  # Generated: "step-4"
    depends: get-version
```

### Naming Conflicts Resolution

```yaml
# Scenario: User has explicit "step-2" name
steps:
  - command: echo "first"          # Generated: "step-1"
  
  - name: step-2                   # Explicit name takes precedence
    command: echo "custom"
  
  - command: echo "third"          # Generated: "step-3" (skips conflicting step-2)
  
  - command: echo "fourth"         # Generated: "step-4"
```

### Chain vs Graph Mode Implications

```yaml
# Chain mode: Names less critical but still useful for debugging
type: chain
steps:
  - command: setup.sh              # Generated: "step-1"
  - command: test.sh               # Generated: "step-2"
  - command: deploy.sh             # Generated: "step-3"
  
# Graph mode: Names critical for dependencies
type: graph  # default
steps:
  - command: setup.sh              # Generated: "step-1"
  - command: test-unit.sh          # Generated: "step-2"
    depends: step-1
  - command: test-integration.sh   # Generated: "step-3"
    depends: step-1
```

## Alternative Approaches Considered

### 1. Command-Based Naming  
- Extract meaningful names from commands (e.g., `npm test` → "npm-test")
- **Rejected**: Adds unnecessary complexity, parsing edge cases, internationalization issues

### 2. Hash-Based Naming
- Use hash of command content
- **Rejected**: Not human-readable

### 3. Mandatory ID with Optional Name
- Require ID field, make name optional
- **Rejected**: Doesn't reduce boilerplate

### 4. Template-Based Generation
- User-defined templates like `${index}-${command}`
- **Rejected**: Too complex for initial implementation

### 5. Type-Based Prefixes
- Different prefixes for different step types (e.g., "cmd-1", "script-1", "parallel-1")
- **Rejected**: More complex without significant benefit over simple "step-N"

## Conclusion

Making step names optional in Dagu will significantly improve the user experience for simple workflows while maintaining full compatibility with existing complex DAGs. The simple `step-{index}` naming convention provides:

- **Zero complexity**: No command parsing or content analysis needed
- **Perfect predictability**: Users always know what name will be generated
- **Easy referencing**: Simple to reference in dependencies (`step-1`, `step-2`, etc.)
- **Minimal code changes**: Simple counter-based implementation

The implementation is straightforward, localized to the build phase, and carries minimal risk. With proper testing and documentation, this feature can be rolled out safely and will be a valuable addition to Dagu's usability.
# Step ID Feature Specification

## Executive Summary

This specification defines the implementation of an optional `id` field for steps in Dagu DAG files, inspired by GitHub Actions. This feature will allow users to reference step outputs more intuitively and concisely without requiring explicit `output` field declarations.

## Motivation

Currently in Dagu, to reference a step's output in another step, users must:
1. Explicitly define an `output` field in the source step
2. Reference it using `${OUTPUT_VAR}` syntax

This can be verbose and unintuitive for simple use cases. By introducing an `id` field, we can simplify DAG authoring and make it more aligned with familiar workflow tools like GitHub Actions.

## Goals

1. **Simplify Output References**: Allow referencing step outputs directly using step IDs
2. **Maintain Backward Compatibility**: Existing DAGs must continue to work without modification
3. **Improve Readability**: Make DAGs more concise and intuitive
4. **Enable Better Tooling**: IDs provide stable references for UI and API interactions

## Current State Analysis

### Step Structure
```go
type Step struct {
    Name        string   `json:"name"`
    Output      string   `json:"output,omitempty"`
    Depends     []string `json:"depends,omitempty"`
    // ... other fields
}
```

### Current Output Usage
```yaml
steps:
  - name: get data
    command: echo "hello world"
    output: MESSAGE
  
  - name: use data
    command: echo "Got: ${MESSAGE}"
    depends: get data
```

### Dependency Resolution
- Steps are identified by their `Name` field
- Names must be unique within a DAG
- Dependencies reference steps by name
- Output variables are stored in a map keyed by the `Output` field value

## Proposed Changes

### 1. Step Structure Enhancement
Add an optional `id` field to the Step struct:
```go
type Step struct {
    Name        string   `json:"name"`
    ID          string   `json:"id,omitempty"`      // NEW: Optional step ID
    Output      string   `json:"output,omitempty"`
    Depends     []string `json:"depends,omitempty"`
    // ... other fields
}
```

### 2. Enhanced YAML Syntax
Users can define steps with IDs:
```yaml
steps:
  - name: Get Current Date
    id: date
    command: date +%Y-%m-%d
  
  - name: Process Data
    id: process
    command: process_data.sh --date ${date}
    depends: date
```

### 3. Output Reference Resolution

#### Priority Rules
When resolving `${identifier}`:
1. Check if `identifier` matches a step ID
2. If not found, check if `identifier` matches an output variable name
3. If still not found, check environment variables
4. Return error if not found

#### Implicit Output Capture
When a step has an `id` but no `output` field:
- Automatically capture stdout to a variable named after the ID
- This is equivalent to setting `output: <id>`

Example:
```yaml
# These two are functionally equivalent:

# Using ID without output field
- name: Get Config
  id: config
  command: cat config.json

# Traditional approach
- name: Get Config
  command: cat config.json
  output: config
```

### 4. Dependency Resolution Enhancement

#### Using IDs in Dependencies
Allow both step names and IDs in the `depends` field:
```yaml
steps:
  - name: First Step
    id: step1
    command: echo "done"
  
  - name: Second Step
    depends: step1  # Can use ID instead of name
    command: echo "depends on first"
```

#### Resolution Priority
When resolving dependencies:
1. First check if the dependency matches a step ID
2. If not found, check if it matches a step name
3. Return error if neither matches

### 5. Validation Rules

1. **ID Uniqueness**: IDs must be unique within a DAG (same as names)
2. **ID Format**: IDs must match pattern `^[a-zA-Z][a-zA-Z0-9_-]*$`
3. **No Conflicts**: An ID cannot match another step's name
4. **Reserved Words**: IDs cannot use reserved words (e.g., "env", "params")

### 6. JSON Path Support
IDs work seamlessly with existing JSON path syntax:
```yaml
steps:
  - name: Get JSON Config
    id: config
    command: echo '{"host": "localhost", "port": 8080}'
  
  - name: Use Config
    command: curl http://${config.host}:${config.port}/api
    depends: config
```

## Implementation Components

### Core Changes

1. **Step Structure** (`internal/digraph/step.go`)
   - Add `ID` field to Step struct
   - Update validation logic

2. **Builder** (`internal/digraph/builder.go`)
   - Add ID validation in `validateSteps`
   - Update dependency resolution in `buildDepends`
   - Implement implicit output capture

3. **Variable Resolution** (`internal/cmdutil/eval.go`)
   - Update `ExpandReferences` to check step IDs
   - Maintain backward compatibility

4. **Graph Building** (`internal/digraph/graph.go`)
   - Update `findStep` to search by both name and ID
   - Enhance dependency resolution logic

5. **Scheduler** (`internal/digraph/scheduler/node.go`)
   - Update output capture logic for ID-based implicit outputs

### Schema Updates

1. **JSON Schema** (`schemas/dag.schema.json`)
   ```json
   "id": {
     "type": "string",
     "pattern": "^[a-zA-Z][a-zA-Z0-9_-]*$",
     "description": "Optional unique identifier for the step"
   }
   ```

2. **OpenAPI Specs** (`api/v1/api.yaml`, `api/v2/api.yaml`)
   - Add `id` field to step definitions
   - Update API documentation

### UI Updates

1. **DAG Editor**
   - Add ID field to step form
   - Show ID in step list/cards
   - Validate ID format client-side

2. **DAG Visualization**
   - Display ID alongside or instead of name when available
   - Update tooltips to show both name and ID

3. **Execution View**
   - Show step ID in execution logs
   - Allow filtering/searching by ID

### Documentation Updates

1. **RST Documentation** (`docs/sources/configuration.rst`)
   - Add ID field documentation
   - Provide examples of ID usage
   - Update output reference section

2. **Example DAGs**
   - Create examples showing ID usage
   - Update existing examples where it improves clarity

3. **Migration Guide**
   - Document how to adopt IDs in existing DAGs
   - Best practices for ID naming

## Migration Strategy

### Phase 1: Implementation (No Breaking Changes)
1. Add ID field support
2. Maintain full backward compatibility
3. All existing DAGs work unchanged

### Phase 2: Adoption
1. Update documentation with ID examples
2. Recommend ID usage for new DAGs
3. Provide tooling to add IDs to existing DAGs

### Phase 3: Future Enhancements
1. Consider deprecating explicit `output` field for simple cases
2. Add linting rules for ID best practices
3. Enhanced IDE support with ID-based autocompletion

## Testing Requirements

### Unit Tests
1. ID validation (format, uniqueness, conflicts)
2. Dependency resolution with IDs
3. Output variable resolution priority
4. Implicit output capture
5. JSON path resolution with IDs

### Integration Tests
1. Full DAG execution with ID-based references
2. Mixed usage of names and IDs
3. Child DAG ID references
4. Parallel step execution with IDs

### Regression Tests
1. Ensure all existing DAG features work unchanged
2. Verify no performance degradation
3. Test upgrade path from older versions

## Example: Before and After

### Before (Current Approach)
```yaml
name: data-pipeline
steps:
  - name: fetch data
    command: curl https://api.example.com/data
    output: RAW_DATA
  
  - name: parse json
    command: echo "${RAW_DATA}" | jq '.items[]'
    output: ITEMS
    depends: fetch data
  
  - name: count items
    command: echo "${ITEMS}" | wc -l
    output: COUNT
    depends: parse json
  
  - name: report
    command: |
      echo "Processed ${COUNT} items"
      echo "Raw data: ${RAW_DATA:0:50}..."
    depends:
      - count items
      - fetch data
```

### After (With IDs)
```yaml
name: data-pipeline
steps:
  - name: fetch data
    id: data
    command: curl https://api.example.com/data
  
  - name: parse json
    id: items
    command: echo "${data}" | jq '.items[]'
    depends: data
  
  - name: count items
    id: count
    command: echo "${items}" | wc -l
    depends: items
  
  - name: report
    command: |
      echo "Processed ${count} items"
      echo "Raw data: ${data:0:50}..."
    depends:
      - count
      - data
```

## Benefits

1. **Reduced Verbosity**: No need for explicit `output` fields for simple cases
2. **Improved Readability**: IDs provide meaningful references
3. **Better Tooling**: Stable IDs enable better UI/API interactions
4. **Familiar Syntax**: Similar to GitHub Actions and other workflow tools
5. **Backward Compatible**: No breaking changes to existing DAGs

## Risks and Mitigations

### Risk 1: Confusion Between Name and ID
**Mitigation**: Clear documentation and validation to prevent conflicts

### Risk 2: Breaking Existing Variable Resolution
**Mitigation**: Strict priority rules and comprehensive testing

### Risk 3: Performance Impact
**Mitigation**: Efficient lookup using maps for both names and IDs

## Success Criteria

1. All existing DAGs continue to work without modification
2. New DAGs can use IDs to reference step outputs
3. UI properly displays and handles step IDs
4. Documentation clearly explains the feature
5. No performance regression
6. Positive user feedback on improved usability
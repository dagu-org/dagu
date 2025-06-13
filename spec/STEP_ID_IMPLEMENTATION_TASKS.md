# Step ID Feature - Implementation Tasks

## Overview
This document provides a comprehensive list of all tasks required to implement the Step ID feature as specified in STEP_ID_FEATURE_SPECIFICATION.md.

## Core Implementation Tasks

### 1. Data Model Updates

#### 1.1 Step Structure Enhancement
- **File**: `internal/digraph/step.go`
- **Tasks**:
  - [x] Add `ID string` field to Step struct with json tag `"id,omitempty"`
  - [x] Add ID field to String() method for debugging

#### 1.2 DAG Definition Update  
- **File**: `internal/digraph/definition.go`
- **Tasks**:
  - [x] Add `ID string` field to stepDef with yaml/json tags
  - [x] Ensure proper serialization/deserialization

#### 1.3 Node State Enhancement
- **File**: `internal/digraph/scheduler/data.go`
- **Tasks**:
  - [ ] Add `OutputFilePaths map[string]string` to NodeState
  - [ ] Update serialization to include output file paths
  - [ ] Add methods to set/get output file paths

### 2. Validation and Building

#### 2.1 Step Validation
- **File**: `internal/digraph/builder.go`
- **Tasks**:
  - [x] Add ID format validation (regex: `^[a-zA-Z][a-zA-Z0-9_-]*$`)
  - [x] Add ID uniqueness validation within DAG
  - [x] Add check for ID/Name conflicts across steps
  - [x] Add reserved word validation for IDs (including "outputs")
  - [x] Update error messages for better clarity

#### 2.2 Dependency Resolution Enhancement
- **File**: `internal/digraph/builder.go` 
- **Tasks**:
  - [x] Update `buildDepends()` to resolve dependencies by ID first, then name
  - [x] Add helper function `findStepByIDOrName()`
  - [x] Update error messages to mention both ID and name

#### 2.3 Graph Building
- **File**: `internal/digraph/scheduler/graph.go`
- **Tasks**:
  - [x] Update `findStep()` to search by ID first, then name
  - [x] Add ID-to-Node mapping in ExecutionGraph
  - [x] Update setup() method to handle ID-based dependencies

### 3. File-based Output Handling

#### 3.1 Output File Management
- **File**: `internal/persistence/filedagrun/store.go`
- **Tasks**:
  - [ ] Add `GetStepOutputPath(dagRunDir, stepID string) string` method
  - [ ] Ensure output directory exists when creating path
  - [ ] Add file cleanup considerations

#### 3.2 Node Execution
- **File**: `internal/digraph/scheduler/node.go`
- **Tasks**:
  - [ ] Set `$DAGU_OUTPUT` environment variable to output file path for steps with ID
  - [ ] After execution, check if output file exists
  - [ ] Store file path in `OutputFilePaths` map
  - [ ] Remove automatic stdout capture logic

#### 3.3 Variable Resolution
- **File**: `internal/cmdutil/eval.go`
- **Tasks**:
  - [ ] Update `ExpandReferences()` to handle `.outputs` suffix
  - [ ] Load file content when `${id.outputs}` is referenced
  - [ ] Implement lazy loading with caching
  - [ ] Handle file not found errors gracefully
  - [ ] Support JSON path on loaded content

#### 3.4 Environment Setup
- **File**: `internal/digraph/executor/env.go`
- **Tasks**:
  - [ ] Add `DAGU_OUTPUT` to step environment when ID is present
  - [ ] Ensure output file path is accessible to executors

### 4. Schema Updates

#### 4.1 JSON Schema
- **File**: `schemas/dag.schema.json`
- **Tasks**:
  - [ ] Add `id` property to step definition
  - [ ] Add pattern validation for ID format
  - [ ] Add description for the field
  - [ ] Update schema version if needed

#### 4.2 OpenAPI v1
- **File**: `api/v1/api.yaml`
- **Tasks**:
  - [ ] Add `id` field to StepDefinition schema
  - [ ] Add `id` field to StepResponse schema
  - [ ] Update API documentation
  - [ ] Run `make apiv1` to regenerate code

#### 4.3 OpenAPI v2
- **File**: `api/v2/api.yaml`
- **Tasks**:
  - [ ] Add `id` field to Step schema
  - [ ] Update relevant endpoints documentation
  - [ ] Run `make apiv2` to regenerate code

### 5. API Implementation

#### 5.1 API v1 Updates
- **Files**: `internal/frontend/api/v1/*`
- **Tasks**:
  - [ ] Update transformers to include ID field
  - [ ] Ensure ID is properly serialized in responses
  - [ ] Update any step-related endpoints

#### 5.2 API v2 Updates
- **Files**: `internal/frontend/api/v2/*`
- **Tasks**:
  - [ ] Update model transformers to handle ID field
  - [ ] Ensure proper JSON marshaling/unmarshaling

### 6. Frontend Updates

#### 6.1 TypeScript Types
- **File**: `ui/src/api/v2/schema.ts`
- **Tasks**:
  - [ ] Regenerate types after API schema update
  - [ ] Ensure ID field is properly typed

#### 6.2 DAG Editor
- **Files**: `ui/src/components/editor/*`
- **Tasks**:
  - [ ] Add ID input field to step form
  - [ ] Add client-side validation for ID format
  - [ ] Display ID in step list/cards
  - [ ] Update step creation/editing logic

#### 6.3 DAG Visualization
- **Files**: `ui/src/components/graph/*`
- **Tasks**:
  - [ ] Display ID in node labels (when present)
  - [ ] Update tooltips to show both name and ID
  - [ ] Consider ID in node sizing/layout

#### 6.4 Execution View
- **Files**: `ui/src/components/execution/*`
- **Tasks**:
  - [ ] Show step ID in execution logs
  - [ ] Display ID in step status cards
  - [ ] Enable searching/filtering by ID

### 7. CLI Updates

#### 7.1 Status Command
- **File**: `internal/cmd/cmd_status.go`
- **Tasks**:
  - [ ] Display step ID in status output when present
  - [ ] Update formatting for better readability

#### 7.2 Dry Run Command
- **File**: `internal/cmd/cmd_dry.go`
- **Tasks**:
  - [ ] Show ID in dry run output
  - [ ] Validate ID references during dry run

### 8. Documentation

#### 8.1 Configuration Documentation
- **File**: `docs/sources/configuration.rst`
- **Tasks**:
  - [ ] Add section explaining the `id` field
  - [ ] Provide examples of ID usage
  - [ ] Document ID naming conventions
  - [ ] Update output reference documentation

#### 8.2 Step Configuration
- **File**: `docs/sources/step_configuration.rst`
- **Tasks**:
  - [ ] Add ID field to step configuration reference
  - [ ] Include examples with and without ID
  - [ ] Document implicit output behavior

#### 8.3 Example DAGs
- **Directory**: `examples/`
- **Tasks**:
  - [ ] Create new example: `step_with_id.yaml`
  - [ ] Update existing examples where ID improves clarity
  - [ ] Add complex example showing ID benefits

#### 8.4 README Updates
- **File**: `README.md`
- **Tasks**:
  - [ ] Add ID feature to feature list
  - [ ] Include simple example in quick start

### 9. Testing

#### 9.1 Unit Tests - Core
- **File**: `internal/digraph/builder_test.go`
- **Tasks**:
  - [x] Test ID validation (format, uniqueness)
  - [x] Test ID/name conflict detection
  - [x] Test dependency resolution with IDs
  - [x] Test reserved word validation

#### 9.2 Unit Tests - File Output
- **File**: `internal/digraph/scheduler/node_test.go`
- **Tasks**:
  - [ ] Test $DAGU_OUTPUT environment variable setting
  - [ ] Test output file path generation
  - [ ] Test OutputFilePaths map updates
  - [ ] Test file existence checking after execution

#### 9.3 Unit Tests - Variable Resolution
- **File**: `internal/cmdutil/eval_test.go`
- **Tasks**:
  - [ ] Test `.outputs` suffix handling
  - [ ] Test file loading when variable is referenced
  - [ ] Test JSON path on file content
  - [ ] Test error handling for missing files
  - [ ] Test caching behavior

#### 9.4 Integration Tests
- **File**: `internal/integration/step_id_test.go`
- **Tasks**:
  - [ ] Test full DAG execution with $DAGU_OUTPUT
  - [ ] Test ${id.outputs} variable resolution
  - [ ] Test large file handling (>100MB)
  - [ ] Test parallel steps writing outputs
  - [ ] Test output persistence after DAG completion

#### 9.5 API Tests
- **Files**: `internal/frontend/api/v1/*_test.go`
- **Tasks**:
  - [ ] Test ID serialization in API responses
  - [ ] Test output file paths in status responses

#### 9.6 Performance Tests
- **File**: `internal/digraph/performance_test.go` (new)
- **Tasks**:
  - [ ] Benchmark variable resolution with file loading
  - [ ] Test memory usage with large outputs
  - [ ] Test concurrent file access

### 10. Migration and Tooling

#### 10.1 Migration Script
- **File**: `scripts/add_ids_to_dag.py` (new)
- **Tasks**:
  - [ ] Create script to add IDs to existing DAGs
  - [ ] Support different ID generation strategies
  - [ ] Add dry-run mode

#### 10.2 Linting Rules
- **File**: TBD based on linting approach
- **Tasks**:
  - [ ] Consider adding ID naming convention checks
  - [ ] Warn about missing IDs for steps with outputs

### 11. Performance Optimization

#### 11.1 Lookup Optimization
- **Files**: Various
- **Tasks**:
  - [ ] Add ID-based lookup maps where needed
  - [ ] Ensure O(1) lookup for both names and IDs
  - [ ] Profile and optimize hot paths

### 12. Error Handling and Logging

#### 12.1 Enhanced Error Messages
- **Files**: Various
- **Tasks**:
  - [ ] Update error messages to mention both ID and name
  - [ ] Add specific error types for ID-related issues
  - [ ] Improve error context for debugging

#### 12.2 Logging Updates
- **Files**: Various
- **Tasks**:
  - [ ] Include step ID in log messages
  - [ ] Update log formatting for clarity

## Testing Checklist

### Functional Tests
- [ ] Step with ID gets $DAGU_OUTPUT environment variable
- [ ] Writing to $DAGU_OUTPUT creates output file
- [ ] Variable reference using ${id.outputs} loads file content
- [ ] JSON path reference works: `${id.outputs.field.subfield}`
- [ ] Dependency using ID works: `depends: step_id`
- [ ] Mix of ID and name dependencies works
- [ ] ID uniqueness is enforced
- [ ] ID format validation works
- [ ] Reserved words (including "outputs") are rejected
- [ ] Existing DAGs without IDs work unchanged
- [ ] Steps without ID cannot use $DAGU_OUTPUT

### File-based Output Tests
- [ ] Output files created in correct directory structure
- [ ] Large outputs (>100MB) handled without memory issues
- [ ] Concurrent steps can write outputs simultaneously
- [ ] Output files persist after DAG completion
- [ ] Missing output files handled gracefully in variable resolution

### Edge Cases
- [ ] Step with both ID and output field uses output field name
- [ ] Circular dependency detection works with IDs
- [ ] Empty ID is treated as not set
- [ ] ID same as another step's name is rejected
- [ ] Special characters in ID are rejected
- [ ] Very long IDs are handled properly
- [ ] Empty $DAGU_OUTPUT file handled correctly
- [ ] Binary data in output files handled appropriately

### Performance Tests
- [ ] Variable resolution performance with file loading
- [ ] Memory usage remains constant with large outputs
- [ ] No significant slowdown vs in-memory variables

### Backward Compatibility
- [ ] All existing example DAGs run successfully
- [ ] API v1 and v2 work with and without IDs
- [ ] UI handles DAGs without IDs properly
- [ ] Existing output field behavior unchanged

## Release Checklist

- [ ] All tests passing
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Migration guide written
- [ ] Example DAGs created
- [ ] UI screenshots updated
- [ ] Performance benchmarks run
- [ ] Beta testing completed
- [ ] Release notes prepared

## Post-Release Tasks

- [ ] Monitor for user feedback
- [ ] Create tutorial/blog post
- [ ] Update any external documentation
- [ ] Consider follow-up enhancements
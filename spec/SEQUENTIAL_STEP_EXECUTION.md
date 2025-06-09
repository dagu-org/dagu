# Chain Step Execution Specification

## Overview

This specification describes a new DAG-level field `type` that allows users to control step execution mode. When set to `chain`, steps will execute sequentially in the order they are defined, without requiring explicit dependency declarations.

## Motivation

Currently, users must explicitly define dependencies between steps using the `depends` field. For workflows where steps should simply run one after another in order, this creates unnecessary boilerplate:

```yaml
# Current approach - verbose for sequential workflows
steps:
  - name: step1
    command: echo "First"
  
  - name: step2
    command: echo "Second"
    depends: step1
  
  - name: step3
    command: echo "Third"
    depends: step2
  
  - name: step4
    command: echo "Fourth"
    depends: step3
```

## Proposed Solution

Add a new DAG-level field `type` that controls how steps are executed:

```yaml
# New approach - clean and simple
type: chain  # New field
steps:
  - name: step1
    command: echo "First"
  
  - name: step2
    command: echo "Second"
  
  - name: step3
    command: echo "Third"
  
  - name: step4
    command: echo "Fourth"
```

## Field Specification

### Field Name: `type`

- **Type**: `string`
- **Location**: DAG-level (same level as `name`, `schedule`, etc.)
- **Valid Values**: 
  - `"graph"` (default) - Current behavior, steps run based on dependency graph
  - `"chain"` - Steps run in the order they are defined
  - `"agent"` - Reserved for future agent mode implementation
- **Default**: `"graph"` (maintains backward compatibility)

## Behavior Specification

### Chain Mode (`type: chain`)

1. **Execution Order**: Steps execute in the exact order they appear in the YAML file
2. **Implicit Dependencies**: Each step automatically depends on the previous step
3. **First Step**: Has no dependencies (runs immediately)
4. **Override**: Explicit `depends` field still honored if specified
5. **Parallel Steps**: Steps with `parallel` field still execute their items according to their configuration

### Graph Mode (`type: graph`)

- Current behavior is maintained
- Steps run based on their `depends` field
- Steps without dependencies run immediately (respecting `maxActiveSteps`)

### Agent Mode (`type: agent`)

- Reserved for future implementation
- Will enable specialized agent-based execution patterns

## Interaction with Other Features

### maxActiveSteps
- In chain mode, effectively becomes 1 for non-parallel steps
- Still applies to items within parallel steps

### depends Field
- In chain mode, explicit `depends` overrides implicit chain dependency
- Allows breaking out of strict sequence when needed

### parallel Field
- Works the same in all modes
- Items within a parallel step respect their own `maxConcurrent` setting

## Examples

### Basic Chain Workflow
```yaml
name: data-pipeline
type: chain

steps:
  - name: download
    command: wget https://example.com/data.csv
  
  - name: validate
    command: validate.py data.csv
  
  - name: process
    command: process.py data.csv
  
  - name: upload
    command: aws s3 cp output.csv s3://bucket/
```

### Mixed Mode (Chain with Parallel Branch)
```yaml
name: build-and-test
type: chain

steps:
  - name: checkout
    command: git clone ${REPO}
  
  - name: build
    command: make build
  
  - name: test-suite
    parallel:
      - unit
      - integration
      - e2e
    command: make test-${ITEM}
  
  - name: package
    command: make package
```

### Chain with Explicit Dependencies
```yaml
name: complex-pipeline
type: chain

steps:
  - name: setup
    command: ./setup.sh
  
  - name: download-a
    command: wget fileA
  
  - name: download-b
    command: wget fileB
  
  - name: process-both
    command: process.py fileA fileB
    depends:  # Override chain to depend on both downloads
      - download-a
      - download-b
  
  - name: cleanup
    command: rm -f fileA fileB
```

## Implementation Considerations

1. **Parser Updates**: Add `type` to DAG configuration structure
2. **Scheduler Changes**: Modify scheduler to inject implicit dependencies in chain mode
3. **Validation**: Ensure no circular dependencies when mixing modes
4. **UI Updates**: Display execution type in DAG details
5. **Migration**: No migration needed due to default value

## Benefits

1. **Simplicity**: Reduces boilerplate for chain workflows
2. **Readability**: Intent is clear from DAG-level declaration
3. **Flexibility**: Can mix chain and parallel patterns
4. **Backward Compatible**: Existing DAGs work unchanged

## Future Extensions

The string-based approach allows for future execution types:
- `"priority"` - Execute based on step priority values
- `"resource"` - Execute based on resource availability
- `"conditional"` - Execute based on runtime conditions
- `"agent"` - Agent-based execution with specialized handling

## Testing Requirements

1. **Chain Execution**: Verify steps run in order
2. **Mixed Dependencies**: Test explicit depends in chain mode
3. **Error Handling**: Ensure failures stop subsequent steps
4. **Parallel Steps**: Verify parallel steps work within chain DAGs
5. **Performance**: No performance regression for graph mode
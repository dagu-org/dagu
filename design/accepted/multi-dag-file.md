# Multi-DAG File Specification

## Overview

This specification defines the ability to write multiple DAG definitions within a single YAML file using YAML document separators (`---`). This feature enables users to define parent and child DAGs together, reducing file proliferation for small, related workflows.

### Quick Example
```yaml
# workflows.yaml - contains 3 related DAGs
name: main-pipeline
steps:
  - name: process
    run: transform-data    # References DAG below
  - name: archive
    run: archive-results   # References DAG below

---
name: transform-data
steps:
  - name: transform
    command: transform.py

---
name: archive-results  
steps:
  - name: archive
    command: archive.sh
```

## Motivation

Currently, each DAG requires its own file, which can lead to:
- File proliferation for small, related workflows
- Difficulty managing parent-child DAG relationships
- Increased complexity when organizing modular workflows

By allowing multiple DAGs in a single file, users can:
- Keep related workflows together
- Reduce the number of files for simple workflows
- Maintain better organization of parent-child relationships
- Simplify deployment and version control

## YAML File Format

### Single DAG File (Current - Remains Supported)
```yaml
name: my-workflow
steps:
  - name: step1
    command: echo "hello"
```

### Multi-DAG File (Primary Format - Using Document Separators)
```yaml
# Parent DAG
name: data-pipeline
description: "Main data processing pipeline"
schedule: "0 2 * * *"
steps:
  - name: extract
    command: echo "extracting data"
  
  - name: transform
    run: transform-module  # References DAG in same file
    params: "INPUT=/tmp/data"
    depends: extract
  
  - name: load
    run: load-module  # References DAG in same file
    params: "OUTPUT=/tmp/output"
    depends: transform

---
# Child DAG 1
name: transform-module
description: "Data transformation module"
params:
  - INPUT: /tmp/input
steps:
  - name: validate
    command: test -f ${INPUT}
  
  - name: process
    command: transform.py --input=${INPUT}
    depends: validate
    output: TRANSFORMED_PATH

---
# Child DAG 2
name: load-module
description: "Data loading module"
params:
  - OUTPUT: /tmp/output
steps:
  - name: prepare
    command: mkdir -p ${OUTPUT}
  
  - name: save
    command: save.py --output=${OUTPUT}
    depends: prepare
```

**Advantages of Document Separator Approach:**
- Natural YAML multi-document format (standard YAML 1.2 feature)
- Each DAG remains a complete, valid DAG definition
- No new schema or wrapper needed
- Easy to split/merge files using standard YAML tools
- Maintains backward compatibility (single DAG = single document)
- Cleaner and more readable

## Key Design Decisions

1. **Document Separators**: Using YAML's standard `---` separator is the most natural approach
2. **DAG Names Required**: Each DAG in a multi-DAG file MUST have a unique `name` field
3. **Same-File References**: Child DAGs in the same file are referenced by name only
4. **Backward Compatibility**: Single-DAG files (no separators) work exactly as before
5. **Persistence**: Each DAG is extracted and saved separately when executed

## Implementation Details

### 1. DAG Loader Changes

#### File Detection
- Multi-DAG files are detected by the presence of `---` document separators
- Standard YAML decoder can handle multiple documents natively
  
#### Parser Enhancement
```go
// internal/digraph/loader.go

// LoadDAGs loads all DAGs from a file (single or multi-DAG)
func LoadDAGs(file string) ([]*DAG, error) {
    f, err := os.Open(file)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    
    var dags []*DAG
    decoder := yaml.NewDecoder(f)
    
    for {
        var doc map[string]interface{}
        err := decoder.Decode(&doc)
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }
        
        // Build DAG from document
        dag, err := buildDAG(doc, file)
        if err != nil {
            return nil, err
        }
        dags = append(dags, dag)
    }
    
    return dags, nil
}

// LoadDAG loads a specific DAG by name from a file
func LoadDAG(file string, dagName string) (*DAG, error) {
    dags, err := LoadDAGs(file)
    if err != nil {
        return nil, err
    }
    
    // If no specific name requested and single DAG, return it
    if dagName == "" && len(dags) == 1 {
        return dags[0], nil
    }
    
    // Find DAG by name
    for _, dag := range dags {
        if dag.Name == dagName {
            return dag, nil
        }
    }
    
    return nil, fmt.Errorf("DAG '%s' not found in file", dagName)
}
```

#### DAG Resolution
- Same-file references work with just the DAG name (e.g., `run: transform-module`)
- Cross-file references could use `file.yaml#dag-name` syntax (future enhancement)
- Maintain backward compatibility for simple DAG names

### 2. Persistence Layer Changes

#### DAG Storage
When a child DAG from a multi-DAG file is executed:
1. Extract the specific DAG definition from the multi-DAG file
2. Create a separate `dag.json` in the child's run directory
3. Store only the relevant DAG definition, not the entire multi-DAG file

#### File Structure
```
history/
└── data-pipeline/
    └── dag-run_xxx/
        ├── dag.json  # Parent DAG definition only
        └── children/
            └── child_transform-module_yyy/
                ├── dag.json  # Extracted child DAG definition
                └── attempt_zzz/
                    └── status.jsonl
```

### 3. DAG Store Modifications

```go
// internal/persistence/filedag/store.go

// DAGLocation represents where a DAG is stored
type DAGLocation struct {
    FilePath string
    DAGName  string  // Empty for single-DAG files
    Index    int     // Document index in multi-DAG file
}

// Enhanced DAG retrieval
func (s *store) GetDAG(ctx context.Context, name string) (*digraph.DAG, error) {
    // Search through all DAG files
    for _, searchPath := range s.searchPaths {
        files, _ := filepath.Glob(filepath.Join(searchPath, "*.yaml"))
        files2, _ := filepath.Glob(filepath.Join(searchPath, "*.yml"))
        files = append(files, files2...)
        
        for _, file := range files {
            dags, err := LoadDAGs(file)
            if err != nil {
                continue
            }
            
            // Check each DAG in the file
            for _, dag := range dags {
                if dag.Name == name {
                    return dag, nil
                }
            }
        }
    }
    
    return nil, fmt.Errorf("DAG not found: %s", name)
}

// List all DAGs including those in multi-DAG files
func (s *store) ListDAGs() ([]*DAGLocation, error) {
    var locations []*DAGLocation
    
    for _, searchPath := range s.searchPaths {
        files, _ := filepath.Glob(filepath.Join(searchPath, "*.yaml"))
        files2, _ := filepath.Glob(filepath.Join(searchPath, "*.yml"))
        files = append(files, files2...)
        
        for _, file := range files {
            dags, err := LoadDAGs(file)
            if err != nil {
                continue
            }
            
            for i, dag := range dags {
                locations = append(locations, &DAGLocation{
                    FilePath: file,
                    DAGName:  dag.Name,
                    Index:    i,
                })
            }
        }
    }
    
    return locations, nil
}
```

### 4. API Changes

#### List DAGs Endpoint
Include metadata about multi-DAG files:
```json
{
  "dags": [
    {
      "name": "data-pipeline",
      "file": "workflows.yaml",
      "isMultiDAG": true,
      "dagCount": 3
    }
  ]
}
```

#### DAG Details Endpoint
Add navigation information:
```json
{
  "dag": { ... },
  "file": {
    "path": "workflows.yaml",
    "isMultiDAG": true,
    "dags": ["data-pipeline", "transform-module", "load-module"],
    "currentDAG": "data-pipeline"
  }
}
```

### 5. UI Changes

#### DAG Details Page Navigation

**Tab-based Navigation (Recommended)**
```tsx
// When viewing a multi-DAG file
<Tabs value={currentDAG} onChange={handleDAGSwitch}>
  <Tab label="data-pipeline" value="data-pipeline" />
  <Tab label="transform-module" value="transform-module" />
  <Tab label="load-module" value="load-module" />
</Tabs>
```

**Benefits of Tabs:**
- Visual indication of related DAGs
- Easy switching between DAGs
- Clear current DAG context
- Supports keyboard navigation

#### DAG List View
- Show indicator for multi-DAG files
- Option to expand/collapse DAGs within a file
- Search should find DAGs within multi-DAG files

#### Visual Indicators
- Icon or badge for multi-DAG files
- Breadcrumb showing: `workflows.yaml > data-pipeline`
- Clear parent-child relationships in graph view

## Backward Compatibility

1. **Single DAG files continue to work unchanged**
2. **Existing child DAG references remain valid**
3. **Migration path**:
   - Provide tool to merge related DAGs into multi-DAG file
   - Support gradual adoption

## Security Considerations

1. **Access Control**: All DAGs in a file share same permissions
2. **Validation**: Ensure DAG names are unique within a file
3. **Circular References**: Detect and prevent circular dependencies

## Examples

### Example 1: ETL Pipeline with Modules
```yaml
# Main ETL pipeline
name: daily-etl
schedule: "0 2 * * *"
steps:
  - name: extract-customers
    run: extract-module
    params: "SOURCE=customers TABLE=users"
  
  - name: extract-orders
    run: extract-module
    params: "SOURCE=orders TABLE=transactions"
  
  - name: join-data
    run: join-module
    params: "LEFT=customers RIGHT=orders"
    depends: [extract-customers, extract-orders]

---
# Reusable extraction module
name: extract-module
params: [SOURCE, TABLE]
steps:
  - name: connect
    command: db_connect.sh ${SOURCE}
  
  - name: extract
    command: extract.py --table=${TABLE}
    output: EXTRACTED_DATA

---
# Reusable join module
name: join-module
params: [LEFT, RIGHT]
steps:
  - name: join
    command: join.py --left=${LEFT} --right=${RIGHT}
```

### Example 2: Deployment Pipeline
```yaml
# Main deployment orchestrator
name: deploy-all
steps:
  - name: deploy-dev
    run: deploy-env
    params: "ENV=dev VERSION=${VERSION}"
  
  - name: test
    command: integration_test.sh dev
    depends: deploy-dev
  
  - name: deploy-prod
    run: deploy-env
    params: "ENV=prod VERSION=${VERSION}"
    depends: test

---
# Reusable deployment module
name: deploy-env
params: [ENV, VERSION]
steps:
  - name: backup
    command: backup.sh ${ENV}
  
  - name: deploy
    command: deploy.sh ${ENV} ${VERSION}
    continueOn:
      failure: false
```

## Migration Guide

### Converting Existing DAGs
```bash
# Tool to merge related DAGs
dagu merge-dags parent.yaml child1.yaml child2.yaml -o combined.yaml

# Validate multi-DAG file
dagu validate combined.yaml
```

### Best Practices
1. Group related workflows in single file
2. Keep unrelated workflows separate
3. Use clear, descriptive DAG names
4. Document relationships in description fields
5. Limit file size (recommend < 1000 lines)

## Implementation Phases

### Phase 1: Core Support
- Multi-DAG file parsing
- DAG resolution within same file
- Basic UI navigation

### Phase 2: Enhanced Features
- Migration tools
- Advanced UI features
- Performance optimizations

### Phase 3: Extended Support
- Cross-file DAG references
- DAG versioning within files
- Advanced validation

## Open Questions

1. **File Size Limits**: Should we impose limits on multi-DAG file sizes?
2. **Validation**: How strict should validation be for circular dependencies?
3. **Search**: How should search results display DAGs from multi-DAG files?
4. **DAG Naming**: Should we enforce unique names across all files or just within a file?
5. **Performance**: Should we cache parsed multi-DAG files to avoid re-parsing?
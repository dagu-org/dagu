# Step ID Feature Architecture

## Overview

This document describes the Step ID feature implementation and future property access capabilities.

## Current State vs. Future Features

### Current Implementation (Phase 1 - Completed)
- ✅ ID field added to Step struct
- ✅ ID format validation and uniqueness checks
- ✅ ID-based dependency resolution (steps can depend on IDs)
- ✅ Reserved word validation

### Not Yet Implemented
- ❌ Property access syntax (`${id.stdout}`, `${id.stderr}`, etc.)
- ❌ Variable resolution for step IDs

## Design Overview

### 1. Current: Step IDs for Dependencies

Step IDs provide an alternative way to reference steps in dependencies:

```yaml
steps:
  - name: Generate Report
    id: report
    command: generate_report.py
    output: report_data  # Explicit output capture still required
  
  - name: Process Report
    command: echo "${report_data}"
    depends: report  # Can use ID instead of step name
```

### 2. Structured Property Access

Users can access different aspects of step execution using dot notation:

```yaml
steps:
  - name: Download Large File
    id: download
    command: wget https://example.com/large-file.zip
  
  - name: Process Results
    command: |
      # Access properties via ID
      echo "Stdout file: ${download.stdout}"    # Path to stdout log file
      echo "Stderr file: ${download.stderr}"    # Path to stderr log file
      echo "Exit code: ${download.exit_code}"   # Process exit code
```

**Benefits:**
- Flexibility to access exactly what's needed
- File paths available for large outputs or debugging
- Metadata access for conditional logic
- Natural extension of existing syntax

### 3. Memory-Efficient Output Handling

The implementation handles outputs efficiently:

**For Small Outputs (<= 1MB default):**
- Use the `output` field to capture stdout into a variable
- Available via `${variable_name}` syntax
- Stored in memory with size limits

**For Large Outputs:**
- Don't use the `output` field (would hit size limit)
- Access file directly via `${id.stdout}` property
- Process the file without loading into memory

### 4. Property Reference Table

| Property | Description | Example | Availability |
|----------|-------------|---------|--------------|
| `${id.stdout}` | Path to stdout log file | `${download.stdout}` | Always |
| `${id.stderr}` | Path to stderr log file | `${download.stderr}` | Always |
| `${id.exit_code}` | Process exit code | `${download.exit_code}` | After execution |

## Implementation Architecture

### Components

1. **Step Building Phase** (Current)
   - Step ID validation and uniqueness checks
   - ID-based dependency resolution

2. **Execution Phase** (No changes)
   - Standard output capture mechanism (existing code)
   - stdout/stderr files always created for all steps
   - Output variables stored in memory (with size limits)

3. **Variable Resolution Enhancement**
   ```
   When encountering ${identifier} or ${identifier.property}:
   1. Parse identifier and property parts
   2. Check if identifier is a step ID
   3. If property specified:
      - Return requested property (stdout, stderr, exit_code, outputs)
   4. Fall back to existing resolution logic
   ```

### Error Handling

- Missing output file: Return empty string or error
- Invalid JSON: Treat as plain text
- File read errors: Log and return error
- Large files: Stream reading with size limits

### Security Considerations

- Output files inherit DAG run permissions
- No arbitrary file access via variable resolution
- Validate step IDs to prevent path traversal

## Implementation Phases

### Phase 1: Basic ID Support
#### Completed:
- ✅ Add ID field to Step struct
- ✅ Validate ID format and uniqueness  
- ✅ Support ID-based dependencies
- ✅ Unit tests for ID functionality

#### Not Yet Implemented:
- ❌ Variable resolution for step IDs
- ❌ JSON schema updates
- ❌ OpenAPI specification updates
- ❌ UI component updates
- ❌ Documentation updates

### Phase 2: Property Access
- Add property resolution for .stdout, .stderr, .exit_code, .outputs
- Enhance variable resolver to parse property syntax
- Add helper methods to access step metadata

### Phase 3: Advanced Features
- Additional properties (.duration, .start_time, etc.)
- Support for parallel execution results
- Output streaming for large files

## Why This Approach?

### Benefits of Step IDs
- **Alternative References**: Steps can be referenced by ID instead of name
- **Cleaner Dependencies**: Short, meaningful IDs instead of long step names
- **Future Extensibility**: Foundation for property access features

### Benefits of Property Access
- **File Access**: Users can process large outputs without loading into memory
- **Debugging**: Easy access to stderr and log files
- **Flexibility**: Access to metadata like exit codes
- **JSON Support**: Access nested JSON fields via outputs property

### Key Advantages
- **Backward Compatible**: Existing DAGs work unchanged
- **Clean Syntax**: Natural property access patterns
- **Extensible**: Property syntax enables future enhancements

## Future Enhancements

1. **Output Metadata**
   - Size, timestamp, content-type
   - Checksums for integrity

2. **Output Transformations**
   - Automatic JSON/YAML parsing
   - Compression for large outputs
   - Encryption for sensitive data

3. **Output Lifecycle**
   - Retention policies
   - Automatic cleanup
   - Archival to object storage

4. **Advanced Access Patterns**
   - Streaming large outputs
   - Partial file reads
   - Binary data handling

## Examples

### Basic ID Usage
```yaml
steps:
  - name: Get System Info
    id: sysinfo
    command: uname -a
    output: system_info
  
  - name: Log Info
    command: echo "System: ${system_info}"
    depends: sysinfo  # Using ID instead of step name
```

### Accessing File Paths
```yaml
steps:
  - name: Generate Large Report
    id: report
    command: generate_report.py --verbose
    output: report_output
  
  - name: Archive Results
    command: |
      # Access file paths via properties
      cp "${report.stdout}" /archive/report-output.log
      cp "${report.stderr}" /archive/report-errors.log
      
      # Also use the output variable for small data
      echo "${report_output}" > /archive/report.txt
    depends: report
```

### JSON Output with Path Access
```yaml
steps:
  - name: Get Config
    id: config
    command: |
      echo '{
        "database": {
          "host": "localhost",
          "port": 5432
        }
      }'
    output: config_data
  
  - name: Connect
    command: |
      # Current: Access JSON paths via output variable
      psql -h ${config_data.database.host} -p ${config_data.database.port}
    depends: config  # Using ID for dependency
```

### Large Output Handling
```yaml
steps:
  - name: Download Dataset
    id: dataset
    command: curl -L https://example.com/large-dataset.csv
    # Note: Don't use output field for large files (>1MB)
    # The stdout will be saved to a file automatically
  
  - name: Process Dataset
    command: |
      # For large outputs, use the file path directly
      wc -l < "${dataset.stdout}"
      
      # Process without loading into memory
      process_csv.py < "${dataset.stdout}" > processed.csv
    depends: dataset
```

### Error Handling with Properties
```yaml
steps:
  - name: Risky Operation
    id: risky
    command: ./might-fail.sh
    output: risky_output
    continueOn:
      failure: true
  
  - name: Handle Result
    command: |
      # Check exit code via property
      if [ "${risky.exit_code}" != "0" ]; then
        echo "Operation failed with code ${risky.exit_code}"
        echo "Checking error log:"
        tail -20 "${risky.stderr}"
      else
        echo "Success! Output: ${risky_output}"
      fi
    depends: risky
```

### Mixed Property Access
```yaml
steps:
  - name: Build Project
    id: build
    command: make all
    output: build_output
  
  - name: Post-Build Analysis
    command: |
      # Different ways to access build information
      echo "Build output: ${build_output}"         # Captured stdout content
      echo "Build logs at: ${build.stdout}"        # Path to stdout file
      echo "Build errors at: ${build.stderr}"      # Path to stderr file
      echo "Exit code: ${build.exit_code}"         # Exit code
      
      # Conditional logic based on exit code
      if [ "${build.exit_code}" = "0" ]; then
        deploy.sh
      fi
    depends: build
```

## Conclusion

The Step ID feature provides:
- **Cleaner Dependencies**: Reference steps by meaningful IDs instead of long names
- **Better Organization**: Separate step identity (ID) from description (name)
- **Foundation for Properties**: Enables property access features
- **Backward Compatibility**: Existing DAGs continue to work unchanged

This approach aligns with Dagu's philosophy of incremental improvement while maintaining stability and backward compatibility.

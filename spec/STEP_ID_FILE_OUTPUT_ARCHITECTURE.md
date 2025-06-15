# Step ID Feature Architecture

## Overview

This document describes the Step ID feature implementation and future property access capabilities.

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

### 3. Property Reference Table

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

### Error Handling

- Missing output file: Return empty string or error
- File read errors: Log and return error
- Large files: Stream reading with size limits

### Security Considerations

- No arbitrary file access via variable resolution
- Validate step IDs to prevent path traversal

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

## Examples

### Basic ID Usage
```yaml
steps:
  - name: Get System Info
    id: sysinfo
    command: uname -a
  
  - name: Log Info
    script: |
      echo System:
      cat ${sysinfo.stdout}
```

### Accessing File Paths
```yaml
steps:
  - name: Generate Large Report
    id: report
    command: generate_report.py --verbose
  
  - name: Archive Results
    command: |
      # Access file paths via properties
      cp "${report.stdout}" /archive/report-output.log
      cp "${report.stderr}" /archive/report-errors.log
    depends: report
```

## Conclusion

The Step ID feature provides:
- **Cleaner Dependencies**: Reference steps by meaningful IDs instead of long names
- **Better Organization**: Separate step identity (ID) from description (name)
- **Foundation for Properties**: Enables property access features
- **Backward Compatibility**: Existing DAGs continue to work unchanged

This approach aligns with Dagu's philosophy of incremental improvement while maintaining stability and backward compatibility.

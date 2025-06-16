# Changelog

All notable changes to Dagu are documented here.

## v1.17.0-beta (2025-05-30)

### Major Features

####  Improved Performance
- Refactored execution history data for more performant history lookup
- Optimized internal data structures for better scalability

####  Hierarchical DAG Execution
Execute nested DAGs with full parameter passing and output bubbling:
```yaml
steps:
  - name: run_sub-dag
    run: sub-dag
    output: OUT
    params: "INPUT=${DATA_PATH}"

  - name: use output
    command: echo ${OUT.outputs.RESULT}
```

####  Multiple DAGs in Single File
Define multiple DAGs in one YAML file using `---` separator:
```yaml
name: main-workflow
steps:
  - name: process
    run: sub-workflow  # Defined below

---

name: sub-workflow
steps:
  - name: task
    command: echo "Hello from sub-workflow"
```

####  Parallel Execution with Parameters
Execute commands or sub-DAGs in parallel with different parameters for batch processing:
```yaml
steps:
  - name: get files
    command: find /data -name "*.csv"
    output: FILES
  
  - name: process files
    run: process-file
    parallel: ${FILES}
    params:
      - FILE_NAME: ${ITEM}
```

####  Enhanced Web UI
- Overall UI improvements with better user experience
- Cleaner design and more intuitive navigation
- Better performance for large DAG visualizations

####  Advanced History Search
New execution history page with:
- Date-range filtering
- Status filtering (success, failure, running, etc.)
- Improved search performance
- Better timeline visualization

####  Better Debugging
- **Precondition Results**: Display actual results of precondition evaluations in the UI
- **Output Variables**: Show output variable values in the UI for easier debugging
- **Separate Logs**: stdout and stderr are now separated by default for clearer log analysis

####  Queue Management
Added enqueue functionality for both API and UI:
```bash
# Queue a DAG for later execution
dagu enqueue --run-id=custom-id my-dag.yaml

# Dequeue
dagu dequeue my-dag.yaml
```

####  Partial Success Status
New "partial success" status for workflows where some steps succeed and others fail, providing better visibility into complex workflow states.

####  API v2
- New `/api/v2` endpoints with refactored schema
- Better abstractions and cleaner interfaces
- Improved error handling and response formats
- See [OpenAPI spec](https://github.com/dagu-org/dagu/blob/main/api/v2/api.yaml) for details

### Docker Improvements

####  Optimized Images
Thanks to @jerry-yuan:
- Significantly reduced Docker image size
- Split into three baseline images for different use cases
- Better layer caching for faster builds

####  Container Enhancements
Thanks to @vnghia:
- Allow specifying container name
- Support for image platform selection
- Better container management options

### Enhanced Features

####  Advanced Repeat Policy
Thanks to @thefishhat:
- Conditions for repeat execution
- Expected output matching
- Exit code-based repeats

```yaml
steps:
  - name: wait for service
    command: check_service.sh
    repeatPolicy:
      condition: "${STATUS}"
      expected: "ready"
      intervalSec: 30
      exitCode: [0, 1]
```

### Bug Fixes & Improvements

- Fixed history data migration issues
- Improved error messages and logging
- Better handling of edge cases in DAG execution
- Performance improvements for large workflows
- Various UI/UX enhancements: #925, #898, #895, #868, #903, #911, #913, #921, #923, #887, #922, #932, #962

### Breaking Changes

####  DAG Type Field (v1.17.0-beta.13+)

Starting from v1.17.0-beta.13, DAGs now have a `type` field that controls step execution behavior:

- **`type: chain`** (new default): Steps are automatically connected in sequence, even if no dependencies are specified
- **`type: graph`** (previous behavior): Steps only depend on explicitly defined dependencies

To maintain the previous behavior, add `type: graph` to your DAG configuration:

```yaml
type: graph
steps:
  - name: task1
    command: echo "runs in parallel"
  - name: task2
    command: echo "runs in parallel"
```

Alternatively, you can explicitly set empty dependencies for parallel steps:

```yaml
steps:
  - name: task1
    command: echo "runs in parallel"
    depends: []
  - name: task2
    command: echo "runs in parallel"
    depends: []
```

### Migration Required

 **History Data Migration**: Due to internal improvements, history data from 1.16.x requires migration:

```bash
# Migrate history data
dagu migrate history
```

After successful migration, legacy history directories are moved to `<DAGU_DATA_DIR>/history_migrated_<timestamp>` for safekeeping.

### Contributors

Huge thanks to our contributors for this release:

| Contribution | Author |
|--------------|--------|
| Optimized Docker image size and split into baseline images | [@jerry-yuan](https://github.com/jerry-yuan) |
| Container name & image platform support | [@vnghia](https://github.com/vnghia) |
| Enhanced repeat-policy conditions | [@thefishhat](https://github.com/thefishhat) |
| Queue functionality implementation | [@kriyanshii](https://github.com/kriyanshii) |
| Partial success status | [@thefishhat](https://github.com/thefishhat) |
| Countless reviews & feedback | [@ghansham](https://github.com/ghansham) |

### Installation

Try the beta version:

```bash
# Docker
docker run --rm -p 8080:8080 ghcr.io/dagu-org/dagu:latest dagu start-all

# Or download specific version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --version v1.17.0-beta
```

---

## v1.16.0 (2025-01-09)

### New Features

####  Enhanced Docker Image
- Base image updated to `ubuntu:24.04`
- Pre-installed common tools: `sudo`, `git`, `curl`, `jq`, `python3`, and more
- Ready for production use with essential utilities

####  Dotenv File Support
Load environment variables from `.env` files:

```yaml
dotenv: /path/to/.env
# or multiple files
dotenv:
  - .env
  - .env.production
```

#### 🔗 JSON Reference Expansion
Access nested JSON values with path syntax:

```yaml
steps:
  - name: sub workflow
    run: sub_workflow
    output: SUB_RESULT
  - name: use output
    command: echo "The result is ${SUB_RESULT.outputs.finalValue}"
```

If `SUB_RESULT` contains:
```json
{
  "outputs": {
    "finalValue": "success"
  }
}
```
Then `${SUB_RESULT.outputs.finalValue}` expands to `success`.

####  Advanced Preconditions

**Regex Support**: Use `re:` prefix for pattern matching:
```yaml
steps:
  - name: some_step
    command: some_command
    preconditions:
      - condition: "`date '+%d'`"
        expected: "re:0[1-9]"  # Run only on days 01-09
```

**Command Preconditions**: Test conditions with commands:
```yaml
steps:
  - name: some_step
    command: some_command
    preconditions:
      - command: "test -f /tmp/some_file"
```

####  Enhanced Parameter Support

**List Format**: Define parameters as key-value pairs:
```yaml
params:
  - PARAM1: value1
  - PARAM2: value2
```

**CLI Flexibility**: Support both named and positional parameters:
```bash
# Positional
dagu start my_dag -- param1 param2

# Named
dagu start my_dag -- PARAM1=value1 PARAM2=value2

# Mixed
dagu start my_dag -- param1 param2 --param3 value3
```

####  Enhanced Continue On Conditions

**Exit Code Matching**:
```yaml
steps:
  - name: some_step
    command: some_command
    continueOn:
      exitCode: [1, 2]  # Continue if exit code is 1 or 2
```

**Mark as Success**:
```yaml
steps:
  - name: some_step
    command: some_command
    continueOn:
      exitCode: 1
      markSuccess: true  # Mark successful even if failed
```

**Output Matching**:
```yaml
steps:
  - name: some_step
    command: some_command
    continueOn:
      output: "WARNING"  # Continue if output contains "WARNING"
      
  # With regex
  - name: another_step
    command: another_command
    continueOn:
      output: "re:^ERROR: [0-9]+"  # Regex pattern matching
```

#### 🐚 Shell Features

**Piping Support**:
```yaml
steps:
  - name: pipe_example
    command: "cat file.txt | grep pattern | wc -l"
```

**Custom Shell Selection**:
```yaml
steps:
  - name: bash_specific
    command: "echo ${BASH_VERSION}"
    shell: bash
    
  - name: python_shell
    command: "print('Hello from Python')"
    shell: python3
```

####  Sub-workflow Output
Parent workflows now receive structured output from sub-workflows:

```json
{
  "name": "some_subworkflow",
  "params": "PARAM1=param1 PARAM2=param2",
  "outputs": {
    "RESULT1": "Some output",
    "RESULT2": "Another output"
  }
}
```

#### 🔗 Simplified Dependencies
String format now supported:
```yaml
steps:
  - name: first
    command: echo "First"
  - name: second
    command: echo "Second"
    depends: first  # Simple string instead of array
```

### Improvements

- **Environment Variable Expansion**: Now supported in most DAG fields
- **UI Enhancements**: Improved DAG visualization for better readability
- **Storage Optimization**: Reduced state file sizes by removing redundant data

### Bug Fixes

- Fixed: DAGs with dots (`.`) in names can now be edited in the Web UI

### Contributors

Thanks to our contributor for this release:

| Contribution | Author |
|--------------|--------|
| Improved parameter handling for CLI - support for both named and positional parameters | [@kriyanshii](https://github.com/kriyanshii) |

---

## Previous Versions

For older versions, please refer to the [GitHub releases page](https://github.com/dagu-org/dagu/releases).

## Version Support

- **Current**: v1.16.x (latest features and bug fixes)
- **Previous**: v1.15.x (bug fixes only)
- **Older**: Best effort support

## Migration Guides

### Upgrading to v1.16.0

Most changes are backward compatible. Key considerations:

1. **Docker Users**: The new Ubuntu base image includes more tools but is slightly larger
2. **Parameter Format**: Both old and new formats are supported
3. **State Files**: Old state files are automatically compatible

### Breaking Changes

None in v1.16.0

## See Also

- [Installation Guide](/getting-started/installation) - Upgrade instructions
- [Configuration Reference](/reference/config) - New configuration options
- [Examples](/examples/) - New feature examples
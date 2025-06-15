# Shell Executor

The shell executor is the default and most commonly used executor in Dagu. It runs commands directly in the system shell, making it perfect for executing scripts, system commands, and any other command-line tools.

## Overview

The shell executor allows you to:

- Run any command available on your system
- Execute scripts in any language (Python, Bash, Node.js, etc.)
- Use different shells (sh, bash, zsh, etc.)
- Leverage nix-shell for reproducible environments
- Access environment variables and command substitution

## Basic Usage

By default, steps use the shell executor:

```yaml
steps:
  - name: hello
    command: echo "Hello, World!"
```

You can also explicitly specify the shell executor:

```yaml
steps:
  - name: hello
    executor: shell
    command: echo "Hello, World!"
```

## Shell Selection

Dagu allows you to specify which shell to use for command execution:

### Default Shell

By default, Dagu uses the shell specified in the `$SHELL` environment variable, falling back to `sh` if not set:

```yaml
steps:
  - name: default-shell
    command: echo $0  # Shows which shell is being used
```

### Specific Shell

You can specify a custom shell for a step:

```yaml
steps:
  - name: use-bash
    shell: bash
    command: echo "Running in bash: $BASH_VERSION"
  
  - name: use-zsh
    shell: zsh
    command: echo "Running in zsh: $ZSH_VERSION"
```

### Nix Shell

For reproducible environments, you can use nix-shell with specific packages. Dagu provides built-in support for nix-shell with automatic package management and pure environment isolation.

#### Basic Nix Shell Usage

```yaml
steps:
  - name: python-with-nix
    shell: nix-shell
    shellPackages: [python3, curl, jq]
    command: |
      python3 --version
      curl --version
      jq --version
```

#### How Nix Shell Works in Dagu

When you specify `shell: nix-shell`, Dagu automatically:

1. **Adds packages**: Uses the `shellPackages` field to include specified Nix packages
2. **Creates pure environment**: Automatically adds `--pure` flag for isolation
3. **Runs commands**: Wraps your command with `--run` to execute in the nix-shell environment

The above example gets translated to:
```bash
nix-shell -p python3 -p curl -p jq --pure --run "python3 --version && curl --version && jq --version"
```

#### Advanced Nix Shell Configuration

**Using specific package versions:**

```yaml
steps:
  - name: specific-versions
    shell: nix-shell
    shellPackages:
      - python314
      - nodejs_24
      - postgresql_16
    script: |
      python3 --version
      node --version
      psql --version
```

**Combining with script files:**

```yaml
steps:
  - name: nix-script
    shell: nix-shell
    shellPackages: [python3, pandas, numpy]
    script: |
      import pandas as pd
      import numpy as np
      
      print(f"Pandas version: {pd.__version__}")
      print(f"NumPy version: {np.__version__}")
      
      # Your data processing logic here
      df = pd.DataFrame({'x': range(10), 'y': np.random.randn(10)})
      print(df.head())
```

**Complex data science environment:**

```yaml
steps:
  - name: data-analysis
    shell: nix-shell
    shellPackages:
      - python3
      - python3Packages.pandas
      - python3Packages.matplotlib
      - python3Packages.jupyter
      - python3Packages.scikit-learn
    script: |
      #!/usr/bin/env python3
      import pandas as pd
      import matplotlib.pyplot as plt
      from sklearn.model_selection import train_test_split
      
      # Load and analyze data
      data = pd.read_csv('/data/input.csv')
      # ... your analysis code ...
```

**Mixed environments with different tools:**

```yaml
steps:
  - name: build-and-test
    shell: nix-shell
    shellPackages: [nodejs, python3, go, docker, git]
    command: |
      # Frontend build
      npm install && npm run build
      
      # Backend tests
      python3 -m pytest tests/
      
      # Go service build
      go build -o service ./cmd/service
      
      # Container operations
      docker build -t myapp .

  - name: deployment-tools
    shell: nix-shell
    shellPackages: [kubectl, terraform, awscli2, ansible]
    command: |
      # Infrastructure deployment
      terraform plan
      kubectl get pods
      aws s3 ls
```

#### Package Discovery

To find available packages, visit the [Nix Packages Search](https://search.nixos.org/packages) or use the command line. For example, to search for Python or Node.js packages, you can use:

```bash
# Search for packages
nix search nixpkgs python
nix search nixpkgs nodejs

# List available Python packages
nix search nixpkgs python3Packages
```

Common package categories:
- **Languages**: `python3`, `nodejs`, `go`, `rust`, `ruby`, `php`
- **Databases**: `postgresql`, `mysql80`, `mongodb`, `redis`
- **Tools**: `git`, `curl`, `jq`, `docker`, `kubectl`, `terraform`
- **Python packages**: `python3Packages.pandas`, `python3Packages.requests`

#### Environment Isolation

Nix shell provides complete environment isolation:

```yaml
steps:
  - name: isolated-environment
    shell: nix-shell
    shellPackages: [python38]  # Specific Python version
    command: |
      # This runs in a pure environment with only specified packages
      # System packages are not available
      python3 --version  # Will use Python 3.8
      which python3      # Shows nix store path
      echo $PATH         # Shows only nix paths
```

#### Best Practices for Nix Shell

1. **Pin package versions** for reproducibility:
```yaml
steps:
  - name: pinned-versions
    shell: nix-shell
    shellPackages: [python39, nodejs-16_x]
    command: python3 --version && node --version
```

2. **Use specific Python packages** instead of pip:
```yaml
steps:
  - name: python-packages
    shell: nix-shell
    shellPackages:
      - python3
      - python3Packages.requests
      - python3Packages.click
    script: |
      import requests
      import click
      # No need for pip install
```

3. **Combine with traditional package managers** when needed:
```yaml
steps:
  - name: mixed-deps
    shell: nix-shell
    shellPackages: [python3, git]
    command: |
      # Use nix for core tools
      git clone https://github.com/user/repo.git
      cd repo
      
      # Use pip for Python packages not in nixpkgs
      pip install --user some-custom-package
```

#### Troubleshooting Nix Shell

**Debug nix-shell execution:**

```yaml
steps:
  - name: debug-nix
    shell: nix-shell
    shellPackages: [python3]
    command: |
      echo "Nix store path: $(which python3)"
      echo "Available packages:"
      ls /nix/store/ | grep python | head -5
      python3 -c "import sys; print(sys.path)"
```

**Handle missing packages:**

```yaml
steps:
  - name: fallback-strategy
    shell: nix-shell
    shellPackages: [python3]
    command: |
      # Try nix package first, fallback to system
      if command -v some-tool >/dev/null 2>&1; then
        some-tool --version
      else
        echo "Tool not available in nix environment"
        exit 1
      fi
```

## Command Execution Methods

### Inline Commands

Simple one-line commands:

```yaml
steps:
  - name: date
    command: date +"%Y-%m-%d %H:%M:%S"
```

### Multi-line Commands

Use pipe notation for complex commands:

```yaml
steps:
  - name: multi-line
    command: |
      echo "Starting process..."
      for i in {1..5}; do
        echo "Step $i"
        sleep 1
      done
      echo "Process complete!"
```

### Script Blocks

For more complex logic, use the script field:

```yaml
steps:
  - name: complex-script
    script: |
      #!/bin/bash
      set -e
      
      # Function definition
      process_data() {
        local input=$1
        echo "Processing: $input"
        # Add your logic here
      }
      
      # Main execution
      files=$(find /data -name "*.csv")
      for file in $files; do
        process_data "$file"
      done
```

## Working Directory

You can specify a working directory for command execution:

```yaml
steps:
  - name: run-in-directory
    dir: /app/src
    command: npm install
```

## Environment Variables

### Step-level Environment Variables

```yaml
steps:
  - name: with-env
    command: echo "API endpoint: $API_ENDPOINT"
    env:
      - API_ENDPOINT: https://api.example.com
      - API_KEY: secret123
```

### Global Environment Variables

```yaml
env:
  - ENVIRONMENT: production
  - LOG_LEVEL: info

steps:
  - name: use-global-env
    command: echo "Running in $ENVIRONMENT with log level $LOG_LEVEL"
```

### Environment Variable Expansion

```yaml
env:
  - BASE_PATH: /data
  - FULL_PATH: ${BASE_PATH}/input

steps:
  - name: use-expanded
    command: ls -la $FULL_PATH
```

## Command Substitution

Use backticks for command substitution:

```yaml
steps:
  - name: dynamic-date
    command: mkdir -p /backup/`date +%Y%m%d`
    
  - name: conditional-execution
    command: echo "System load is `uptime | awk '{print $10}'`"
```

## Output Handling

### Standard Output and Error

Redirect stdout and stderr to files:

```yaml
steps:
  - name: redirect-output
    command: ./process_data.sh
    stdout: /logs/process.out
    stderr: /logs/process.err
```

### Capture Output in Variables

```yaml
steps:
  - name: get-version
    command: git rev-parse --short HEAD
    output: GIT_COMMIT
    
  - name: use-version
    command: echo "Deploying version $GIT_COMMIT"
    depends: get-version
```

## Error Handling

### Exit Code Handling

```yaml
steps:
  - name: check-file
    command: test -f /data/input.csv
    continueOn:
      exitCode: [1]  # Continue if file doesn't exist
```

### Shell Options

Use shell options for better error handling:

```yaml
steps:
  - name: strict-mode
    command: |
      set -euo pipefail  # Exit on error, undefined vars, pipe failures
      
      # Your commands here
      process_data.sh
      validate_output.sh
```

## Advanced Features

### Signal Handling

Specify custom signals for graceful shutdown:

```yaml
steps:
  - name: long-running
    command: ./server.sh
    signalOnStop: SIGTERM  # Send SIGTERM instead of SIGKILL
```

### Timeout Configuration

```yaml
steps:
  - name: with-timeout
    command: ./slow_process.sh
    timeout: 300  # 5 minute timeout
```

### Output Size Limits

Control maximum output size to prevent memory issues:

```yaml
maxOutputSize: 5242880  # 5MB limit

steps:
  - name: large-output
    command: ./generate_report.sh
```

## Best Practices

### 1. Use Appropriate Shell Features

```yaml
steps:
  - name: bash-features
    shell: bash
    command: |
      # Use bash arrays
      files=(*.txt)
      for file in "${files[@]}"; do
        echo "Processing $file"
      done
```

### 2. Quote Variables Properly

```yaml
steps:
  - name: safe-variables
    command: |
      # Always quote variables to handle spaces
      file_path="/path/with spaces/file.txt"
      cat "$file_path"
```

### 3. Check Command Availability

```yaml
steps:
  - name: check-prerequisites
    command: |
      # Ensure required commands exist
      command -v jq >/dev/null 2>&1 || { echo "jq is required"; exit 1; }
      command -v curl >/dev/null 2>&1 || { echo "curl is required"; exit 1; }
```

### 4. Use Exit Codes Meaningfully

```yaml
steps:
  - name: meaningful-exits
    script: |
      #!/bin/bash
      
      # Define exit codes
      SUCCESS=0
      ERROR_MISSING_FILE=1
      ERROR_INVALID_DATA=2
      ERROR_NETWORK=3
      
      # Use them in your logic
      if [[ ! -f "input.json" ]]; then
        echo "Error: input.json not found"
        exit $ERROR_MISSING_FILE
      fi
```

## Common Patterns

### Running Scripts in Different Languages

```yaml
steps:
  - name: python-script
    command: python3 analyze.py --input data.csv
    
  - name: node-script
    command: node process.js
    
  - name: go-program
    command: go run main.go
    
  - name: ruby-script
    command: ruby transform.rb
```

### Conditional Execution

```yaml
steps:
  - name: check-condition
    command: |
      if [[ -f "/tmp/skip_processing" ]]; then
        echo "Skip flag found, exiting"
        exit 0
      fi
      ./process_data.sh
```

### Parallel Processing

```yaml
steps:
  - name: parallel-jobs
    command: |
      # Run multiple jobs in parallel
      ./job1.sh &
      ./job2.sh &
      ./job3.sh &
      
      # Wait for all background jobs
      wait
```

## Troubleshooting

### Debug Shell Execution

```yaml
steps:
  - name: debug-shell
    command: |
      set -x  # Enable debug mode
      echo "Shell: $0"
      echo "PATH: $PATH"
      echo "Working directory: $(pwd)"
```

### Handle Special Characters

```yaml
steps:
  - name: special-chars
    command: |
      # Escape special characters properly
      message="Hello \"World\" with \$pecial chars"
      echo "$message"
```

### Path Issues

```yaml
steps:
  - name: fix-path
    command: |
      # Add custom paths if needed
      export PATH="/custom/bin:$PATH"
      custom-command --version
```

## Integration Examples

### With Git

```yaml
steps:
  - name: git-operations
    command: |
      git pull origin main
      git log --oneline -5
      git status
```

### With Package Managers

```yaml
steps:
  - name: install-deps
    command: |
      # Python
      pip install -r requirements.txt
      
      # Node.js
      npm install
      
      # Go
      go mod download
```

### With Cloud CLIs

```yaml
steps:
  - name: aws-operations
    command: |
      aws s3 cp data.csv s3://bucket/data/
      aws ec2 describe-instances --query 'Reservations[].Instances[].InstanceId'
```

## Security Considerations

### Avoid Hardcoding Secrets

```yaml
# Bad
steps:
  - name: bad-example
    command: curl -H "API-Key: hardcoded-secret" https://api.example.com

# Good
steps:
  - name: good-example
    command: curl -H "API-Key: $API_KEY" https://api.example.com
    env:
      - API_KEY: ${API_KEY}  # From environment or .env file
```

### Validate Input

```yaml
steps:
  - name: validate-input
    command: |
      # Validate user input
      input="${1:-}"
      if [[ ! "$input" =~ ^[a-zA-Z0-9_-]+$ ]]; then
        echo "Invalid input format"
        exit 1
      fi
```

## Next Steps

- Learn about [Docker Executor](/features/executors/docker) for containerized execution
- Explore [SSH Executor](/features/executors/ssh) for remote command execution
- Check out [Execution Control](/features/execution-control) for advanced patterns

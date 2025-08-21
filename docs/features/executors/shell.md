# Shell Executor

The default executor for running system commands.

## Basic Usage

```yaml
steps:
  - echo "Hello, World!"  # Shell executor is default
```

## Errexit Mode (Exit on Error)

Starting from v1.XX, Dagu enables the errexit flag (`-e`) by default for shell executors when no specific shell is configured. This means multi-line commands will stop execution on the first error:

```yaml
steps:
  # Default behavior - errexit enabled
  - name: safe-by-default
    command: |
      false  # This will cause the step to fail
      echo "This won't execute"  # Script stops here

  # Specify shell to bypass default errexit
  - name: continue-on-error
    shell: bash  # No -e flag when shell is specified
    command: |
      false  # Command fails but continues
      echo "This will execute"

  # Explicitly enable errexit with options
  - name: explicit-errexit
    shell: bash -e  # Add -e flag manually
    command: |
      false  # Step fails immediately
      echo "This won't execute"
      
  # Multiple shell options
  - name: strict-mode
    shell: bash -euo pipefail  # Enable strict error handling
    command: |
      set -x  # Also enable debug output
      echo "Running with strict mode"

  # Disable errexit if needed
  - name: disable-errexit
    command: |
      set +e  # Disable errexit
      false  # Command fails but continues
      echo "This will execute"
```

## Writing Scripts

```yaml
steps:
  - name: script-example
    shell: bash  # Specify shell if needed
    script: |
      # No need for 'set -e' with default shell
      echo "Running script..."
      python process.py  # Run a Python script
```

## Shell Selection

```yaml
steps:
  - name: default
    command: echo $0  # Uses $SHELL or sh
    
  - name: bash-specific
    shell: bash
    command: echo "Bash version: $BASH_VERSION"
    
  - name: custom-shell
    shell: /usr/local/bin/fish
    command: echo "Using Fish shell"
    
  - name: with-options
    shell: bash -euo pipefail  # Add custom shell options
    command: echo "Strict mode enabled"
```

### Nix Shell

Use nix-shell for reproducible environments with specific packages:

```yaml
steps:
  - name: with-packages
    shell: nix-shell
    shellPackages: [python3, curl, jq]
    command: |
      python3 --version
      curl --version
      jq --version
```

#### Examples

```yaml
# Specific versions
steps:
  - name: pinned-versions
    shell: nix-shell
    shellPackages: [python314, nodejs_24]
    command: python3 --version && node --version

# Data science stack
steps:
  - name: data-analysis
    shell: nix-shell
    shellPackages:
      - python3
      - python3Packages.pandas
      - python3Packages.numpy
    command: python analyze.py

# Multiple tools
steps:
  - name: build-env
    shell: nix-shell
    shellPackages: [go, docker, kubectl]
    command: |
      go build -o app
      docker build -t app:latest .
```

Find packages at [search.nixos.org](https://search.nixos.org/packages).

## Execution Methods

```yaml
steps:
  # Single command
  - name: date
    command: date +"%Y-%m-%d %H:%M:%S"
    
  # Multi-line command
  - name: multi-line
    command: |
      echo "Starting..."
      process_data.sh
      echo "Done"
      
  # Script block
  - name: script
    script: |
      #!/bin/bash
      # errexit is enabled by default, no need for 'set -e'
      find /data -name "*.csv" -exec process {} \;
      
  # Working directory
  - name: in-directory
    dir: /app/src
    command: npm install
```

## Environment Variables

```yaml
# Global variables
env:
  - ENVIRONMENT: production
  - BASE_PATH: /data
  - FULL_PATH: ${BASE_PATH}/input  # Variable expansion

steps:
  # Step-level variables
  - name: with-env
    env:
      - API_KEY: ${API_KEY}
    command: curl -H "X-API-Key: $API_KEY" api.example.com
    
  # Command substitution
  - name: dynamic
    command: mkdir -p /backup/`date +%Y%m%d`
```

## See Also

- [Docker Executor](/features/executors/docker) - Container execution
- [SSH Executor](/features/executors/ssh) - Remote commands
- [Command Substitution](/writing-workflows/data-variables#command-substitution) - Dynamic values

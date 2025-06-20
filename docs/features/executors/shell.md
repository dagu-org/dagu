# Shell Executor

The default executor for running system commands.

## Basic Usage

```yaml
steps:
  - name: hello
    command: echo "Hello, World!"  # Shell executor is default
```

## Writing Scripts

```yaml
steps:
  - name: script-example
    shell: bash  # Specify shell if needed
    script: |
      set -e  # Exit on error
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
      set -e
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

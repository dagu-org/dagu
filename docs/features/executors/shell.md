# Shell Executor

The default executor for running system commands.

## Basic Usage

```yaml
steps:
  - echo "Hello, World!"  # Shell executor is default
```

## Writing Scripts

```yaml
steps:
  - shell: bash  # Specify shell if needed
    script: |
      # No need for 'set -e' with default shell
      echo "Running script..."
      python process.py  # Run a Python script
```

## Shell Selection

```yaml
steps:
  - echo $0  # Uses $SHELL or sh
    
  - shell: bash
    command: echo "Bash version: $BASH_VERSION"
    
  - shell: /usr/local/bin/fish
    command: echo "Using Fish shell"
```

### Nix Shell

Use nix-shell for reproducible environments with specific packages:

```yaml
steps:
  - shell: nix-shell
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
  - shell: nix-shell
    shellPackages: [python314, nodejs_24]
    command: python3 --version && node --version

# Data science stack
steps:
  - shell: nix-shell
    shellPackages:
      - python3
      - python3Packages.pandas
      - python3Packages.numpy
    command: python analyze.py

# Multiple tools
steps:
  - shell: nix-shell
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
  - date +"%Y-%m-%d %H:%M:%S"
    
  # Multi-line command
  - |
      echo "Starting..."
      echo "Processing data..."
      echo "Done"
      
  # Script block
  - script: |
      #!/bin/bash
      # errexit is enabled by default, no need for 'set -e'
      find /data -name "*.csv" -exec process {} \;
      
  # Working directory
  - workingDir: /app/src
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
  - env:
      - API_KEY: ${API_KEY}
    command: curl -H "X-API-Key: $API_KEY" api.example.com
    
  # Command substitution
  - mkdir -p /backup/`date +%Y%m%d`
```

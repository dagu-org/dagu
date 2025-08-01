# Quick Start

Get up and running with Dagu in under 2 minutes.

## Install Dagu

::: code-group

```bash [npm]
npm install -g dagu
```

```bash [macOS/Linux]
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash
```

```bash [Docker]
docker pull ghcr.io/dagu-org/dagu:latest
```

```bash [Homebrew]
brew install dagu-org/brew/dagu
```

:::

See [Installation Guide](/getting-started/installation) for more options.

## Your First Workflow

### 1. Create a workflow

::: code-group

```bash [Binary]
mkdir -p ~/.config/dagu/dags && cat > ~/.config/dagu/dags/hello.yaml << 'EOF'
steps:
  - name: hello
    command: echo "Hello from Dagu!"
    
  - name: world
    command: echo "Running step 2"
EOF
```

```bash [Docker]
mkdir -p ~/.dagu/dags && cat > ~/.dagu/dags/hello.yaml << 'EOF'
steps:
  - name: hello
    command: echo "Hello from Dagu!"
    
  - name: world
    command: echo "Running step 2"
EOF
```

### 2. Run it

::: code-group

```bash [Binary]
dagu start hello
```

```bash [Docker]
docker run --rm \
  -v ~/.dagu:/var/lib/dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu start hello
```

:::

Output:
```
┌─ DAG: hello ─────────────────────────────────────────────────────┐
│ Status: Success ✓           | Started: 23:34:57 | Elapsed: 471ms │
└──────────────────────────────────────────────────────────────────┘

Progress: ████████████████████████████████████████ 100% (2/2 steps)
```

*Note: The output may vary if you are using Docker.*

### 3. Check the status

::: code-group

```bash [Binary]
dagu status hello
```

```bash [Docker]
docker run --rm \
  -v ~/.dagu:/var/lib/dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu status hello
```

:::

### 4. View in the UI

::: code-group

```bash [Binary]
dagu start-all
```

```bash [Docker]
docker run -d \
  -p 8080:8080 \
  -v ~/.dagu:/var/lib/dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu start-all
```

:::

Open [http://localhost:8080](http://localhost:8080)

## Understanding Workflows

A workflow is a YAML file that defines steps and their dependencies:

```yaml
steps:
  - name: step1
    command: echo "First step"
    
  - name: step2
    command: echo "Second step"  # Runs after step1 automatically
```

Key concepts:
- **Steps**: Individual tasks that run commands
- **Dependencies**: Control execution order
- **Commands**: Any shell command you can run

## Practical Example

Here's a practical backup workflow:

```yaml
# backup.yaml
name: daily-backup
params:
  - SOURCE: /data
  - DEST: /backup

steps:
  - name: timestamp
    command: date +%Y%m%d_%H%M%S
    output: TS
    
  - name: backup
    command: tar -czf ${DEST}/backup_${TS}.tar.gz ${SOURCE}
    
  - name: cleanup
    command: find ${DEST} -name "backup_*.tar.gz" -mtime +7 -delete
```

Run with parameters:

```bash
dagu start backup.yaml -- SOURCE=/important/data DEST=/backups
```

## Parallel Execution

Run steps concurrently by specifying the same dependencies:

```yaml
steps:
  - name: prepare
    command: echo "Starting"
    
  - name: task1
    command: ./process-images.sh
    depends: prepare
    
  - name: task2
    command: ./process-videos.sh
    depends: prepare
    
  - name: task3
    command: ./process-docs.sh
    depends: prepare
    
  - name: combine
    command: ./merge-results.sh
    depends: [task1, task2, task3]
```

```mermaid
graph LR
    prepare --> task1
    prepare --> task2
    prepare --> task3
    task1 --> combine
    task2 --> combine
    task3 --> combine
```

## Error Handling

Add retries and error handlers:

```yaml
steps:
  - name: download
    command: curl -f https://example.com/data.zip -o data.zip
    retryPolicy:
      limit: 3
      intervalSec: 30
      
  - name: process
    command: unzip data.zip && ./process.sh
    continueOn:
      failure: true  # Continue even if this fails
      
handlerOn:
  failure:
    command: echo "Workflow failed!" | mail -s "Alert" admin@example.com
  success:
    command: echo "Success at $(date)"
```

## Scheduling

Run workflows automatically:

```yaml
name: nightly-job
schedule: "0 2 * * *"  # 2 AM daily

steps:
  - name: run
    command: ./nightly-process.sh
```

The workflow will execute every day at 2 AM.

## What's Next?

- [Core Concepts](/getting-started/concepts) - Understand Dagu's architecture
- [Writing Workflows](/writing-workflows/) - Learn advanced features
- [Examples](/writing-workflows/examples) - Ready-to-use workflow patterns
- [CLI Reference](/reference/cli) - All command options

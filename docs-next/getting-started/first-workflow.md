# First Workflow

Create and run your first Dagu workflow in minutes.

## Creating Your First DAG

Let's start with a simple "Hello World" workflow.

### Step 1: Create the workflow file

Create a new file called `hello.yaml`:

```yaml
# hello.yaml
name: hello-world
description: My first Dagu workflow

steps:
  - name: greet
    command: echo "Hello from Dagu!"
    
  - name: show-date
    command: date
    
  - name: done
    command: echo "Workflow complete! 🎉"
```

### Step 2: Run the workflow

```bash
dagu start hello.yaml
```

You'll see output like:
```
Starting DAG: hello-world
Step 'greet' started
Hello from Dagu!
Step 'greet' completed successfully
Step 'show-date' started
Mon Jan 15 10:30:45 EST 2024
Step 'show-date' completed successfully
Step 'done' started
Workflow complete! 🎉
Step 'done' completed successfully
DAG 'hello-world' completed successfully
```

### Step 3: Check the status

```bash
dagu status hello.yaml
```

## Understanding the Workflow

Let's break down what we just created:

```yaml
name: hello-world           # Unique identifier for your workflow
description: My first...    # Human-readable description

steps:                      # List of tasks to execute
  - name: greet            # First step
    command: echo "..."    # Command to run
    
  - name: show-date        # Second step
    command: date
    
  - name: done             # Final step
    command: echo "..."
```

### Workflow Execution Flow

```mermaid
graph TD
    A[greet] --> B[show-date]
    B --> C[done]
    
    style A fill:white,stroke:lightblue,stroke-width:1.6px,color:#333
    style B fill:white,stroke:lightblue,stroke-width:1.6px,color:#333
    style C fill:white,stroke:lightblue,stroke-width:1.6px,color:#333
```

Since no dependencies are specified, steps run sequentially by default.

### Key Concepts

1. **Steps**: Individual tasks in your workflow
2. **Commands**: Any shell command you can run

## A More Practical Example

Let's create a workflow that actually does something useful:

```yaml
# backup.yaml
name: daily-backup
description: Backup important files

params:
  - SOURCE_DIR: /home/user/documents
  - BACKUP_DIR: /backup

steps:
  - name: create-timestamp
    command: date +%Y%m%d_%H%M%S
    output: TIMESTAMP
    
  - name: create-backup-dir
    command: mkdir -p ${BACKUP_DIR}/${TIMESTAMP}
    
  - name: copy-files
    command: |
      cp -r ${SOURCE_DIR}/* ${BACKUP_DIR}/${TIMESTAMP}/
      echo "Backed up to ${BACKUP_DIR}/${TIMESTAMP}"
    
  - name: compress
    command: |
      cd ${BACKUP_DIR}
      tar -czf backup_${TIMESTAMP}.tar.gz ${TIMESTAMP}/
      rm -rf ${TIMESTAMP}/
    
  - name: cleanup-old
    command: |
      find ${BACKUP_DIR} -name "backup_*.tar.gz" -mtime +7 -delete
      echo "Cleaned up backups older than 7 days"
```

Run it with custom parameters:
```bash
dagu start backup.yaml -- SOURCE_DIR=/important/data BACKUP_DIR=/mnt/backups
```

## Viewing in the Web UI

Dagu comes with a beautiful web interface to monitor your workflows.

### Start the web server:
```bash
dagu start-all
```

### Open your browser:
Navigate to http://localhost:8080

You'll see:
- Dashboard with all your workflows
- Execution history
- Real-time log viewing
- Visual DAG representation

## Adding Error Handling

Let's make our workflow more robust:

```yaml
# robust-backup.yaml
name: robust-backup
description: Backup with error handling

params:
  - SOURCE_DIR: /home/user/documents
  - BACKUP_DIR: /backup

env:
  - LOG_LEVEL: info

steps:
  - name: check-source
    command: |
      if [ ! -d "${SOURCE_DIR}" ]; then
        echo "ERROR: Source directory does not exist!"
        exit 1
      fi
      echo "Source directory verified"
    
  - name: check-space
    command: |
      AVAILABLE=$(df ${BACKUP_DIR} | awk 'NR==2 {print $4}')
      NEEDED=$(du -s ${SOURCE_DIR} | awk '{print $1}')
      
      if [ $AVAILABLE -lt $NEEDED ]; then
        echo "ERROR: Not enough space!"
        exit 1
      fi
      echo "Sufficient space available"
    
  - name: backup
    command: |
      rsync -av ${SOURCE_DIR}/ ${BACKUP_DIR}/current/
    retryPolicy:
      limit: 3
      intervalSec: 30
    
  - name: notify
    command: echo "Backup completed successfully"
    mailOnError: true

handlerOn:
  failure:
    command: |
      echo "Backup failed! Check logs at ${DAG_RUN_LOG_FILE}"
      # Send alert to monitoring system
      curl -X POST https://alerts.example.com/webhook \
        -d "workflow=backup&status=failed"
  success:
    command: echo "Backup completed at $(date)"
```

## Running on a Schedule

To run your workflow automatically:

```yaml
# scheduled-backup.yaml
name: scheduled-backup
description: Daily backup at 2 AM
schedule: "0 2 * * *"  # Cron expression

steps:
  # ... same steps as before ...
```

Now it will run automatically every day at 2 AM!

## Parallel Execution

Run multiple tasks simultaneously:

```yaml
# parallel-tasks.yaml
name: parallel-processing
description: Process multiple files in parallel

steps:
  - name: prepare
    command: echo "Starting parallel processing"
    
  - name: process-images
    command: ./process-images.sh
    depends: prepare
    
  - name: process-videos
    command: ./process-videos.sh
    depends: prepare
    
  - name: process-documents
    command: ./process-documents.sh
    depends: prepare
    
  - name: combine-results
    command: ./combine-all.sh
    depends:
      - process-images
      - process-videos
      - process-documents
```

### Parallel Execution Flow

```mermaid
graph TD
    A[prepare] --> B[process-images]
    A --> C[process-videos]
    A --> D[process-documents]
    B --> E[combine-results]
    C --> E
    D --> E
    
    style A fill:white,stroke:lightblue,stroke-width:1.6px,color:#333
    style B fill:white,stroke:lime,stroke-width:1.6px,color:#333
    style C fill:white,stroke:lime,stroke-width:1.6px,color:#333
    style D fill:white,stroke:lime,stroke-width:1.6px,color:#333
    style E fill:white,stroke:green,stroke-width:1.6px,color:#333
```

The three processing steps run in parallel after `prepare` completes!

## Tips for Writing Workflows

### 1. Start Simple
Begin with basic sequential steps, then add complexity.

### 2. Use Descriptive Names
```yaml
# Good
- name: validate-input-data
  command: ./validate.sh

# Not so good
- name: step1
  command: ./validate.sh
```

### 3. Handle Errors Gracefully
```yaml
steps:
  - name: might-fail
    command: ./risky-operation.sh
    continueOn:
      failure: true
    retryPolicy:
      limit: 3
```

### 4. Use Output Variables
```yaml
steps:
  - name: get-version
    command: cat VERSION
    output: VERSION
    
  - name: build
    command: docker build -t myapp:${VERSION} .
    depends: get-version
```

### 5. Test Before Scheduling
```bash
# Dry run to validate
dagu dry hello.yaml

# Run once manually
dagu start hello.yaml

# Then add schedule
```

## Common Patterns

### Sequential Processing
```yaml
steps:
  - name: step1
    command: echo "First"
  - name: step2
    command: echo "Second"
    depends: step1
  - name: step3
    command: echo "Third"
    depends: step2
```

### Fan-out/Fan-in
```yaml
steps:
  - name: split
    command: split-data.sh
    
  - name: process-part-1
    command: process.sh part1
    depends: split
    
  - name: process-part-2
    command: process.sh part2
    depends: split
    
  - name: merge
    command: merge-results.sh
    depends:
      - process-part-1
      - process-part-2
```

```mermaid
graph TD
    A[split] --> B[process-part-1]
    A --> C[process-part-2]
    B --> D[merge]
    C --> D
    
    style A fill:white,stroke:lightblue,stroke-width:1.6px,color:#333
    style B fill:white,stroke:lime,stroke-width:1.6px,color:#333
    style C fill:white,stroke:lime,stroke-width:1.6px,color:#333
    style D fill:white,stroke:green,stroke-width:1.6px,color:#333
```

### Conditional Execution
```yaml
steps:
  - name: check-condition
    command: ./check.sh
    output: SHOULD_PROCEED
    
  - name: conditional-step
    command: ./process.sh
    depends: check-condition
    preconditions:
      - condition: "${SHOULD_PROCEED}"
        expected: "yes"
```

```mermaid
flowchart TD
    A[check-condition] --> B{SHOULD_PROCEED == yes?}
    B -->|Yes| C[conditional-step]
    B -->|No| D[Skip]
    
    style A fill:white,stroke:lightblue,stroke-width:1.6px,color:#333
    style B fill:white,stroke:lightblue,stroke-width:1.6px,color:#333
    style C fill:white,stroke:green,stroke-width:1.6px,color:#333
    style D fill:white,stroke:gray,stroke-width:1.6px,color:#333
```

## Debugging Workflows

### View Logs
```bash
# See all logs for a workflow
dagu logs hello.yaml

# See logs for a specific execution
dagu logs hello.yaml --run-id=20240115_103045_abc123
```

### Check History
```bash
# View execution history
dagu history hello.yaml
```

### Use the Web UI
The web interface provides:
- Visual DAG representation
- Step-by-step execution view
- Real-time log streaming
- Error highlighting

## What's Next?

Now that you've created your first workflow, explore more:

1. **[Core Concepts](/getting-started/concepts)** - Understand Dagu's architecture
2. **[Examples](/writing-workflows/examples/)** - See more workflow patterns
3. **[Writing Workflows](/writing-workflows/)** - Deep dive into workflow creation
4. **[Features](/features/)** - Explore advanced capabilities

### Try These Challenges

1. **Add Email Notifications**: Modify the backup workflow to send an email on completion
2. **Use Docker**: Create a workflow that runs steps in Docker containers
3. **Process Data**: Build a simple ETL pipeline that extracts, transforms, and loads data
4. **Schedule Multiple**: Create several workflows with different schedules

Remember: Dagu is designed to be simple. If something seems complicated, there's probably an easier way!

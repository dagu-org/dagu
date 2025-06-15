# Quick Start Guide

Get Dagu running in less than 5 minutes and see why developers love its simplicity and power.

## Install Dagu

<div class="interactive-terminal">
<div class="terminal-command">curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash</div>
<div class="terminal-output">Downloading Dagu v1.14.0...</div>
<div class="terminal-output">Installing to /usr/local/bin/dagu</div>
<div class="terminal-output">Installation complete!</div>
</div>

Or choose your preferred installation method:

::: code-group

```bash [Homebrew]
brew install dagu-org/brew/dagu
```

```bash [Docker]
docker run -p 8080:8080 ghcr.io/dagu-org/dagu:latest
```

```bash [Docker Compose]
# Clone the repository
git clone https://github.com/dagu-org/dagu.git
cd dagu
# Launch with docker compose
docker compose up
```

```bash [Go Install]
go install github.com/dagu-org/dagu/cmd/dagu@latest
```

:::

## Launch the Web UI

You have two ways to get started with Dagu:

### Option A: Direct Launch (Recommended)

Start the server and scheduler together:

```bash
dagu start-all
```

Then browse to http://localhost:8080 to explore the Web UI.

::: tip
The server runs on port `8080` by default. You can change it with the `--port` option:
```bash
dagu start-all --port 9000
```
:::

### Option B: Using Docker Compose

If you prefer Docker, use the included docker-compose.yaml:

```bash
# Clone the repository if you haven't already
git clone https://github.com/dagu-org/dagu.git
cd dagu

# Launch with docker compose
docker compose up
```

Then browse to http://localhost:8080 to access the Web UI.

## Create Your First DAG

1. Navigate to the DAGs page by:
   - Clicking the second button in the left sidebar (📋 icon), or
   - Going directly to http://localhost:8080/dags

2. Click the **New DAG** button at the top of the page

3. Enter `example` as the DAG name in the dialog

## Write Your Workflow

1. Go to the **SPEC** tab in your new DAG
2. Click the **Edit** button
3. Replace the content with this example workflow:

```yaml
schedule: "* * * * *" # Run every minute
params:
  - NAME: "Dagu"
steps:
  - name: Hello world
    command: echo Hello $NAME

  - name: Simulate unclean Command Output
    command: |
      cat <<EOF
      INFO: Starting process...
      DEBUG: Initializing variables...
      DATA: User count is 42
      INFO: Process completed successfully.
      EOF
    output: raw_output
  
  - name: Extract Relevant Data
    command: |
      echo "$raw_output" | grep '^DATA:' | sed 's/^DATA: //'
    output: cleaned_data

  - name: Done
    command: echo Done!
```

This workflow demonstrates:
- **Scheduled execution** - Runs every minute
- **Parameters** - Uses the `NAME` parameter
- **Output capturing** - Saves command output to variables
- **Data processing** - Filters and cleans output between steps

## Execute Your Workflow

Click the **Start** button to run your workflow immediately. Watch as:

1. Each step executes in sequence
2. Output is captured and passed between steps
3. The visual graph updates in real-time
4. Logs stream live in the UI

## Understanding What Happened

Your workflow just demonstrated several key Dagu concepts:

- **Steps**: Individual commands that make up your workflow
- **Schedule**: Automatic execution using cron syntax
- **Parameters**: Variables you can pass into workflows
- **Output Variables**: Capturing and reusing command output
- **Data Pipeline**: Processing and transforming data between steps

## Create a More Realistic Workflow

Let's build something you might actually use - a data processing pipeline:

```yaml
# data-pipeline.yaml
name: Daily Data Processing
schedule: "0 2 * * *"  # Run at 2 AM daily

params:
  - SOURCE_URL: "https://api.example.com/data"
  - ENVIRONMENT: "production"

env:
  - DATA_DIR: /tmp/data/${ENVIRONMENT}
  - LOG_LEVEL: INFO

steps:
  - name: prepare-workspace
    command: |
      mkdir -p ${DATA_DIR}
      echo "Workspace ready at ${DATA_DIR}"
    
  - name: fetch-data
    command: |
      curl -s ${SOURCE_URL} | jq '.' > ${DATA_DIR}/raw.json
      echo "Downloaded $(wc -l < ${DATA_DIR}/raw.json) lines"
    depends: prepare-workspace
    retryPolicy:
      limit: 3
      intervalSec: 30
    
  - name: validate-data
    command: python validate.py ${DATA_DIR}/raw.json
    depends: fetch-data
    continueOn:
      failure: false  # Stop pipeline if validation fails
    
  - name: transform-data
    command: |
      python transform.py \
        --input ${DATA_DIR}/raw.json \
        --output ${DATA_DIR}/processed.parquet
    depends: validate-data
    output: PROCESSED_FILE
    
  - name: upload-results
    command: |
      aws s3 cp ${PROCESSED_FILE} \
        s3://my-bucket/${ENVIRONMENT}/data/$(date +%Y%m%d).parquet
    depends: transform-data
    
  - name: notify-success
    command: |
      curl -X POST https://hooks.slack.com/services/YOUR/WEBHOOK/URL \
        -H 'Content-type: application/json' \
        -d '{"text":"Data pipeline completed successfully!"}'
    depends: upload-results
```

This real-world example shows:
- **Scheduled execution** for daily runs
- **Error handling** with retry policies
- **Conditional execution** based on validation
- **Integration** with external services (S3, Slack)
- **Environment-based** configuration

## Key Features You've Just Seen

✅ **Visual Workflow Editor** - Edit DAGs directly in the browser  
✅ **Real-time Monitoring** - Watch workflows execute live  
✅ **Scheduled Execution** - Cron-based automatic runs  
✅ **Parameter Passing** - Dynamic workflow configuration  
✅ **Output Capturing** - Pass data between steps  
✅ **Error Handling** - Retries and conditional execution  
✅ **Zero Dependencies** - Just a single binary  

## Common Commands

```bash
# Workflow Management
dagu start <workflow.yaml>      # Run a workflow once
dagu stop <workflow.yaml>       # Stop a running workflow
dagu restart <workflow.yaml>    # Restart a workflow
dagu retry --run-id=<id> <workflow.yaml>  # Retry a failed run

# Monitoring & Debugging
dagu status <workflow.yaml>     # Check workflow status
dagu logs <workflow.yaml>       # View workflow logs
dagu dry <workflow.yaml>        # Test without executing

# Server Management
dagu start-all                  # Start server and scheduler
dagu start-all --port=9000      # Use custom port
dagu start-all --dags=/path     # Use custom DAG directory
```

## Next Steps

Now that you've seen Dagu in action, explore:

- 📚 [**Core Concepts**](/getting-started/concepts) - Understand DAGs, steps, and execution models
- 🎯 [**Writing Workflows**](/writing-workflows/) - Master YAML syntax and advanced features
- 🔧 [**Examples**](https://github.com/dagu-org/dagu/tree/main/examples) - Production-ready workflow patterns
- ⚙️ [**Configuration**](/configurations/) - Set up Dagu for your environment
- 🔌 [**Executors**](/executors/) - Use Docker, SSH, HTTP, and more

## Getting Help

- 💬 [GitHub Discussions](https://github.com/dagu-org/dagu/discussions) - Ask questions and share ideas
- 🐛 [GitHub Issues](https://github.com/dagu-org/dagu/issues) - Report bugs or request features
- 📖 [Full Documentation](/writing-workflows/) - Deep dive into all features

## Why Developers Choose Dagu

- **No Infrastructure** - Single binary, no database required
- **Language Agnostic** - Use any tool or language
- **Version Control Friendly** - DAGs are just YAML files
- **Production Ready** - Battle-tested in enterprise environments
- **Open Source** - No vendor lock-in, forever free

Ready to simplify your workflows? You've just scratched the surface of what Dagu can do!
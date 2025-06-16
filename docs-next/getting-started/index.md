# Quick Start Guide

Get Dagu running in 5 minutes using Docker.

## Step 1: Create Your First Workflow

Create a workflow file in your Dagu directory:

```bash
mkdir -p ~/.dagu/dags
cd ~/.dagu/dags
```

Create `hello.yaml`:

```bash
cat > hello.yaml << EOF
steps:
  - name: greet
    command: echo "Hello from Dagu!"
  - name: finish  
    command: echo "Workflow completed successfully!"
    depends: greet
EOF
```

## Step 2: Run Your Workflow via Docker

Execute the workflow using Docker:

```bash
docker run --rm \
  -v ~/.dagu:/app/.dagu \
  -e DAGU_HOME=/app/.dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu start hello
```

You'll see the output from both steps in your terminal.

## Step 3: Start the Server via Docker

Start the Dagu server and scheduler:

```bash
docker run -d \
  --name dagu-server \
  -p 8080:8080 \
  -v ~/.dagu:/app/.dagu \
  -e DAGU_HOME=/app/.dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu start-all
```

## Step 4: Open the Web UI

Open your browser to http://localhost:8080

You can now:
- View your `hello.yaml` workflow in the DAGs list
- Click on it to see the visual graph
- Execute it directly from the UI
- Monitor execution logs in real-time
- Edit workflows in the browser

## Stopping the Server

To stop the Dagu server when you're done:

```bash
docker stop dagu-server
docker rm dagu-server
```

## Next: Create a More Complex Workflow

Try this example that demonstrates parameters and scheduling:

```yaml
# scheduled-workflow.yaml
schedule: "0 * * * *"  # Run every hour
params:
  - NAME: ${USER}
  - DATE: "`date +%Y-%m-%d`"

steps:
  - name: greet
    command: echo "Hello ${NAME}!"
    
  - name: show date
    command: echo "Today is ${DATE}"
    
  - name: process data
    command: |
      echo "Processing data for ${DATE}..."
      sleep 2
      echo "Processing complete!"
    depends: 
      - greet
      - show date
```

Run with custom parameters:

```bash
dagu start scheduled-workflow.yaml -- NAME=Alice DATE=2024-03-15
```

## Common Commands

```bash
# Workflow execution
dagu start <workflow.yaml>              # Run a workflow
dagu start <workflow.yaml> -- KEY=VALUE # Run with parameters
dagu status <workflow.yaml>             # Check status
dagu stop <workflow.yaml>               # Stop running workflow

# Server management  
dagu start-all                          # Start server and scheduler
dagu start-all --port=9000              # Use custom port
```

## What's Next?

- 📚 [**Core Concepts**](/getting-started/concepts) - Understand DAGs and workflow basics
- 🎯 [**Writing Workflows**](/writing-workflows/) - Learn YAML syntax and features
- 🔧 [**Examples**](https://github.com/dagu-org/dagu/tree/main/examples) - Real-world workflow patterns
- 💬 [**Join Discord**](https://discord.gg/gpahPpqyAP) - Get help from the community

## Why Developers Love Dagu

✅ **Zero Dependencies** - Single binary, no database required  
✅ **Language Agnostic** - Use any tool or programming language  
✅ **Simple YAML** - Readable, version-controllable workflows  
✅ **Local First** - Run anywhere, even offline  
✅ **Rich UI** - Beautiful web interface for monitoring  

Ready to automate your workflows? You've just scratched the surface!
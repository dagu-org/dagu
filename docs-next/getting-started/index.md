# Quick Start Guide

Get Dagu running in 5 minutes with this simple guide.

## Step 1: Install Dagu

Choose your preferred installation method:

::: code-group

```bash [Script (Recommended)]
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash
```

```bash [Homebrew]
brew install dagu-org/brew/dagu
```

```bash [Docker]
docker run \
  --rm \
  -p 8080:8080 \
  -v ~/.dagu:/app/dagu \
  -e DAGU_HOME=/app/dagu \
  -e DAGU_TZ=`ls -l /etc/localtime | awk -F'/zoneinfo/' '{print $2}'` \
  ghcr.io/dagu-org/dagu:latest dagu start-all

# Options explained:
# --rm                     Remove container after exit
# -p 8080:8080            Map port 8080 for web UI access
# -v ~/.dagu:/app/dagu    Mount local directory for persistent storage
# -e DAGU_HOME=/app/dagu  Set Dagu home directory inside container
# -e DAGU_TZ=...          Auto-detect timezone from host system
```

:::

## Step 2: Create Your First Workflow

Create a file named `hello.yaml` with this content:

```yaml
steps:
  - name: step 1
    command: echo "Hello from Dagu!"
  - name: step 2  
    command: echo "DAG completed successfully!"
    depends: step 1
```

## Step 3: Run Your Workflow

Execute the workflow:

```bash
dagu start hello.yaml
```

You'll see the output from both steps in your terminal.

## Step 4: Launch the Web UI

Start the Dagu server and scheduler:

```bash
dagu start-all
```

Open your browser to http://localhost:8080

You can now:
- View all your workflows
- Monitor execution in real-time
- Edit workflows in the browser
- Check execution history

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
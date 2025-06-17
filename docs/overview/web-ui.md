# Web UI

Monitor and manage workflows through Dagu's built-in web interface.

## Overview

Dagu includes a modern, responsive web UI that provides:
- Real-time DAG execution monitoring
- Visual DAG representation
- Log viewing and search
- DAG execution history
- DAG (YAML) editor with syntax highlighting and auto-completion
- Interactive DAG management (start, stop, retry, etc.)

::: tip Configuration
For Web UI configuration options, see [Configuration Reference](/reference/config#ui-configuration).
:::

## Accessing the UI

```bash
# Start Dagu with web UI
dagu start-all

# Open in browser
# http://localhost:8080
```

Custom host/port:
```bash
dagu start-all --host 0.0.0.0 --port 9000
```

## Dashboard

The main dashboard shows:

![Dashboard](/dashboard.png)

### Recent Executions
- Timeline of recent workflow runs
- Quick status indicators
- Click to view details

### Filters
- Filter by date range
- Filter by status (success, failed, running)
- Search by workflow name

## DAG Definitions

The DAGs page shows all DAGs and their real-time status. This gives you an immediate overview of your workflows.

![Definitions](/dag-definitions.png)

## DAG Details

Click any DAG to see detailed information including real-time status, logs, and DAG configurations. You can edit DAG configurations directly in the browser.

![DAG Details](/dag-status.png)

### Controls
- **Start**: Run the workflow
- **Stop**: Cancel running execution
- **Retry**: Retry failed execution
- **Edit**: Modify workflow (if permitted)

### Information Tabs
- **Graph**: Visual representation
  - **Drill-down**: Navigate to child DAG executions by double-clicking steps
  - **Update Status**: Change step status manually by right-clicking steps
- **Config**: YAML definition
- **History**: Past executions
- **Log**: Current execution logs

## Execution Details

The execution details page provides in-depth information about a specific workflow run, including real-time updates and logs.

![Execution Details](/status-details.png)

### Real-time Updates
- Live status changes
- Streaming logs
- Progress indicators

### Log Viewer
- Combined workflow log
- Per-step stdout/stderr
- Search within logs
- Download logs

### Step Information
- Start/end times
- Duration
- Exit code
- Output variables

## Execution History

The execution history page shows past execution results and logs, providing a comprehensive view of workflow performance over time.

![Execution History](/dag-history.png)

### Execution List
- Sortable by date, status, duration
- Pagination for large histories
- Quick actions (retry, view logs)

### Execution Timeline
- Visual timeline of executions
- Identify patterns and issues
- Performance trends

## Execution Log

The execution log view shows detailed logs and standard output of each execution and step, helping you debug and monitor workflow behavior.

![Execution Log](/dag-logs.png)

## DAG Editor

Edit workflows directly in the browser:

![DAG Editor](/dag-editor.png)

### Features
- Syntax highlighting
- YAML validation
- Auto-completion
- Save with validation

### Permissions
Requires `writeDAGs` permission:
```yaml
permissions:
  writeDAGs: true
```

## Search

The search functionality allows you to search for specific text across all DAGs in your system, making it easy to find workflows by content, variables, or any other text within the DAG definitions.

![Search](/search.png)

### Global Search
- Search across all DAGs
- Find by name, tags, or content

## UI Customization

### Branding
```yaml
# config.yaml
ui:
  navbarColor: "#00D9FF"
  navbarTitle: "My Workflows"
```

### Display Options
```yaml
ui:
  maxDashboardPageLimit: 100  # Items per page
  logEncodingCharset: utf-8   # Log encoding
```

## Remote Nodes

Monitor multiple Dagu instances:

```yaml
remoteNodes:
  - name: staging
    apiBaseURL: https://staging.example.com/api/v1
    
  - name: production
    apiBaseURL: https://prod.example.com/api/v1
    authToken: ${PROD_TOKEN}
```

### Features
- Unified dashboard
- Centralized management

## Security Considerations

### HTTPS Setup
```yaml
tls:
  certFile: /path/to/cert.pem
  keyFile: /path/to/key.pem
```

### CORS Configuration
Configure for API access from different domains:
```yaml
cors:
  enabled: true
  allowedOrigins:
    - https://app.example.com
```

## See Also

- [Learn the REST API](/overview/api) for automation
- [Configure authentication](/configurations/server#authentication) for security
- [Set up monitoring](/configurations/operations#monitoring) for production

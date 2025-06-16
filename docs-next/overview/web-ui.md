# Web UI

Monitor and manage workflows through Dagu's built-in web interface.

## Overview

Dagu includes a modern, responsive web UI that provides:
- Real-time DAG execution monitoring
- Visual DAG representation
- Log viewing and search
- DAG execution history
- DAG editing capabilities with syntax highlighting and auto-completion
- Interactive DAG management (start, stop, retry, etc.)

No additional setup required - just start Dagu and open your browser.

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

### Status Overview
- **Running**: Currently executing workflows
- **Queued**: Waiting to run
- **Success**: Completed successfully (last 24h)
- **Failed**: Failed executions (last 24h)
- **Total**: All workflows in the system

### Recent Executions
- Timeline of recent workflow runs
- Quick status indicators
- Click to view details

### Filters
- Filter by date range
- Filter by status (success, failed, running)
- Search by workflow name

## Workflows

The main workflows page shows all DAGs and their real-time status. This gives you an immediate overview of your entire workflow ecosystem.

![Workflows](https://raw.githubusercontent.com/dagu-org/dagu/main/assets/images/ui-dags.webp)

## Workflow Details

Click any workflow to see detailed information including real-time status, logs, and DAG configurations. You can edit DAG configurations directly in the browser.

![Workflow Details](https://raw.githubusercontent.com/dagu-org/dagu/main/assets/images/ui-details.webp)

### Visual Graph Layout Options

You can switch between horizontal and vertical graph layouts using the button on the top right corner for better visualization based on your DAG structure.

![Workflow Details Vertical](https://raw.githubusercontent.com/dagu-org/dagu/main/assets/images/ui-details2.webp)

### Visual Graph
- Interactive DAG visualization
- Node colors indicate status:
  - 🟢 Green: Success
  - 🔴 Red: Failed
  - 🔵 Blue: Running
  - ⚪ Gray: Pending
  - ⚫ Black: Cancelled
- Toggle between horizontal and vertical layouts

### Controls
- **Start**: Run the workflow
- **Stop**: Cancel running execution
- **Retry**: Retry failed execution
- **Edit**: Modify workflow (if permitted)

### Information Tabs
- **Graph**: Visual representation
- **Config**: YAML definition
- **History**: Past executions
- **Log**: Current execution logs

## Execution Details

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

![Execution History](https://raw.githubusercontent.com/dagu-org/dagu/main/assets/images/ui-history.webp)

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

![Execution Log](https://raw.githubusercontent.com/dagu-org/dagu/main/assets/images/ui-logoutput.webp)

## DAG Editor

Edit workflows directly in the browser:

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

![Search](https://raw.githubusercontent.com/dagu-org/dagu/main/assets/images/ui-search.webp)

### Global Search
- Search across all DAGs
- Find by name, tags, or content
- Quick navigation

### Log Search
- Full-text search in logs
- Regular expression support
- Highlight matches

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
- Cross-environment monitoring
- Centralized management

## Performance

### Optimizations
- Lazy loading for large logs
- Efficient polling intervals
- Compressed API responses
- Client-side caching

### Recommendations
- Use pagination for history
- Limit dashboard items for performance
- Archive old logs regularly

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `/` | Focus search |
| `r` | Refresh current view |
| `g d` | Go to dashboard |
| `g s` | Go to settings |
| `?` | Show help |

## Browser Support

Tested and supported:
- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)
- Mobile browsers

## Troubleshooting

### UI Not Loading

Check server is running:
```bash
curl http://localhost:8080/api/v1/health
```

### Blank Page

Check browser console for errors:
- Network issues
- Authentication problems
- JavaScript errors

### Slow Performance

- Reduce history retention
- Increase page limits
- Check server resources

### Authentication Issues

For basic auth:
```yaml
auth:
  basic:
    enabled: true
    username: admin
    password: secret
```

Access with: `http://admin:secret@localhost:8080`

## Security Considerations

### HTTPS Setup
```yaml
tls:
  certFile: /path/to/cert.pem
  keyFile: /path/to/key.pem
```

### Reverse Proxy
```nginx
location /dagu/ {
    proxy_pass http://localhost:8080/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### CORS Configuration
Configure for API access from different domains:
```yaml
cors:
  enabled: true
  allowedOrigins:
    - https://app.example.com
```

## Tips and Tricks

### 1. **Quick Status Check**
Bookmark the dashboard with status filter:
```
http://localhost:8080/?status=failed
```

### 2. **Direct DAG Links**
Share direct links to workflows:
```
http://localhost:8080/dags/my-workflow
```

### 3. **Mobile Access**
The UI is responsive - monitor on the go

### 4. **Export Configurations**
Use the API to export DAG configurations:
```bash
curl http://localhost:8080/api/v1/dags/my-workflow
```

### 5. **Bulk Operations**
Use the API for bulk operations while monitoring in UI

## See Also

- [Learn the REST API](/overview/api) for automation
- [Configure authentication](/configurations/server#authentication) for security
- [Set up monitoring](/configurations/operations#monitoring) for production

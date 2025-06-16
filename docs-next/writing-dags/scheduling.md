# Scheduling

Dagu provides flexible scheduling options for automating workflow execution.

## Basic Scheduling

### Cron Expression

Use standard cron expressions to schedule DAGs:

```yaml
schedule: "0 2 * * *"  # Daily at 2 AM
```

### Multiple Schedules

Run a DAG at multiple times:

```yaml
schedule:
  - "0 9 * * MON-FRI"   # 9 AM on weekdays
  - "0 14 * * MON-FRI"  # 2 PM on weekdays
```

### Timezone Support

Specify timezone for schedules:

```yaml
schedule: "CRON_TZ=America/New_York 0 9 * * *"  # 9 AM ET
```

## Advanced Scheduling

### Start/Stop Schedules

Control when long-running DAGs start and stop:

```yaml
schedule:
  start:
    - "0 8 * * MON-FRI"   # Start at 8 AM weekdays
  stop:
    - "0 18 * * MON-FRI"  # Stop at 6 PM weekdays
  restart:
    - "0 12 * * MON-FRI"  # Restart at noon

restartWaitSec: 60  # Wait 60 seconds before restart
```

### Skip If Successful

Prevent redundant executions:

```yaml
schedule: "*/30 * * * *"  # Every 30 minutes
skipIfSuccessful: true    # Skip if last run succeeded
```

## Cron Expression Reference

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

### Common Patterns

```yaml
# Every minute
schedule: "* * * * *"

# Every 5 minutes
schedule: "*/5 * * * *"

# Every hour at minute 0
schedule: "0 * * * *"

# Daily at midnight
schedule: "0 0 * * *"

# Weekly on Sunday at 3 AM
schedule: "0 3 * * 0"

# First day of month at 2 AM
schedule: "0 2 1 * *"

# Weekdays at 9 AM and 5 PM
schedule:
  - "0 9 * * MON-FRI"
  - "0 17 * * MON-FRI"
```

### Special Characters

- `*` - Any value
- `,` - Value list separator
- `-` - Range of values
- `/` - Step values

## Scheduling Controls

### Max Active Runs

Limit concurrent executions:

```yaml
schedule: "*/5 * * * *"
maxActiveRuns: 1  # Only one instance at a time
```

### Delay Start

Add initial delay before execution:

```yaml
schedule: "0 * * * *"
delaySec: 30  # Wait 30 seconds after trigger
```

### Conditional Scheduling

Use preconditions with schedules:

```yaml
schedule: "0 6 * * *"
preconditions:
  - condition: "`date +%d`"
    expected: "01"  # Only run on first of month
```

## Manual Triggers

### Queue Management

Queue DAGs for later execution:

```bash
# Queue with custom run ID
dagu enqueue --run-id=manual-2024-01-15 my-dag.yaml

# Queue with parameters
dagu enqueue my-dag.yaml -- PRIORITY=high

# View queue
dagu queue list

# Remove from queue
dagu dequeue my-dag.yaml
```

### On-Demand Execution

```bash
# Start immediately
dagu start my-dag.yaml

# Start with parameters
dagu start my-dag.yaml -- DATE=2024-01-15 MODE=full
```

## Best Practices

1. **Choose Appropriate Intervals**
   - Consider resource usage
   - Avoid overlapping runs
   - Account for execution time

2. **Use Skip If Successful**
   - Prevent duplicate processing
   - Save resources
   - Maintain idempotency

3. **Set Max Active Runs**
   - Prevent resource exhaustion
   - Ensure sequential processing
   - Control concurrency

4. **Test Schedules**
   - Use dry run first
   - Monitor initial runs
   - Check timezone settings

## Examples

### Data Pipeline Schedule

```yaml
name: daily-etl
schedule: "0 2 * * *"
skipIfSuccessful: true
maxActiveRuns: 1
histRetentionDays: 30

steps:
  - name: extract
    command: extract_data.py
  - name: transform
    command: transform_data.py
    depends: extract
  - name: load
    command: load_data.py
    depends: transform
```

### Business Hours Service

```yaml
name: business-service
schedule:
  start:
    - "0 8 * * MON-FRI"
  stop:
    - "0 18 * * MON-FRI"
restartWaitSec: 60

steps:
  - name: service
    command: ./run-service.sh
    signalOnStop: SIGTERM
```

### Multi-Region Sync

```yaml
name: regional-sync
schedule:
  - "CRON_TZ=America/New_York 0 22 * * *"    # 10 PM ET
  - "CRON_TZ=Europe/London 0 22 * * *"       # 10 PM GMT
  - "CRON_TZ=Asia/Tokyo 0 22 * * *"          # 10 PM JST

params:
  - REGION: "`date +%Z`"

steps:
  - name: sync
    command: sync_region.sh ${REGION}
```
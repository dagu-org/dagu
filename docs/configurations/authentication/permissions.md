# Permissions

Control user capabilities in Dagu.

## Configuration

### YAML Configuration

```yaml
# ~/.config/dagu/config.yaml
permissions:
  writeDAGs: true   # Create/edit/delete workflows
  runDAGs: true     # Execute workflows
```

### Environment Variables

```bash
export DAGU_PERMISSIONS_WRITE_DAGS=true
export DAGU_PERMISSIONS_RUN_DAGS=true

dagu start-all
```

## Permission Levels

### Read-Only Access

```yaml
permissions:
  writeDAGs: false
  runDAGs: false
```

Users can:
- View DAG list and details
- View execution history
- View logs
- Access dashboard

Users cannot:
- Create, edit, or delete DAGs
- Execute DAGs
- Stop running DAGs

### Operator Access

```yaml
permissions:
  writeDAGs: false
  runDAGs: true
```

Users can:
- Everything in read-only access
- Execute existing DAGs
- Stop/restart DAG runs
- Retry failed executions

Users cannot:
- Create, edit, or delete DAGs

### Developer Access

```yaml
permissions:
  writeDAGs: true
  runDAGs: true
```

Users can:
- All operations
- Create, edit, delete DAGs
- Execute DAGs
- Full control

## Use Cases

### Monitoring Dashboard

```yaml
# Read-only access for monitoring
permissions:
  writeDAGs: false
  runDAGs: false
```

### Production Operators

```yaml
# Can run but not modify workflows
permissions:
  writeDAGs: false
  runDAGs: true
```

## Notes

- Permissions apply to all authenticated users
- Default is full access (both permissions true)
- Permissions work with any authentication method
- No user-specific permissions (all users have same permissions)
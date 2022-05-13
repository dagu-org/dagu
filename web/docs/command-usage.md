# Command Reference

**Content**

  - [start](#start)
  - [status](#status)
  - [retry](#retry)
  - [stop](#stop)
  - [dry](#dry)
  - [server](#server)

## start

Starts the specified workflow that is defined in YAML format.

```
dagu start [--params=<params>] <file>
```

## status

Displays the current status of the workflow.

```
dagu status <file>
```

## retry

Retries a workflow execution. You need to specify `--req=<request-id>` and `<file>`.

## stop

Stops a running workflow. TERM signal will be sent to running processes.

```
dagu stop <file>
```

## dry

Dry-runs a workflow. You can check the DAG is correctly configured without running tasks.

```
dagu dry [--params=<params>] <file>
```

## server

Starts a web server for admin UI. Default server URL is `127.0.0.1:8000`. If you want to configure it, see [Web UI Configuration Reference](/docs/admin/web-config)

```
dagu server
```
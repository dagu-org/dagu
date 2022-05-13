# With State Handlers

It is often desirable to take action when a specific event happens, for example, when a workflow fails. To achieve this, you can use `handlerOn` fields.

```yaml
name: example
handlerOn:
  failure:
    command: notify_error.sh
  exit:
    command: cleanup.sh
steps:
  - name: A task
    command: main.sh
```
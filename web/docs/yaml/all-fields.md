# All Available Fields

Combining these settings gives you granular control over how the workflow runs.

```yaml
name: all configuration              # DAG's name
description: run a DAG               # DAG's description
env:                                 # Environment variables
  LOG_DIR: ${HOME}/logs
  PATH: /usr/local/bin:${PATH}
logDir: ${LOG_DIR}                   # Log directory to write standard output
histRetentionDays: 3                 # Execution history retention days (not for log files)
delaySec: 1                          # Interval seconds between steps
maxActiveRuns: 1                     # Max parallel number of running step
params: param1 param2                # Default parameters for the DAG that can be referred to by $1, $2, and so on
preconditions:                       # Precondisions for whether the DAG is allowed to run
  - condition: "`echo 1`"            # Command or variables to evaluate
    expected: "1"                    # Expected value for the condition
mailOn:
  failure: true                      # Send a mail when the DAG failed
  success: true                      # Send a mail when the DAG finished
MaxCleanUpTimeSec: 300               # The maximum amount of time to wait after sending a TERM signal to running steps before killing them
handlerOn:                           # Handlers on Success, Failure, Cancel, and Exit
  success:
    command: "echo succeed"          # Command to execute when the DAG execution succeed
  failure:
    command: "echo failed"           # Command to execute when the DAG execution failed
  cancel:
    command: "echo canceled"         # Command to execute when the DAG execution canceled
  exit:
    command: "echo finished"         # Command to execute when the DAG execution finished
steps:
  - name: some task                  # Step's name
    description: some task           # Step's description
    dir: ${HOME}/logs                # Working directory
    command: python main.py $1       # Command and parameters
    mailOn:
      failure: true                  # Send a mail when the step failed
      success: true                  # Send a mail when the step finished
    continueOn:
      failure: true                   # Continue to the next regardless of the step failed or not
      skipped: true                  # Continue to the next regardless the preconditions are met or not
    retryPolicy:                     # Retry policy for the step
      limit: 2                       # Retry up to 2 times when the step failed
    repeatPolicy:                    # Repeat policy for the step
      repeat: true                   # Boolean whether to repeat this step
      intervalSec: 60                # Interval time to repeat the step in seconds
    preconditions:                   # Precondisions for whether the step is allowed to run
      - condition: "`echo 1`"        # Command or variables to evaluate
        expected: "1"                # Expected Value for the condition
```

The global configuration file `~/.dagu/config.yaml` is useful to gather common settings, such as `logDir` or `env`.

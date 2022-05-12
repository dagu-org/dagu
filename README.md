# Dagu
<img align="right" width="150" src="https://user-images.githubusercontent.com/1475839/165412252-4fbb28ae-0845-4af2-9183-0aa1de5bf707.png" alt="dagu" title="dagu" />

[![Go Report Card](https://goreportcard.com/badge/github.com/yohamta/dagu)](https://goreportcard.com/report/github.com/yohamta/dagu)
[![codecov](https://codecov.io/gh/yohamta/dagu/branch/main/graph/badge.svg?token=CODZQP61J2)](https://codecov.io/gh/yohamta/dagu)
[![GitHub release](https://img.shields.io/github/release/yohamta/dagu.svg)](https://github.com/yohamta/dagu/releases)
[![GoDoc](https://godoc.org/github.com/yohamta/dagu?status.svg)](https://godoc.org/github.com/yohamta/dagu)

**A No-code workflow executor**

Dagu executes [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) from declarative YAML definitions. Dagu also comes with a web UI for visualizing workflows.

## Contents
- [Dagu](#dagu)
  - [Contents](#contents)
  - [Why not Airflow or Prefect?](#why-not-airflow-or-prefect)
  - [️How does it work?](#️how-does-it-work)
  - [️Quick start](#️quick-start)
    - [1. Installation](#1-installation)
    - [2. Download an example YAML file](#2-download-an-example-yaml-file)
    - [3. Launch the web UI](#3-launch-the-web-ui)
    - [4. Running the example](#4-running-the-example)
  - [Command usage](#command-usage)
  - [Web interface](#web-interface)
  - [YAML format](#yaml-format)
    - [Minimal](#minimal)
    - [Environment variables](#environment-variables)
    - [Parameters](#parameters)
    - [Command substitution](#command-substitution)
    - [Conditional logic](#conditional-logic)
    - [State handlers](#state-handlers)
    - [Repeating task](#repeating-task)
    - [All available fields](#all-available-fields)
  - [Admin configuration](#admin-configuration)
    - [Environment variables](#environment-variables-1)
    - [Web UI configuration](#web-ui-configuration)
    - [Global configuration](#global-configuration)
  - [FAQ](#faq)
    - [How to contribute?](#how-to-contribute)
    - [Where is the history data stored?](#where-is-the-history-data-stored)
    - [Where are the log files stored?](#where-are-the-log-files-stored)
    - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
    - [How can a workflow be retried from a specific task?](#how-can-a-workflow-be-retried-from-a-specific-task)
    - [Does it have a scheduler function?](#does-it-have-a-scheduler-function)
    - [How can it communicate with running processes?](#how-can-it-communicate-with-running-processes)
  - [License](#license)
  - [Contributors](#contributors)

## Why not Airflow or Prefect?

Airflow and Prefect are powerful and valuable tools, but they require writing Python code to manage workflows. Our ETL pipeline is already hundreds of thousands of lines of complex code in Perl and shell scripts. Adding another layer of Python on top of this would make it even more complicated. Instead, we needed a more lightweight solution. So we have developed a No-code workflow execution engine that doesn't require writing code. Dagu is easy to use and self-contained, making it ideal for smaller projects with fewer people. We hope that this tool will help others in the same situation.

## ️How does it work?

- Dagu is a single command and it uses the file system to store data in JSON format. Therefore, no DBMS or cloud service is required.
- Dagu executes DAGs defined in declarative YAML format. Existing programs can be used without any modification.

## ️Quick start

### 1. Installation

Download the latest binary from the [Releases page](https://github.com/yohamta/dagu/releases) and place it in your `$PATH`. For example, you can download it in `/usr/local/bin`.

### 2. Download an example YAML file

Download this [example YAML](https://github.com/yohamta/dagu/blob/main/examples/complex_dag.yaml) and place it in the current directory with extension `*.yaml`.

### 3. Launch the web UI

Start the server with `dagu server` and browse to `http://127.0.0.1:8000` to explore the Web UI.

### 4. Running the example

You can start the example by pressing `Start` on the UI.

![example](https://user-images.githubusercontent.com/1475839/165764122-0bdf4bd5-55bb-40bb-b56f-329f5583c597.gif)

## Command usage

- `dagu start [--params=<params>] <file>` - start a workflow
- `dagu status <file>` - display the current status of a workflow
- `dagu retry --req=<request-id> <file>` - retry the failed/canceled workflow
- `dagu stop <file>` - stop a workflow execution by sending a TERM signal
- `dagu dry [--params=<params>] <file>` - dry-run a workflow
- `dagu server` - start a web server for web UI

## Web interface

You can launch the web UI by `dagu server` command. Default URL is `http://127.0.0.1:8000`.

- **DAGs**: Overview of all DAGs (workflows).

  DAGs page displays all workflows and real-time status. To create a new workflow, you can click the button in the top-right corner.

  ![DAGs](https://user-images.githubusercontent.com/1475839/167070248-743b5e8f-ee24-49bf-a4f4-a5225dfc755a.png)

- **Detail**: Realtime status of the workflow.

  The detail page displays the real-time status, logs, and all workflow configurations.

  ![Detail](https://user-images.githubusercontent.com/1475839/166269521-03098e46-6608-43fa-b363-0d00b069c808.png)

- **History**: History of the execution of the workflow.

  The history page allows you to check past execution results and logs.

  ![History](https://user-images.githubusercontent.com/1475839/166269714-18e0b85c-33a6-4da0-92bc-d8ffb7ccd992.png)

## YAML format

### Minimal

A minimal definition is as follows:

```yaml
name: minimal configuration          # DAG's name
steps:                               # Steps inside the DAG
  - name: step 1                     # Step's name (should be unique within the file)
    command: python main_1.py        # Command and arguments to execute
  - name: step 2
    command: python main_2.py
    depends:
      - step 1                       # [optional] Name of the step to depend on
```

### Environment variables

You can define Environment variables and refer using `env` field.

```yaml
name: example
env:
  SOME_DIR: ${HOME}/batch
steps:
  - name: some task in some dir
    dir: ${SOME_DIR}
    command: python main.py
```

### Parameters

You can define parameters using `params` field and refer to each parameter as $1, $2, etc. Parameters can also be command substitutions or environment variables. It can be overridden by `--params=` parameter of `start` command.

```yaml
name: example
params: param1 param2
steps:
  - name: some task with parameters
    command: python main.py $1 $2
```

### Command substitution

You can use command substitution in field values. I.e., a string enclosed in backquotes (`` ` ``) is evaluated as a command and replaced with the result of standard output.

```yaml
name: example
env:
  TODAY: "`date '+%Y%m%d'`"
steps:
  - name: hello
    command: "echo hello, today is ${TODAY}"
```

### Conditional logic

Sometimes you have parts of a workflow that you only want to run under certain conditions. You can use the `precondition` field to add conditional branches to your workflow.

For example, the below task only runs on the first date of each month.

```yaml
name: example
steps:
  - name: A monthly task
    command: monthly.sh
    preconditions:
      - condition: "`date '+%d'`"
        expected: "01"
```

If you want the workflow to continue to the next step regardless of the step's conditional check result, you can use the `continueOn` field:

```yaml
name: example
steps:
  - name: A monthly task
    command: monthly.sh
    preconditions:
      - condition: "`date '+%d'`"
        expected: "01"
    continueOn:
      skipped: true
```

### State handlers

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

### Repeating task

If you want a task to repeat execution at regular intervals, you can use the `repeatPolicy` field. If you want to stop the repeating task, you can use the `stop` command to gracefully stop the task.

```yaml
name: example
steps:
  - name: A task
    command: main.sh
    repeatPolicy:
      repeat: true
      intervalSec: 60
```

### All available fields

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

## Admin configuration

### Environment variables

- `DAGU__DATA` - path to directory for internal use by dagu (default : `~/.dagu/data`)
- `DAGU__LOGS` - path to directory for logging (default : `~/.dagu/logs`)
- `DAGU__ADMIN_PORT` - port number for web URL (default : `8000`)
- `DAGU__ADMIN_NAVBAR_COLOR` - navigation header color for web UI (optional)
- `DAGU__ADMIN_NAVBAR_TITLE` - navigation header title for web UI (optional)

### Web UI configuration

Please create `~/.dagu/admin.yaml`.

```yaml
host: <hostname for web UI address>                          # default value is 127.0.0.1
port: <port number for web UI address>                       # default value is 8000
dags: <the location of DAG configuration files>              # default value is current working directory
command: <Absolute path to the dagu binary>                  # [optional] required if the dagu command not in $PATH
isBasicAuth: <true|false>                                    # [optional] basic auth config
basicAuthUsername: <username for basic auth of web UI>       # [optional] basic auth config
basicAuthPassword: <password for basic auth of web UI>       # [optional] basic auth config
```

### Global configuration

Creating a global configuration `~/.dagu/config.yaml` is a convenient way to organize shared settings.

```yaml
logDir: <path-to-write-log>         # log directory to write standard output
histRetentionDays: 3                # history retention days
smtp:                               # [optional] mail server configuration to send notifications
  host: <smtp server host>
  port: <stmp server port>
errorMail:                          # [optional] mail configuration for error-level
  from: <from address>
  to: <to address>
  prefix: <prefix of mail subject>
infoMail:
  from: <from address>              # [optional] mail configuration for info-level
  to: <to address>
  prefix: <prefix of mail subject>
```

## FAQ

### How to contribute?

Feel free to contribute in any way you want. Share ideas, questions, submit issues, and create pull requests. Thank you!

### Where is the history data stored?

It will store execution history data in the `DAGU__DATA` environment variable path. The default location is `$HOME/.dagu/data`.

### Where are the log files stored?

It will store log files in the `DAGU__LOGS` environment variable path. The default location is `$HOME/.dagu/logs`. You can override the setting by the `logDir` field in a YAML file.

### How long will the history data be stored?

The default retention period for execution history is seven days. However, you can override the setting by the `histRetentionDays` field in a YAML file.

### How can a workflow be retried from a specific task?

You can change the status of any task to a `failed` state. Then, when you retry the workflow, it will execute the failed one and any subsequent.

![Update Status](https://user-images.githubusercontent.com/1475839/166289470-f4af7e14-28f1-45bd-8c32-59cd59d2d583.png)

### Does it have a scheduler function?

No, it doesn't have scheduler functionality. It is meant to be used with cron or other schedulers.

### How can it communicate with running processes?

Dagu uses Unix sockets to communicate with running processes.

![dagu Architecture](https://user-images.githubusercontent.com/1475839/166390371-00bb4af0-3689-406a-a4d5-af943a1fd2ce.png)

## License

This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details

## Contributors

<a href="https://github.com/yohamta/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=yohamta/dagu" />
</a>

Made with [contrib.rocks](https://contrib.rocks).

# ![logo](assets/images/logo-with-background.png?raw=true)

[![Go Report Card](https://goreportcard.com/badge/github.com/yohamta/dagu)](https://goreportcard.com/report/github.com/yohamta/dagu)
[![codecov](https://codecov.io/gh/yohamta/dagu/branch/main/graph/badge.svg?token=CODZQP61J2)](https://codecov.io/gh/yohamta/dagu)
[![GitHub release](https://img.shields.io/github/release/yohamta/dagu.svg)](https://github.com/yohamta/dagu/releases)
[![GoDoc](https://godoc.org/github.com/yohamta/dagu?status.svg)](https://godoc.org/github.com/yohamta/dagu)
![Test](https://github.com/yohamta/dagu/actions/workflows/test.yaml/badge.svg)

**A single binary workflow executor with built-in web UI**

It executes [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) defined in a simple YAML format that is similar to [GitHub Actions](https://github.com/features/actions).

## Contents

  - [Usecases](#usecases)
  - [Why not other popular workflow engines like Airflow?](#why-not-other-popular-workflow-engines-like-airflow)
  - [️How does it work?](#️how-does-it-work)
  - [Install `dagu`](#install-dagu)
    - [via Homebrew](#via-homebrew)
    - [via Bash script](#via-bash-script)
    - [via GitHub Release Page](#via-github-release-page)
  - [️Quick start](#️quick-start)
    - [1. Launch the web UI](#1-launch-the-web-ui)
    - [2. Create a new workflow](#2-create-a-new-workflow)
    - [3. Edit the workflow](#3-edit-the-workflow)
    - [4. Execute the workflow](#4-execute-the-workflow)
  - [Command Line User Interface](#command-line-user-interface)
  - [Web User Interface](#web-user-interface)
  - [YAML format](#yaml-format)
    - [Minimal Definition](#minimal-definition)
    - [Code Snippet](#code-snippet)
    - [Environment Variables](#environment-variables)
    - [Parameters](#parameters)
    - [Command Substitution](#command-substitution)
    - [Conditional Logic](#conditional-logic)
    - [Output](#output)
    - [Redirection](#redirection)
    - [State Handlers](#state-handlers)
    - [Repeating Task](#repeating-task)
    - [All Available Fields](#all-available-fields)
  - [Admin Configuration](#admin-configuration)
    - [Environment Variables](#environment-variables-1)
    - [Web UI Configuration](#web-ui-configuration)
    - [Global Configuration](#global-configuration)
  - [FAQ](#faq)
    - [How to contribute?](#how-to-contribute)
    - [Where is the history data stored?](#where-is-the-history-data-stored)
    - [Where are the log files stored?](#where-are-the-log-files-stored)
    - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
    - [How can I retry a workflow from a specific task?](#how-can-i-retry-a-workflow-from-a-specific-task)
    - [Does it provide sucheduler function?](#does-it-provide-sucheduler-function)
    - [How does it track running processes without DBMS?](#how-does-it-track-running-processes-without-dbms)
  - [License](#license)
  - [Contributors](#contributors)

## Usecases
- Small projects workflow
- Personal task automation
- Batch processing
- ETL pipeline
- Machine learning / AI

## Why not other popular workflow engines like Airflow?

Popular workflow engines, Airflow, Prefect, or Temporal, are powerful and valuable tools, but they require writing code such as Python to run workflows. In many cases, there are already hundreds of thousands of existing lines of code written in other languages such as shell scripts or Perl. Adding another layer of Python on top of these would make it even more complicated. So we decided to develop a new workflow engine, Dagu, which allows you to define DAGs in a simple declarative YAML format without coding for workflow definition. It is self-contained, standalone, has zero dependencies, and does not require DBMS.

## ️How does it work?

- Self-contained - It is a single binary with zero dependency, No DBMS or cloud service is required.
- Simple - It executes DAGs defined in a simple declarative YAML format. Existing programs can be used without any modification.

## Install `dagu`

### via Homebrew
```sh
brew install yohamta/tap/dagu
```

Upgrade to the latest version:
```sh
brew upgrade yohamta/tap/dagu
```

### via Bash script

```sh
curl -L https://raw.githubusercontent.com/yohamta/dagu/main/scripts/downloader.sh | bash
```

### via GitHub Release Page 

Download the latest binary from the [Releases page](https://github.com/yohamta/dagu/releases) and place it in your `$PATH`. For example, you can download it in `/usr/local/bin`.

## ️Quick start

### 1. Launch the web UI

Start the server with `dagu server` and browse to `http://127.0.0.1:8080` to explore the Web UI.

### 2. Create a new workflow

Create a workflow by clicking the `New DAG` button on the top page of the web UI. Input `example.yaml` in the dialog.

### 3. Edit the workflow

Go to the workflow detail page and click the `Edit` button in the `Config` Tab. Copy and paste from this [example YAML](https://github.com/yohamta/dagu/blob/main/examples/example.yaml) and click the `Save` button.

### 4. Execute the workflow

You can execute the example by pressing the `Start` button.
 
![example](assets/images/example.gif?raw=true)

## Command Line User Interface

- `dagu start [--params=<params>] <file>` - Runs the workflow
- `dagu status <file>` - Displays the current status of the workflow
- `dagu retry --req=<request-id> <file>` - Re-runs the specified workflow run
- `dagu stop <file>` - Stops the workflow execution by sending TERM signals
- `dagu dry [--params=<params>] <file>` - Dry-runs the workflow
- `dagu server` - Starts the web server for web UI
- `dagu version` - Shows the current binary version

## Web User Interface

- **Dashboard**: It shows the overall status and executions timeline of the day.

  ![Workflows](assets/images/ui-dashboard.png?raw=true)

- **Workflows**: It shows all workflows and the real-time status.

  ![Workflows](assets/images/ui-workflows.png?raw=true)

- **Workflow Details**: It shows the real-time status, logs, and workflow configurations. You can edit workflow configurations on a browser.

  ![Details](assets/images/ui-details.png?raw=true)

- **Execution History**: It shows past execution results and logs.

  ![History](assets/images/ui-history.png?raw=true)

- **Workflow Execution Log**: It shows the detail log and standard output of each execution and steps.

  ![Workflow Log](assets/images/ui-logoutput.png?raw=true)

## YAML format

### Minimal Definition

Minimal workflow definition is as simple as follows:

```yaml
name: hello world
steps:
 
  - name: step 1
    command: echo hello

  - name: step 2
    command: echo world
    depends:
      - step 1
```

### Code Snippet

`script` field provides a way to run arbitrary snippets of code in any language.

```yaml
name: example
steps:
  - name: step 1
    command: "bash"
    script: |
      cd /tmp
      echo "hello world" > hello
      cat hello
    output: RESULT
  - name: step 2
    command: echo ${RESULT} # hello world
    depends:
      - step 1
```

### Environment Variables

You can define environment variables and refer using `env` field.

```yaml
name: example
env:
  - SOME_DIR: ${HOME}/batch
  - SOME_FILE: ${SOME_DIR}/some_file 
steps:
  - name: some task in some dir
    dir: ${SOME_DIR}
    command: python main.py ${SOME_FILE}
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

Named parameters are also available as follows:

```yaml
name: example
params: ONE=1 TWO=`echo 2`
steps:
  - name: some task with parameters
    command: python main.py $ONE $TWO
```

### Command Substitution

You can use command substitution in field values. I.e., a string enclosed in backquotes (`` ` ``) is evaluated as a command and replaced with the result of standard output.

```yaml
name: example
env:
  TODAY: "`date '+%Y%m%d'`"
steps:
  - name: hello
    command: "echo hello, today is ${TODAY}"
```

### Conditional Logic

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

### Output

`output` field can be used to set a environment variable with standard output. Leading and trailing space will be trimmed automatically. The environment variables can be used in subsequent steps.

```yaml
name: example
steps:
  - name: step 1
    command: "echo foo"
    output: FOO # will contain "foo"
```

### Redirection

`stdout` field can be used to write standard output to a file.

```yaml
name: example
steps:
  - name: create a file
    command: "echo hello"
    stdout: "/tmp/hello" # the content will be "hello\n"
```

### State Handlers

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

### Repeating Task

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

### All Available Fields

Combining these settings gives you granular control over how the workflow runs.

```yaml
name: all configuration              # DAG's name
description: run a DAG               # DAG's description
env:                                 # Environment variables
  - LOG_DIR: ${HOME}/logs
  - PATH: /usr/local/bin:${PATH}
logDir: ${LOG_DIR}                   # Log directory to write standard output
histRetentionDays: 3                 # Execution history retention days (not for log files)
delaySec: 1                          # Interval seconds between steps
maxActiveRuns: 1                     # Max parallel number of running step
params: param1 param2                # Default parameters for the DAG that can be referred to by $1, $2, and so on
preconditions:                       # Precondisions for whether the DAG is allowed to run
  - condition: "`echo $2`"           # Command or variables to evaluate
    expected: "param2"               # Expected value for the condition
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
    command: bash                    # Command and parameters
    stdout: /tmp/outfile
    ouptut: RESULT_VARIABLE
    script: |
      echo "any script"
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
      - condition: "`echo $1`"       # Command or variables to evaluate
        expected: "param1"           # Expected Value for the condition
```

The global configuration file `~/.dagu/config.yaml` is useful to gather common settings, such as `logDir` or `env`.

## Admin Configuration

### Environment Variables

You can customize the admin web UI by environment variables.

- `DAGU__DATA` - path to directory for internal use by dagu (default : `~/.dagu/data`)
- `DAGU__LOGS` - path to directory for logging (default : `~/.dagu/logs`)
- `DAGU__ADMIN_PORT` - port number for web URL (default : `8080`)
- `DAGU__ADMIN_NAVBAR_COLOR` - navigation header color for web UI (optional)
- `DAGU__ADMIN_NAVBAR_TITLE` - navigation header title for web UI (optional)

### Web UI Configuration

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

### Global Configuration

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

Feel free to contribute in any way you want. Share ideas, questions, submit issues, and create pull requests. Take a look at this [TODO list](https://github.com/yohamta/dagu/issues/102). Thanks!

### Where is the history data stored?

It will store execution history data in the `DAGU__DATA` environment variable path. The default location is `$HOME/.dagu/data`.

### Where are the log files stored?

It will store log files in the `DAGU__LOGS` environment variable path. The default location is `$HOME/.dagu/logs`. You can override the setting by the `logDir` field in a YAML file.

### How long will the history data be stored?

The default retention period for execution history is seven days. However, you can override the setting by the `histRetentionDays` field in a YAML file.

### How can I retry a workflow from a specific task?

You can change the status of any task to a `failed` state. Then, when you retry the workflow, it will execute the failed one and any subsequent.

### Does it provide sucheduler function?

No, it doesn't provide scheduler function at this moment. It is meant to be used with cron or other schedulers. But we may add a scheduler functionality in the future development.

### How does it track running processes without DBMS?

Dagu uses Unix sockets to communicate with running processes.

## License

This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details

## Contributors

<a href="https://github.com/yohamta/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=yohamta/dagu" />
</a>

Made with [contrib.rocks](https://contrib.rocks).

# <img src="./assets/images/logo-with-background.png" width="400">

[![Go Report Card](https://goreportcard.com/badge/github.com/yohamta/dagu)](https://goreportcard.com/report/github.com/yohamta/dagu)
[![codecov](https://codecov.io/gh/yohamta/dagu/branch/main/graph/badge.svg?token=CODZQP61J2)](https://codecov.io/gh/yohamta/dagu)
[![GitHub release](https://img.shields.io/github/release/yohamta/dagu.svg)](https://github.com/yohamta/dagu/releases)
[![GoDoc](https://godoc.org/github.com/yohamta/dagu?status.svg)](https://godoc.org/github.com/yohamta/dagu)
![Test](https://github.com/yohamta/dagu/actions/workflows/test.yaml/badge.svg)

**A No-code workflow scheduler with built-in web UI**

It executes [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) defined in a simple, declarative YAML format.

## Contents

  - [Motivation](#motivation)
  - [Why not existing tools, like Airflow?](#why-not-existing-tools-like-airflow)
    - [Simple Example](#simple-example)
    - [Admin Web UI](#admin-web-ui)
  - [Features](#features)
  - [Use cases](#use-cases)
  - [Install `dagu`](#install-dagu)
    - [via Homebrew](#via-homebrew)
    - [via Bash script](#via-bash-script)
    - [via GitHub Release Page](#via-github-release-page)
  - [️Quick start](#️quick-start)
    - [1. Launch the Admin Web UI](#1-launch-the-admin-web-ui)
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
    - [Admin Web UI Configuration](#admin-web-ui-configuration)
    - [Global Configuration](#global-configuration)
  - [REST API Interface](#rest-api-interface)
    - [GET `dags/`](#get-dags)
    - [GET `dags/<workflow name>`](#get-dagsworkflow-name)
    - [POST `dags/<workflow name>`](#post-dagsworkflow-name)
  - [FAQ](#faq)
    - [How to contribute?](#how-to-contribute)
    - [Where is the history data stored?](#where-is-the-history-data-stored)
    - [Where are the log files stored?](#where-are-the-log-files-stored)
    - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
    - [How can I retry a workflow from a specific task?](#how-can-i-retry-a-workflow-from-a-specific-task)
    - [Does it provide sucheduler daemon?](#does-it-provide-sucheduler-daemon)
    - [How does it track running processes without DBMS?](#how-does-it-track-running-processes-without-dbms)
  - [License](#license)
  - [Contributors](#contributors)

## Motivation

We had **many problems** in our ETL pipelines. Hundreds of cron jobs are on the server's crontab, and it is impossible to keep track of those dependencies between them. If one job failed, we were not sure which to re-run. We also have to SSH into the server to see the logs and run each shell script one by one. So we needed a tool that can explicitly visualize and manage the dependencies of the pipeline. ***How nice it would be to be able to visually see the job dependencies, execution status, and logs of each job in a Web UI, and to be able to rerun or stop a series of jobs with just a mouse click!***

## Why not existing tools, like Airflow?

There are many popular workflow engines such as Airflow, Prefect, etc. They are compelling and valuable tools, but they require writing Python code to run workflows. In many situations like above, there are already hundreds of thousands of existing lines of code in other languages such as shell scripts or Perl in many cases. Adding another layer of Python on top of these would make it even more complicated. So we began to develop an easy-to-use workflow scheduler that does not need any coding to manage workflows.

Dagu is self-contained, zero dependency, and does not require DBMS. These features make it an ideal workflow scheduler to use for existing code base, personal projects, or smaller use cases with fewer groups of people.

### Simple Example

Here's a simple example workflow definition that creates a SQL file, executes it on a Postgres DB, and writes results to a file.

```yaml
steps:
  - name: step1
    command: bash" 
    script: |
      echo "select * from A;" > select.sql
  - name: step2
    command: "psql -U username -d myDataBase -f select.sql"
    stdout: output.txt
    depends:
      - step1
```

### Admin Web UI

Dagu also comes with a featureful admin Web UI for visualization. You can visualize, create, edit, and run workflows on a browser.

![example](assets/images/example.gif?raw=true)

## Features

- Simple command interface
- Simple configuration YAML format
- Simple architecture (no DBMS or agent process is required)
- Web UI to visualize, manage jobs and watch logs
- Parameterization
- Conditions
- Automatic retry
- Cancellation
- Retry
- Prallelism limits
- Environment variables
- Repeat
- Basic Authentication
- E-mail notifications
- REST api interface
- onExit / onSuccess / onFailure / onCancel handlers
- Automatic data cleaning

## Use cases
- ETL Pipeline
- Batches
- Machine Learning
- Data Processing
- Automation

## Install `dagu`

You can quickly install `dagu` command and try it out.

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

### 1. Launch the Admin Web UI

Start the server with `dagu server` and browse to `http://127.0.0.1:8080` to explore the Admin Web UI.

### 2. Create a new workflow

Create a workflow by clicking the `New DAG` button on the top page of the web UI. Input `example.yaml` in the dialog.

### 3. Edit the workflow

Go to the workflow detail page and click the `Edit` button in the `Config` Tab. Copy and paste from this [example YAML](https://github.com/yohamta/dagu/blob/main/examples/example.yaml) and click the `Save` button.

### 4. Execute the workflow

You can execute the example by pressing the `Start` button.

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
params: param1 param2
steps:
  - name: some task with parameters
    command: python main.py $1 $2
```

Named parameters are also available as follows:

```yaml
params: ONE=1 TWO=`echo 2`
steps:
  - name: some task with parameters
    command: python main.py $ONE $TWO
```

### Command Substitution

You can use command substitution in field values. I.e., a string enclosed in backquotes (`` ` ``) is evaluated as a command and replaced with the result of standard output.

```yaml
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
steps:
  - name: A monthly task
    command: monthly.sh
    preconditions:
      - condition: "`date '+%d'`"
        expected: "01"
```

If you want the workflow to continue to the next step regardless of the step's conditional check result, you can use the `continueOn` field:

```yaml
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
steps:
  - name: step 1
    command: "echo foo"
    output: FOO # will contain "foo"
```

### Redirection

`stdout` field can be used to write standard output to a file.

```yaml
steps:
  - name: create a file
    command: "echo hello"
    stdout: "/tmp/hello" # the content will be "hello\n"
```

### State Handlers

It is often desirable to take action when a specific event happens, for example, when a workflow fails. To achieve this, you can use `handlerOn` fields.

```yaml
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
name: all configuration              # name (optional, default is filename)
description: run a DAG               # description
tags: daily job                      # Free tags (separated by comma)
env:                                 # Environment variables
  - LOG_DIR: ${HOME}/logs
  - PATH: /usr/local/bin:${PATH}
logDir: ${LOG_DIR}                   # Log directory to write standard output
histRetentionDays: 3                 # Execution history retention days (not for log files)
delaySec: 1                          # Interval seconds between steps
maxActiveRuns: 1                     # Max parallel number of running step
params: param1 param2                # Default parameters that can be referred to by $1, $2, ...
preconditions:                       # Precondisions for whether the it is allowed to run
  - condition: "`echo $2`"           # Command or variables to evaluate
    expected: "param2"               # Expected value for the condition
mailOn:
  failure: true                      # Send a mail when the it failed
  success: true                      # Send a mail when the it finished
MaxCleanUpTimeSec: 300               # The maximum amount of time to wait after sending a TERM signal to running steps before killing them
handlerOn:                           # Handlers on Success, Failure, Cancel, and Exit
  success:
    command: "echo succeed"          # Command to execute when the execution succeed
  failure:
    command: "echo failed"           # Command to execute when the execution failed
  cancel:
    command: "echo canceled"         # Command to execute when the execution canceled
  exit:
    command: "echo finished"         # Command to execute when the execution finished
steps:
  - name: some task                  # Step name
    description: some task           # Step description
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

### Admin Web UI Configuration

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

## REST API Interface

Dagu server has simple APIs to query and control workflows.

### GET `dags/`
TBU

### GET `dags/<workflow name>`
TBU

### POST `dags/<workflow name>`
TBU

## FAQ

### How to contribute?

Feel free to contribute in any way you want. Share ideas, questions, submit issues, and create pull requests. Thanks!

### Where is the history data stored?

It will store execution history data in the `DAGU__DATA` environment variable path. The default location is `$HOME/.dagu/data`.

### Where are the log files stored?

It will store log files in the `DAGU__LOGS` environment variable path. The default location is `$HOME/.dagu/logs`. You can override the setting by the `logDir` field in a YAML file.

### How long will the history data be stored?

The default retention period for execution history is seven days. However, you can override the setting by the `histRetentionDays` field in a YAML file.

### How can I retry a workflow from a specific task?

You can change the status of any task to a `failed` state. Then, when you retry the workflow, it will execute the failed one and any subsequent.

### Does it provide sucheduler daemon?

No, not yet. It is meant to be used with cron or other schedulers. If you could implement the scheduler daemon function and create a PR, it would be greatly appreciated.

### How does it track running processes without DBMS?

Dagu uses Unix sockets to communicate with running processes.

## License

This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details

## Contributors

<a href="https://github.com/yohamta/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=yohamta/dagu" />
</a>

Made with [contrib.rocks](https://contrib.rocks).

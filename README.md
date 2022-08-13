# Dagu
<img align="right" src="./assets/images/dagu.png" width="200" alt="logo">

[![Go Report Card](https://goreportcard.com/badge/github.com/yohamta/dagu)](https://goreportcard.com/report/github.com/yohamta/dagu)
[![codecov](https://codecov.io/gh/yohamta/dagu/branch/main/graph/badge.svg?token=CODZQP61J2)](https://codecov.io/gh/yohamta/dagu)
[![GitHub release](https://img.shields.io/github/release/yohamta/dagu.svg)](https://github.com/yohamta/dagu/releases)
[![GoDoc](https://godoc.org/github.com/yohamta/dagu?status.svg)](https://godoc.org/github.com/yohamta/dagu)
![Test](https://github.com/yohamta/dagu/actions/workflows/test.yaml/badge.svg)

**A just another Cron alternative with a Web UI, but with much more capabilities**

It runs [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) defined in a simple YAML format.

## Contents

- [Dagu](#dagu)
  - [Contents](#contents)
  - [Highlights](#highlights)
  - [Motivation](#motivation)
  - [Why not existing workflow schedulers, such as Airflow?](#why-not-existing-workflow-schedulers-such-as-airflow)
  - [How does it work?](#how-does-it-work)
  - [Install `dagu`](#install-dagu)
    - [via Homebrew](#via-homebrew)
    - [via Bash script](#via-bash-script)
    - [via GitHub Release Page](#via-github-release-page)
  - [️Quick start](#️quick-start)
    - [1. Launch the Web UI](#1-launch-the-web-ui)
    - [2. Create a new DAG](#2-create-a-new-dag)
    - [3. Edit the DAG](#3-edit-the-dag)
    - [4. Execute the DAG](#4-execute-the-dag)
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
    - [Lifecycle Hooks](#lifecycle-hooks)
    - [Repeating Task](#repeating-task)
    - [Calling Sub DAGs](#calling-sub-dags)
    - [All Available Fields](#all-available-fields)
  - [Admin Configuration](#admin-configuration)
    - [Environment Variables](#environment-variables-1)
    - [Admin Configuration](#admin-configuration-1)
    - [Base Config for all DAGs](#base-config-for-all-dags)
  - [Scheduler](#scheduler)
    - [Execution Schedule](#execution-schedule)
    - [Run Scheduler as a daemon](#run-scheduler-as-a-daemon)
    - [Scheduler Configuration](#scheduler-configuration)
  - [REST API Interface](#rest-api-interface)
  - [FAQ](#faq)
    - [How to contribute?](#how-to-contribute)
    - [Where is the history data stored?](#where-is-the-history-data-stored)
    - [Where are the log files stored?](#where-are-the-log-files-stored)
    - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
    - [How can I retry a DAG from a specific task?](#how-can-i-retry-a-dag-from-a-specific-task)
    - [How does it track running processes without DBMS?](#how-does-it-track-running-processes-without-dbms)
  - [License](#license)
  - [Contributors](#contributors)

## Highlights

- Install by placing just a single binary file
- Schedule executions of DAGs with Cron expressions
- Define dependencies between related jobs and represent them as a single DAG (unit of execution)

## Motivation

In the projects I worked on, our ETL pipeline had **many problems**. There were hundreds of cron jobs on the server's crontab, and it is impossible to keep track of those dependencies between them. If one job failed, we were not sure which to rerun. We also have to SSH into the server to see the logs and run each shell script one by one. So we needed a tool that could explicitly visualize and manage the dependencies of the pipeline. ***How nice it would be to be able to visually see the job dependencies, execution status, and logs of each job in a Web UI and to be able to rerun or stop a series of jobs with just a mouse click!***

## Why not existing workflow schedulers, such as Airflow?

There are existing tools such as Airflow, Prefect, Temporal, etc, but in most cases they require writing code in a programming language such as Python to define DAGs. In systems that have been in operation for a long time, there are already complex jobs written in hundreds of thousands of lines of code in other languages such as Perl or Shell Scripts, and there is concern that adding another layer of Python code will further decrease maintainability. So we developed Dagu, which requires no coding, and is easy-to-use and self-contained, making it ideal for smaller projects with fewer people.

## How does it work?
Dagu is a single command and has zero dependency. No DBMS or cloud is required.
It runs DAGs defined in a simple YAML text. Any existing programs are available as it is.

## Install `dagu`

Quickly install `dagu` command and try it out.

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

Download the latest binary from the [Releases page](https://github.com/yohamta/dagu/releases) and place it in your `$PATH` (e.g. `/usr/local/bin`).

## ️Quick start

### 1. Launch the Web UI

Run `dagu server` and browse to `http://127.0.0.1:8080` to explore the Web UI.

### 2. Create a new DAG

Press the `New DAG` button in the web UI. Input `example` in the input dialog.

*Note: DAG (YAML) files will be placed in `~/.dagu/dags` by default. See [Admin Configuration](#admin-configuration) for more details.*

### 3. Edit the DAG

Go to the `SPEC` Tab and hit the `Edit` button. Copy & Paste this [example YAML](https://github.com/yohamta/dagu/blob/main/examples/example.yaml) and click the `Save` button.

### 4. Execute the DAG

Press the `Start` button and the DAG will run.

![example](assets/images/example.gif?raw=true)

## Command Line User Interface

- `dagu start [--params=<params>] <file>` - Runs the DAG
- `dagu status <file>` - Displays the current status of the DAG
- `dagu retry --req=<request-id> <file>` - Re-runs the specified DAG run
- `dagu stop <file>` - Stops the DAG execution by sending TERM signals
- `dagu dry [--params=<params>] <file>` - Dry-runs the DAG
- `dagu server` - Starts the web server for web UI
- `dagu scheduler [--dags=<DAGs directory>]` - Starts the scheduler process
- `dagu version` - Shows the current binary version

The `--config=<config>` option is available to all commands. It allows to specify different Dagu configuration for the commands. Which enables you to manage multiple Dagu process in a single instance. See [Admin Configuration](#admin-configuration) for more details.

For example:

```bash
dagu server --config=~/.dagu/dev.yaml
dagu scheduler --config=~/.dagu/dev.yaml
```

## Web User Interface

- **Dashboard**: It shows the overall status and executions timeline of the day.

  ![DAGs](assets/images/ui-dashboard.png?raw=true)

- **DAGs**: It shows all DAGs and the real-time status.

  ![DAGs](assets/images/ui-dags.png?raw=true)

- **DAG Details**: It shows the real-time status, logs, and DAG configurations. You can edit DAG configurations on a browser.

  ![Details](assets/images/ui-details.png?raw=true)

  You can switch to the vertical graph with the button on the top right corner.

  ![Details-TD](assets/images/ui-details2.png?raw=true)

- **Execution History**: It shows past execution results and logs.

  ![History](assets/images/ui-history.png?raw=true)

- **DAG Execution Log**: It shows the detail log and standard output of each execution and step.

  ![DAG Log](assets/images/ui-logoutput.png?raw=true)

## YAML format

### Minimal Definition

Specify steps with only `name` and `commands`.

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

Define environment variables and refer to using `env` field.

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

Define parameters using `params` field and refer to each parameter as $1, $2, etc. Parameters can also be command substitutions or environment variables. It can be overridden by `--params=` parameter of `start` command.

```yaml
params: param1 param2
steps:
  - name: some task with parameters
    command: python main.py $1 $2
```

Named parameters are avialble as below:

```yaml
params: ONE=1 TWO=`echo 2`
steps:
  - name: some task with parameters
    command: python main.py $ONE $TWO
```

### Command Substitution

A string enclosed in backquotes (`` ` ``) is evaluated as a command and replaced with the result of standard output.

```yaml
env:
  TODAY: "`date '+%Y%m%d'`"
steps:
  - name: hello
    command: "echo hello, today is ${TODAY}"
```

### Conditional Logic

Sometimes you have parts of a DAG that you only want to run under certain conditions. You can use the `precondition` field to add conditional branches to your DAG.

For example, the below task only runs on the first date of each month.

```yaml
steps:
  - name: A monthly task
    command: monthly.sh
    preconditions:
      - condition: "`date '+%d'`"
        expected: "01"
```

If you want a DAG to continue to the next step regardless of the step's conditional check result, you can use the `continueOn` field:

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

### Lifecycle Hooks

It is often desirable to take action when a specific event happens, for example, when a DAG fails. To achieve this, you can use `handlerOn` fields.

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

`repeatPolicy` field is available to repeat a task. `dagu stop <DAG>` command will gracefully stop the task.

```yaml
steps:
  - name: A task
    command: main.sh
    repeatPolicy:
      repeat: true
      intervalSec: 60
```

### Calling Sub DAGs

`dagu start` command can call other DAG (you can omit `.yaml` in the DAG name).

```yaml
steps:
  - name: Sub DAG
    command: dagu start other_dag
```

You need to specify absolute path when you want to call the DAGs in other directory.

```yaml
steps:
  - name: Sub DAG
    command: dagu start /path/to/dag.yaml
```

### All Available Fields

Combining these settings gives you granular control over how the DAG runs.

```yaml
name: all configuration              # Name (optional, default is filename)
description: run a DAG               # Description
schedule: "0 * * * *"                # Execution schedule (cron expression)
group: DailyJobs                     # Group name to organize DAGs (optional)
tags: example                        # Free tags (separated by comma)
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
    dir: ${HOME}/logs                # Working directory (default: the same directory of the DAG file)
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
      intervalSec: 5                 # Interval time before retry
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

Define `DAGU_HOME` environment variables to configure the base path of the Dagu's internal work directory. Default is `~/.dagu/`.

### Admin Configuration

Create the admin config file (default: `~/.dagu/admin.yaml`) to configure Dagu. All fields are optional.

```yaml
baseConfig: <base DAG config path> .                         # default: ${DAG_HOME}/config.yaml

host: <hostname for web UI address>                          # default: 127.0.0.1
port: <port number for web UI address>                       # default: 8000
dags: <the location of DAG configuration files>              # default: ${DAG_HOME}/dags
logDir: <internal logdirectory>                              # default: ${DAG_HOME}/logs/admin
command: <Absolute path to the dagu binary>                  # default: dagu

navbarColor: <admin-web header color>                        # header color for web UI (e.g. "#ff0000")
navbarTitle: <admin-web title text>                          # header title for web UI (e.g. "PROD")

isBasicAuth: <true|false>                                    # enables basic auth
basicAuthUsername: <username for basic auth of web UI>       # basic auth user
basicAuthPassword: <password for basic auth of web UI>       # basic auth password
```

### Base Config for all DAGs

Create the base config file (default: `~/.dagu/config.yaml`) to define the common settings. All DAGs will share the same config once you create the config file. Each DAGs can overrite the setting if it defines the same field inside the DAG.

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

## Scheduler

Run `dagu scheduler` process on your system to schedule DAGs automatically.

### Execution Schedule
Specify `schedule` field in the config file to schedule the DAG. Use Cron expression:

```yaml
schedule: "5 4 * * *" # Run at 04:05.
steps:
  - name: scheduled job
    command: job.sh
```

You can specify multiple schedules for a DAG:

```yaml
schedule:
  - "30 7 * * *" # Run at 7:30
  - "0 20 * * *" # Also run at 20:00
steps:
  - name: scheduled job
    command: job.sh
```

### Run Scheduler as a daemon

The easiest way is to keep scheduler running is create the below script and call it every minute using Cron:

```bash
#!/bin/bash
process="dagu scheduler"
command="/usr/bin/dagu scheduler"

if ps ax | grep -v grep | grep "$process" > /dev/null
then
    exit
else
    $command &
fi

exit
```

### Scheduler Configuration

Set the `dags` field to specify the directory of the DAGs.

```yaml
dags: <the location of DAG configuration files> # default: (~/.dagu/dags)
```

## REST API Interface

Refer to [REST API Docs](./docs/restapi.md)

## FAQ

### How to contribute?

Feel free to contribute in any way you want. Share ideas, questions, submit issues, and create pull requests. Thanks!

### Where is the history data stored?

It will store execution history data in the `DAGU__DATA` environment variable path. The default location is `$HOME/.dagu/data`.

### Where are the log files stored?

It will store log files in the `DAGU__LOGS` environment variable path. The default location is `$HOME/.dagu/logs`. You can override the setting by the `logDir` field in a YAML file.

### How long will the history data be stored?

The default retention period for execution history is 30 days. However, you can override the setting by the `histRetentionDays` field in a YAML file.

### How can I retry a DAG from a specific task?

You can change the status of any task to a `failed` state. Then, when you retry the DAG, it will execute the failed one and any subsequent.

### How does it track running processes without DBMS?

Dagu uses Unix sockets to communicate with running processes.

## License

This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details

## Contributors

<a href="https://github.com/yohamta/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=yohamta/dagu" />
</a>

Made with [contrib.rocks](https://contrib.rocks).

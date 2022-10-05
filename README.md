<p align="center">
  <img src="./assets/images/dagu-logo-dark.png#gh-dark-mode-only" width="300" alt="dagu-logo">
  <img src="./assets/images/dagu-logo-light.png#gh-light-mode-only" width="300" alt="dagu-logo">
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/yohamta/dagu">
    <img src="https://goreportcard.com/badge/github.com/yohamta/dagu" />
  </a>
  <a href="https://codecov.io/gh/yohamta/dagu">
    <img src="https://codecov.io/gh/yohamta/dagu/branch/main/graph/badge.svg?token=CODZQP61J2" />
  </a>
  <a href="https://github.com/yohamta/dagu/releases">
    <img src="https://img.shields.io/github/release/yohamta/dagu.svg" />
  </a>
  <a href="https://godoc.org/github.com/yohamta/dagu">
    <img src="https://godoc.org/github.com/yohamta/dagu?status.svg" />
  </a>
  <img src="https://github.com/yohamta/dagu/actions/workflows/test.yaml/badge.svg" />
</p>

<p align="center">
<b>A just another Cron alternative with a Web UI, but with much more capabilities</b><br />
It runs <a href="https://en.wikipedia.org/wiki/Directed_acyclic_graph">DAGs (Directed acyclic graph)</a> defined in a simple YAML format.
</p>

---

## Highlights
- Install by placing just a single binary file
- Schedule executions of DAGs with Cron expressions
- Define dependencies between related jobs and represent them as a single DAG (unit of execution)

## Contents

- [Highlights](#highlights)
- [Contents](#contents)
- [Getting started](#getting-started)
- [Motivation](#motivation)
- [Why not an existing workflow scheduler like Airflow?](#why-not-an-existing-workflow-scheduler-like-airflow)
- [How does it work?](#how-does-it-work)
- [Install `dagu`](#install-dagu)
  - [via Homebrew](#via-homebrew)
  - [via Bash script](#via-bash-script)
  - [via Docker Image](#via-docker-image)
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
  - [Stdout and Stderr Redirection](#stdout-and-stderr-redirection)
  - [Lifecycle Hooks](#lifecycle-hooks)
  - [Repeating Task](#repeating-task)
  - [Other Available Fields](#other-available-fields)
- [Executor](#executor)
  - [HTTP Executor](#http-executor)
- [Admin Configuration](#admin-configuration)
- [Environment Variable](#environment-variable)
- [Sending email notifications](#sending-email-notifications)
- [Base Configuration for all DAGs](#base-configuration-for-all-dags)
- [Scheduler](#scheduler)
  - [Execution Schedule](#execution-schedule)
  - [Stop Schedule](#stop-schedule)
  - [Restart Schedule](#restart-schedule)
  - [Run Scheduler as a daemon](#run-scheduler-as-a-daemon)
  - [Scheduler Configuration](#scheduler-configuration)
- [REST API Interface](#rest-api-interface)
- [FAQ](#faq)
  - [How to contribute?](#how-to-contribute)
- [How to build `dagu` locally?](#how-to-build-dagu-locally)
  - [Where is the history data stored?](#where-is-the-history-data-stored)
  - [Where are the log files stored?](#where-are-the-log-files-stored)
  - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
  - [How to use specific `host` and `port` for `dagu server`?](#how-to-use-specific-host-and-port-for-dagu-server)
  - [How to specify the DAGs directory for `dagu server` and `dagu scheduler`?](#how-to-specify-the-dags-directory-for-dagu-server-and-dagu-scheduler)
  - [How can I retry a DAG from a specific task?](#how-can-i-retry-a-dag-from-a-specific-task)
  - [How does it track running processes without DBMS?](#how-does-it-track-running-processes-without-dbms)
- [License](#license)
- [Contributors](#contributors)

## Getting started

See [Install `dagu`](#install-dagu) and [️Quick start](#️quick-start).

## Motivation

In legacy complex systems there are implicit dependencies of jobs on each other. When there are hundreds of cron jobs in a server's crontab, it is impossible to keep track of the dependencies between them. If one job fails, it is impossible to know which one to rerun. You also have to SSH into the server to see the logs. And to rerun them, you have to manually run the shell scripts one by one. This is a huge hassle and makes operation impossible. We need a tool that can explicitly visualize and manage pipeline dependencies as a DAG. ***How nice it would be if we could visually check the dependencies, execution status, and logs of each job in a Web UI, and rerun or stop a series of jobs with just a mouse click!***

## Why not an existing workflow scheduler like Airflow?

There are existing tools such as Airflow, Prefect, Temporal, etc., but many libraries require you to write code in a programming language such as Python to define the DAG. In systems that have been in operation for a long time, there are already complex jobs with hundreds of thousands of lines of code written in other languages such as Perl or Shell Scripts. Adding another layer of Python on top of these codes would further reduce maintainability. So we developed dagu, which requires no coding, is easy to use, self-contained, and ideal for small projects.

## How does it work?
dagu is a single command and it uses the local file system to store data. Therefore, no DBMS or cloud service is required.
dagu executes DAGs defined in declarative YAML format. Existing programs can be used without any modification.

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

### via Docker Image

```sh
docker run \
--rm \
-p 8080:8080 \
-v $HOME/.dagu/dags:/home/dagu/.dagu/dags \
-v $HOME/.dagu/data:/home/dagu/.dagu/data \
-v $HOME/.dagu/logs:/home/dagu/.dagu/logs \
yohamta/dagu:latest
```

### via GitHub Release Page 

Download the latest binary from the [Releases page](https://github.com/yohamta/dagu/releases) and place it in your `$PATH` (e.g. `/usr/local/bin`).

## ️Quick start

### 1. Launch the Web UI

Start the server with `dagu server` and browse to `http://127.0.0.1:8080` to explore the Web UI.

### 2. Create a new DAG

Create a DAG by clicking the `New DAG` button on the top page of the web UI. Input `example` in the dialog.

*Note: DAG (YAML) files will be placed in `~/.dagu/dags` by default. See [Admin Configuration](#admin-configuration) for more details.*

### 3. Edit the DAG

Go to the `SPEC` Tab and hit the `Edit` button. Copy & Paste this [example YAML](https://github.com/yohamta/dagu/blob/main/examples/example.yaml) and click the `Save` button.

### 4. Execute the DAG

You can execute the example by pressing the `Start` button.

*Note: Leave the parameter field in the dialog blank and press OK.*

![example](assets/images/demo.gif?raw=true)

## Command Line User Interface

- `dagu start [--params=<params>] <file>` - Runs the DAG
- `dagu status <file>` - Displays the current status of the DAG
- `dagu retry --req=<request-id> <file>` - Re-runs the specified DAG run
- `dagu stop <file>` - Stops the DAG execution by sending TERM signals
- `dagu restart <file>` - Restart the current running DAG
- `dagu dry [--params=<params>] <file>` - Dry-runs the DAG
- `dagu server [--host=<host>] [--port=<port>] [--dags=<path/to/the DAGs directory>]` - Starts the web server for web UI
- `dagu scheduler [--dags=<path/to/the DAGs directory>]` - Starts the scheduler process
- `dagu version` - Shows the current binary version

The `--config=<config>` option is available to all commands. It allows to specify different dagu configuration for the commands. Which enables you to manage multiple dagu process in a single instance. See [Admin Configuration](#admin-configuration) for more details.

For example:

```bash
dagu server --config=~/.dagu/dev.yaml
dagu scheduler --config=~/.dagu/dev.yaml
```

## Web User Interface

- **DAGs**: It shows all DAGs and the real-time status.

  ![DAGs](assets/images/ui-dags.png?raw=true)

- **DAG Details**: It shows the real-time status, logs, and DAG configurations. You can edit DAG configurations on a browser.

  ![Details](assets/images/ui-details.png?raw=true)

  You can switch to the vertical graph with the button on the top right corner.

  ![Details-TD](assets/images/ui-details2.png?raw=true)

- **Search DAGs**: It greps given text across all DAGs.

  ![History](assets/images/ui-search.png?raw=true)

- **Execution History**: It shows past execution results and logs.

  ![History](assets/images/ui-history.png?raw=true)

- **DAG Execution Log**: It shows the detail log and standard output of each execution and step.

  ![DAG Log](assets/images/ui-logoutput.png?raw=true)

## YAML format

### Minimal Definition

The minimal DAG definition is as simple as follows:

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

You can define environment variables and refer to using `env` field.

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

Sometimes you have parts of a DAG that you only want to run under certain conditions. You can use the `preconditions` field to add conditional branches to your DAG.

For example, the below task only runs on the first date of each month.

```yaml
steps:
  - name: A monthly task
    command: monthly.sh
    preconditions:
      - condition: "`date '+%d'`"
        expected: "01"
```

If you want the DAG to continue to the next step regardless of the step's conditional check result, you can use the `continueOn` field:

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

### Stdout and Stderr Redirection

`stdout` field can be used to write standard output to a file.

```yaml
steps:
  - name: create a file
    command: "echo hello"
    stdout: "/tmp/hello" # the content will be "hello\n"
```

`stderr` field allows to redirect stderr to other file without writing to the normal log file.

```yaml
steps:
  - name: output error file
    command: "echo error message >&2"
    stderr: "/tmp/error.txt"
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

If you want a task to repeat execution at regular intervals, you can use the `repeatPolicy` field. If you want to stop the repeating task, you can use the `stop` command to gracefully stop the task.

```yaml
steps:
  - name: A task
    command: main.sh
    repeatPolicy:
      repeat: true
      intervalSec: 60
```

### Other Available Fields

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
logDir: ${LOG_DIR}                   # Log directory to write standard output, default: ${DAGU_HOME}/logs/dags
restartWaitSec: 60                   # Wait 60s after the process is stopped, then restart the DAG.
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
    signalOnStop: "SIGINT"           # Specify signal name (e.g. SIGINT) to be sent when process is stopped
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

## Executor

Executor is a different way of executing a Step; Executor can be set in the `executor` field.

### HTTP Executor

The HTTP Executor allows you to send arbitrary HTTP requests.

```yaml
steps:
  - name: send POST request
    executor: http
    command: POST https://foo.bar.com
    script: |
      {
        "timeout": 10,
        "headers": {
          "Authorization": "Bearer $TOKEN"
        },
        "query": {
          "key": "value"
        },
        "body": "post body"
      }      
```

## Admin Configuration

To configure dagu, please create the config file (default path: `~/.dagu/admin.yaml`). All fields are optional.

```yaml
# Web Server Host and Port
host: <hostname for web UI address>                          # default: 127.0.0.1
port: <port number for web UI address>                       # default: 8080

# path to the DAGs directory
dags: <the location of DAG configuration files>              # default: ${DAGU_HOME}/dags

# Web UI Color & Title
navbarColor: <admin-web header color>                        # header color for web UI (e.g. "#ff0000")
navbarTitle: <admin-web title text>                          # header title for web UI (e.g. "PROD")

# Basic Auth
isBasicAuth: <true|false>                                    # enables basic auth
basicAuthUsername: <username for basic auth of web UI>       # basic auth user
basicAuthPassword: <password for basic auth of web UI>       # basic auth password

# Base Config
baseConfig: <base DAG config path> .                         # default: ${DAGU_HOME}/config.yaml

# Others
logDir: <internal logdirectory>                              # default: ${DAGU_HOME}/logs/admin
command: <Absolute path to the dagu binary>                  # default: dagu
```

## Environment Variable

You can configure the dagu's internal work directory by defining `DAGU_HOME` environment variables. Default path is `~/.dagu/`.

## Sending email notifications

Email notifications can be sent when a DAG finished with an error or successfully. To do so, you can set the `smtp` field and related fields in the DAG specs. You can use any email delivery services (e.g., Sendgrid, Mailgun, etc).

```yaml
# Eamil notification settings
mailOn:
  failure: true
  success: true

# SMTP server settings
smtp:
  host: "smtp.foo.bar"
  port: "587"
  username: "<username>"
  password: "<password>"

# Error mail configuration
errorMail:
  from: "foo@bar.com"
  to: "foo@bar.com"
  prefix: "[Error]"

# Info mail configuration
infoMail:
  from: "foo@bar.com"
  to: "foo@bar.com"
  prefix: "[Info]"
```

If you want to use the same settings for all DAGs, set them to the [base configuration](#base-configuration-for-all-dags).


## Base Configuration for all DAGs

Creating a base configuration (default path: `~/.dagu/config.yaml`) is a convenient way to organize shared settings among all DAGs. The path to the base configuration file can be configured. See [Admin Configuration](#admin-configuration) for more details.

```yaml
# directory path to save logs from standard output
logDir: /path/to/stdout-logs/

# history retention days (default: 30)
histRetentionDays: 3

# Eamil notification settings
mailOn:
  failure: true
  success: true

# SMTP server settings
smtp:
  host: "smtp.foo.bar"
  port: "587"
  username: "<username>"
  password: "<password>"

# Error mail configuration
errorMail:
  from: "foo@bar.com"
  to: "foo@bar.com"
  prefix: "[Error]"

# Info mail configuration
infoMail:
  from: "foo@bar.com"
  to: "foo@bar.com"
  prefix: "[Info]"
```

## Scheduler

To run DAGs automatically, you need to run `dagu scheduler` process on your system.

### Execution Schedule

You can specify the schedule with cron expression in the `schedule` field in the config file as follows:

```yaml
schedule: "5 4 * * *" # Run at 04:05.
steps:
  - name: scheduled job
    command: job.sh
```

Or you can set multiple schedules:

```yaml
schedule:
  - "30 7 * * *" # Run at 7:30
  - "0 20 * * *" # Also run at 20:00
steps:
  - name: scheduled job
    command: job.sh
```

### Stop Schedule

If you want to start and stop a long-running process on a fixed schedule, you can define `start` and `stop` times as follows. At the stop time, each step's process receives a stop signal.

```yaml
schedule:
  start: "0 8 * * *" # starts at 8:00
  stop: "0 13 * * *" # stops at 13:00
steps:
  - name: scheduled job
    command: job.sh
```

You can also set multiple start/stop schedules. In the following example, the process will run at 0:00-5:00 and 12:00-17:00.

```yaml
schedule:
  start:
    - "0 0 * * *"
    - "12 0 * * *"
  stop:
    - "5 0 * * *"
    - "17 0 * * *"
steps:
  - name: some long-process
    command: main.sh
```

### Restart Schedule

If you want to restart a DAG process on a fixed schedule, the `restart` field is also available. At the restart time, the DAG execution will be stopped and restarted again.

```yaml
schedule:
  start: "0 8 * * *"    # starts at 8:00
  restart: "0 12 * * *" # restarts at 12:00
  stop: "0 13 * * *"    # stops at 13:00
steps:
  - name: scheduled job
    command: job.sh
```

The wait time after the job is stopped before restart can be configured in the DAG definition as follows. The default value is `0` (zero).

```yaml
restartWaitSec: 60 # Wait 60s after the process is stopped, then restart the DAG.

steps:
  - name: step1
    command: python some_app.py
```

### Run Scheduler as a daemon

The easiest way to make sure the process is always running on your system is to create the script below and execute it every minute using cron (you don't need `root` account in this way):

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

Please refer to [REST API Docs](./docs/restapi.md)

## FAQ

### How to contribute?

Feel free to contribute in any way you want. Share ideas, questions, submit issues, and create pull requests. Thanks!

## How to build `dagu` locally?

1. Install the latest version of [Node.js](https://nodejs.org/en/download/) and Go (>= 1.18)

2. Install Yarn 
```sh
npm i -g yarn
```

3. Check out the repository

4. Build 
```sh
make build
```

### Where is the history data stored?

It will store execution history data in the `DAGU__DATA` environment variable path. The default location is `$HOME/.dagu/data`.

### Where are the log files stored?

It will store log files in the `DAGU__LOGS` environment variable path. The default location is `$HOME/.dagu/logs`. You can override the setting by the `logDir` field in a YAML file.

### How long will the history data be stored?

The default retention period for execution history is 30 days. However, you can override the setting by the `histRetentionDays` field in a YAML file.

### How to use specific `host` and `port` for `dagu server`?

dagu server's host and port can be configured in the admin configuration file as below. See [Admin Configuration](#admin-configuration) for more details.

```yaml
host: <hostname for web UI address>                          # default: 127.0.0.1
port: <port number for web UI address>                       # default: 8000
```

### How to specify the DAGs directory for `dagu server` and `dagu scheduler`?

You can customize DAGs directory that will be used by `dagu server` and `dagu scheduler`. See [Admin Configuration](#admin-configuration) for more details.

```yaml
dags: <the location of DAG configuration files>              # default: ${DAGU_HOME}/dags
```

### How can I retry a DAG from a specific task?

You can change the status of any task to a `failed` state. Then, when you retry the DAG, it will execute the failed one and any subsequent.

### How does it track running processes without DBMS?

dagu uses Unix sockets to communicate with running processes.

## License

This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details

## Contributors

<a href="https://github.com/yohamta/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=yohamta/dagu" />
</a>

Made with [contrib.rocks](https://contrib.rocks).

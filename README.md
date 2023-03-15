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
<b>Just another Cron alternative with a Web UI, but with much more capabilities</b><br />
It runs <a href="https://en.wikipedia.org/wiki/Directed_acyclic_graph">DAGs (Directed acyclic graph)</a> defined in a simple YAML format.
</p>

Dagu is a tool for scheduling and running tasks based on a directed acyclic graph (DAG). It allows you to define dependencies between commands and represent them as a single DAG, schedule the execution of DAGs with Cron expressions, and natively support running Docker containers, making HTTP requests, and executing commands over SSH.

## Highlights
- Single binary file installation
- Declarative YAML format for defining DAGs
- Web UI for visualizing, managing, and rerunning pipelines
- No programming required, making it easy to use and ideal for small projects
- Self-contained, with no need for a DBMS or cloud service

---

## Contents

- [Highlights](#highlights)
- [Contents](#contents)
- [Getting started](#getting-started)
- [Motivation](#motivation)
- [Why not use an existing workflow scheduler like Airflow?](#why-not-use-an-existing-workflow-scheduler-like-airflow)
- [How does it work?](#how-does-it-work)
- [Installation](#installation)
  - [via Homebrew](#via-homebrew)
  - [via Bash script](#via-bash-script)
  - [via Docker](#via-docker)
  - [via GitHub Release Page](#via-github-release-page)
- [️Quick start](#️quick-start)
  - [1. Launch the Web UI](#1-launch-the-web-ui)
  - [2. Create a new DAG](#2-create-a-new-dag)
  - [3. Edit the DAG](#3-edit-the-dag)
  - [4. Execute the DAG](#4-execute-the-dag)
- [Command Line User Interface](#command-line-user-interface)
- [Web User Interface](#web-user-interface)
- [YAML Format](#yaml-format)
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
- [Executors](#executors)
  - [Running Docker Containers](#running-docker-containers)
  - [Making HTTP Requests](#making-http-requests)
  - [Sending E-mail](#sending-e-mail)
  - [Executing jq Command](#executing-jq-command)
  - [Command Execution over SSH](#command-execution-over-ssh)
- [Configuration Options](#configuration-options)
- [Sending email notifications](#sending-email-notifications)
- [Base Configuration for all DAGs](#base-configuration-for-all-dags)
- [Scheduler](#scheduler)
  - [Execution Schedule](#execution-schedule)
  - [Stop Schedule](#stop-schedule)
  - [Restart Schedule](#restart-schedule)
  - [Run Scheduler as a daemon](#run-scheduler-as-a-daemon)
  - [Scheduler Configuration](#scheduler-configuration)
- [Running with Docker Compose](#running-with-docker-compose)
- [Building Docker Image](#building-docker-image)
- [REST API Interface](#rest-api-interface)
- [Local Development Setup](#local-development-setup)
- [FAQ](#faq)
  - [How to contribute?](#how-to-contribute)
  - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
  - [How to use specific `host` and `port` for `dagu server`?](#how-to-use-specific-host-and-port-for-dagu-server)
  - [How to specify the DAGs directory for `dagu server` and `dagu scheduler`?](#how-to-specify-the-dags-directory-for-dagu-server-and-dagu-scheduler)
  - [How can I retry a DAG from a specific task?](#how-can-i-retry-a-dag-from-a-specific-task)
  - [How does it track running processes without DBMS?](#how-does-it-track-running-processes-without-dbms)
- [Contributions](#contributions)
- [License](#license)

## Getting started

To get started with Dagu, see the [installation instructions](#install-dagu) below and then check out the [️Quick start](#️quick-start) guide.

## Motivation

Legacy systems often have complex and implicit dependencies between jobs. When there are hundreds of cron jobs on a server, it can be difficult to keep track of these dependencies and to determine which job to rerun if one fails. It can also be a hassle to SSH into a server to view logs and manually rerun shell scripts one by one. Dagu aims to solve these problems by allowing you to explicitly visualize and manage pipeline dependencies as a DAG, and by providing a web UI for checking dependencies, execution status, and logs and for rerunning or stopping jobs with a simple mouse click.

## Why not use an existing workflow scheduler like Airflow?

There are many existing tools such as Airflow, Prefect, and Temporal, but many of these require you to write code in a programming language like Python to define your DAG. For systems that have been in operation for a long time, there may already be complex jobs with hundreds of thousands of lines of code written in languages like Perl or Shell Script. Adding another layer of complexity on top of these codes can reduce maintainability. Dagu was designed to be easy to use, self-contained, and require no coding, making it ideal for small projects.

## How does it work?

Dagu is a single command line tool that uses the local file system to store data, so no database management system or cloud service is required. DAGs are defined in a declarative YAML format, and existing programs can be used without modification.

## Installation

You can install Dagu quickly using Homebrew or by downloading the latest binary from the Releases page on GitHub.

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

### via Docker

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
- `dagu restart <file>` - Restarts the current running DAG
- `dagu dry [--params=<params>] <file>` - Dry-runs the DAG
- `dagu server [--host=<host>] [--port=<port>] [--dags=<path/to/the DAGs directory>]` - Launches the Dagu web UI server
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

## YAML Format

To view all examples, visit [this](https://github.com/yohamta/dagu/tree/main/examples) page.

### Minimal Definition

The minimal DAG definition is as simple as follows.

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

You can define environment variables and refer to them using the `env` field.

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

You can define parameters using the `params` field and refer to each parameter as $1, $2, etc. Parameters can also be command substitutions or environment variables. It can be overridden by the `--params=` parameter of the `start` command.

```yaml
params: param1 param2
steps:
  - name: some task with parameters
    command: python main.py $1 $2
```

Named parameters are also available as follows.

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

For example, the task below only runs on the first date of each month.

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

The `output` field can be used to set an environment variable with standard output. Leading and trailing space will be trimmed automatically. The environment variables can be used in subsequent steps.

```yaml
steps:
  - name: step 1
    command: "echo foo"
    output: FOO # will contain "foo"
```

### Stdout and Stderr Redirection

The `stdout` field can be used to write standard output to a file.

```yaml
steps:
  - name: create a file
    command: "echo hello"
    stdout: "/tmp/hello" # the content will be "hello\n"
```

The `stderr` field allows to redirect stderr to other file without writing to the normal log file.

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

## Executors

The `executor` field provides different execution methods for each step.

### Running Docker Containers

*Note: It requires Docker daemon running on the host.*

The `docker` executor allows us to run Docker containers instead of bare commands.

In the example below, it pulls and runs [Deno's docker image](https://hub.docker.com/r/denoland/deno) and prints 'Hello World'.

```yaml
steps:
  - name: deno_hello_world
    executor: 
      type: docker
      config:
        image: "denoland/deno:1.10.3"
        autoRemove: true
    command: run https://examples.deno.land/hello-world.ts
```

Example Log output:

![docker](./examples/images/docker.png)

To see more configurations, visit [this](https://github.com/yohamta/dagu/tree/main/examples#running-docker-containers) page.

### Making HTTP Requests

The `http` executor allows us to make an arbitrary HTTP request.

```yaml
steps:
  - name: send POST request
    command: POST https://foo.bar.com
    executor: 
      type: http
      config:
        timeout: 10,
        headers:
          Authorization: "Bearer $TOKEN"
        silent: true # If silent is true, it outputs response body only.
        query:
          key: "value"
        body: "post body"
```

### Sending E-mail

The `mail` executor can be used to send e-mail.

Example:

```yaml
smtp:
  host: "smtp.foo.bar"
  port: "587"
  username: "<username>"
  password: "<password>"

steps:
  - name: step1
    executor:
      type: mail
      config:
        to: <to address>
        from: <from address>
        subject: "Urgent Request: Help Me Find My Sanity"
        message: |
          I'm in a bit of a pickle.
          I seem to have lost my sanity somewhere between my third cup of coffee
          and my fourth Zoom meeting of the day.
          
          If you see it lying around, please let me know.
          Thanks for your help!

          Best,
```

### Executing jq Command

The `jq` executor can be used to transform, query, and format JSON.

Query Example:

```yaml
steps:
  - name: run query
    executor: jq
    command: '{(.id): .["10"].b}'
    script: |
      {"id": "sample", "10": {"b": 42}}
```

output:
```json
{
    "sample": 42
}
```

Formatting JSON:

```yaml
steps:
  - name: format json
    executor: jq
    script: |
      {"id": "sample", "10": {"b": 42}}
```

output:
```json
{
    "10": {
        "b": 42
    },
    "id": "sample"
}
```

The `jq` result can be used in following steps via [Output](#output) or [Stdout Redirection](#stdout-and-stderr-redirection).

### Command Execution over SSH

The `ssh` executor allows us to execute commands on remote hosts over SSH.

```yaml
steps:
  - name: step1
    executor: 
      type: ssh
      config:
        user: dagu
        ip: XXX.XXX.XXX.XXX
        port: 22
        key: /Users/dagu/.ssh/private.pem
    command: /usr/sbin/ifconfig
```

## Configuration Options

The following environment variables can be used to configure the Dagu. Default values are provided in the parentheses:

- `DAGU_HOST` (`127.0.0.1`): The host to bind the server to.
- `DAGU_PORT` (`8080`): The port to bind the server to.
- `DAGU_DAGS` (`$DAGU_HOME/dags`): The directory containing the DAGs.
- `DAGU_COMMAND` (`dagu`): The command used to start the application.
- `DAGU_IS_BASIC_AUTH` (`0`): Set to 1 to enable basic authentication.
- `DAGU_BASIC_AUTH_USERNAME` (`""`): The username to use for basic authentication.
- `DAGU_BASIC_AUTH_PASSWORD` (`""`): The password to use for basic authentication.
- `DAGU_LOG_DIR` (`$DAGU_HOME/logs`): The directory where logs will be stored.
- `DAGU_DATA_DIR` (`$DAGU_HOME/data`): The directory where application data will be stored.
- `DAGU_SUSPEND_FLAGS_DIR` (`$DAGU_HOME/suspend`): The directory containing DAG suspend flags.
- `DAGU_ADMIN_LOG_DIR` (`$DAGU_HOME/logs/admin`): The directory where admin logs will be stored.
- `DAGU_BASE_CONFIG` (`$DAGU_HOME/config.yaml`): The path to the base configuration file.
- `DAGU_NAVBAR_COLOR` (`""`): The color to use for the navigation bar.
- `DAGU_NAVBAR_TITLE` (`Dagu`): The title to display in the navigation bar.

Note: All of the above environment variables are optional. If not set, the default values shown above will be used. If DAGU_HOME environment variable is not set, the default value is $HOME/.dagu.

## Sending email notifications

Email notifications can be sent when a DAG finished with an error or successfully. To do so, you can set the `smtp` field and related fields in the DAG specs. You can use any email delivery services (e.g. Sendgrid, Mailgun, etc).

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

To run DAGs automatically, you need to run the `dagu scheduler` process on your system.

### Execution Schedule

You can specify the schedule with cron expression in the `schedule` field in the config file as follows.

```yaml
schedule: "5 4 * * *" # Run at 04:05.
steps:
  - name: scheduled job
    command: job.sh
```

Or you can set multiple schedules.

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

You can also set multiple start/stop schedules. In the following example, the process will run from 0:00-5:00 and 12:00-17:00.

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

The easiest way to make sure the process is always running on your system is to create the script below and execute it every minute using cron (you don't need `root` account in this way).

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

## Running with Docker Compose

To automate workflows based on cron expressions, it is necessary to run both the admin server and scheduler process. Here is an example `docker-compose.yml` setup for running Dagu using Docker Compose.

[Example setup](./examples/docker-compose/)

```yaml
version: "3.9"
services:

  # init container updates permission
  init:
    image: "yohamta/dagu:latest"
    user: root
    volumes:
      - dagu:/home/dagu/.dagu
    command: chown -R dagu /home/dagu/.dagu/

  # admin web server process
  server:
    image: "yohamta/dagu:latest"
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - dagu:/home/dagu/.dagu
      - ./dags/:/home/dagu/.dagu/dags
    depends_on:
      - init

  # scheduler process
  scheduler:
    image: "yohamta/dagu:latest"
    restart: unless-stopped
    volumes:
      - dagu:/home/dagu/.dagu
      - ./dags/:/home/dagu/.dagu/dags
    command: dagu scheduler
    depends_on:
      - init

volumes:
  dagu: {}
```

## Building Docker Image

Download the [Dockerfile](https://github.com/yohamta/dagu/blob/main/Dockerfile) to your local PC and you can build an image.

For example:

```sh
DAGU_VERSION=1.9.0
docker build -t dagu:${DAGU_VERSION} \
--build-arg VERSION=${DAGU_VERSION} \
--no-cache .
```

## REST API Interface

Please refer to [REST API Docs](./docs/restapi.md)

## Local Development Setup

1. Install the latest version of [Node.js](https://nodejs.org/en/download/).
2. Install [yarn](https://yarnpkg.com/) by running the command below.
```sh
npm i -g yarn
```
3. Build frontend project
```sh
make build-admin
```
4. Build `dagu` binary to `bin/dagu`
```sh
make build
```

## FAQ

### How to contribute?

Feel free to contribute in any way you want. Share ideas, questions, submit issues, and create pull requests. Thanks!

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

## Contributions
 
<a href="https://github.com/yohamta/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=yohamta/dagu" />
</a>

We welcome contributions to Dagu! If you have an idea for a new feature or have found a bug, please open an issue on the GitHub repository. If you would like to contribute code, please follow these steps:

1. Fork the repository
2. Create a new branch for your changes
3. Make your changes and commit them to your branch
4. Push your branch to your fork and open a pull request


## License

This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details
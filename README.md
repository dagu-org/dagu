# Dagu 
<img align="right" width="150" src="https://user-images.githubusercontent.com/1475839/165412252-4fbb28ae-0845-4af2-9183-0aa1de5bf707.png" alt="dagu" title="dagu" />

[![Go Report Card](https://goreportcard.com/badge/github.com/yohamta/dagu)](https://goreportcard.com/report/github.com/yohamta/dagu)
[![codecov](https://codecov.io/gh/yohamta/dagu/branch/main/graph/badge.svg?token=CODZQP61J2)](https://codecov.io/gh/yohamta/dagu)
[![GitHub release](https://img.shields.io/github/release/yohamta/dagu.svg)](https://github.com/yohamta/dagu/releases)
[![GoDoc](https://godoc.org/github.com/yohamta/dagu?status.svg)](https://godoc.org/github.com/yohamta/dagu)

**A No-code workflow executor**

Dagu execute [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) from declarative YAML definitions. Dagu also comes with a web UI for visualizing workflow.

## Contents
- [Dagu](#dagu)
  - [Contents](#contents)
  - [Why not existing tools, like Airflow or Prefect?](#why-not-existing-tools-like-airflow-or-prefect)
  - [️How does it work?](#️how-does-it-work)
  - [️Quick start](#️quick-start)
    - [1. Installation](#1-installation)
    - [2. Download an example YAML file](#2-download-an-example-yaml-file)
    - [3. Launch web server](#3-launch-web-server)
    - [4. Running the example](#4-running-the-example)
  - [Command usage](#command-usage)
  - [Web interface](#web-interface)
  - [YAML format](#yaml-format)
    - [Minimal](#minimal)
    - [Using environment variables](#using-environment-variables)
    - [Using parameters](#using-parameters)
    - [Using command substitution](#using-command-substitution)
    - [All available fields](#all-available-fields)
  - [Admin configuration](#admin-configuration)
    - [Environment variables](#environment-variables)
    - [Web UI configuration](#web-ui-configuration)
    - [Global configuration](#global-configuration)
  - [FAQ](#faq)
    - [How to contribute?](#how-to-contribute)
    - [Where is the history data stored?](#where-is-the-history-data-stored)
    - [Where is the log files stored?](#where-is-the-log-files-stored)
    - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
    - [Is it possible to retry a DAG from a specific step?](#is-it-possible-to-retry-a-dag-from-a-specific-step)
    - [Does it have a scheduler function?](#does-it-have-a-scheduler-function)
    - [How it can communicate with running processes?](#how-it-can-communicate-with-running-processes)
  - [GoDoc](#godoc)
  - [License](#license)

## Why not Airflow or Prefect?
Airflow or Prefect requires us to write Python code for workflow definitions. For my specific situation, there were hundreds of thousands of existing Perl or ShellScript codes. Adding another layer of Python would add too much complexity for us. We needed more light-weight solution. So, we developed a No-code workflow executor that doesn't require writing code. We hope that this tool will help other people in the same situation.

## ️How does it work?

- Dagu is a single command and it uses the file system to stores data in JSON format. Therefore, no DBMS or cloud service is required.
- Dagu executes DAGs defined in declarative YAML format. Existing programs can be used without any modification.

## ️Quick start

### 1. Installation

Download the latest binary from the [Releases page](https://github.com/dagu/dagu/releases) and place it in your `$PATH`. For example, you can download it in `/usr/local/bin`.

### 2. Download an example YAML file

Download this [example YAML](https://github.com/yohamta/dagu/blob/main/examples/complex_dag.yaml) and place it in the current directory with extension `*.yaml`.

### 3. Launch web server

Start the server with `dagu server` and browse to `http://localhost:8000` to explore the Web UI.

### 4. Running the example

You can start the example by pressing `Start` on the UI.

![example](https://user-images.githubusercontent.com/1475839/165764122-0bdf4bd5-55bb-40bb-b56f-329f5583c597.gif)

## Command usage

- `dagu start [--params=<params>] <file>` - run a DAG
- `dagu status <file>` - display the current status of the DAG
- `dagu retry --req=<request-id> <file>` - retry the failed/canceled DAG
- `dagu stop <file>` - stop a DAG execution by sending a TERM signal
- `dagu dry [--params=<params>] <file>` - dry-run a DAG
- `dagu server` - start a web server for web UI

## Web interface

You can launch web UI by `dagu server` command. Default URL is `http://localhost:8000`.

- **DAGs**: Overview of all DAGs.

  ![DAGs](https://user-images.githubusercontent.com/1475839/166269631-f031106e-dd13-49dc-9d00-0f6d1e22e4dc.png)

- **Detail**: Realtime status of the DAG.

  ![Detail](https://user-images.githubusercontent.com/1475839/166269521-03098e46-6608-43fa-b363-0d00b069c808.png)

- **History**: History of the execution of the DAG.

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

### Using environment variables

Environment variables can be defined and used using `env` field.

```yaml
name: example
env:
  SOME_DIR: ${HOME}/batch
steps:
  - name: some task in some dir
    dir: ${SOME_DIR}
    command: python main.py
```

### Using parameters

Parameters can be defined using `params` field. Each parameter can be referenced as $1, $2, etc. Parameters can also be command substitutions or environment variables. You can override the parameters with the `--params=` parameter for `start` command.

```yaml
name: example
params: param1 param2
steps:
  - name: some task with parameters
    command: python main.py $1 $2
```

### Using command substitution

You can use command substitution in field values. A string enclosed in backquotes (`` ` ``) is evaluated as a command and replaced with the result of standard output.

```yaml
name: minimal configuration          
env:
  TODAY: "`date '+%Y%m%d'`"
steps:                               
  - name: hello
    command: "echo hello, today is ${TODAY}"
```

### All available fields

By combining these settings, you have granular control over how the workflow runs.

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
handlerOn:                           # Handler on Success, Failure, Cancel, Exit
  success:                           
    command: "echo succeed"          # Command to execute when the DAG execution succeed
  failure:                           
    command: "echo failed"           # Command to execute when the DAG execution failed
  cancel:                            
    command: "echo canceled"         # Command to execute when the DAG execution canceled
  exit:                              
    command: "echo finished"         # Command to execute when the DAG execution finished
steps:
  - name: som task                   # Step's name
    description: some task           # Step's description
    dir: ${HOME}/logs                # Working directory
    command: python main.py $1       # Command and parameters
    mailOn:
      failure: true                  # Send a mail when the step failed
      success: true                  # Send a mail when the step finished
    continueOn:
      failed: true                   # Continue to the next regardless of the step failed or not
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

The global configuration file `~/.dagu/config.yaml` is useful to gather common settings, such as the directory to write log files.

## Admin configuration

### Environment variables

- `DAGU__DATA` - path to directory for internal use by dagu (default : `~/.dagu/data`)
- `DAGU__LOGS` - path to directory for logging (default : `~/.dagu/logs`)

### Web UI configuration

Please create `~/.dagu/admin.yaml`.

```yaml
host: <hostname for web UI address>                          # default value is 127.0.0.1 
port: <port number for web UI address>                       # default value is 8080
dags: <the location of DAG configuration files>              # default value is current working directory
command: <Absolute path to the dagu binary>                  # [optional] required if the dagu command not in $PATH
isBasicAuth: <true|false>                                    # [optional] basic auth config
basicAuthUsername: <username for basic auth of web UI>       # [optional] basic auth config
basicAuthPassword: <password for basic auth of web UI>       # [optional] basic auth config
```

### Global configuration

Creating a global configuration `~/.dagu/config.yaml` is a convenient way to organize common settings.

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

Dagu's history data will be stored in the path of `DAGU__DATA` environment variable. The default location is `$HOME/.dagu/data`.

### Where is the log files stored?

Log files are stored in the path of the `DAGU__LOGS` environment variable. The default location is `$HOME/.dagu/logs`. You can override this setting by `logDir` option in a YAML file.

### How long will the history data be stored?

The default retention period for execution history is 7 days. This setting can be changed with `histRetentionDays` option in a YAML file.

### Is it possible to retry a DAG from a specific step?

You can change the status of any task to a `failed` status. Then, when the job is retried, the tasks after the failed node will be executed.

![Update Status](https://user-images.githubusercontent.com/1475839/166289470-f4af7e14-28f1-45bd-8c32-59cd59d2d583.png)

### Does it have a scheduler function?

No, there is no scheduler functionality so far. It is intended to be used with cron.

### How it can communicate with running processes?

Dagu uses unix sockets to communicate with running processes.

![dagu Architecture](https://user-images.githubusercontent.com/1475839/166124202-e0deeded-c4ce-4a96-982c-498cf8db9118.png)

## GoDoc

https://pkg.go.dev/github.com/yohamta/dagu

## License
This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details

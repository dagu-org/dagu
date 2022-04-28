#  dagu 
<img align="right" width="150" src="https://user-images.githubusercontent.com/1475839/165412252-4fbb28ae-0845-4af2-9183-0aa1de5bf707.png" alt="dagu" title="dagu" />

**A simpler Airflow alternative to run workflows (DAGs) defined in declarative YAML format**

dagu is a simple workflow engine to generate and executes [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) from YAML definiton. dagu comes with an admin web UI and REST API interfaces are also included.

## Contents
- [dagu](#dagu)
  - [Contents](#contents)
  - [Motivation](#motivation)
  - [Why not existing tools, like Airflow?](#why-not-existing-tools-like-airflow)
  - [Quick start](#quick-start)
    - [Installation](#installation)
    - [Download an example DAG definition](#download-an-example-dag-definition)
    - [Start Web UI server](#start-web-ui-server)
    - [Running the DAG](#running-the-dag)
    - [Usage](#usage)
  - [Use cases](#use-cases)
  - [Features](#features)
  - [User interface](#user-interface)
  - [DAG definiton](#dag-definiton)
    - [Minimal](#minimal)
    - [Using environment Variables](#using-environment-variables)
    - [Using DAG parameters](#using-dag-parameters)
    - [Using command substitution](#using-command-substitution)
    - [All available fields](#all-available-fields)
    - [Examples](#examples)
  - [Admin configurations](#admin-configurations)
    - [Environment variables](#environment-variables)
    - [Web UI configuration](#web-ui-configuration)
    - [Global DAG configuration](#global-dag-configuration)
  - [Architecture](#architecture)
  - [FAQ](#faq)
    - [How to contribute?](#how-to-contribute)
    - [Where is the history data stored?](#where-is-the-history-data-stored)
    - [Where is the log files stored?](#where-is-the-log-files-stored)
    - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
    - [Is it possible to retry a DAG from a specific step?](#is-it-possible-to-retry-a-dag-from-a-specific-step)
    - [Does it have a scheduler function?](#does-it-have-a-scheduler-function)
  - [GoDoc](#godoc)
  - [License](#license)

## Motivation
Currently, my environment has **many problems**. Hundreds of complex cron jobs are registered on huge servers and it is impossible to keep track of the dependencies between them. If one job fails, I don't know which job to re-run. I also have to SSH into the server to see the logs and manually run the shell scripts one by one.

So I needed a tool that can explicitly visualize and manage the dependencies of the pipeline.

***How nice it would be to be able to visually see the job dependencies, execution status, and logs of each job in a web browser, and to be able to rerun or stop a series of jobs with just a mouse click!***

## Why not existing tools, like Airflow?
I considered many potential tools such as Airflow, Rundeck, Luigi, DigDag, JobScheduler, etc.

But unfortunately, they were not suitable for my existing environment. Because they required a DBMS (Database Management System) installation, relatively high learning curves, and more operational overheads. We only have a small group of engineers in our office and use a less common DBMS.

Finally, I decided to build my own tool that would not require any DBMS server, any daemon process, or any additional operational burden and is easy to use.  I hope this tool will help others with the same thoughts.

## Quick start

### Installation
Download the binary from [Releases page](https://github.com/dagu/dagu/releases) and place it in your `$PATH`.

### Download an example DAG definition

Download this [example](https://github.com/yohamta/dagu/blob/main/examples/complex_dag.yaml) and place it in the current directory with extension `*.yaml`.

### Start Web UI server

Start the server with `dagu server` and browse to `http://localhost:8080` to explore the UI.

### Running the DAG

Then you can start the example DAG from the Web UI.

![example](https://user-images.githubusercontent.com/1475839/165764122-0bdf4bd5-55bb-40bb-b56f-329f5583c597.gif)

### Usage

- `dagu start [--params=<params>] <DAG file>` - run a DAG
- `dagu status <DAG file>` - display the current status of the DAG
- `dagu retry --req=<request-id> <DAG file>` - retry the failed/canceled DAG
- `dagu stop <DAG file>` - cancel a DAG
- `dagu dry [--params=<params>] <DAG file>` - dry-run a DAG
- `dagu server` - start a web server for web UI

## Use cases
- ETL Pipeline
- Machine Learning model training
- Automated generation of reports
- Backups and other DevOps tasks
- Visualizing existing workflows

## Features

- Simple command interface (See [Usage](#usage))
- Simple configuration YAML format (See [Simple example](#simple-example))
- Web UI to visualize, manage DAGs and watch logs
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
- REST API interface
- onExit / onSuccess / onFailure / onCancel handlers
- Automatic history cleaning

## User interface

- **DAGs**: Overview of all DAGs in your environment.

  ![DAGs](https://user-images.githubusercontent.com/1475839/165417789-18d29f3d-aecf-462a-8cdf-0b575ba613d0.png)

- **Detail**: Current status of the dag.

  ![Detail](https://user-images.githubusercontent.com/1475839/165418393-d7d876bc-329f-4299-977e-7726e8ef0fa1.png)

- **Timeline**: Timeline of each steps in the pipeline.

  ![Timeline](https://user-images.githubusercontent.com/1475839/165418430-1fe3b100-33eb-4d81-a68a-c8a881890b61.png)

- **History**: History of the execution of the pipeline.

  ![History](https://user-images.githubusercontent.com/1475839/165426067-02c4f72f-e3f0-4cd8-aa38-35fa98f0382f.png)

## DAG definiton

### Minimal

A minimal DAG definition is as simple as:

```yaml
name: minimal configuration          # DAG's name
steps:                               # Steps inside the DAG
  - name: step 1                     # Step's name (should be unique within the file)
    description: step 1              # [optional] Description of the step
    command: python main_1.py        # Command and arguments to execute
    dir: ${HOME}/dags/               # [optional] Working directory
  - name: step 2
    description: step 2
    command: python main_2.py
    dir: ${HOME}/dags/
    depends:
      - step 1                       # [optional] Name of the step to depend on
```

### Using environment Variables

Environment variables can be defined and used throughout the file.

```yaml
name: example
env:
  SOME_DIR: ${HOME}/batch
steps:
  - name: some task in some dir
    dir: ${SOME_DIR}
    command: python main.py
```

### Using DAG parameters

Parameters can be defined and referenced throughout a file. Each parameter can be referenced as $1, $2, etc. Parameters can also be command substitutions or environment variables. You can override the default values of the parameters with the `--params=` parameter of the `start` command.

```yaml
name: example
params: param1 param2
steps:
  - name: some task with parameters
    command: python main.py $1 $2
```

### Using command substitution

You can use command substitution. A string enclosed in backquotes is evaluated as a command and replaced with the result of standard output.

```yaml
name: minimal configuration          
env:
  TODAY: "`date '+%Y%m%d'`"
steps:                               
  - name: hello
    command: "echo hello, today is ${TODAY}"
```

### All available fields

All of the following settings are available. By combining settings, you have granular control over how the workflow runs.

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
params: param1 param2                # Default parameters for the DAG that can be refered by $1, $2, and so on
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
      failed: true                   # Continue to the next regardless the step failed or not
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

The global config file `~/.dagu/config.yaml` is useful to gather common settings such as log directory.

### Examples

To check all examples, visit [this page](https://github.com/dagu/dagu/tree/main/examples).

-  Sample 1

  ![sample_1](https://user-images.githubusercontent.com/1475839/164965036-fede5675-cba0-410b-b371-22deec55b9e8.png)
```yaml
name: example DAG
steps:
  - name: "1"
    command: echo hello world
  - name: "2"
    command: sleep 10
    depends:
      - "1"
  - name: "3"
    command: echo done!
    depends:
      - "2"
```

-  Sample 2

  ![sample_2](https://user-images.githubusercontent.com/1475839/164965143-b10a0511-35f3-45fa-9eba-69c6db4614a2.png)
```yaml
name: example DAG
env:
  LOG_DIR: ${HOME}/logs
logDir: ${LOG_DIR}
params: foo bar
steps:
  - name: "check precondition"
    command: echo start
    preconditions:
      - condition: "`echo $1`"
        expected: foo
  - name: "print foo"
    command: echo $1
    depends:
      - "check precondition"
  - name: "print bar"
    command: echo $2
    depends:
      - "print foo"
  - name: "failure and continue"
    command: "false"
    continueOn:
      failure: true
    depends:
      - "print bar"
  - name: "print done"
    command: echo done!
    depends:
      - "failure and continue"
handlerOn:
  exit:
    command: echo finished!
  success:
    command: echo success!
  failure:
    command: echo failed!
  cancel:
    command: echo canceled!
```

-  Complex example

  ![complex](https://user-images.githubusercontent.com/1475839/164965345-977de1bc-d042-4d3f-bf0e-bb648e534a78.png)
```yaml
name: complex DAG
steps:
  - name: "Initialize"
    command: "sleep 2"
  - name: "Copy TAB_1"
    description: "Extract data from TAB_1 to TAB_2"
    command: "sleep 2"
    depends:
      - "Initialize"
  - name: "Update TAB_2"
    description: "Update TAB_2"
    command: "sleep 2"
    depends:
      - Copy TAB_1
  - name: Validate TAB_2
    command: "sleep 2"
    depends:
      - "Update TAB_2"
  - name: "Load TAB_3"
    description: "Read data from files"
    command: "sleep 2"
    depends:
      - Initialize
  - name: "Update TAB_3"
    command: "sleep 2"
    depends:
      - "Load TAB_3"
  - name: Merge
    command: "sleep 2"
    depends:
      - Update TAB_3
      - Validate TAB_2
      - Validate File
  - name: "Check File"
    command: "sleep 2"
  - name: "Copy File"
    command: "sleep 2"
    depends:
      - Check File
  - name: "Validate File"
    command: "sleep 2"
    depends:
      - Copy File
  - name: Calc Result
    command: "sleep 2"
    depends:
      - Merge
  - name: "Report"
    command: "sleep 2"
    depends:
      - Calc Result
  - name: Reconcile
    command: "sleep 2"
    depends:
      - Calc Result
  - name: "Cleaning"
    command: "sleep 2"
    depends:
      - Reconcile
```

## Admin configurations

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

### Global DAG configuration

Please create `~/.dagu/config.yaml`. All settings can be overridden by individual DAG configurations.

Creating a global configuration is a convenient way to organize common settings.

```yaml
logDir: <path-to-write-log>         # log directory to write standard output
histRetentionDays: 3                # history retention days
smtp:                               # [optional] mail server configurations to send notifications
  host: <smtp server host>
  port: <stmp server port>
errorMail:                          # [optional] mail configurations for error-level
  from: <from address>
  to: <to address>
  prefix: <prefix of mail subject>
infoMail:
  from: <from address>              # [optional] mail configurations for info-level
  to: <to address>
  prefix: <prefix of mail subject>
```

## Architecture

- uses plain JSON files as history database, and unix sockets to communicate with running processes.
  ![dagu Architecture](https://user-images.githubusercontent.com/1475839/164869015-769bfe1d-ad38-4aca-836b-bf3ffe0665df.png)

## FAQ

### How to contribute?

Feel free to contribute in any way you want. Share ideas, submit issues, create pull requests. 
You can start by improving this [README.md](https://github.com/dagu/dagu/blob/main/README.md) or suggesting new [features](https://github.com/dagu/dagu/issues)
Thank you!

### Where is the history data stored?

dagu's DAG execution history data is stored in plain json files in the path of the `DAGU__DATA` environment variable with extension `*.dat`. The default location is `$HOME/.dagu/data`.

### Where is the log files stored?

Log files are stored in the path of the `DAGU__LOGS` environment variable. The default location is `$HOME/.dagu/logs`. You can override this setting by `logDir` option in the config file.

### How long will the history data be stored?

The default retension period for execution history is 7 days. This setting can be changed with `histRetentionDays` option in the config file.

### Is it possible to retry a DAG from a specific step?

Just like Airflow, you can change the status of any task to failed. Then, when the job is retried, the tasks after the failed node will be executed.

![Update Status](https://user-images.githubusercontent.com/1475839/165755497-923828f8-1992-43fe-8618-979128c38c79.png)

### Does it have a scheduler function?

No, there is no scheduler functionality so far. It is intended to be used with cron.

## GoDoc

https://pkg.go.dev/github.com/yohamta/dagu

## License
This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details

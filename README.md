# dagman 
<img align="right" width="150" src="https://user-images.githubusercontent.com/1475839/165412252-4fbb28ae-0845-4af2-9183-0aa1de5bf707.png" alt="dagman" title="dagman" />

**An easy-to-use command to manage workflows (DAGs) defined in declarative YAML format**

dagman (DAG manager) is a easy-to-use workflow engine to generate and executes [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) from YAML definition. dagman comes with a web UI and REST API interfaces are also included.

## 🚀 Contents
- [dagman](#dagman)
  - [🚀 Contents](#-contents)
  - [🌟 Project goal](#-project-goal)
  - [⚙️ How does it work?](#️-how-does-it-work)
  - [🔥 Motivation](#-motivation)
  - [🤔 Why?](#-why)
  - [⚡️ Quick start](#️-quick-start)
    - [Installation](#installation)
    - [Download an example DAG definition](#download-an-example-dag-definition)
    - [Start Web UI server](#start-web-ui-server)
    - [Running the DAG](#running-the-dag)
  - [📖 Usage](#-usage)
  - [🛠 Use cases](#-use-cases)
  - [✨ Features](#-features)
  - [🖥 User interface](#-user-interface)
  - [📋 DAG definition](#-dag-definition)
    - [Minimal](#minimal)
    - [Using environment variables](#using-environment-variables)
    - [Using DAG parameters](#using-dag-parameters)
    - [Using command substitution](#using-command-substitution)
    - [All available fields](#all-available-fields)
    - [Examples](#examples)
  - [🧑‍💻 Admin configurations](#-admin-configurations)
    - [Environment variables](#environment-variables)
    - [Web UI configuration](#web-ui-configuration)
    - [Global DAG configuration](#global-dag-configuration)
  - [💡 Architecture](#-architecture)
  - [❓FAQ](#faq)
    - [How to contribute?](#how-to-contribute)
    - [Where is the history data stored?](#where-is-the-history-data-stored)
    - [Where is the log files stored?](#where-is-the-log-files-stored)
    - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
    - [Is it possible to retry a DAG from a specific step?](#is-it-possible-to-retry-a-dag-from-a-specific-step)
    - [Does it have a scheduler function?](#does-it-have-a-scheduler-function)
  - [🔗 GoDoc](#-godoc)
  - [⚠️ License](#️-license)

## 🌟 Project goal

dagman aims to be one of the easiest options to manage and run DAGs without a DBMS, operational burden, high learning curve, or even writing code.

## ⚙️ How does it work?

- dagman is a single binary and it uses the file system as the database and stores the data in plain JSON files. Therefore, no DBMS or cloud service is required.
- You can easily define DAGs using the declarative YAML format. Existing shell scripts or arbitrary programs can be used without modification. This makes the migration of existing workflows very easy.

## 🔥 Motivation

There were many problems in our ETL pipelines. Hundreds of cron jobs are on the server's crontab, and it is impossible to keep track of those dependencies between them. If one job failed, we were not sure which to re-run. We also have to SSH into the server to see the logs and run each shell script one by one manually. So We needed a tool that explicitly visualizes and allows us to manage the dependencies of the jobs in the pipeline.

***How nice it would be to be able to visually see the job dependencies, execution status, and logs of each job in a Web UI, and to be able to rerun or stop a series of jobs with just a mouse click!***

## 🤔 Why?
We considered many potential tools such as Airflow, Rundeck, Luigi, DigDag, JobScheduler, etc. But unfortunately, they were not suitable for our existing environment. Because in order to use one of those tools, we had to setup DBMS (Database Management System), accepet relatively high learning curves, and more operational overheads. We only have a small group of engineers in our office and use a less common DBMS. We couldn't afford them. Therefore, we developed a simple and easy-to-use workflow engine that fills the gap between cron and Airflow, that does not require DBMS, scheduler process or other daemons. I hope that this tool will help people in the same situation.

## ⚡️ Quick start

### Installation

Download the latest binary from the [Releases page](https://github.com/dagman/dagman/releases) and place it in your `$PATH`. For example, you can download it in `/usr/local/bin`.

### Download an example DAG definition

Download this [example](https://github.com/yohamta/dagman/blob/main/examples/complex_dag.yaml) and place it in the current directory with extension `*.yaml`.

### Start Web UI server

Start the server with `dagman server` and browse to `http://localhost:8080` to explore the Web UI.

### Running the DAG

You can start the example DAG from the Web UI by submitting `Start` button on the top right corner of the UI.

![example](https://user-images.githubusercontent.com/1475839/166093236-d5fd1633-55c9-46da-b77c-3c8f083c2f4b.gif)

## 📖 Usage

- `dagman start [--params=<params>] <file>` - run a DAG
- `dagman status <file>` - display the current status of the DAG
- `dagman retry --req=<request-id> <file>` - retry the failed/canceled DAG
- `dagman stop <file>` - stop a DAG execution by sending a TERM signal
- `dagman dry [--params=<params>] <file>` - dry-run a DAG
- `dagman server` - start a web server for web UI

## 🛠 Use cases
- ETL Pipeline
- Machine Learning model training
- Automated generation of reports
- Backups and other DevOps tasks
- Visualizing existing workflows

## ✨ Features

- Simple command interface (See [Usage](#usage))
- Simple configuration YAML format (See [Simple example](#simple-example))
- Web UI to visualize, manage DAGs and watch logs
- Parameterization
- Conditions
- Automatic retry
- Cancellation
- Retry
- Parallelism limits
- Environment variables
- Repeat
- Basic Authentication
- E-mail notifications
- REST API interface
- onExit / onSuccess / onFailure / onCancel handlers
- Automatic history cleaning

## 🖥 User interface

- **DAGs**: Overview of all DAGs.

  ![DAGs](https://user-images.githubusercontent.com/1475839/166093298-c978fd56-9476-41ac-94b1-47744c073ff5.png)

- **Detail**: Realtime status of the DAG.

  ![Detail](https://user-images.githubusercontent.com/1475839/166093310-03d9ea95-5180-4887-9896-a43939100def.png)

- **Timeline**: Timeline of each step in the DAG.

  ![Timeline](https://user-images.githubusercontent.com/1475839/166093320-1b3bf2c1-c23f-47b8-a67d-43f34a708450.png)

- **History**: History of the execution of the DAG.

  ![History](https://user-images.githubusercontent.com/1475839/166093335-47732cb8-ab8d-4c29-be16-baf547aae47c.png)

## 📋 DAG definition

### Minimal

A minimal DAG definition is as simple as:

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

Environment variables can be defined and used throughout the file using `env` field.

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

Parameters can be defined and referenced throughout a file using `params` field. Each parameter can be referenced as $1, $2, etc. Parameters can also be command substitutions or environment variables. You can override the default values of the parameters with the `--params=` parameter of the `start` command.

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

The global configuration file `~/.dagman/config.yaml` is useful to gather common settings, such as the directory to write log files.

### Examples

To check all examples, visit [this page](https://github.com/dagman/dagman/tree/main/examples).

-  Sample 1

  ![sample_1](https://user-images.githubusercontent.com/1475839/164965036-fede5675-cba0-410b-b371-22deec55b9e8.png)
```yaml
name: example
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

  ![sample_2](https://user-images.githubusercontent.com/1475839/166093413-ebcc3d7b-1757-4fdf-8747-32acf4fa9473.png)
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

  ![complex](https://user-images.githubusercontent.com/1475839/166093433-5245e39b-4f80-4e6e-a2f9-9be48d85133e.png)
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

## 🧑‍💻 Admin configurations

### Environment variables

- `dagman__DATA` - path to directory for internal use by dagman (default : `~/.dagman/data`)
- `dagman__LOGS` - path to directory for logging (default : `~/.dagman/logs`)

### Web UI configuration

Please create `~/.dagman/admin.yaml`.

```yaml
host: <hostname for web UI address>                          # default value is 127.0.0.1 
port: <port number for web UI address>                       # default value is 8080
dags: <the location of DAG configuration files>              # default value is current working directory
command: <Absolute path to the dagman binary>                  # [optional] required if the dagman command not in $PATH
isBasicAuth: <true|false>                                    # [optional] basic auth config
basicAuthUsername: <username for basic auth of web UI>       # [optional] basic auth config
basicAuthPassword: <password for basic auth of web UI>       # [optional] basic auth config
```

### Global DAG configuration

Please create `~/.dagman/config.yaml`. All settings can be overridden by individual DAG configurations.

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

## 💡 Architecture

- uses plain JSON files as history database, and unix sockets to communicate with running processes.
  ![dagman Architecture](https://user-images.githubusercontent.com/1475839/164869015-769bfe1d-ad38-4aca-836b-bf3ffe0665df.png)

## ❓FAQ

### How to contribute?

Feel free to contribute in any way you want. Share ideas, submit issues, and create pull requests. 
You can start by improving this [README.md](https://github.com/dagman/dagman/blob/main/README.md) or suggesting new [features](https://github.com/dagman/dagman/issues)
Thank you!

### Where is the history data stored?

dagman's DAG execution history data is stored in plain JSON files in the path of the `dagman__DATA` environment variable with extension `*.dat`. The default location is `$HOME/.dagman/data`.

### Where is the log files stored?

Log files are stored in the path of the `dagman__LOGS` environment variable. The default location is `$HOME/.dagman/logs`. You can override this setting by `logDir` option in a YAML file.

### How long will the history data be stored?

The default retention period for execution history is 7 days. This setting can be changed with `histRetentionDays` option in a YAML file.

### Is it possible to retry a DAG from a specific step?

You can change the status of any task to a `failed` status. Then, when the job is retried, the tasks after the failed node will be executed.

![Update Status](https://user-images.githubusercontent.com/1475839/165755497-923828f8-1992-43fe-8618-979128c38c79.png)

### Does it have a scheduler function?

No, there is no scheduler functionality so far. It is intended to be used with cron.

## 🔗 GoDoc

https://pkg.go.dev/github.com/yohamta/dagman

## ⚠️ License
This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details

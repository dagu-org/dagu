#  dagu 
<img align="right" width="150" src="https://user-images.githubusercontent.com/1475839/165412252-4fbb28ae-0845-4af2-9183-0aa1de5bf707.png" alt="dagu" title="dagu" />

**A simpler Airflow aLternative to run workflows (DAGs) defined in declarative YAML format**

dagu is a simple workflow engine to executes [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) defined in YAML format. dagu also comes with a rich web UI and REST API interface.

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
  - [Features](#features)
  - [Use cases](#use-cases)
  - [User interface](#user-interface)
  - [Configuration](#configuration)
    - [Environment variables](#environment-variables)
    - [Web UI configuration](#web-ui-configuration)
    - [Global DAG configuration](#global-dag-configuration)
  - [Individual DAG configuration](#individual-dag-configuration)
    - [Minimal](#minimal)
    - [Available configurations](#available-configurations)
  - [Examples](#examples)
  - [Architecture](#architecture)
  - [FAQ](#faq)
    - [How to contribute?](#how-to-contribute)
    - [Where is the history data stored?](#where-is-the-history-data-stored)
    - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
    - [Is it possible to retry a DAG from a specific step?](#is-it-possible-to-retry-a-dag-from-a-specific-step)
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

## Use cases
- ETL Pipeline
- Batches
- Machine Learning
- Data Processing
- Automation

## User interface

- **DAGs**: Overview of all DAGs in your environment.

  ![DAGs](https://user-images.githubusercontent.com/1475839/165417789-18d29f3d-aecf-462a-8cdf-0b575ba613d0.png)

- **Detail**: Current status of the dag.

  ![Detail](https://user-images.githubusercontent.com/1475839/165418393-d7d876bc-329f-4299-977e-7726e8ef0fa1.png)

- **Timeline**: Timeline of each steps in the pipeline.

  ![Timeline](https://user-images.githubusercontent.com/1475839/165418430-1fe3b100-33eb-4d81-a68a-c8a881890b61.png)

- **History**: History of the execution of the pipeline.

  ![History](https://user-images.githubusercontent.com/1475839/165426067-02c4f72f-e3f0-4cd8-aa38-35fa98f0382f.png)

## Configuration

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

## Individual DAG configuration

### Minimal

```yaml
name: minimal configuration          # DAG name
steps:                               # steps inside the DAG
  - name: step 1                     # step name (should be unique within the file)
    description: step 1              # [optional] description of the step
    command: python main_1.py        # command and arguments
    dir: ${HOME}/dags/               # [optional] working directory
  - name: step 2
    description: step 2
    command: python main_2.py
    dir: ${HOME}/dags/
    depends:
      - step 1                       # [optional] dependant steps
```

### Available configurations

```yaml
name: all configuration              # DAG name
description: run a DAG               # DAG description
env:                                 # Environment variables
  LOG_DIR: ${HOME}/logs
  PATH: /usr/local/bin:${PATH}
logDir: ${LOG_DIR}                   # log directory to write standard output
histRetentionDays: 3                 # execution history retention days (not for log files)
delaySec: 1                          # interval seconds between steps
maxActiveRuns: 1                     # max parallel number of running step
params: param1 param2                # parameters can be refered by $1, $2 and so on.
preconditions:                       # precondisions for whether the DAG is allowed to run
  - condition: "`printf 1`"          # command or variables to evaluate
    expected: "1"                    # value to be expected to run the DAG
mailOn:
  failure: true                      # send a mail when the DAG failed
  success: true                      # send a mail when the DAG finished
handlerOn:                           # Handler on Success, Failure, Cancel, Exit
  success:                           # will be executed when the DAG succeed
    command: "echo succeed"
  failure:                           # will be executed when the DAG failed 
    command: "echo failed"
  cancel:                            # will be executed when the DAG canceled 
    command: "echo canceled"
  exit:                              # will be executed when the DAG exited
    command: "echo finished"
steps:
  - name: step 1                     # DAG name
    description: step 1              # DAG description
    dir: ${HOME}/logs                # working directory
    command: python main.py $1       # command and parameters
    mailOn:
      failure: true                  # send a mail when the step failed
      success: true                  # send a mail when the step finished
    continueOn:
      failed: true                   # continue to the next regardless the step failed or not
      skipped: true                  # continue to the next regardless the preconditions are met or not 
    retryPolicy:                     # retry policy for the step
      limit: 2                       # retry up to 2 times when the step failed
    preconditions:                   # precondisions for whether the step is allowed to run
      - condition: "`printf 1`"      # command or variables to evaluate
        expected: "1"                # value to be expected to run the step
```

The global config file `~/.dagu/config.yaml` is useful to gather common settings such as log directory.

## Examples

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

## Architecture

- uses plain JSON files as history database, and unix sockets to communicate with running processes.
  ![dagu Architecture](https://user-images.githubusercontent.com/1475839/164869015-769bfe1d-ad38-4aca-836b-bf3ffe0665df.png)

## FAQ

### How to contribute?

Feel free to contribute in any way you want. Share ideas, submit issues, create pull requests. 
You can start by improving this [README.md](https://github.com/dagu/dagu/blob/main/README.md) or suggesting new [features](https://github.com/dagu/dagu/issues)
Thank you!

### Where is the history data stored?

DAGU's DAG execution history data is stored in json files in the path of the `DAGU__DATA` environment variable. However the extension is `*.dat`

The default location is `$HOME/.dagu/data`.

### How long will the history data be stored?

The default retension period for execution history is 7 days.

This setting can be changed with `histRetentionDays` option in the config file.

### Is it possible to retry a DAG from a specific step?

Just like Airflow, you can change the status of any task to failed. Then, when the job is retried, the tasks after the failed node will be executed.

![Update Status](https://user-images.githubusercontent.com/1475839/165755497-923828f8-1992-43fe-8618-979128c38c79.png)

## License
This project is licensed under the GNU GPLv3 - see the [LICENSE.md](LICENSE.md) file for details

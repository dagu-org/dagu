<p align="center">
  <img src="./assets/images/dagu-logo.webp" width="960" alt="dagu-logo">
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/dagu-org/dagu">
    <img src="https://goreportcard.com/badge/github.com/dagu-org/dagu" />
  </a>
  <a href="https://codecov.io/gh/dagu-org/dagu">
    <img src="https://codecov.io/gh/dagu-org/dagu/branch/main/graph/badge.svg?token=CODZQP61J2" />
  </a>
  <a href="https://github.com/dagu-org/dagu/releases">
    <img src="https://img.shields.io/github/release/dagu-org/dagu.svg" />
  </a>
  <a href="https://godoc.org/github.com/dagu-org/dagu">
    <img src="https://godoc.org/github.com/dagu-org/dagu?status.svg" />
  </a>
  <img src="https://github.com/dagu-org/dagu/actions/workflows/ci.yaml/badge.svg" />
</p>

<div align="center">

[Installation](https://dagu.readthedocs.io/en/latest/installation.html) | [Community](https://discord.gg/gpahPUjGRk) | [Quick Start](https://dagu.readthedocs.io/en/latest/quickstart.html)

</div>

<h1><b>Dagu</b></h1>

Dagu is a powerful Cron alternative that comes with a Web UI. It allows you to define dependencies between commands in a declarative YAML Format. Additionally, Dagu natively supports running Docker containers, making HTTP requests, and executing commands over SSH. Dagu was designed to be easy to use, self-contained, and require no coding, making it ideal for small projects.

## Why Dagu?

Dagu is a modern workflow engine that combines simplicity with power, designed for developers who need reliable automation without the overhead. Here's what makes Dagu stand out:

- **Language Agnostic**: Run any command or script regardless of programming language. Whether you're working with Python, Node.js, Bash, or any other language, Dagu seamlessly integrates with your existing tools and scripts.

- **Local-First Architecture**: Deploy and run workflows directly on your machine without external dependencies. This local-first approach ensures complete control over your automation while maintaining the flexibility to scale to distributed environments when needed.

- **Zero Configuration**: Get started in minutes with minimal setup. Dagu uses simple YAML files to define workflows, eliminating the need for complex configurations or infrastructure setup.

- **Built for Developers**: Designed with software engineers in mind, Dagu provides powerful features like dependency management, retry logic, and parallel execution while maintaining a clean, intuitive interface.

- **Cloud Native Ready**: While running perfectly on local environments, Dagu is built to seamlessly integrate with modern cloud infrastructure when you need to scale.

## **Features**

- Environment variables
- Flexible parameter passing
- Conditional logic with regex and shell commands
- Automatic retries
- Parallel steps
- Repeat steps at regular intervals
- Running sub workflows
- Execution timeouts
- Automatic logging
- Lifecycle hooks (on failure, on exit, etc.)
- Email notifications
- Running Docker containers in steps
- JSON handling support
- Controlling remote Dagu nodes from a single UI
- SSH remote commands in steps
- Flexible scheduling with cron expressions

## **Community**

- Issues: [GitHub Issues](https://github.com/dagu-org/dagu/issues)
- Discussion: [GitHub Discussions](https://github.com/dagu-org/dagu/discussions)
- Chat: [Discord](https://discord.gg/gpahPUjGRk)

## **Installation**

Dagu can be installed in multiple ways, such as using Homebrew or downloading a single binary from GitHub releases.

### Via Bash script

```sh
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash
```

### Via GitHub Releases Page

Download the latest binary from the [Releases page](https://github.com/dagu-org/dagu/releases) and place it in your `$PATH` (e.g. `/usr/local/bin`).

### Via Homebrew (macOS)

```sh
brew install dagu-org/brew/dagu
```

Upgrade to the latest version:

```sh
brew upgrade dagu-org/brew/dagu
```

### Via Docker

```sh
docker run \
--rm \
-p 8080:8080 \
-v ~/.config/dagu:/config \
-e DAGU_TZ=`ls -l /etc/localtime | awk -F'/zoneinfo/' '{print $2}'` \
ghcr.io/dagu-org/dagu:latest dagu start-all
```

Note: The environment variable `DAGU_TZ` is the timezone for the scheduler and server. You can set it to your local timezone (e.g. `America/New_York`).

See [Environment variables](https://dagu.readthedocs.io/en/latest/config.html#environment-variables) to configure those default directories.

## **Quick Start Guide**

### 1. Launch the Web UI

Start the server and scheduler with the command `dagu start-all` and browse to `http://127.0.0.1:8080` to explore the Web UI.

### 2. Create a New DAG

Navigate to the DAG List page by clicking the menu in the left panel of the Web UI. Then create a DAG by clicking the `NEW` button at the top of the page. Enter `example` in the dialog.

_Note: DAG (YAML) files will be placed in `~/.config/dagu/dags` by default. See [Configuration Options](https://dagu.readthedocs.io/en/latest/config.html) for more details._

### 3. Edit the DAG

Go to the `SPEC` Tab and hit the `Edit` button. Copy & Paste the following example and click the `Save` button.

Example:

```yaml
schedule: "* * * * *" # Run the DAG every minute
params:
  - NAME: "Dagu"
steps:
  - name: Hello world
    command: echo Hello $NAME
  - name: Done
    command: echo Done!
    depends: Hello world
```

### 4. Execute the DAG

You can execute the example by pressing the `Start` button. You can see "Hello Dagu" in the log page in the Web UI.

## **Usage / Command Line Interface**

```sh
# Runs the DAG
dagu start <file or DAG name>

# Runs the DAG with named parameters
dagu start <file or DAG name> [-- <key>=<value> ...]

# Runs the DAG with positional parameters
dagu start <file or DAG name> [-- value1 value2 ...]

# Displays the current status of the DAG
dagu status <file or DAG name>

# Re-runs the specified DAG run
dagu retry --req=<request-id> <file or DAG name>

# Stops the DAG execution
dagu stop <file or DAG name>

# Restarts the current running DAG
dagu restart <file or DAG name>

# Dry-runs the DAG
dagu dry <file or DAG name> [-- <key>=<value> ...]

# Launches both the web UI server and scheduler process
dagu start-all [--host=<host>] [--port=<port>] [--dags=<path to directory>]

# Launches the Dagu web UI server
dagu server [--host=<host>] [--port=<port>] [--dags=<path to directory>]

# Starts the scheduler process
dagu scheduler [--dags=<path to directory>]

# Shows the current binary version
dagu version
```

## **Example DAG**

### Minimal examples

A DAG with two steps:

```yaml
params:
  - NAME: "Dagu"
steps:
  - name: Hello world
    command: echo Hello $NAME
  - name: Done
    command: echo Done!
    depends:
      - Hello world
```

Using a pipe:

```yaml
steps:
  - name: step 1
    command: echo hello world | xargs echo
```

Specifying a shell:

```yaml
steps:
  - name: step 1
    command: echo hello world | xargs echo
    shell: bash # The default shell is `$SHELL` or `sh`.
```

You can also define each steps as map instead of list:

```yaml
steps:
  step1:
    command: echo "Hello"
  step2:
    command: echo "Bye"
    depends: step1
```

### Conditional DAG

You can add conditional logic to a DAG:

```yaml
steps:
  - name: monthly task
    command: monthly.sh
    preconditions:
      - condition: "`date '+%d'`"
        expected: "re:0[1-9]" # Run only if the day is between 01 and 09
```

### Scheduling

You can specify the schedule with cron expression:

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

If you want to start and stop a long-running process on a fixed schedule, you can define ``start`` and ``stop`` times:

```yaml
schedule:
  start: "0 8 * * *" # starts at 8:00
  stop: "0 13 * * *" # stops at 13:00
steps:
  - name: scheduled job
    command: job.sh
```

### Calling a sub-DAG

You can call a sub-DAG from a parent DAG:

```yaml
steps:
  - name: parent
    run: sub-dag
    output: OUT
  - name: use output
    command: echo ${OUT.outputs.result}
    depends: parent
```

The sub-DAG `sub-dag.yaml`:

```yaml
steps:
  - name: sub-dag
    command: echo "Hello from sub-dag"
    output: result
```

THe parent DAG will call the sub-DAG and write the output to the log (stdout).
The output will be `Hello from sub-dag`.

### Running a docker image

You can run a docker image as a step:

```yaml
steps:
  - name: hello
    executor:
      type: docker
      config:
        image: alpine
        autoRemove: true
    command: echo "hello"
```

More examples can be found in the [documentation](https://dagu.readthedocs.io/en/latest/yaml_format.html).

## **Web UI**

### DAG Details

Real-time status, logs, and configuration for each DAG. Toggle graph orientation from the top-right corner.

![example](assets/images/demo.gif?raw=true)

![Details-TD](assets/images/ui-details2.webp?raw=true)

### DAGs

View all DAGs in one place with live status updates.

![DAGs](assets/images/ui-dags.webp?raw=true)

### Search

Search across all DAG definitions.

![History](assets/images/ui-search.webp?raw=true)

### Execution History

Review past DAG executions and logs at a glance.

![History](assets/images/ui-history.webp?raw=true)

### Log Viewer

Examine detailed step-level logs and outputs.

![DAG Log](assets/images/ui-logoutput.webp?raw=true)

## **Running as a daemon**

The easiest way to make sure the process is always running on your system is to create the script below and execute it every minute using cron (you don't need `root` account in this way):

```bash
#!/bin/bash
process="dagu start-all"
command="/usr/bin/dagu start-all"

if ps ax | grep -v grep | grep "$process" > /dev/null
then
    exit
else
    $command &
fi

exit
```

## Contributing

We welcome new contributors! Check out our [Contribution Guide](https://dagu.readthedocs.io/en/latest/contrib.html) for guidelines on how to get started.

## **Contributors**

<a href="https://github.com/dagu-org/dagu/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=dagu-org/dagu" />
</a>

## **License**

Dagu is released under the [GNU GPLv3](./LICENSE.md).

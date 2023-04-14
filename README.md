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

# **Dagu**

### **Just another Cron alternative with a Web UI, but with much more capabilities**

Dagu is a tool for scheduling and running tasks based on [DAGs](https://en.wikipedia.org/wiki/Directed_acyclic_graph) defined in a simple YAML format. It allows you to define dependencies between commands and represent them as a single DAG, schedule the execution of DAGs with Cron expressions, and natively support running Docker containers, making HTTP requests, and executing commands over SSH.

- [Documentation](https://dagu.readthedocs.io) 
- [Discord](https://discord.gg/4s4feC8r)

## **Highlights**
- Single binary file installation
- Declarative YAML format for defining DAGs
- Web UI for visually managing, rerunning, and monitoring pipelines
- Use existing programs without any modification
- Self-contained, with no need for a DBMS

## **Features**

- Web User Interface
- Command Line Interface (CLI) with several commands for running and managing DAGs
- YAML format for defining DAGs, with support for various features including:
  - Execution of custom code snippetts
  - Parameters
  - Command substitution
  - Conditional logic
  - Redirection of stdout and stderr
  - Lifecycle hooks
  - Repeating task
  - Automatic retry
- Executors for running different types of tasks:
  - Running arbitrary Docker containers
  - Making HTTP requests
  - Sending emails
  - Running jq command
  - Executing remote commands via SSH
- Support for Email notification
- Support for configuration options through environment variables
- Scheduling with Cron expressions
- Support for REST API Interface

## **Documentation**

- [Installation Instructions](https://dagu.readthedocs.io/en/latest/installation.html)
- ️[Quick Start Guide](https://dagu.readthedocs.io/en/latest/quickstart.html)
- [Command Line Interface](https://dagu.readthedocs.io/en/latest/cli.html)
- YAML Format
  - [Minimal DAG Definition](https://dagu.readthedocs.io/en/latest/yaml_format.html#minimal-dag-definition)
  - [Running Arbitrary Code Snippets](https://dagu.readthedocs.io/en/latest/yaml_format.html#running-arbitrary-code-snippets)
  - [Defining Environment Variables](https://dagu.readthedocs.io/en/latest/yaml_format.html#defining-environment-variables)
  - [Defining and Using Parameters](https://dagu.readthedocs.io/en/latest/yaml_format.html#defining-and-using-parameters)
  - [Using Command Substitution](https://dagu.readthedocs.io/en/latest/yaml_format.html#using-command-substitution)
  - [Adding Conditional Logic](https://dagu.readthedocs.io/en/latest/yaml_format.html#adding-conditional-logic)
  - [Setting Environment Variables with Standard Output](https://dagu.readthedocs.io/en/latest/yaml_format.html#setting-environment-variables-with-standard-output)
  - [Redirecting Stdout and Stderr](https://dagu.readthedocs.io/en/latest/yaml_format.html#redirecting-stdout-and-stderr)
  - [Adding Lifecycle Hooks](https://dagu.readthedocs.io/en/latest/yaml_format.html#adding-lifecycle-hooks)
  - [Repeating a Task at Regular Intervals](https://dagu.readthedocs.io/en/latest/yaml_format.html#repeating-a-task-at-regular-intervals)
  - [All Available Fields for DAGs](https://dagu.readthedocs.io/en/latest/yaml_format.html#all-available-fields-for-dags)
  - [All Available Fields for Steps](https://dagu.readthedocs.io/en/latest/yaml_format.html#all-available-fields-for-steps)
- Example DAGs
  - [Hello World](https://dagu.readthedocs.io/en/latest/examples.html#hello-world)
  - [Conditional Steps](https://dagu.readthedocs.io/en/latest/examples.html#conditional-steps)
  - [File Output](https://dagu.readthedocs.io/en/latest/examples.html#file-output)
  - [Passing Output to Next Step](https://dagu.readthedocs.io/en/latest/examples.html#passing-output-to-next-step)
  - [Running a Docker Container](https://dagu.readthedocs.io/en/latest/examples.html#running-a-docker-container)
  - [Sending HTTP Requests](https://dagu.readthedocs.io/en/latest/examples.html#sending-http-requests)
  - [Querying JSON Data with jq](https://dagu.readthedocs.io/en/latest/examples.html#querying-json-data-with-jq)
  - [Sending Email](https://dagu.readthedocs.io/en/latest/examples.html#sending-email)
- [Configurations](https://dagu.readthedocs.io/en/latest/config.html)
- [Scheduler](https://dagu.readthedocs.io/en/latest/scheduler.html)
- [Docker Compose](https://dagu.readthedocs.io/en/latest/docker-compose.html)

## **Web User Interface**

![example](assets/images/demo.gif?raw=true)

## **Hello World**

This example outputs "hello world" to the log.

```yaml
steps:
  - name: s1
    command: echo hello world
  - name: s2
    command: echo done!
    depends:
      - s1
```

## **Example Workflow**

This example workflow calls the ChatGPT API and sends the result to your email address.

```yaml
env:
  - OPENAI_API_KEY: "OPEN_API_KEY"
  - MY_EMAIL: "YOUR_EMAIL_ADDRESS"

smtp:
  host: "smtp.mailgun.org"
  port: "587"
  username: "MAILGUN_USERNAME"
  password: "MAILGUN_PASSWORD"

steps:
  - name: ask chatgpt
    executor:
      type: http
      config:
        timeout: 180
        headers:
          Authorization: "Bearer $OPENAI_API_KEY"
          Content-Type: "application/json"
        silent: true
        body: |
          { "model": "gpt-3.5-turbo", "messages": [
              {"role": "system", "content": "You are a philosopher of the Roman Imperial Period"},
              {"role": "user", "content": "$QUESTION"}
            ] 
          }
    command: POST https://api.openai.com/v1/chat/completions
    output: API_RESPONSE

  - name: get result
    executor:
      type: jq
      config:
        raw: true
    command: ".choices[0].message.content"
    script: "$API_RESPONSE"
    output: MESSAGE_CONTENT
    depends:
      - ask chatgpt
  
  - name: send mail
    executor:
      type: mail
      config:
        to: "$MY_EMAIL"
        from: "$MY_EMAIL"
        subject: "philosopher's reply"
        message: |
          <html>
            <body>
              <div>$QUESTION</div>
              <div>$MESSAGE_CONTENT<div>
            </body>
          </html>
    depends:
      - get result
```

You can input the ChatGPT prompt on the Web UI.

![params-input](./assets/images/ui-params.png)

## **Motivation**

Legacy systems often have complex and implicit dependencies between jobs. When there are hundreds of cron jobs on a server, it can be difficult to keep track of these dependencies and to determine which job to rerun if one fails. It can also be a hassle to SSH into a server to view logs and manually rerun shell scripts one by one. Dagu aims to solve these problems by allowing you to explicitly visualize and manage pipeline dependencies as a DAG, and by providing a web UI for checking dependencies, execution status, and logs and for rerunning or stopping jobs with a simple mouse click.

## **Why Not Use an Existing Workflow Scheduler Like Airflow?**

There are many existing tools such as Airflow, but many of these require you to write code in a programming language like Python to define your DAG. For systems that have been in operation for a long time, there may already be complex jobs with hundreds of thousands of lines of code written in languages like Perl or Shell Script. Adding another layer of complexity on top of these codes can reduce maintainability. Dagu was designed to be easy to use, self-contained, and require no coding, making it ideal for small projects.

## **How It Works**

Dagu is a single command line tool that uses the local file system to store data, so no database management system or cloud service is required. DAGs are defined in a declarative YAML format, and existing programs can be used without modification.

## **Roadmap**

- Writing dags in the starlark language
- AWS Lambda Execution
- DAG Versioning
- Slack Integration
- Database Option
- Cluster Mode

## **Contributing**

Feel free to contribute in any way you want. Share ideas, questions, submit issues, and create pull requests.

- Request a feature in our [Discord community](https://discord.gg/4s4feC8r).
- Open a PR or Issue

## **License**

This project is licensed under the GNU GPLv3.

## **Support and Community**

Join our [Discord community](https://discord.gg/4s4feC8r).

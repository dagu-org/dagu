# Examples

- [Examples](#examples)
  - [Printing "Hello World"](#printing-hello-world)
  - [Executing Conditional Steps](#executing-conditional-steps)
  - [Writing to a File](#writing-to-a-file)
  - [Passing Output to Next Step](#passing-output-to-next-step)
  - [Running a Docker Container](#running-a-docker-container)
    - [Configuring Container Volumes, Environment Variables, and More](#configuring-container-volumes-environment-variables-and-more)
    - [Running Containers on the Host's Docker Environment](#running-containers-on-the-hosts-docker-environment)
  - [Executing Commands over SSH](#executing-commands-over-ssh)
  - [Sending HTTP Requests](#sending-http-requests)
  - [Querying JSON Data with jq](#querying-json-data-with-jq)
  - [Formatting JSON Data with jq](#formatting-json-data-with-jq)
  - [Outputting Raw Values with jq](#outputting-raw-values-with-jq)
  - [Sending Email Notifications](#sending-email-notifications)
  - [Sending Email](#sending-email)
  - [Customizing Signal Handling on Stop](#customizing-signal-handling-on-stop)

## Printing "Hello World"

This example demonstrates how to print "hello world" to the log.

```yaml
name: hello world
steps:
  - name: "hello"
    command: echo hello world
  - name: "done"
    command: echo done!
    depends:
      - "1"
```

## Executing Conditional Steps

This example demonstrates how to execute conditional steps.

```yaml
params: foo
steps:
  - name: "step1"
    command: echo start
  - name: "foo"
    command: echo foo
    depends:
      - "step1"
    preconditions:
      - condition: "$1"
        expected: foo
  - name: "bar"
    command: echo bar
    depends:
      - "step1"
    preconditions:
      - condition: "$1"
        expected: bar
```

![conditional](./images/conditional.png)

## Writing to a File

This example demonstrates how to write text to a file.

```yaml
steps:
  - name: write hello to '/tmp/hello.txt'
    command: echo hello
    stdout: /tmp/hello.txt
```

## Passing Output to Next Step

This example demonstrates how to pass output from one step to the next. It will print "hello world" to the log.

```yaml
steps:
  - name: pass 'hello'
    command: echo hello
    output: OUT1
  - name: output 'hello world'
    command: bash
    script: |
      echo $OUT1 world
    depends:
      - pass 'hello'
```

## Running a Docker Container

This example demonstrates how to run a Docker container.

```yaml
steps:
  - name: deno_hello_world
    executor: 
      type: docker
      config:
        image: "denoland/deno:1.10.3"
        host:
          autoRemove: true
    command: run https://examples.deno.land/hello-world.ts
```

Example log output:

![docker](./images/docker.png)

You can configure the Docker host with the environment variable `DOCKER_HOST`.

For example:
```yaml
env:
  - DOCKER_HOST : "tcp://XXX.XXX.XXX.XXX:2375"
steps:
  - name: deno_hello_world
    executor: 
      type: docker
      config:
        image: "denoland/deno:1.10.3"
        autoRemove: true
    command: run https://examples.deno.land/hello-world.ts
```

### Configuring Container Volumes, Environment Variables, and More

You can config the Docker container (e.g., `volumes`, `env`, etc) by passing more detailed options.

For example:
```yaml
steps:
  - name: deno_hello_world
    executor: 
      type: docker
      config:
        image: "denoland/deno:1.10.3"
        container:
          volumes:
            /app:/app:
          env:
            - FOO=BAR
        host:
          autoRemove: true
    command: run https://examples.deno.land/hello-world.ts
```

See the Docker's API documentation for all available options.

- For `container`, see [ContainerConfig](https://pkg.go.dev/github.com/docker/docker/api/types/container#Config).
- For `host`, see [HostConfig](https://pkg.go.dev/github.com/docker/docker/api/types/container#HostConfig).

### Running Containers on the Host's Docker Environment

If you are running `dagu` using a container, you need the setup below.

1. Run a `socat` conainer with the command below.

```sh
docker run -v /var/run/docker.sock:/var/run/docker.sock -p 2376:2375 bobrik/socat TCP4-LISTEN:2375,fork,reuseaddr UNIX-CONNECT:/var/run/docker.sock
```

2. Then you can set the `DOCKER_HOST` environment as follows.

```yaml
env:
  - DOCKER_HOST : "tcp://host.docker.internal:2376"
steps:
  - name: deno_hello_world
    executor: 
      type: docker
      config:
        image: "denoland/deno:1.10.3"
        autoRemove: true
    command: run https://examples.deno.land/hello-world.ts
```

For more details, see [this page](https://forums.docker.com/t/remote-api-with-docker-for-mac-beta/15639/2).

## Executing Commands over SSH

```yaml
steps:
  - name: print ec2 instance id
    executor: 
      type: ssh
      config:
        user: ec2-user
        ip: "XXX.XXX.XXX.XXX"
        key: /Users/XXXXX/.ssh/prod-ec2instance-keypair.pem
        StrictHostKeyChecking: false
    command: ec2-metadata -i

```

## Sending HTTP Requests

```yaml
steps:
  - name: get fake json data
    executor: http
    command: GET https://jsonplaceholder.typicode.com/comments
    script: |
      {
        "timeout": 10,
        "headers": {},
        "query": {
          "postId": "1"
        },
        "body": ""
      }      
```

## Querying JSON Data with jq
```yaml
steps:
  - name: run query
    executor: jq
    command: '{(.id): .["10"].b}'
    script: |
      {"id": "sample", "10": {"b": 42}}
```

log output:
```json
{
    "sample": 42
}
```

## Formatting JSON Data with jq
```yaml
steps:
  - name: format json
    executor: jq
    script: |
      {"id": "sample", "10": {"b": 42}}
```

log output:
```json
{
    "10": {
        "b": 42
    },
    "id": "sample"
}
```

## Outputting Raw Values with jq
```yaml
steps:
  - name: output raw value
    executor:
      type: jq
      config:
        raw: true
    command: '.id'
    script: |
      {"id": "sample", "10": {"b": 42}}
```

log output:
```json
sample
```

## Sending Email Notifications

![sample](./images/email.png)

```yaml
steps:
  - name: Sending Email on Finish or Error
    command: echo "hello world"

mailOn:
  failure: true
  success: true

smtp:
  host: "smtp.foo.bar"
  port: "587"
  username: "<username>"
  password: "<password>"
errorMail:
  from: "foo@bar.com"
  to: "foo@bar.com"
  prefix: "[Error]"
infoMail:
  from: "foo@bar.com"
  to: "foo@bar.com"
  prefix: "[Info]"
```

## Sending Email

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
          I seem to have lost my sanity somewhere between my third cup of coffee and my fourth Zoom meeting of the day.
          
          If you see it lying around, please let me know.
          Thanks for your help!

          Best,
```

## Customizing Signal Handling on Stop

```yaml
steps:
  - name: step1
    command: bash
    script: |
      for s in {1..64}; do trap "echo trap $s" $s; done
      sleep 60
    signalOnStop: "SIGINT"
```

In the above example, the script waits indefinitely for a `SIGINT` signal, but a custom handler is defined to print a message and exit with code 130 when the signal is received. The signals section is used to configure dagu to send `SIGINT` signals when the user stops the pipeline.

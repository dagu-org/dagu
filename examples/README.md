# Examples

- [Examples](#examples)
  - [Printing Hello World](#printing-hello-world)
  - [Conditional step](#conditional-step)
  - [Writing to a file](#writing-to-a-file)
  - [Passing output to the next step](#passing-output-to-the-next-step)
  - [Runing Docker image](#runing-docker-image)
  - [Runing command via SSH](#runing-command-via-ssh)
  - [Sending HTTP request](#sending-http-request)
  - [Sending Email Notification](#sending-email-notification)
  - [Customizing Signal on Stop](#customizing-signal-on-stop)
- [How to contribute?](#how-to-contribute)

## Printing Hello World

![hello world](./images/helloworld.png)

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

## Conditional step

![conditional](./images/conditional.png)

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

## Writing to a file

```yaml
steps:
  - name: write hello to '/tmp/hello.txt'
    command: echo hello
    stdout: /tmp/hello.txt
```

## Passing output to the next step

![output](./images/output.png)

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

## Runing Docker image

Example Log output

![docker](./images/docker.png)

```yaml
steps:
  - name: deno_hello_world
    executor: 
      type: docker
      config:
        image: "denoland/deno:1.10.3"
        container:
          # See https://pkg.go.dev/github.com/docker/docker/api/types/container#Config
          volumes:
            /app:/app:
          env:
            - FOO=BAR
        host:
          # See https://pkg.go.dev/github.com/docker/docker/api/types/container#HostConfig
          autoRemove: true
    command: run https://examples.deno.land/hello-world.ts
```

## Runing command via SSH

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

## Sending HTTP request

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

## Sending Email Notification

Email example

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

## Customizing Signal on Stop

```yaml
steps:
  - name: step1
    command: bash
    script: |
      for s in {1..64}; do trap "echo trap $s" $s; done
      sleep 60
    signalOnStop: "SIGINT"
```

# How to contribute?

Feel free to contribute interesting examples in this page. Thanks!

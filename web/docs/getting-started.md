---
sidebar_position: 2
---

# Getting Started

### 1. Installation

Download the latest binary from the [Releases page](https://github.com/yohamta/dagu/releases) and place it in your `$PATH`. For example, you can download it in `/usr/local/bin`.

### 2. Launch the web UI

Start the server with `dagu server` and browse to `http://127.0.0.1:8000` to explore the Web UI.

### 3. Your first workflow on Dagu

Create a workflow by clicking the `New DAG` button on the top page of the web UI. Input `hello.yaml` in the dialog.

Go to the workflow detail page and click `Edit` button in the `Config` Tab. Then, copy and paste the below snippet and click the `Save` button.

```yaml
name: your First DAG!
steps:
  - name: Hello
    command: "echo Hello"
  - name: Dagu
    command: "echo Dagu!"
    depends:
      - Hello
```

### 4. Running the example

You can start the example by pressing `Start` button.

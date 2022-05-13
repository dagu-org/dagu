# FAQs

  - [How to contribute?](#how-to-contribute)
  - [Where is the history data stored?](#where-is-the-history-data-stored)
  - [Where are the log files stored?](#where-are-the-log-files-stored)
  - [How long will the history data be stored?](#how-long-will-the-history-data-be-stored)
  - [How can a workflow be retried from a specific task?](#how-can-a-workflow-be-retried-from-a-specific-task)
  - [Does it have a scheduler function?](#does-it-have-a-scheduler-function)
  - [How can it communicate with running processes?](#how-can-it-communicate-with-running-processes)

## How to contribute?

Feel free to contribute in any way you want. Share ideas, questions, submit issues, and create pull requests. Thank you!

## Where is the history data stored?

It will store execution history data in the `DAGU__DATA` environment variable path. The default location is `$HOME/.dagu/data`.

## Where are the log files stored?

It will store log files in the `DAGU__LOGS` environment variable path. The default location is `$HOME/.dagu/logs`. You can override the setting by the `logDir` field in a YAML file.

## How long will the history data be stored?

The default retention period for execution history is seven days. However, you can override the setting by the `histRetentionDays` field in a YAML file.

## How can a workflow be retried from a specific task?

You can change the status of any task to a `failed` state. Then, when you retry the workflow, it will execute the failed one and any subsequent.

![Update Status](https://user-images.githubusercontent.com/1475839/166289470-f4af7e14-28f1-45bd-8c32-59cd59d2d583.png)

## Does it have a scheduler function?

No, it doesn't have scheduler functionality. It is meant to be used with cron or other schedulers.

## How can it communicate with running processes?

Dagu uses Unix sockets to communicate with running processes.

![dagu Architecture](https://user-images.githubusercontent.com/1475839/166390371-00bb4af0-3689-406a-a4d5-af943a1fd2ce.png)

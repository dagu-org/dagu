---
sidebar_position: 1
---

# Introduction

Dagu executes [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) from declarative YAML definitions. Dagu also comes with a web UI for visualizing workflows.

![example](https://user-images.githubusercontent.com/1475839/165764122-0bdf4bd5-55bb-40bb-b56f-329f5583c597.gif)

## Motivation

There were many problems in our ETL pipelines. Hundreds of cron jobs are on the server's crontab, and it is impossible to keep track of those dependencies between them. If one job failed, we were not sure which to rerun. We also have to SSH into the server to see the logs and run each shell script one by one manually. So We needed a tool that explicitly visualizes and allows us to manage the dependencies of the jobs in the pipeline.

***How nice it would be to be able to visually see the job dependencies, execution status, and logs of each job in a Web UI and to be able to rerun or stop a series of jobs with just a mouse click!***

## Why not Airflow or Prefect?

Airflow and Prefect are powerful and valuable tools, but they require writing Python code to manage workflows. Our ETL pipeline is already hundreds of thousands of lines of complex code in Perl and shell scripts. Adding another layer of Python on top of this would make it even more complicated. Instead, we needed a more lightweight solution. So we have developed a No-code workflow execution engine that doesn't require writing code. Dagu is easy to use and self-contained, making it ideal for smaller projects with fewer people. We hope that this tool will help others in the same situation.

---
sidebar_position: 1
---

# Introduction

Dagu executes [DAGs (Directed acyclic graph)](https://en.wikipedia.org/wiki/Directed_acyclic_graph) from declarative YAML definitions. Dagu also comes with a web UI for visualizing workflows.

![example](https://user-images.githubusercontent.com/1475839/165764122-0bdf4bd5-55bb-40bb-b56f-329f5583c597.gif)

## Motivation : Why not Airflow or Prefect?

Popular workflow engines, Airflow and Prefect, are powerful and valuable tools, but they require writing Python code to manage the workflow. In many cases, there is already a large codebase written in other languages such as shell scripts or Perl. Adding another layer of Python code on top of the existing code is not ideal. Also, it is usually not practical to rewrite existing code in Python. For this reason, a more lightweight solution is needed; Dagu is easy to use and self-contained, making it ideal for small projects with a small number of people.
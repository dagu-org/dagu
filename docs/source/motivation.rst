.. _MOTIVATION:

Motivation
==========

Why we built Dagu
------------------

In many organizations, legacy systems still rely on hundreds of cron jobs running across multiple servers. These jobs are often written in various languages like Perl or Shell scripts, with implicit interdependencies. When one job fails, troubleshooting requires manually logging into servers via SSH and checking individual logs. To perform recovery, one must understand these implicit dependencies, which often rely on tribal knowledge. Dagu was developed to eliminate this complexity by providing a clear and understandable tool for workflow definition and dependency management.

A Lightweight and Self-Contained Solution
------------------------------------------

While Cron is lightweight and suitable for simple scheduling, it doesn't scale well for complex workflows or provide features like retries, dependencies, or observability out of the box. On the other hand, tools like Airflow or other workflow engines can be overly complex for smaller projects or legacy environments, with steep learning curves and burdensome to maintain. Dagu strikes a balance: it's easy to use, self-contained, and require no coding, making it ideal for smaller projects.

Built By and For In-House Developers
-------------------------------------

Dagu's design philosophy stems from the real-world experience in managing complex jobs across diverse environments, from small startups to enterprise companies. By focusing on simplicity, transparency, and minimal setup overhead, Dagu aims to make life easier for developers who need a robust workflow engine without the heavy lift of a more complex tool.

.. _scheduler configuration:

Scheduler
==========

To run DAGs automatically, you need to run the ``dagu scheduler`` process on your system. Also, you can use `cron expression generator <https://crontab.cronhub.io/>`_ for your scheduler calculation. 

Cron Expression
----------------

You can specify the schedule with cron expression in the ``schedule`` field in the config file as follows.

.. code-block:: yaml

    schedule: "5 4 * * *" # Run at 04:05.
    steps:
      - name: scheduled job
        command: job.sh

Or you can set multiple schedules.

.. code-block:: yaml

    schedule:
      - "30 7 * * *" # Run at 7:30
      - "0 20 * * *" # Also run at 20:00
    steps:
      - name: scheduled job
        command: job.sh

You can also specify a cron expression to run within a specific timezone. See `list of tz database timezones <https://en.wikipedia.org/wiki/List_of_tz_database_time_zones>`_

.. code-block:: yaml

    schedule: "CRON_TZ=Asia/Tokyo 5 9 * * *" # Run at 09:05 in Tokyo
    steps:
      - name: scheduled job
        command: job.sh

Stop Schedule
--------------

If you want to start and stop a long-running process on a fixed schedule, you can define ``start`` and ``stop`` times as follows. At the stop time, each step's process receives a stop signal.

.. code-block:: yaml

    schedule:
      start: "0 8 * * *" # starts at 8:00
      stop: "0 13 * * *" # stops at 13:00
    steps:
      - name: scheduled job
        command: job.sh

You can also set multiple start/stop schedules. In the following example, the process will run from 0:00-5:00 and 12:00-17:00.

.. code-block:: yaml

    schedule:
    start:
      - "0 0 * * *"
      - "12 0 * * *"
    stop:
      - "5 0 * * *"
      - "17 0 * * *"
    steps:
      - name: some long-process
        command: main.sh

Restart Schedule
----------------

If you want to restart a DAG process on a fixed schedule, the ``restart`` field is also available. At the restart time, the DAG execution will be stopped and restarted again.

.. code-block:: yaml

    schedule:
      start: "0 8 * * *"    # starts at 8:00
      restart: "0 12 * * *" # restarts at 12:00
      stop: "0 13 * * *"    # stops at 13:00
    steps:
      - name: scheduled job
        command: job.sh

The wait time after the job is stopped before restart can be configured in the DAG definition as follows. The default value is ``0`` (zero).

.. code-block:: yaml

    restartWaitSec: 60 # Wait 60s after the process is stopped, then restart the DAG.
    steps:
      - name: step1
        command: python some_app.py

Run Scheduler as a Daemon
-------------------------

The easiest way to make sure the process is always running on your system is to create the script below and execute it every minute using cron (you don't need ``root`` account in this way).

.. code-block:: bash

    #!/bin/bash
    process="dagu scheduler"
    command="/usr/bin/dagu scheduler"

    if ps ax | grep -v grep | grep "$process" > /dev/null
    then
        exit
    else
        $command &
    fi

    exit

Skip Successful Runs
-------------------

To prevent redundant executions of scheduled DAGs, you can set ``skipIfSuccessful`` flag to ``true``. When enabled, Dagu will check if the DAG has completed successfully since its last scheduled time. If it has, the current run will be skipped.

.. code-block:: yaml

    schedule: "0 */4 * * *"  # Run every 4 hours
    skipIfSuccessful: true    # Skip if already succeeded since last schedule
    steps:
      - name: resource-intensive-job
        command: process_data.sh

This is particularly useful for resource-intensive tasks where unnecessary re-runs should be avoided. Note that this only affects scheduled runs - manual triggers will always execute regardless of this setting.

For example, with the above configuration:
- If the DAG runs successfully at 04:00
- And someone triggers it at 05:00
- The run will be skipped because there's already a successful run since the last schedule
- The next run will occur at the next scheduled time (08:00)

The default value is ``false``, meaning DAGs will run on every schedule by default.


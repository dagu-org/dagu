FAQ
===

How Long Will the History Data be Stored?
------------------------------------------

By default, the execution history data is retained for 30 days. However, you can customize this setting by modifying the `histRetentionDays` field in a YAML file.

How to Use Specific Host and Port or `dagu server`?
-----------------------------------------------------

To configure the host and port for `dagu server`, you can set the environment variables `DAGU_HOST` and `DAGU_PORT`. Refer to the :ref:`Configuration Options` for more details.

How to Specify the DAGs Directory for `dagu server` and `dagu scheduler`?
--------------------------------------------------------------------------

You can customize the directory used to store DAG files by setting the environment variable `DAGU_DAGS`. See :ref:`Configuration Options` for more information.

How Can I Retry a DAG from a Specific Task?
--------------------------------------------

If you want to retry a DAG from a specific task, you can set the status of that task to `failed` by clicking the step in the Web UI. When you rerun the DAG, it will execute the failed task and any subsequent tasks.

How Does It Track Running Processes Without DBMS?
-------------------------------------------------

`dagu` uses Unix sockets to communicate with running processes.

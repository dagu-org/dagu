Quick Start Guide
=================

.. contents::
    :local:

1. Launch the Web UI
---------------------

Start the server with ``dagu server`` and browse to http://127.0.0.1:8080 to explore the Web UI.

Note: The server will be started on port ``8080`` by default. You can change the port by passing ``--port`` option. See :ref:`Host and Port Configuration` for more details.

2. Create a New DAG
-------------------

Create a DAG by clicking the ``New DAG`` button on the top page of the web UI. Input ``example`` in the dialog.

3. Edit the DAG
---------------

Go to the ``SPEC`` Tab and hit the ``Edit`` button. Copy & Paste the following YAML code into the editor.

.. code-block:: yaml

    schedule: "0 0 * * *"
    tags: Daily
    steps:
      - name: "S1"
        command: "sleep 2"
      - name: "S2"
        command: "sleep 2"
        depends:
          - "S1"
      - name: "S3"
        command: "sleep 2"
        depends:
          - "S1"
      - name: "S4"
        command: "sleep 2"
        depends:
          - "S2"
          - "S3"

4. Execute the DAG
-------------------

You can execute the example by pressing the `Start` button.

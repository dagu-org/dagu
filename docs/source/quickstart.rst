Quick Start Guide
=================

.. contents::
    :local:

1. Launch the Web UI
---------------------

Start the server with ``dagu start-all`` and browse to http://127.0.0.1:8080 to explore the Web UI.

Note: The server will be started on port ``8080`` by default. You can change the port by passing ``--port`` option. See :ref:`Host and Port Configuration` for more details.

2. Create a New DAG
-------------------

Create a DAG by clicking the ``New DAG`` button on the top page of the web UI. Input ``example`` in the dialog.

3. Edit the DAG
---------------

Go to the ``SPEC`` Tab and hit the ``Edit`` button. Copy & Paste the following YAML code into the editor.

.. code-block:: yaml

    schedule: "* * * * *" # Run the DAG every minute
    params:
      - NAME: "Dagu"
    steps:
      - name: Hello world
        command: echo Hello $NAME

      - name: Simulate unclean Command Output
        command: |
          cat <<EOF
          INFO: Starting process...
          DEBUG: Initializing variables...
          DATA: User count is 42
          INFO: Process completed successfully.
          EOF
        output: raw_output
    
      - name: Extract Relevant Data
        command: |
          echo "$raw_output" | grep '^DATA:' | sed 's/^DATA: //'
        output: cleaned_data
        depends:
          - pass 'Simulate unclean Command Output'

      - name: Done
        command: echo Done!
        depends:
          - Hello world

4. Execute the DAG
-------------------

You can execute the example by pressing the `Start` button.

5. Next Steps
--------------

Check out the `Examples provided by Dagu team <https://github.com/dagu-org/dagu/tree/main/examples>`_ to learn more



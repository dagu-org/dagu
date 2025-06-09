Quick Start Guide
=================

.. contents::
    :local:

1. Launch the Web UI
---------------------

There are two ways to launch the Dagu Web UI:

**Option A: Direct Launch**

Start the server with ``dagu start-all`` and browse to http://127.0.0.1:8080 to explore the Web UI.
After the Web UI loads, navigate to the DAGs page by either:

#. Clicking the second button from the top in the left sidebar (the one that looks like a list)
#. Directly accessing http://localhost:8080/dags

.. note::
   The server will be started on port ``8080`` by default. You can change the port by passing ``--port`` option. See `configurations options </config.html>`_ for more details.

**Option B: Using Docker Compose**

If you prefer using Docker, you can use `docker-compose.yaml <https://github.com/dagu-org/dagu/blob/main/docker-compose.yaml>`_ to launch the Web UI:

.. code-block:: bash

    # Clone the repository
    git clone https://github.com/dagu-org/dagu.git
    # Navigate to the project root
    cd dagu
    # Launch with docker compose from the project root
    docker compose up

Then browse to http://127.0.0.1:8080/dags to access the Web UI.
After the Web UI loads, navigate to the DAGs page by either:

#. Clicking the second button from the top in the left sidebar (the one that looks like a list)
#. Directly accessing http://localhost:8080/dags

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

      - name: Done
        command: echo Done!

4. Execute the DAG
-------------------

.. note::
   In the `project root <https://github.com/dagu-org/dagu>`_, there is a `docker-compose.yaml <https://github.com/dagu-org/dagu/blob/main/docker-compose.yaml>`_ which can be used to test out DAGs, and get up to speed with examples like the above.

You can execute the example by pressing the `Start` button.

5. Next Steps
--------------

Check out the `Examples provided by Dagu team <https://github.com/dagu-org/dagu/tree/main/examples>`_ to learn more



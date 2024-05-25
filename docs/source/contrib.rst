Contribution Guide
===================

We welcome contributions of any size and skill level. If you have an idea for a new feature or have found a bug, please open an issue on the GitHub repository.

Prerequisite
-------------

* `Go version 1.19 or later. <https://go.dev/doc/install>`_
* Latest version of `Node.js <https://nodejs.org/en/download/>`_.
* `yarn <https://yarnpkg.com/>`_ package manager.

Setting up your local environment
----------------------------------

#. Clone the repository to your local machine.
#. Navigate to the root directory of the cloned repository and build the frontend project by running the following command:

   .. code-block:: sh

      make build-ui

#. Run the following command to start the `Dagu` application:

   .. code-block:: sh

      go run main.go

#. Now you can change the source code and build the binary by running the following command:

   .. code-block:: sh

      make build

#. Run the following command to start the `Dagu` application:

   .. code-block:: sh

      ./bin/dagu

Running Tests
-------------

   Run the following command to run the tests:

   .. code-block:: sh

      make test

Code Structure
---------------

- ``ui``: Frontend code for the Web UI.
- ``cmd``: Contains the main application entry point.
- ``docs``: Contains the documentation for the project.
- ``examples``: Contains the example DAGs.
- ``internal``: Contains the internal code for the project.
  - ``agent``: Contains the code for running the DAGs.
  - ``config``: Contains the code for loading the configuration.
  - ``dag``: Contains the code for parsing the DAG definition.
  - ``engine``: Contains the code for managing the DAGs.
  - ``persistence``: Contains the code for interacting with the database.
  - ``executor``: Contains the code for different types of executors.
  - ``scheduler``: Contains the code for running a DAG and steps.
  - ``sock``: Contains the code for interacting with the socket.
  - ``service``: Contains the internal code for the project.
    - ``frontend``: Contains the code for the frontend Web UI.
    - ``scheduelr``: Contains the code for scheduler.

Setting up your local environment for front end development
-------------------------------------------------------------

#. Clone the repository to your local machine.
#. Navigate to the root directory of the cloned repository and build the frontend project by running the following command:

   .. code-block:: sh

      make build-ui

#. Run the following command to start the `Dagu` application:

   .. code-block:: sh

      go run main.go server

#. Navigate to ``ui`` directory and run the following command to install the dependencies:

   .. code-block:: sh

      yarn install
      yarn start

#. Open the browser and navigate to http://localhost:8081.

#. Make changes to the source code and refresh the browser to see the changes.

Branches
---------

* ``main``: The main branch where the source code always reflects a production-ready state.

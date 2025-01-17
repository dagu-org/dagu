Contribution Guide
===================

We welcome any contributions to the `Dagu` project. If you have an idea for a new feature or have found a bug, please open an issue on the GitHub repository.

Prerequisite
-------------

* `Go version 1.23 or later. <https://go.dev/doc/install>`_
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

      make run

#. Now you can change the source code and build the binary by running the following command:

   .. code-block:: sh

      make build

Running Tests
-------------

   Run the following command to run the tests:

   .. code-block:: sh

      make test

Setting up your local environment for front end development
-------------------------------------------------------------

#. Clone the repository to your local machine.
#. Navigate to the root directory of the cloned repository and build the frontend project by running the following command:

   .. code-block:: sh

      make build-ui

#. Run the following command to start the `Dagu` application:

   .. code-block:: sh

      go run ./cmd/ server

#. Navigate to ``ui`` directory and run the following command to install the dependencies:

   .. code-block:: sh

      yarn install
      yarn start

#. Open the browser and navigate to http://localhost:8081.

#. Make changes to the source code and refresh the browser to see the changes.

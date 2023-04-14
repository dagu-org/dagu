Contribution Guide
===================

We welcome contributions to Dagu! If you have an idea for a new feature or have found a bug, please open an issue on the GitHub repository. If you would like to contribute code, please follow these steps:

1. Fork the repository
2. Create a new branch for your changes
3. Make your changes and commit them to your branch
4. Push your branch to your fork and open a pull request

Building Binary From Source Code
--------------------------------

Prerequisite
~~~~~~~~~~~~

Before building the binary from the source code, make sure that you have the following software installed on your system:

1. `Go version 1.18 or later. <https://go.dev/doc/install>`_
2. Latest version of `Node.js <https://nodejs.org/en/download/>`_.
3. `yarn <https://yarnpkg.com/>`_ package manager.

Build Binary
~~~~~~~~~~~~

To build the binary from the source code, follow these steps:

1. Clone the repository to your local machine.
2. Navigate to the root directory of the cloned repository and build the frontend project by running the following command:

   .. code-block:: sh

      make build-admin

3. Build the `dagu` binary by running the following command:

   .. code-block:: sh

      make build

You can now use the `dagu` binary that is created in the `./bin` directory.
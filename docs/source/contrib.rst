Contribution Guide
===================

We welcome contributions to the `Dagu` project. If you have an idea for a new feature or find a bug, open an issue on the GitHub repository.

The `Dagu` community and maintainers are active and helpful. If you're unsure whether something is a bug, start by asking a question in the `community <https://discord.gg/gpahPUjGRk>`_.

Asking Support Questions
------------------------

We have an active `community <https://discord.gg/gpahPUjGRk>`_ where `Dagu` community members and maintainers ask and answer questions.

Reporting Issues
----------------

If you find a technical problem in `Dagu` or its documentation, use the GitHub issue tracker to report it. If you're unsure whether it's a bug, start by asking in the `community <https://discord.gg/gpahPUjGRk>`_.

Code Contribution Guidelines
----------------------------

Using conventional commit standards makes commit messages clearer. Format each commit message as follows:

.. code-block:: sh
   
   TYPE(SCOPE): MESSAGE

`SCOPE` describes the area of the changes, `MESSAGE` concisely summarizes them, and `TYPE` is a short label from the following list:

* `feat`: Introduces a new feature
* `fix`: Fixes a bug
* `docs`: Changes in documentation only
* `style`: Code changes that do not impact functionality (e.g., running `go fmt`)
* `refactor`: Code changes to improving code readability or structure
* `perf`: Code changes that improve performance
* `test`: Addition of missing tests or corrections to existing tests
* `chore`: Changes that do not modify source code or test files 
* `build`: Changes affecting the build system or external dependencies 
* `ci`: Changes to Continuous Integration configuration files and scripts
* `revert`: Reverts a previously made commit

Conventional Commit Message Examples
-------------------------------------
.. code-block:: sh
   
   feat: add user notifications on dag runs

.. code-block:: sh

   test : add unit tests for docker

.. code-block:: sh

   fix(cmd): add checks for missing parameters

.. code-block:: sh

   docs: add new dag scheme page

Please refer to `Conventional Commits <https://www.conventionalcommits.org>`_ for more information.


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

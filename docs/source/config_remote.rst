.. _Remote Node Configuration:

Remote Node
===========

.. contents::
    :local:

Introduction
-------------
Dagu UI can be configured to connect to remote nodes, allowing management of DAGs across different environments from a single interface.

How to configure
----------------
Create ``config.yaml`` in ``$HOME/.config/dagu/`` to configure remote nodes. Example configuration:

.. code-block:: yaml

    # Remote Node Configuration
    remoteNodes:
    - name: "dev"                                # name of the remote node
      apiBaseUrl: "http://localhost:8080/api/v1" # Base API URL of the remote node it must end with /api/v1

      # Authentication settings for the remote node
      # Basic authentication
      isBasicAuth: true              # Enable basic auth (optional)
      basicAuthUsername: "admin"     # Basic auth username (optional)
      basicAuthPassword: "secret"    # Basic auth password (optional)

      # api token authentication
      isAuthToken: true              # Enable API token (optional)
      authToken: "your-secret-token" # API token value (optional)

      # TLS settings
      skipTLSVerify: false           # Skip TLS verification (optional)

Using Remote Nodes
-----------------
Once configured, remote nodes can be selected from the dropdown menu in the top right corner of the UI. This allows you to:

- Switch between different environments
- View and manage DAGs on remote nodes
- Monitor execution status across nodes

The UI will maintain all functionality while operating on the selected remote node.

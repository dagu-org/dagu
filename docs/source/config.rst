.. _Configuration Options:

Configuration Options
=====================

The following environment variables can be used to configure the Dagu. Default values are provided in the parentheses:

- ``DAGU_HOST`` (``127.0.0.1``): The host to bind the server to.
- ``DAGU_PORT`` (``8080``): The port to bind the server to.
- ``DAGU_DAGS`` (``$DAGU_HOME/dags``): The directory containing the DAGs.
- ``DAGU_IS_BASIC_AUTH`` (``0``): Set to 1 to enable basic authentication.
- ``DAGU_BASIC_AUTH_USERNAME`` (``""``): The username to use for basic authentication.
- ``DAGU_BASIC_AUTH_PASSWORD`` (``""``): The password to use for basic authentication.
- ``DAGU_LOG_DIR`` (``$DAGU_HOME/logs``): The directory where logs will be stored.
- ``DAGU_DATA_DIR`` (``$DAGU_HOME/data``): The directory where application data will be stored.
- ``DAGU_SUSPEND_FLAGS_DIR`` (``$DAGU_HOME/suspend``): The directory containing DAG suspend flags.
- ``DAGU_ADMIN_LOG_DIR`` (``$DAGU_HOME/logs/admin``): The directory where admin logs will be stored.
- ``DAGU_BASE_CONFIG`` (``$DAGU_HOME/config.yaml``): The path to the base configuration file.
- ``DAGU_NAVBAR_COLOR`` (``""``): The color to use for the navigation bar. E.g., ``red`` or ``#ff0000``.
- ``DAGU_NAVBAR_TITLE`` (``Dagu``): The title to display in the navigation bar. E.g., ``Dagu - PROD`` or ``Dagu - DEV``

Note: If ``DAGU_HOME`` environment variable is not set, the default value is ``$HOME/.dagu``.

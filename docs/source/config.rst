.. _Configuration Options:

Configurations
=============

.. contents::
    :local:

Introduction
-----------
Dagu offers multiple ways to configure its behavior, from environment variables to configuration files. This document covers all available configuration options for setting up Dagu in different environments.

Configuration Methods
-------------------
There are three ways to configure Dagu:

1. Command-line arguments
2. Environment variables
3. Configuration file

Environment Variables
-------------------

Server Configuration
~~~~~~~~~~~~~~~~~~
- ``DAGU_HOST`` (``127.0.0.1``): Server binding host
- ``DAGU_PORT`` (``8080``): Server binding port
- ``DAGU_BASE_PATH`` (``""``): Base path to serve the application (e.g., ``/dagu``)
- ``DAGU_TZ`` (``""``): Server timezone (default: system timezone, e.g., ``Asia/Tokyo``)
- ``DAGU_CERT_FILE``: SSL certificate file path
- ``DAGU_KEY_FILE``: SSL key file path

Directory Paths
~~~~~~~~~~~~~
- ``DAGU_DAGS_DIR`` (``$HOME/.config/dagu/dags``): DAG definitions directory
- ``DAGU_LOG_DIR`` (``$HOME/.local/share/dagu/logs``): Log files directory
- ``DAGU_DATA_DIR`` (``$HOME/.local/share/dagu/history``): Application data directory
- ``DAGU_SUSPEND_FLAGS_DIR`` (``$HOME/.config/dagu/suspend``): DAG suspend flags directory
- ``DAGU_ADMIN_LOG_DIR`` (``$HOME/.local/share/admin``): Admin logs directory
- ``DAGU_BASE_CONFIG`` (``$HOME/.config/dagu/base.yaml``): Base configuration file path
- ``DAGU_WORK_DIR``: Default working directory for DAGs (default: DAG location)

Authentication
~~~~~~~~~~~~
- ``DAGU_IS_BASICAUTH`` (``0``): Enable basic authentication (1=enabled)
- ``DAGU_BASICAUTH_USERNAME`` (``""``): Basic auth username
- ``DAGU_BASICAUTH_PASSWORD`` (``""``): Basic auth password

UI Customization
~~~~~~~~~~~~~~
- ``DAGU_NAVBAR_COLOR`` (``""``): Navigation bar color (e.g., ``red`` or ``#ff0000``)
- ``DAGU_NAVBAR_TITLE`` (``Dagu``): Navigation bar title (e.g., ``Dagu - PROD``)

Configuration File
----------------
Create ``config.yaml`` in ``$HOME/.config/dagu/`` to override default settings. Below is a complete example with all available options:

.. code-block:: yaml

    # Server Configuration
    host: "127.0.0.1" # Web UI hostname
    port: 8080        # Web UI port
    basePath: ""      # Base path to serve the application
    tz: "Asia/Tokyo"  # Timezone (e.g., "America/New_York")
    
    # Directory Configuration
    dagsDir: "${HOME}/.config/dagu/dags"          # DAG definitions location
    workDir: "/path/to/work"                      # Default working directory
    baseConfig: "${HOME}/.config/dagu/base.yaml"  # Base DAG config
    
    # UI Configuration
    navbarColor: "#ff0000"     # Header color
    navbarTitle: "Dagu - PROD" # Header title
    latestStatusToday: true    # Show today's latest status
    
    # Authentication
    isBasicAuth: true           # Enable basic auth
    basicAuthUsername: "admin"  # Basic auth username
    basicAuthPassword: "secret" # Basic auth password
    
    # API Authentication
    isAuthToken: true              # Enable API token
    authToken: "your-secret-token" # API token value
    
    # SSL Configuration
    tls:
        certFile: "/path/to/cert.pem"
        keyFile: "/path/to/key.pem"

Server Configuration
------------------
There are multiple ways to configure the server's host and port:

1. Command-line arguments (highest precedence):
  .. code-block:: sh
      
      dagu server --host=0.0.0.0 --port=8000
 
2. Environment variables:
  .. code-block:: sh
      
      DAGU_HOST=0.0.0.0 DAGU_PORT=8000 dagu server
 
3. Configuration file (config.yaml):
  .. code-block:: yaml
      
      host: "0.0.0.0"
      port: 8000

Quick Reference
-------------
Most commonly used configurations:

1. Basic server setup:
 .. code-block:: yaml
     
   host: "127.0.0.1"
   port: 8080
   dags: "${HOME}/dags"

2. Production setup:
 .. code-block:: yaml
     
   host: "0.0.0.0"
   port: 443
   isBasicAuth: true
   basicAuthUsername: "admin"
   basicAuthPassword: "strong-password"
   tls:
       certFile: "/path/to/cert.pem"
       keyFile: "/path/to/key.pem"
   navbarColor: "#ff0000"
   navbarTitle: "Dagu - PROD"

3. Development setup:
 .. code-block:: yaml
     
   host: "127.0.0.1"
   port: 8080
   navbarColor: "#00ff00"
   navbarTitle: "Dagu - DEV"

.. _Basic Auth:

Basic Authentication
=====================

.. contents::
    :local:

To enable basic authentication for Dagu, follow these steps:

#. Set the environment variables to configure basic authentication:
  
   .. code-block:: bash
  
       export DAGU_IS_BASIC_AUTH=1
       export DAGU_BASIC_AUTH_USERNAME="<your-username>"
       export DAGU_BASIC_AUTH_PASSWORD="<your-password>"
  
   Replace ``<your-username>`` and ``<your-password>`` with your desired username and password.

#. Alternatively, create an ``admin.yaml`` file in the ``$DAGU_HOME`` directory (default: ``$HOME/.dagu/``) to override the default configuration values. Add the following lines under the ``# Basic Auth`` section:

   .. code-block:: yaml
  
       # Basic Auth
       isBasicAuth: true
       basicAuthUsername: "<your-username>"
       basicAuthPassword: "<your-password>"
  
   See :ref:`Configuration Options` for more information on the configuration file.

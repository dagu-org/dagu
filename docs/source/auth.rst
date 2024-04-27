.. _Basic Auth:

Basic Authentication
=====================

.. contents::
    :local:

To enable basic authentication for Dagu, follow these steps:

#. Set the environment variables to configure basic authentication:
  
   .. code-block:: bash
  
       export DAGU_IS_BASICAUTH=1
       export DAGU_BASICAUTH_USERNAME="<your-username>"
       export DAGU_BASICAUTH_PASSWORD="<your-password>"
  
   Replace ``<your-username>`` and ``<your-password>`` with your desired username and password.

#. Alternatively, create an ``admin.yaml`` file in the ``$DAGU_HOME`` directory (default: ``$HOME/.dagu/``) to override the default configuration values.

   .. code-block:: yaml
  
       # Basic Auth
       isBasicAuth: true
       basicAuthUsername: "<your-username>"
       basicAuthPassword: "<your-password>"

#. You can enable HTTPS by configuring the following environment variables:

   .. code-block:: bash
  
       export DAGU_CERT_FILE="<path-to-cert-file>"
       export DAGU_KEY_FILE="<path-to-key-file>"
  
   Replace ``<path-to-cert-file>`` and ``<path-to-key-file>`` with the paths to your certificate and key files.

   See :ref:`Configuration Options` for more information on the configuration file.

.. _API Token:

API Token
=====================

.. contents::
    :local:

To enable API token for Dagu, follow these steps:

#. Set the environment variables to configure basic authentication:
  
   .. code-block:: bash
  
       export DAGU_IS_AUTHTOKEN=1
       export DAGU_AUTHTOKEN="<arbitrary token string>"
  
   Replace ``<arbitrary token string>`` with a random string of your choice. This string will be used as the API token for Dagu.

#. Alternatively, create an ``admin.yaml`` file in the ``$DAGU_HOME`` directory (default: ``$HOME/.dagu/``) to override the default configuration values.

   .. code-block:: yaml
  
       # API Token
       isAuthToken: true
       authToken: "<arbitrary token string>"

#. You can enable HTTPS by configuring the following environment variables:

   .. code-block:: bash
  
       export DAGU_CERT_FILE="<path-to-cert-file>"
       export DAGU_KEY_FILE="<path-to-key-file>"
  
   Replace ``<path-to-cert-file>`` and ``<path-to-key-file>`` with the paths to your certificate and key files.

   See :ref:`Configuration Options` for more information on the configuration file.

#. Enable Basic Authentication as well if you want to use the Web UI along with the API token. **Without basic authentication config, you will not be able to access the Web UI.**

   .. code-block:: bash
  
       export DAGU_IS_BASICAUTH=1
       export DAGU_USERNAME="<username>"
       export DAGU_PASSWORD="<password>"
  
   Replace ``<username>`` and ``<password>`` with your username and password.

   See :ref:`Basic Auth` for more information on the basic authentication.

   See :ref:`REST API` for more information on the REST API.
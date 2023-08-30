Installation
============

.. contents::
    :local:

You can install Dagu quickly using Homebrew or by downloading the latest binary from the Releases page on GitHub.

Via Homebrew
------------

.. code-block:: bash

   brew install yohamta/tap/dagu

Upgrade to the latest version:

.. code-block:: bash

   brew upgrade yohamta/tap/dagu

Via Bash script
---------------

.. code-block:: bash

   curl -L https://raw.githubusercontent.com/yohamta/dagu/main/scripts/downloader.sh | bash

Via Docker
----------

.. code-block:: bash

   docker run \
   --rm \
   -p 8080:8080 \
   -v $HOME/.dagu/dags:/home/dagu/.dagu/dags \
   -v $HOME/.dagu/data:/home/dagu/.dagu/data \
   -v $HOME/.dagu/logs:/home/dagu/.dagu/logs \
   yohamta/dagu:latest

Via GitHub Release Page
-----------------------

Download the latest binary from the `Releases page <https://github.com/dagu-dev/dagu/releases>`_ and place it in your ``$PATH`` (e.g. ``/usr/local/bin``).
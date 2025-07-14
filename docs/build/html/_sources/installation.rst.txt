.. _Installation:

Installation
============

.. contents::
    :local:

You can install Dagu quickly using Homebrew or by downloading the latest binary from the Releases page on GitHub.

Via Homebrew
------------

.. code-block:: bash

   brew install dagu-org/brew/dagu

Upgrade to the latest version:

.. code-block:: bash

   brew upgrade dagu-org/brew/dagu

Via Bash script
---------------

.. code-block:: bash

   curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash

Via Docker
----------

.. code-block:: bash

   docker run \
   --rm \
   -p 8080:8080 \
   -v ~/.config/dagu:/config \
   -e DAGU_TZ=`ls -l /etc/localtime | awk -F'/zoneinfo/' '{print $2}'` \
   ghcr.io/dagu-org/dagu:latest dagu start-all

Via GitHub Release Page
-----------------------

Download the latest binary from the `Releases page <https://github.com/dagu-org/dagu/releases>`_ and place it in your ``$PATH`` (e.g. ``/usr/local/bin``).
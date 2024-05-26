.. Dagu documentation master file, created by
   sphinx-quickstart on Thu Apr 13 22:21:41 2023.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.

Dagu
======================================

.. raw:: html

   <div style="margin-bottom: 16px;">
      <div class="github-star-button">
      <iframe src="https://ghbtns.com/github-btn.html?user=dagu-dev&repo=dagu&type=star&count=true&size=large" frameborder="0" scrolling="0" width="160px" height="30px"></iframe>
      </div>
   </div>

.. image:: _static/dagu-logo.webp
   :alt: Dew overview
   :width: 800px

Dagu is a powerful Cron alternative that comes with a Web UI. It allows you to define dependencies between commands as a `Directed Acyclic Graph (DAG) <https://en.wikipedia.org/wiki/Directed_acyclic_graph>`_ in a declarative :ref:`YAML Format`. Additionally, Dagu natively supports running Docker containers, making HTTP requests, and executing commands over SSH. Dagu was designed to be easy to use, self-contained, and require no coding, making it ideal for small projects.

Quick Start
------------

:doc:`installation`
   How to install Dagu.

:doc:`quickstart`
   A quick start guide to get you up and running.

:ref:`cli`
   Command line interface reference.

:ref:`YAML Format`
   Writing DAGs.

:ref:`examples`
   Examples of DAGs.

:ref:`Configuration Options`
   Configuration options.

.. toctree::
   :caption: Installation
   :hidden:

   installation
   quickstart

.. toctree::
   :caption: Interface
   :hidden:

   cli
   web_interface
   rest
   api_token

.. toctree::
   :caption: Writing DAGs
   :hidden:

   yaml_format
   base_config
   examples

.. toctree::
   :caption: Configuration
   :hidden:

   config
   scheduler
   auth
   email

.. toctree::
   :caption: Container Setup
   :hidden:

   docker
   docker-compose

.. toctree::
   :caption: Development
   :hidden:

   faq
   contrib
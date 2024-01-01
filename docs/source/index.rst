.. Dagu documentation master file, created by
   sphinx-quickstart on Thu Apr 13 22:21:41 2023.
   You can adapt this file completely to your liking, but it should at least
   contain the root `toctree` directive.

Welcome to Dagu's documentation!
======================================

Dagu
-----

.. raw:: html

   <div>
      <div class="github-star-button">
      <iframe src="https://ghbtns.com/github-btn.html?user=dagu-dev&repo=dagu&type=star&count=true&size=large" frameborder="0" scrolling="0" width="160px" height="30px"></iframe>
      </div>
   </div>

Dagu is a powerful Cron alternative that comes with a Web UI. It allows you to define dependencies between commands as a `Directed Acyclic Graph (DAG) <https://en.wikipedia.org/wiki/Directed_acyclic_graph>`_ in a declarative :ref:`YAML Format`. Additionally, Dagu natively supports running Docker containers, making HTTP requests, and executing commands over SSH. Dagu was designed to be easy to use, self-contained, and require no coding, making it ideal for small projects.

Contents
--------

.. toctree::
   :maxdepth: 2

   installation
   quickstart
   cli
   web_interface
   yaml_format
   base_config
   examples
   config
   auth
   api_token
   executors
   email
   scheduler
   docker-compose
   rest
   docker
   faq
   contrib
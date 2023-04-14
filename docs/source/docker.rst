Building Docker Image
=====================

Download the `Dockerfile <https://github.com/yohamta/dagu/blob/main/Dockerfile>`_ to your local PC and you can build an image.

For example::

    DAGU_VERSION=<X.X.X>
    docker build -t dagu:${DAGU_VERSION} \
    --build-arg VERSION=${DAGU_VERSION} \
    --no-cache .

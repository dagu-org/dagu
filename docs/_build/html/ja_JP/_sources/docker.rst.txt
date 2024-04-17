Building Docker Image
=====================

Create the ``Dockerfile`` and you can build an image.

.. code-block:: dockerfile

    # syntax=docker/dockerfile:1.4
    FROM --platform=$BUILDPLATFORM alpine:latest

    ARG TARGETARCH
    ARG VERSION=
    ARG RELEASES_URL="https://github.com/dagu-dev/dagu/releases"

    ARG USER="dagu"
    ARG USER_UID=1000
    ARG USER_GID=$USER_UID

    EXPOSE 8080

    RUN <<EOF
        #User and permissions setup
        apk update
        apk add --no-cache sudo tzdata
        addgroup -g ${USER_GID} ${USER}
        adduser ${USER} -h /home/${USER} -u ${USER_UID} -G ${USER} -D -s /bin/ash
        echo ${USER} ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/${USER}
        chmod 0440 /etc/sudoers.d/${USER}
    EOF

    USER dagu
    WORKDIR /home/dagu
    RUN <<EOF
        #dagu binary setup
        if [ "${TARGETARCH}" == "amd64" ]; then 
            arch="x86_64";
        else 
            arch="${TARGETARCH}"
        fi
        export TARGET_FILE="dagu_${VERSION}_Linux_${arch}.tar.gz"
        wget ${RELEASES_URL}/download/v${VERSION}/${TARGET_FILE}
        tar -xf ${TARGET_FILE} && rm *.tar.gz 
        sudo mv dagu /usr/local/bin/ 
        mkdir .dagu
    EOF

    ENV DAGU_HOST=0.0.0.0
    ENV DAGU_PORT=8080

    CMD dagu server

For example::

    DAGU_VERSION=<X.X.X>
    docker build -t dagu:${DAGU_VERSION} \
    --build-arg VERSION=${DAGU_VERSION} \
    --no-cache .

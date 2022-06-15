# syntax=docker/dockerfile:1.4
FROM alpine:latest

ARG TARGETARCH
ARG VERSION=1.3.15 
ARG RELEASES_URL="https://github.com/yohamta/dagu/releases"

ARG USER="dagu"
ARG USER_UID=1000
ARG USER_GID=$USER_UID

EXPOSE 8080

RUN <<EOF
    #User and permissions setup
    apk update
    apk add sudo
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

RUN <<EOF
#Creation of a minimal admin config file for the web ui
cat <<EOF2 > ~/.dagu/admin.yaml
host: 0.0.0.0
port: 8080
EOF2
EOF

CMD dagu server
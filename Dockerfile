# syntax=docker/dockerfile:1.4
FROM --platform=$BUILDPLATFORM alpine:latest

ARG TARGETARCH
ARG VERSION=1.12.9 
ARG RELEASES_URL="https://github.com/dagu-dev/dagu/releases"
ARG TARGET_FILE="dagu_${VERSION}_linux_${TARGETARCH}.tar.gz"

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
    wget ${RELEASES_URL}/download/v${VERSION}/${TARGET_FILE}
    tar -xf ${TARGET_FILE} && rm *.tar.gz 
    sudo mv dagu /usr/local/bin/ 
    mkdir .dagu
EOF

ENV DAGU_HOST=0.0.0.0
ENV DAGU_PORT=8080

CMD dagu server

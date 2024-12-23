Building Docker Image
=====================

Example Dockerfile for building a multi-platform image:

.. code-block:: dockerfile

    # syntax=docker/dockerfile:1.4

    # Stage 1: UI Builder
    FROM --platform=$BUILDPLATFORM node:18-alpine as ui-builder

    WORKDIR /app
    COPY ui/ ./

    RUN rm -rf node_modules; \
        yarn install --frozen-lockfile --non-interactive; \
        yarn build

    # Stage 2: Go Builder
    FROM --platform=$TARGETPLATFORM golang:1.23-alpine as go-builder

    ARG LDFLAGS
    ARG TARGETOS
    ARG TARGETARCH

    WORKDIR /app
    COPY . .

    RUN go mod download && rm -rf frontend/assets
    COPY --from=ui-builder /app/dist/ ./internal/frontend/assets/

    RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="${LDFLAGS}" -o ./bin/dagu .

    # Stage 3: Final Image
    FROM --platform=$TARGETPLATFORM alpine:latest

    ARG USER="dagu"
    ARG USER_UID=1000
    ARG USER_GID=$USER_UID

    # Create user and set permissions
    RUN apk update; \
        apk add --no-cache sudo tzdata; \
        addgroup -g ${USER_GID} ${USER}; \
        adduser ${USER} -h /home/${USER} -u ${USER_UID} -G ${USER} -D -s /bin/ash; \
        echo ${USER} ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/${USER}; \
        chmod 0440 /etc/sudoers.d/${USER}; \
        mkdir -p .config/dagu/dags; \
        chown -R ${USER}:${USER} /home/${USER};

    COPY --from=go-builder /app/bin/dagu /usr/local/bin/

    USER ${USER}
    WORKDIR /home/${USER}

    # Add the hello_world.yaml file
    COPY --chown=${USER}:${USER} <<EOF .config/dagu/dags/hello_world.yaml
    schedule: "* * * * *"
    steps:
      - name: hello world
        command: sh
        script: |
          echo "Hello, world!"
    EOF

    ENV DAGU_HOST=0.0.0.0
    ENV DAGU_PORT=8080
    ENV DAGU_TZ="Etc/UTC"

    EXPOSE 8080

    CMD ["dagu", "start-all"]


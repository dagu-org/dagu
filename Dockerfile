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
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="${LDFLAGS}" -o ./bin/dagu ./cmd

# Stage 3: Final Image
FROM --platform=$TARGETPLATFORM ubuntu:24.04
ARG USER="dagu"
ARG USER_UID=1000
ARG USER_GID=$USER_UID
COPY --from=go-builder /app/bin/dagu /usr/local/bin/
COPY ./entrypoint.sh /entrypoint.sh

# Install common tools
RUN apt-get update && \
    apt-get install -y \
    git \
    curl \
    wget \
    zip \
    unzip \
    sudo \
    tzdata \
    build-essential \
    jq \
    python3 \
    python3-pip \
    openjdk-11-jdk \
    nodejs \
    npm \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* && \
    # Create user and set permissions
    groupadd --force -g ${USER_GID} ${USER} || true  && \
    useradd -m -d /config -u ${USER_UID} -g ${USER_GID} -s /bin/bash ${USER} && \
    chown -R ${USER}:${USER} /config && \
    chmod +x /entrypoint.sh

WORKDIR /config
ENV DAGU_HOST=0.0.0.0
ENV DAGU_PORT=8080
ENV DAGU_TZ="Etc/UTC"
ENV PUID=${USER_UID}
ENV PGID=${USER_GID}
ENV DOCKER_GID=-1
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
CMD ["dagu", "start-all"]
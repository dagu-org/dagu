# syntax=docker/dockerfile:1.4
# Stage 1: UI Builder
FROM --platform=$BUILDPLATFORM node:25-alpine AS ui-builder
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN rm -f /usr/local/bin/yarn /usr/local/bin/yarnpkg && \
    npm install -g corepack@latest && corepack enable

WORKDIR /app
COPY ui/ ./
RUN rm -rf node_modules; \
  pnpm install --frozen-lockfile; \
  pnpm build

# Stage 2: Go Builder
FROM --platform=$TARGETPLATFORM golang:1.26-alpine AS go-builder
ARG LDFLAGS
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY . .
RUN go mod download && rm -rf frontend/assets
COPY --from=ui-builder /app/dist/ ./internal/service/frontend/assets/
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="${LDFLAGS}" -o ./bin/boltbase ./cmd

# Stage 3: Final Image
FROM --platform=$TARGETPLATFORM ubuntu:24.04

ARG USER="boltbase"
ARG USER_UID=1000
ARG USER_GID=$USER_UID
ARG BOLTBASE_HOME="/var/lib/boltbase"

# WORKAROUND — Ubuntu 24.04 switched repo signatures to Ed25519.
# Older base images ship an outdated ubuntu-keyring that cannot verify
# these signatures, causing “invalid signature” errors on apt update.
# Temporarily disable signature checking just long enough to install the
# new keyring, then re-enable normal verification for everything else.
RUN set -eux; \
    apt-get update -o Acquire::AllowInsecureRepositories=true \
                   -o Acquire::AllowDowngradeToInsecureRepositories=true; \
    DEBIAN_FRONTEND=noninteractive \
      apt-get install -y --no-install-recommends ubuntu-keyring ca-certificates; \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Install common tools
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install -y \
    sudo \
    tzdata \
    jq \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /app/bin/boltbase /usr/local/bin/
COPY ./entrypoint.sh /entrypoint.sh

RUN set -eux; \
    # Try to create group with specified GID, fallback if GID in use
    (groupadd -g "${USER_GID}" "${USER}" || groupadd "${USER}") && \
    # Try to create user with specified UID, fallback if UID in use
    (useradd -m -d /home/boltbase \
              -u "${USER_UID}" \
              -g "$(getent group "${USER}" | cut -d: -f3)" \
              -s /bin/bash \
              "${USER}" \
    || useradd -m -d /home/boltbase \
               -g "$(getent group "${USER}" | cut -d: -f3)" \
               -s /bin/bash \
               "${USER}") && \
    chmod +x /entrypoint.sh

# Create user and set permissions
RUN set -eux; \
    { \
        echo 'boltbase ALL=(ALL) NOPASSWD:ALL'; \
        echo 'Defaults:boltbase !requiretty'; \
    } > /etc/sudoers.d/99-boltbase && \
    chmod 0440 /etc/sudoers.d/99-boltbase && \
    visudo -cf /etc/sudoers.d/99-boltbase

# Delete the default ubuntu user if it exists
RUN userdel -f ubuntu || true

# Create the BOLTBASE_HOME directory and set permissions
RUN mkdir -p "${BOLTBASE_HOME}" && \
    chown -R "${USER_UID}:${USER_GID}" "${BOLTBASE_HOME}" && \
    chmod 755 "${BOLTBASE_HOME}"

WORKDIR /home/boltbase
ENV BOLTBASE_HOST=0.0.0.0
ENV BOLTBASE_PORT=8080
ENV BOLTBASE_HOME=${BOLTBASE_HOME}
ENV BOLTBASE_TZ="Etc/UTC"
ENV PUID=${USER_UID}
ENV PGID=${USER_GID}
ENV DOCKER_GID=-1
ENV DEBIAN_FRONTEND=noninteractive
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
CMD ["boltbase", "start-all"]

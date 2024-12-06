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

COPY --from=go-builder /app/bin/dagu /usr/local/bin/
COPY ./entrypoint.sh /entrypoint.sh

# Create user and set permissions
RUN apk update && \
  apk add --no-cache su-exec shadow tzdata && \
  addgroup -g ${USER_GID} ${USER} && \
  adduser ${USER} -h /config -u ${USER_UID} -G ${USER} -D -s /bin/ash && \
  chown -R ${USER}:${USER} /config && \
  chmod +x /entrypoint.sh;

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
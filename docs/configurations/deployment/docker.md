# Docker

## Quick Start

```bash
docker run -d \
  --name dagu \
  -p 8525:8080 \
  -v dagu-data:/var/lib/dagu \
  ghcr.io/dagu-org/dagu:latest
```

## With Custom DAGs Directory

```bash
docker run -d \
  --name dagu \
  -p 8525:8080 \
  -v ./dags:/var/lib/dagu/dags \
  -v dagu-data:/var/lib/dagu \
  -e DAGU_HOST=0.0.0.0 \
  -e DAGU_PORT=8080 \
  ghcr.io/dagu-org/dagu:latest
```

## With Docker Executor Support

```bash
docker run -d \
  --name dagu \
  -p 8525:8080 \
  -v dagu-data:/var/lib/dagu \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --user 0:0 \
  ghcr.io/dagu-org/dagu:latest
```

## Environment Variables

```bash
docker run -d \
  --name dagu \
  -p 8525:8080 \
  -v dagu-data:/var/lib/dagu \
  -e DAGU_HOST=0.0.0.0 \
  -e DAGU_PORT=8080 \
  -e DAGU_TZ=America/New_York \
  -e DAGU_AUTH_BASIC_USERNAME=admin \
  -e DAGU_AUTH_BASIC_PASSWORD=password \
  ghcr.io/dagu-org/dagu:latest
```

## Container Management

```bash
# View logs
docker logs -f dagu

# Stop container
docker stop dagu

# Start container
docker start dagu

# Remove container
docker rm -f dagu
```

## Access

Open http://localhost:8080 in your browser.

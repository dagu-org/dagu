# Docker Compose

## Minimal Setup

```yaml
# compose.yaml
services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    container_name: dagu-dev
    hostname: dagu-dev

    # Expose the web UI on localhost:8080
    ports:
      - "8525:8080"

    entrypoint: [] # Override any default entrypoint for Docker in Docker (DinD) support

    # Default command: run all-in-one (server + scheduler + coordinator in-process)
    # You may override this at runtime, e.g.:
    #   docker compose -f compose.dev.yaml run --rm dagu dagu server
    command: ["dagu", "start-all"]

    # Recommended dev environment variables
    environment:
      # Frontend server
      - DAGU_HOST=0.0.0.0 # Bind inside container
      - DAGU_PORT=8080 # Web/UI port inside container
      - DAGU_DEBUG=true # More verbose logs during dev
      # Paths
      - DAGU_DAGS_DIR=/var/lib/dagu/dags
      # Timezone / base path (optional)
      # - DAGU_TZ=UTC
      # - DAGU_BASE_PATH=/dagu
      # User and Group IDs (optional, for file permissions)
      # - PUID=1000 # optional. default is 1000
      # - PGID=1000 # optional. default is 1000

    # Dev-friendly mounts (persistent data + read-only DAGs)
    volumes:
      - dagu-data:/var/lib/dagu
      - ./dags:/var/lib/dagu/dags:ro
      # For Docker in Docker (DinD) support, mount the host Docker socket:
      - /var/run/docker.sock:/var/run/docker.sock

    # Quick command presets (uncomment any one to run specific component):
    # command: ["dagu", "server"]
    # command: ["dagu", "scheduler"]
    # command: ["dagu", "coordinator"]
    # command: ["dagu", "worker"]

    # Example: enable basic auth in dev (uncomment and set values)
    # environment:
    #   - DAGU_AUTH_BASIC_USERNAME=dev
    #   - DAGU_AUTH_BASIC_PASSWORD=devpass

    user: "0:0" # Run as root for Docker in Docker (DinD) support

volumes:
  dagu-data:
    driver: local

```

Start with:
```bash
docker compose up -d
```

## Commands

```bash
# Start services
docker compose up -d

# View logs
docker compose logs -f

# Stop services
docker compose down

# Stop and remove volumes
docker compose down -v

# Scale workers
docker compose up -d --scale dagu-worker=3
```

## Access

Open http://localhost:8525 in your browser.

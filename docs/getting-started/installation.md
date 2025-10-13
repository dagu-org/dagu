# Installation

Install Dagu on your system.

## Quick Install

### Script Install

```bash
# Install to ~/.local/bin (default, no sudo required)
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash
```

This detects your OS/architecture and installs to `~/.local/bin` by default.

### Docker

```bash
docker run -d \
  --name dagu \
  -p 8080:8080 \
  -v ~/.dagu:/var/lib/dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu start-all
```

Visit http://localhost:8080

## Package Managers

### npm

```bash
npm install -g --ignore-scripts=false @dagu-org/dagu
```

This installs Dagu globally with automatic platform detection.

### Homebrew

```bash
brew update && brew install dagu
```

### Manual Download

Download from [GitHub Releases](https://github.com/dagu-org/dagu/releases).

## Installation Options

### Custom Directory & Version

```bash
# Install to custom location
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | \
  bash -s -- --install-dir ~/bin

# Install to system-wide location (requires sudo)
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | \
  bash -s -- --install-dir /usr/local/bin

# Install specific version
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | \
  bash -s -- --version v1.17.0

# Combine options
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | \
  bash -s -- --version v1.17.0 --install-dir ~/bin
```

### Docker Compose

```yaml
services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    ports:
      - "8080:8080"
    environment:
      - DAGU_TZ=America/New_York
      - DAGU_PORT=8080 # optional. default is 8080
      - DAGU_HOME=/dagu # optional.
      - PUID=1000 # optional. default is 1000
      - PGID=1000 # optional. default is 1000
    volumes:
      - dagu:/var/lib/dagu
volumes:
  dagu: {}
```

Run with `docker compose up -d`.

### Docker with Host Docker Access

For Docker executor support:

```yaml
services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    ports:
      - "8080:8080"
    volumes:
      - dagu:/var/lib/dagu
      - /var/run/docker.sock:/var/run/docker.sock
    entrypoint: [] # Override default entrypoint
    user: "0:0"    # Run as root for Docker access
volumes:
  dagu: {}
```

⚠️ **Security**: Mounting Docker socket grants full Docker access.

## Build from Source

Requirements:
- Go 1.25+
- Node.js & pnpm
- Make

```bash
git clone https://github.com/dagu-org/dagu.git
cd dagu

# Build everything
make build

# Install
sudo cp .local/bin/dagu /usr/local/bin/
```

Development:
```bash
make ui          # Build frontend
make test        # Run tests
make run         # Start with hot reload
```

## Directory Structure

Dagu uses standard locations:

```
~/.config/dagu/
├── dags/         # Workflows
├── config.yaml   # Configuration
└── base.yaml     # Shared config

~/.local/share/dagu/
├── logs/         # Execution logs
├── data/         # History
└── suspend/      # Pause flags
```

Override with environment variables:
```bash
export DAGU_HOME=/opt/dagu           # All-in-one directory
export DAGU_DAGS_DIR=/workflows      # Custom workflow location
export DAGU_LOG_DIR=/var/log/dagu    # Custom log location
export DAGU_DATA_DIR=/var/lib/dagu    # Custom data location
```

## Verify Installation

```bash
dagu version
```

## Next Steps

- [Quick Start](/getting-started/quickstart) - Create your first workflow
- [Configuration](/configurations/) - Customize Dagu
- [Web UI](/overview/web-ui) - Explore the interface

# Installation

Multiple ways to install Dagu on your system.

## Quick Install (Recommended)

The fastest way to get started with Docker:

```bash
docker run \
--rm \
-p 8080:8080 \
-v ~/.dagu:/app/.dagu \
-e DAGU_HOME=/app/.dagu \
-e DAGU_TZ=`ls -l /etc/localtime | awk -F'/zoneinfo/' '{print $2}'` \
ghcr.io/dagu-org/dagu:latest dagu start-all
```

**What each parameter does:**
- `--rm` - Automatically remove container when it exits
- `-p 8080:8080` - Expose port 8080 for web interface
- `-v ~/.dagu:/app/.dagu` - Mount local ~/.dagu directory for persistent data
- `-e DAGU_HOME=/app/.dagu` - Set Dagu home directory inside container
- `-e DAGU_TZ=...` - Set timezone for scheduler (auto-detects your system timezone)
  - Examples: `America/New_York`, `Europe/London`, `Asia/Tokyo`
  - Find your timezone: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones

Open http://localhost:8080 in your browser.

## Install Script

Automated installation script:

<div class="interactive-terminal">
<div class="terminal-command">curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash</div>
<div class="terminal-output">Downloading latest version...</div>
<div class="terminal-output">Installing to /usr/local/bin/dagu</div>
<div class="terminal-output">Installation complete! Run 'dagu version' to verify.</div>
</div>

This script:
- Detects your OS and architecture
- Downloads the appropriate binary
- Installs to `/usr/local/bin` (customizable)
- Makes it executable

### Custom Installation Path

```bash
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --install-dir ~/bin
```

**Options:**
- `--install-dir ~/bin` - Install to a custom directory (default: `/usr/local/bin`)

### Specific Version

```bash
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash -s -- --version <version>
```

**Options:**
- `--version <version>` - Install a specific version (default: latest release)

## Package Managers

### Homebrew (macOS/Linux)

```bash
# Install
brew install dagu-org/brew/dagu

# Upgrade
brew upgrade dagu-org/brew/dagu
```

## Manual Binary Download

Download pre-built binaries from [GitHub Releases](https://github.com/dagu-org/dagu/releases).

## Docker

### Quick Start

```bash
docker run --rm -p 8080:8080 ghcr.io/dagu-org/dagu:latest dagu start-all
```

### With Persistent Storage

```bash
docker run -d \
  --name dagu-server \
  -p 8080:8080 \
  -v ~/.dagu:/app/.dagu \
  -e DAGU_HOME=/app/.dagu \
  ghcr.io/dagu-org/dagu:latest \
  dagu start-all
```

### Docker Compose

Create `docker-compose.yml`:

```yaml
services:
  dagu:
    image: "ghcr.io/dagu-org/dagu:latest"
    container_name: dagu
    hostname: dagu
    ports:
      - "8080:8080"
    environment:
      - DAGU_PORT=8080 # optional. default is 8080
      - DAGU_TZ=Asia/Tokyo # optional. default is local timezone
      - DAGU_BASE_PATH=/dagu # optional. default is /
      - PUID=1000 # optional. default is 1000
      - PGID=1000 # optional. default is 1000
      - DOCKER_GID=999 # optional. default is -1 and it will be ignored
    volumes:
      - dagu_config:/config
volumes:
  dagu_config: {}
```

Run with:
```bash
docker compose up -d
```

**Environment Variables:**
- `DAGU_PORT`: Port for the web UI (default: 8080)
- `DAGU_TZ`: Timezone setting (default: local timezone)
- `DAGU_BASE_PATH`: Base path for reverse proxy setups (default: /)
- `PUID/PGID`: User/Group IDs for file permissions (default: 1000)
- `DOCKER_GID`: Docker group ID for Docker-in-Docker support (default: -1, disabled)

#### Docker-in-Docker Configuration

To enable Docker executor support (running Docker containers from within Dagu), use this configuration:

```yaml
services:
  dagu:
    image: "ghcr.io/dagu-org/dagu:latest"
    container_name: dagu
    hostname: dagu
    ports:
      - "8080:8080"
    environment:
      - DAGU_PORT=8080
      - DAGU_TZ=Asia/Tokyo
      - DAGU_BASE_PATH=/dagu
    volumes:
      - dagu_config:/config
      - /var/run/docker.sock:/var/run/docker.sock
    user: "0:0"
    entrypoint: []
volumes:
  dagu_config: {}
```

⚠️ **Security Note**: Mounting the Docker socket gives Dagu full access to the Docker daemon. Use with caution in production environments.

## Build from Source

### Prerequisites

- Go 1.23 or later
- Node.js (Latest LTS) and pnpm
- Make

### Build Steps

```bash
# Clone repository
git clone https://github.com/dagu-org/dagu.git
cd dagu

# Build UI
make ui

# Build binary
make bin

# Install
sudo cp .local/bin/dagu /usr/local/bin/
```

### Development Build

```bash
# Build UI assets (required before running server)
make ui

# Run tests with race detection
make test

# Start server and scheduler with hot reload
make run
```

**What each command does:**
- `make ui` - Builds the React frontend and copies assets to Go binary
- `make test` - Runs Go tests with gotestsum and race detection (no coverage)
- `make run` - Starts both web server and scheduler with `go run` (requires UI to be built first)

**Additional development commands:**
```bash
# Run tests with coverage
make test-coverage

# Open coverage report in browser
make open-coverage

# Run linter with auto-fixes
make golangci-lint
```

## System Requirements

### Minimum Requirements

- **OS**: Linux or macOS
- **Architecture**: AMD64 or ARM64
- **Memory**: 128 MB RAM
- **Disk**: 40 MB for binary + space for logs
- **CPU**: Any x86_64 or ARM64 processor

## Directory Structure

Dagu follows the [XDG Base Directory specification](https://specifications.freedesktop.org/basedir-spec/latest/):

```
~/.config/dagu/
├── dags/              # Workflow definitions (YAML files)
├── config.yaml        # Main configuration
└── base.yaml          # Shared base configuration

~/.local/share/dagu/
├── logs/              # Execution logs
│   ├── admin/         # Scheduler/admin logs
│   └── dags/          # Per-DAG execution logs
├── data/              # Workflow state and history
└── suspend/           # Workflow suspend flags
```

### Custom Directory Structure

Use `DAGU_HOME` to organize everything under one directory:

```bash
export DAGU_HOME=/opt/dagu

# Creates:
# /opt/dagu/
# ├── dags/
# ├── logs/
# ├── data/
# ├── suspend/
# ├── config.yaml
# └── base.yaml
```

Or configure individual paths:

```bash
export DAGU_DAGS_DIR=/opt/workflows
export DAGU_LOG_DIR=/var/log/dagu
export DAGU_DATA_DIR=/var/lib/dagu
```

## Verify Installation

After installation, verify Dagu is working:

```bash
# Check version
dagu version
```

## Running as a Service

### systemd (Linux)

Create `/etc/systemd/system/dagu.service`:

```ini
[Unit]
Description=Dagu Workflow Engine
After=network.target

[Service]
Type=simple
User=dagu
Group=dagu
WorkingDirectory=/opt/dagu
ExecStart=/usr/local/bin/dagu start-all
Restart=always
RestartSec=10

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/dagu

# Environment
Environment="DAGU_HOME=/opt/dagu"
Environment="DAGU_HOST=0.0.0.0"
Environment="DAGU_PORT=8080"

[Install]
WantedBy=multi-user.target
```

Setup and start:

```bash
# Create user and directories
sudo useradd -r -s /bin/false -d /opt/dagu dagu
sudo mkdir -p /opt/dagu/{dags,logs,data,suspend}
sudo chown -R dagu:dagu /opt/dagu

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable dagu
sudo systemctl start dagu
sudo systemctl status dagu
```

### launchd (macOS)

Create `~/Library/LaunchAgents/org.dagu.agent.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" 
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>org.dagu.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/dagu</string>
        <string>start-all</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/usr/local/var/log/dagu.log</string>
    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/dagu.error.log</string>
</dict>
</plist>
```

Load the service:
```bash
launchctl load ~/Library/LaunchAgents/org.dagu.agent.plist
```

## Upgrading

### Using Install Script

```bash
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | bash
```

### Using Homebrew

```bash
brew upgrade dagu-org/brew/dagu
```

### Manual Upgrade

```bash
# Backup current version
sudo cp /usr/local/bin/dagu /usr/local/bin/dagu.backup

# Download and install new version (use script above)
# Then restart service
sudo systemctl restart dagu
```

### Using Docker

```bash
# Pull latest image
docker pull ghcr.io/dagu-org/dagu:latest

# Restart container
docker compose down
docker compose up -d
```

## Troubleshooting

### Permission Denied

```bash
# Fix binary permissions
chmod +x /usr/local/bin/dagu

# Fix directory permissions
chown -R $USER:$USER ~/.config/dagu
chown -R $USER:$USER ~/.local/share/dagu
```

### Command Not Found

```bash
# Add to PATH
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Or create symlink
sudo ln -s /opt/dagu/bin/dagu /usr/bin/dagu
```

### Port Already in Use

```bash
# Find process
lsof -i :8080

# Change port
export DAGU_PORT=9000
dagu start-all
```

### Docker Issues

#### Volume Permissions

```bash
# Fix ownership
docker exec dagu chown -R dagu:dagu /app/.dagu
```

#### Container Exits Immediately

```bash
# Check logs
docker logs dagu

# Run interactively for debugging
docker run --rm -it ghcr.io/dagu-org/dagu:latest /bin/sh
```

## Uninstalling

### Remove Binary

```bash
sudo rm /usr/local/bin/dagu
```

### Remove Data (Optional)

```bash
# Remove configuration
rm -rf ~/.config/dagu

# Remove logs and data
rm -rf ~/.local/share/dagu
```

### Homebrew

```bash
brew uninstall dagu-org/brew/dagu
```

### Stop Service

```bash
# systemd
sudo systemctl stop dagu
sudo systemctl disable dagu
sudo rm /etc/systemd/system/dagu.service

# launchd
launchctl unload ~/Library/LaunchAgents/org.dagu.agent.plist
rm ~/Library/LaunchAgents/org.dagu.agent.plist
```

## See Also

Now that Dagu is installed:

1. [Quick Start](/getting-started/quickstart)
2. [Learn core concepts](/getting-started/concepts)
3. [Explore the Web UI](/overview/web-ui)
4. [Configure Dagu](/configurations/) for your needs

# Installation

Multiple ways to install Dagu on your system.

## Quick Install (Recommended)

The fastest way to get started:

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

## Package Managers

### Homebrew (macOS/Linux)

```bash
# Install
brew install dagu-org/brew/dagu

# Upgrade
brew upgrade dagu
```

### Go Install

If you have Go 1.21+ installed:

```bash
go install github.com/dagu-org/dagu/cmd/dagu@latest
```

This installs to `$GOPATH/bin` (usually `~/go/bin`).

## Binary Download

Download pre-built binaries from [GitHub Releases](https://github.com/dagu-org/dagu/releases).

### Linux

```bash
# AMD64
wget https://github.com/dagu-org/dagu/releases/latest/download/dagu_linux_amd64
chmod +x dagu_linux_amd64
sudo mv dagu_linux_amd64 /usr/local/bin/dagu

# ARM64
wget https://github.com/dagu-org/dagu/releases/latest/download/dagu_linux_arm64
chmod +x dagu_linux_arm64
sudo mv dagu_linux_arm64 /usr/local/bin/dagu
```

### macOS

```bash
# Intel
wget https://github.com/dagu-org/dagu/releases/latest/download/dagu_darwin_amd64
chmod +x dagu_darwin_amd64
sudo mv dagu_darwin_amd64 /usr/local/bin/dagu

# Apple Silicon
wget https://github.com/dagu-org/dagu/releases/latest/download/dagu_darwin_arm64
chmod +x dagu_darwin_arm64
sudo mv dagu_darwin_arm64 /usr/local/bin/dagu
```

### Windows

Download the Windows executable and add to your PATH:

1. Download `dagu_windows_amd64.exe`
2. Rename to `dagu.exe`
3. Move to a directory in your PATH (e.g., `C:\Program Files\dagu`)
4. Or add the directory to your PATH

## Docker

### Quick Start

```bash
docker run -p 8080:8080 ghcr.io/dagu-org/dagu:latest
```

### With Volume Mounts

```bash
docker run -d \
  --name dagu \
  -p 8080:8080 \
  -v ~/.config/dagu:/home/dagu/.config/dagu \
  -v ~/.local/share/dagu:/home/dagu/.local/share/dagu \
  -v ~/workflows:/home/dagu/workflows \
  ghcr.io/dagu-org/dagu:latest
```

### With Timezone Configuration

```bash
docker run \
  --rm \
  -p 8080:8080 \
  -v ~/.config/dagu:/config \
  -e DAGU_TZ=`ls -l /etc/localtime | awk -F'/zoneinfo/' '{print $2}'` \
  ghcr.io/dagu-org/dagu:latest dagu start-all
```

### Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  dagu:
    image: ghcr.io/dagu-org/dagu:latest
    container_name: dagu
    ports:
      - "8080:8080"
    volumes:
      - ./dags:/home/dagu/.config/dagu/dags
      - ./logs:/home/dagu/.local/share/dagu/logs
      - ./data:/home/dagu/.local/share/dagu/data
    environment:
      - DAGU_HOST=0.0.0.0
      - DAGU_PORT=8080
    restart: unless-stopped
```

Run with:
```bash
docker-compose up -d
```

## Build from Source

### Prerequisites

- Go 1.21 or later
- Node.js 18+ and pnpm (for UI)
- Make

### Build Steps

```bash
# Clone repository
git clone https://github.com/dagu-org/dagu.git
cd dagu

# Build UI
make ui

# Build binary
make build

# Install
sudo make install
```

The binary will be installed to `/usr/local/bin/dagu`.

### Development Build

```bash
# Build and run with hot reload
make run

# Run tests
make test

# Run linter
make lint
```

## Verify Installation

After installation, verify Dagu is working:

```bash
# Check version
dagu version

# Expected output:
# Dagu version: 1.14.0
# Go version: go1.21.5
# Git commit: abc123def
# Built: 2024-01-01T00:00:00Z
```

## System Requirements

### Minimum Requirements

- **OS**: Linux, macOS, or Windows
- **Architecture**: AMD64 or ARM64
- **Memory**: 512MB RAM
- **Disk**: 100MB free space
- **Permissions**: Read/write access to config and data directories

### Recommended

- **Memory**: 2GB+ RAM
- **Disk**: 1GB+ for logs and data
- **CPU**: 2+ cores for parallel execution

## Directory Structure

Dagu creates the following directories:

```
~/
├── .config/dagu/           # Configuration
│   ├── dags/              # DAG files
│   ├── config.yaml        # Main config
│   └── base.yaml          # Base config
└── .local/share/dagu/      # Data and logs
    ├── logs/              # Execution logs
    ├── data/              # State data
    ├── history/           # Historical data
    └── suspend/           # Suspend flags
```

### Custom Locations

Set custom directories via environment variables:

```bash
export DAGU_DAGS=/opt/workflows
export DAGU_LOG_DIR=/var/log/dagu
export DAGU_DATA_DIR=/var/lib/dagu
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
ExecStart=/usr/local/bin/dagu start-all --host 0.0.0.0 --port 8080
Restart=always
RestartSec=10

# Security
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable dagu
sudo systemctl start dagu
sudo systemctl status dagu
```

### launchd (macOS)

Create `~/Library/LaunchAgents/org.dagu.scheduler.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" 
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>org.dagu.scheduler</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/dagu</string>
        <string>start-all</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

Load the service:
```bash
launchctl load ~/Library/LaunchAgents/org.dagu.scheduler.plist
```

## Upgrading

### Using Install Script

```bash
curl -L https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/install.sh | bash
```

### Using Homebrew

```bash
brew upgrade dagu
```

### Manual Upgrade

1. Stop running services
2. Download new binary
3. Replace old binary
4. Restart services

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
brew uninstall dagu
```

## Troubleshooting

### Permission Denied

If you get "permission denied" errors:

```bash
# Make binary executable
chmod +x /usr/local/bin/dagu

# Check file permissions
ls -la /usr/local/bin/dagu
```

### Command Not Found

Add Dagu to your PATH:

```bash
# Add to ~/.bashrc or ~/.zshrc
export PATH=$PATH:/usr/local/bin

# Reload shell
source ~/.bashrc
```

### Port Already in Use

If port 8080 is taken:

```bash
# Use different port
dagu start-all --port 9000

# Or find what's using port 8080
lsof -i :8080
```

## Next Steps

Now that Dagu is installed:

1. [Create your first workflow](/getting-started/first-workflow)
2. [Explore the Web UI](/features/interfaces/web-ui)
3. [Configure Dagu](/configurations/) for your needs